package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/nelsong6/fzt/core"
)

// Config is an alias for core.Config so existing callers keep compiling.
type Config = core.Config

func renderFrame(c Canvas, s *core.State, cfg Config) {
	w, h := c.Size()

	usableH := h
	if cfg.Height > 0 && cfg.Height < 100 {
		usableH = h * cfg.Height / 100
		if usableH < 3 {
			usableH = 3
		}
	}

	startY := 0
	if cfg.Height > 0 && cfg.Height < 100 {
		startY = h - usableH
	}

	if cfg.Layout == "reverse" {
		drawReverse(c, s, cfg, w, startY, usableH)
	} else {
		drawDefault(c, s, cfg, w, startY, usableH)
	}
}

// Run launches the interactive TUI. Returns the selected item's output string, or "" if cancelled.
func Run(items []core.Item, cfg Config) (string, error) {
	if cfg.Height > 0 && cfg.Height < 100 {
		return RunInline(items, cfg)
	}

	screen, err := tcell.NewScreen()
	if err != nil {
		return "", fmt.Errorf("creating screen: %w", err)
	}
	if err := screen.Init(); err != nil {
		return "", fmt.Errorf("initializing screen: %w", err)
	}
	defer screen.Fini()

	screen.SetStyle(tcell.StyleDefault.Background(tcell.ColorDefault).Foreground(tcell.ColorDefault))
	screen.EnablePaste()

	if cfg.TreeMode {
		return runWithSession(screen, items, cfg)
	}

	s, searchCols := core.NewState(items, cfg)
	canvas := &tcellCanvas{screen: screen}

	for {
		screen.Clear()
		renderFrame(canvas, s, cfg)
		screen.Show()

		ev := screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			action := handleKeyEvent(s, ev.Key(), ev.Rune(), cfg, searchCols)
			switch {
			case action == "cancel":
				return "", nil
			case len(action) > 7 && action[:7] == "select:":
				return action[7:], nil
			}
		case *tcell.EventResize:
			screen.Sync()
		}
	}
}

// runWithSession renders directly to a tcell screen, supporting tree mode + search switching.
func runWithSession(screen tcell.Screen, items []core.Item, cfg Config) (string, error) {
	s, searchCols := core.NewState(items, cfg)
	ctx := s.TopCtx()
	ctx.TreeExpanded = make(map[int]bool)
	ctx.QueryExpanded = make(map[int]bool)
	ctx.TreeCursor = -1

	canvas := &tcellCanvas{screen: screen}

	for {
		screen.Clear()
		w, h := screen.Size()
		drawUnified(canvas, s, cfg, w, 0, h)
		screen.Sync() // full redraw — avoids stale content from layout changes

		ev := screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			action := handleUnifiedKey(s, ev.Key(), ev.Rune(), cfg, searchCols)
			switch {
			case action == "cancel" || action == "abort":
				return "", nil
			case action == "update":
				screen.Fini()
				RunUpdate()
				os.Exit(0)
			case len(action) > 7 && action[:7] == "select:":
				return action[7:], nil
			}
		case *tcell.EventResize:
			screen.Sync()
		}
	}
}

// handleUnifiedKey handles all key events in unified tree+search mode.
// The tree is the single navigation surface. Typing filters and auto-expands
// the tree to reveal matches. Up/Down always move the tree cursor.
func handleUnifiedKey(s *core.State, key tcell.Key, ch rune, cfg Config, searchCols []int) string {
	ctx := s.TopCtx()

	// ':' enters command mode — push a command context
	if key == tcell.KeyRune && ch == ':' {
		cmdCtx := newCommandContext(s)
		s.PushContext(cmdCtx)
		return ""
	}

	// Shift+HJKL → vim-style navigation (capitals bypass search input)
	if key == tcell.KeyRune {
		var navKey tcell.Key
		switch ch {
		case 'H':
			navKey = tcell.KeyLeft
		case 'J':
			navKey = tcell.KeyDown
		case 'K':
			navKey = tcell.KeyUp
		case 'L':
			navKey = tcell.KeyRight
		}
		if navKey != 0 {
			action, _ := handleTreeKey(s, navKey, 0, cfg, searchCols)
			return action
		}
	}

	// Nav mode + Ctrl+U: clean slate — exit nav, clear query, deselect
	if ctx.NavMode && key == tcell.KeyCtrlU {
		ctx.NavMode = false
		ctx.Query = nil
		ctx.Cursor = 0
		ctx.TreeCursor = -1
		ctx.QueryExpanded = make(map[int]bool)
		if len(ctx.Scope) <= 1 {
			ctx.SearchActive = false
			ctx.Filtered = nil
		} else {
			core.FilterItems(s, cfg, searchCols)
		}
		return ""
	}

	// Nav mode + Backspace: edit the displayed item name (remove last char)
	if ctx.NavMode && (key == tcell.KeyBackspace || key == tcell.KeyBackspace2) {
		visible := core.TreeVisibleItems(s)
		if ctx.TreeCursor >= 0 && ctx.TreeCursor < len(visible) && len(visible[ctx.TreeCursor].Item.Fields) > 0 {
			name := []rune(visible[ctx.TreeCursor].Item.Fields[0])
			if len(name) > 1 {
				ctx.Query = name[:len(name)-1]
				ctx.Cursor = len(ctx.Query)
			} else {
				ctx.Query = nil
				ctx.Cursor = 0
			}
		}
		ctx.NavMode = false
		if len(ctx.Query) > 0 {
			ctx.SearchActive = true
			core.FilterItems(s, cfg, searchCols)
			core.UpdateQueryExpansion(s)
			core.SyncTreeCursorToTopMatch(s)
		} else {
			ctx.SearchActive = false
			ctx.Filtered = nil
			ctx.TreeCursor = -1
			ctx.QueryExpanded = make(map[int]bool)
		}
		return ""
	}

	// When no search active, delegate to tree navigation (except printable chars)
	if !ctx.SearchActive {
		if key == tcell.KeyRune {
			if ch == '/' {
				// Activate search without inserting the /
				ctx.SearchActive = true
				ctx.NavMode = false
				return ""
			}
			// Space on a folder → push scope (same as Enter)
			if ch == ' ' {
				visible := core.TreeVisibleItems(s)
				if ctx.TreeCursor >= 0 && ctx.TreeCursor < len(visible) {
					row := visible[ctx.TreeCursor]
					if row.Item.HasChildren {
						core.PushScope(s, row.ItemIdx, cfg, searchCols)
						return ""
					}
				}
			}
			// Printable character → activate search
			ctx.SearchActive = true
			ctx.NavMode = false
			ctx.Query = []rune{ch}
			ctx.Cursor = 1
			core.FilterItems(s, cfg, searchCols)
			core.UpdateQueryExpansion(s)
			core.SyncTreeCursorToTopMatch(s)
			return ""
		}
		action, _ := handleTreeKey(s, key, ch, cfg, searchCols)
		return action
	}

	// Search active — unified handling
	return handleSearchKey(s, key, ch, cfg, searchCols)
}

// handleKeyEvent processes a single key event against the TUI state (flat mode).
// Returns "" for normal continuation, "cancel" to quit, or "select:<output>" for leaf selection.
func handleKeyEvent(s *core.State, key tcell.Key, ch rune, cfg Config, searchCols []int) string {
	ctx := s.TopCtx()
	switch key {
	case tcell.KeyCtrlC:
		s.Cancelled = true
		return "cancel"

	case tcell.KeyEscape:
		if len(ctx.Query) > 0 {
			ctx.Query = nil
			ctx.Cursor = 0
			ctx.Offset = 0
			core.FilterItems(s, cfg, searchCols)
			if len(ctx.Filtered) > 0 {
				ctx.Index = 0
			} else {
				ctx.Index = -1
			}
			return ""
		}
		if cfg.Tiered && len(ctx.Scope) > 1 {
			ctx.Scope = ctx.Scope[:len(ctx.Scope)-1]
			prev := ctx.Scope[len(ctx.Scope)-1]
			if prev.ParentIdx < 0 {
				ctx.Items = core.RootItemsOf(ctx.AllItems)
			} else {
				ctx.Items = core.ChildrenOf(ctx.AllItems, prev.ParentIdx)
			}
			ctx.Query = prev.Query
			ctx.Cursor = prev.Cursor
			ctx.Index = prev.Index
			ctx.Offset = prev.Offset
			core.FilterItems(s, cfg, searchCols)
			return ""
		}
		s.Cancelled = true
		return "cancel"

	case tcell.KeyEnter:
		if ctx.Index >= 0 && ctx.Index < len(ctx.Filtered) {
			selected := ctx.Filtered[ctx.Index]
			if selected.HasChildren && cfg.Tiered {
				parentIdx := core.FindInAll(ctx.AllItems, selected)
				if parentIdx >= 0 {
					ctx.Scope[len(ctx.Scope)-1].Query = ctx.Query
					ctx.Scope[len(ctx.Scope)-1].Cursor = ctx.Cursor
					ctx.Scope[len(ctx.Scope)-1].Index = ctx.Index
					ctx.Scope[len(ctx.Scope)-1].Offset = ctx.Offset
					ctx.Scope = append(ctx.Scope, core.ScopeLevel{ParentIdx: parentIdx})
					ctx.Items = core.ChildrenOf(ctx.AllItems, parentIdx)
					ctx.Query = nil
					ctx.Cursor = 0
					ctx.Index = -1
					ctx.Offset = 0
					core.FilterItems(s, cfg, searchCols)
				}
			} else {
				return "select:" + formatOutput(selected, cfg)
			}
		}

	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if ctx.Cursor > 0 {
			ctx.Query = append(ctx.Query[:ctx.Cursor-1], ctx.Query[ctx.Cursor:]...)
			ctx.Cursor--
			ctx.Offset = 0
			core.FilterItems(s, cfg, searchCols)
			if len(ctx.Filtered) > 0 {
				ctx.Index = 0
			} else {
				ctx.Index = -1
			}
		}

	case tcell.KeyDelete:
		if ctx.Cursor < len(ctx.Query) {
			ctx.Query = append(ctx.Query[:ctx.Cursor], ctx.Query[ctx.Cursor+1:]...)
			core.FilterItems(s, cfg, searchCols)
		}

	case tcell.KeyLeft:
		if cfg.Tiered && len(ctx.Query) == 0 && len(ctx.Scope) > 1 {
			ctx.Scope = ctx.Scope[:len(ctx.Scope)-1]
			prev := ctx.Scope[len(ctx.Scope)-1]
			if prev.ParentIdx < 0 {
				ctx.Items = core.RootItemsOf(ctx.AllItems)
			} else {
				ctx.Items = core.ChildrenOf(ctx.AllItems, prev.ParentIdx)
			}
			ctx.Query = prev.Query
			ctx.Cursor = prev.Cursor
			ctx.Index = prev.Index
			ctx.Offset = prev.Offset
			core.FilterItems(s, cfg, searchCols)
		} else if ctx.Index >= 0 {
			ctx.Index = -1
		} else if ctx.Cursor > 0 {
			ctx.Cursor--
		}

	case tcell.KeyRight:
		if ctx.Index >= 0 && cfg.Tiered && len(ctx.Query) == 0 && len(ctx.Filtered) > 0 && ctx.Filtered[ctx.Index].HasChildren {
			selected := ctx.Filtered[ctx.Index]
			parentIdx := core.FindInAll(ctx.AllItems, selected)
			if parentIdx >= 0 {
				ctx.Scope[len(ctx.Scope)-1].Query = ctx.Query
				ctx.Scope[len(ctx.Scope)-1].Cursor = ctx.Cursor
				ctx.Scope[len(ctx.Scope)-1].Index = ctx.Index
				ctx.Scope[len(ctx.Scope)-1].Offset = ctx.Offset
				ctx.Scope = append(ctx.Scope, core.ScopeLevel{ParentIdx: parentIdx})
				ctx.Items = core.ChildrenOf(ctx.AllItems, parentIdx)
				ctx.Query = nil
				ctx.Cursor = 0
				ctx.Index = -1
				ctx.Offset = 0
				core.FilterItems(s, cfg, searchCols)
			}
		} else if ctx.Index == -1 && ctx.Cursor < len(ctx.Query) {
			ctx.Cursor++
		}

	case tcell.KeyTab:
		if len(ctx.Filtered) > 0 {
			if ctx.Index < len(ctx.Filtered)-1 {
				ctx.Index++
			} else {
				ctx.Index = -1
			}
		}

	case tcell.KeyBacktab:
		if len(ctx.Filtered) > 0 {
			if ctx.Index == -1 {
				ctx.Index = len(ctx.Filtered) - 1
			} else if ctx.Index > 0 {
				ctx.Index--
			} else {
				ctx.Index = -1
			}
		}

	case tcell.KeyUp, tcell.KeyCtrlP:
		if ctx.Index > 0 {
			ctx.Index--
		} else if ctx.Index == 0 {
			ctx.Index = -1
		}

	case tcell.KeyDown, tcell.KeyCtrlN:
		if ctx.Index < len(ctx.Filtered)-1 {
			ctx.Index++
		}

	case tcell.KeyCtrlA:
		ctx.Cursor = 0

	case tcell.KeyCtrlE:
		ctx.Cursor = len(ctx.Query)

	case tcell.KeyCtrlU:
		ctx.Query = ctx.Query[ctx.Cursor:]
		ctx.Cursor = 0
		ctx.Offset = 0
		core.FilterItems(s, cfg, searchCols)
		if len(ctx.Filtered) > 0 {
			ctx.Index = 0
		} else {
			ctx.Index = -1
		}

	case tcell.KeyCtrlW:
		if ctx.Cursor > 0 {
			end := ctx.Cursor
			for ctx.Cursor > 0 && ctx.Query[ctx.Cursor-1] == ' ' {
				ctx.Cursor--
			}
			for ctx.Cursor > 0 && ctx.Query[ctx.Cursor-1] != ' ' {
				ctx.Cursor--
			}
			ctx.Query = append(ctx.Query[:ctx.Cursor], ctx.Query[end:]...)
			ctx.Offset = 0
			core.FilterItems(s, cfg, searchCols)
			if len(ctx.Filtered) > 0 {
				ctx.Index = 0
			} else {
				ctx.Index = -1
			}
		}

	case tcell.KeyRune:
		ctx.Query = append(ctx.Query[:ctx.Cursor], append([]rune{ch}, ctx.Query[ctx.Cursor:]...)...)
		ctx.Cursor++
		ctx.Offset = 0
		core.FilterItems(s, cfg, searchCols)
		if len(ctx.Filtered) > 0 {
			ctx.Index = 0
		} else {
			ctx.Index = -1
		}
	}

	return ""
}

// Simulate runs a headless simulation: renders the initial frame, then one frame
// per character of the query. Returns all frames as text snapshots.
// simKey represents a parsed key event from the sim-query string.
type simKey struct {
	key   tcell.Key
	ch    rune
	label string
}

// parseSimQuery parses a sim-query string into key events.
// Supports {up}, {down}, {left}, {right}, {enter}, {tab}, {esc}, {bs}, {space},
// {ctrl+u}, {ctrl+w}. Plain characters are literal key presses.
func parseSimQuery(query string) []simKey {
	var keys []simKey
	runes := []rune(query)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '{' {
			end := -1
			for j := i + 1; j < len(runes); j++ {
				if runes[j] == '}' {
					end = j
					break
				}
			}
			if end > i {
				name := strings.ToLower(string(runes[i+1 : end]))
				var sk simKey
				switch name {
				case "up":
					sk = simKey{key: tcell.KeyUp, label: "Up"}
				case "down":
					sk = simKey{key: tcell.KeyDown, label: "Down"}
				case "left":
					sk = simKey{key: tcell.KeyLeft, label: "Left"}
				case "right":
					sk = simKey{key: tcell.KeyRight, label: "Right"}
				case "enter":
					sk = simKey{key: tcell.KeyEnter, label: "Enter"}
				case "tab":
					sk = simKey{key: tcell.KeyTab, label: "Tab"}
				case "esc":
					sk = simKey{key: tcell.KeyEscape, label: "Esc"}
				case "bs":
					sk = simKey{key: tcell.KeyBackspace2, label: "Backspace"}
				case "space":
					sk = simKey{key: tcell.KeyRune, ch: ' ', label: "Space"}
				case "ctrl+u":
					sk = simKey{key: tcell.KeyCtrlU, label: "Ctrl+U"}
				case "ctrl+w":
					sk = simKey{key: tcell.KeyCtrlW, label: "Ctrl+W"}
				default:
					// Unknown — skip
					i = end
					continue
				}
				keys = append(keys, sk)
				i = end
				continue
			}
		}
		keys = append(keys, simKey{key: tcell.KeyRune, ch: runes[i], label: fmt.Sprintf("'%c'", runes[i])})
	}
	return keys
}

func Simulate(items []core.Item, cfg Config, query string, w, h int, styled bool) []Frame {
	s, searchCols := core.NewState(items, cfg)

	if cfg.TreeMode {
		ctx := s.TopCtx()
		ctx.TreeExpanded = make(map[int]bool)
		ctx.QueryExpanded = make(map[int]bool)
		ctx.TreeCursor = -1
	}

	var frames []Frame

	renderOne := func() string {
		mem := NewMemScreen(w, h)
		if cfg.TreeMode {
			drawUnified(mem, s, cfg, w, 0, h)
		} else {
			renderFrame(mem, s, cfg)
		}
		if styled {
			return mem.StyledSnapshot()
		}
		return mem.Snapshot()
	}

	// Frame 0: initial state
	frames = append(frames, Frame{Label: "(initial)", Content: renderOne()})

	// One frame per key event
	keys := parseSimQuery(query)
	for _, sk := range keys {
		if cfg.TreeMode {
			handleUnifiedKey(s, sk.key, sk.ch, cfg, searchCols)
		} else {
			handleKeyEvent(s, sk.key, sk.ch, cfg, searchCols)
		}

		label := fmt.Sprintf("key: %s  query: \"%s\"", sk.label, string(s.TopCtx().Query))
		frames = append(frames, Frame{Label: label, Content: renderOne()})
	}

	return frames
}

// Frame represents one rendered screen state.
type Frame struct {
	Label   string // description of what triggered this frame
	Content string // text grid snapshot
}

// FormatFrames renders all frames as a single string for file output.
func FormatFrames(frames []Frame) string {
	var b strings.Builder
	for i, f := range frames {
		fmt.Fprintf(&b, "=== Frame %d [%s] ===\n", i, f.Label)
		b.WriteString(f.Content)
		b.WriteString("\n\n")
	}
	return b.String()
}

func drawItemRow(c Canvas, item core.Item, isSelected bool, isSearching bool, cfg Config, ctx *core.TreeContext, borderOffset, y, w int) {
	maxW := w - borderOffset*2

	// Selection highlight
	selStyle := tcell.StyleDefault
	if isSelected {
		selStyle = selStyle.Background(tcell.ColorDarkBlue)
	}

	// Fill entire row with background if selected
	if isSelected {
		for fx := borderOffset; fx < w-borderOffset; fx++ {
			c.SetContent(fx, y, ' ', nil, selStyle)
		}
	}

	x := borderOffset

	// Indicator: ▸ for selected, space otherwise
	if isSelected {
		drawText(c, x, y, "▸ ", selStyle.Foreground(tcell.ColorYellow).Bold(true), 2)
	} else {
		drawText(c, x, y, "  ", tcell.StyleDefault, 2)
	}
	x += 2

	// Name field
	if len(item.Fields) > 0 {
		nameStyle := tcell.StyleDefault
		if item.HasChildren {
			nameStyle = nameStyle.Foreground(tcell.ColorDarkCyan).Bold(true)
		}
		if isSelected {
			nameStyle = nameStyle.Background(tcell.ColorDarkBlue)
			if !item.HasChildren {
				nameStyle = nameStyle.Foreground(tcell.ColorWhite)
			}
		}

		var indices []int
		if item.MatchIndices != nil && len(item.MatchIndices) > 0 {
			indices = item.MatchIndices[0]
		}
		var sr []core.StyledRune
		if item.StyledFields != nil && len(item.StyledFields) > 0 {
			sr = item.StyledFields[0]
		}

		name := item.Fields[0]
		// Draw name text with highlighting
		startX := x
		x = drawFieldText(c, x, y, name, sr, indices, nameStyle, isSelected, maxW)
		// Pad name to fixed column width + gap
		padStyle := nameStyle
		targetX := startX + ctx.NameColWidth + ctx.ColGap
		for x < targetX && x < maxW+borderOffset {
			c.SetContent(x, y, ' ', nil, padStyle)
			x++
		}
	}

	// Icon columns: file (selectable) + folder (drillable)
	// Nerd font icons may render as double-width, so allocate 2 cells each
	if cfg.Tiered {
		bgStyle := tcell.StyleDefault
		if isSelected {
			bgStyle = bgStyle.Background(tcell.ColorDarkBlue)
		}

		// Single icon: folder for containers, file for leaves
		if item.HasChildren {
			c.SetContent(x, y, '\U000F024B', nil, bgStyle.Foreground(tcell.ColorYellow).Bold(true))
		} else {
			c.SetContent(x, y, '\uF15B', nil, bgStyle.Foreground(tcell.ColorDarkGray))
		}
		x++
		c.SetContent(x, y, ' ', nil, bgStyle) // width buffer
		x++
	}

	// Description field (dimmer)
	if len(item.Fields) > 1 {
		descStyle := tcell.StyleDefault
		if isSelected {
			descStyle = descStyle.Background(tcell.ColorDarkBlue)
		}

		var indices []int
		if item.MatchIndices != nil && len(item.MatchIndices) > 1 {
			indices = item.MatchIndices[1]
		}
		var sr []core.StyledRune
		if item.StyledFields != nil && len(item.StyledFields) > 1 {
			sr = item.StyledFields[1]
		}

		x = drawFieldText(c, x, y, item.Fields[1], sr, indices, descStyle, isSelected, maxW)
	}

	// Breadcrumb path when searching nested results
	if isSearching && cfg.Tiered && item.Depth > 0 && item.Path != "" {
		pathStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkGray).Italic(true)
		if isSelected {
			pathStyle = pathStyle.Background(tcell.ColorDarkBlue)
		}
		// Find the parent part of the path (everything before the last ›)
		parentPath := ""
		if lastSep := strings.LastIndex(item.Path, " › "); lastSep >= 0 {
			parentPath = item.Path[:lastSep]
		}
		if parentPath != "" {
			pathStr := "  (" + parentPath + ")"
			drawText(c, x, y, pathStr, pathStyle, maxW-x+borderOffset)
		}
	}

}

func drawReverse(c Canvas, s *core.State, cfg Config, w, startY, h int) {
	ctx := s.TopCtx()
	y := startY

	borderOffset := 0
	if cfg.Border {
		versionStr := ""
		if s.ShowVersion {
			versionStr = Version
		}
		drawBorderTopWithTitle(c, w, y, cfg.Title, cfg.TitlePos, versionStr, cfg.Label)
		y++
		borderOffset = 1
	}

	promptStr := cfg.Prompt
	if promptStr == "" {
		promptStr = "> "
	}
	promptLen := len([]rune(promptStr))

	if len(ctx.Query) > 0 {
		// Typing: show query with cursor
		promptStyle := tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true)
		drawText(c, borderOffset, y, promptStr, promptStyle, w-borderOffset*2)
		drawText(c, promptLen+borderOffset, y, string(ctx.Query), tcell.StyleDefault, w-promptLen-borderOffset*2)
		c.ShowCursor(promptLen+ctx.Cursor+borderOffset, y)
	} else if ctx.Index >= 0 && ctx.Index < len(ctx.Filtered) {
		// No query, item selected — show item name as preview, dim prompt
		dimPrompt := tcell.StyleDefault.Foreground(tcell.ColorDarkGray)
		drawText(c, borderOffset, y, promptStr, dimPrompt, w-borderOffset*2)
		previewText := ctx.Filtered[ctx.Index].Fields[0]
		drawText(c, promptLen+borderOffset, y, previewText, tcell.StyleDefault.Foreground(tcell.ColorDarkGray).Italic(true), w-promptLen-borderOffset*2)
		c.HideCursor()
	} else {
		promptStyle := tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true)
		drawText(c, borderOffset, y, promptStr, promptStyle, w-borderOffset*2)
		c.ShowCursor(promptLen+borderOffset, y)
	}
	y++

	// Breadcrumb trail
	scopePath := core.BuildScopePath(s)
	if scopePath != "" {
		bcStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkCyan)
		sepStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkGray)
		bx := borderOffset + 1
		drawText(c, bx, y, "◂ ", sepStyle, w-borderOffset*2)
		bx += 2
		drawText(c, bx, y, scopePath, bcStyle, w-borderOffset*2-bx)
	}
	y++

	for _, hdr := range ctx.Headers {
		hdrStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkCyan).Bold(true)
		hx := borderOffset + 2
		// Name header
		if len(hdr.Fields) > 0 {
			drawText(c, hx, y, hdr.Fields[0], hdrStyle, w-borderOffset*2-2)
			hx += ctx.NameColWidth + ctx.ColGap
		}
		// Skip icon column width if tiered (icon + buffer = 2)
		if cfg.Tiered {
			hx += 2
		}
		// Description header
		if len(hdr.Fields) > 1 {
			drawText(c, hx, y, hdr.Fields[1], hdrStyle, w-borderOffset*2-hx)
		}
		y++
	}

	// Divider line between header and items
	if len(ctx.Headers) > 0 {
		divStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkGray)
		for dx := borderOffset + 1; dx < w-borderOffset-1; dx++ {
			c.SetContent(dx, y, '─', nil, divStyle)
		}
		y++
	}

	itemLines := startY + h - y
	if cfg.Border {
		itemLines--
	}
	if itemLines < 0 {
		itemLines = 0
	}

	if ctx.Index >= 0 {
		if ctx.Index < ctx.Offset {
			ctx.Offset = ctx.Index
		}
		if ctx.Index >= ctx.Offset+itemLines {
			ctx.Offset = ctx.Index - itemLines + 1
		}
	} else {
		ctx.Offset = 0
	}

	isSearching := len(ctx.Query) > 0

	for i := 0; i < itemLines && i+ctx.Offset < len(ctx.Filtered); i++ {
		idx := i + ctx.Offset
		item := ctx.Filtered[idx]
		isSelected := idx == ctx.Index
		drawItemRow(c, item, isSelected, isSearching, cfg, ctx, borderOffset, y+i, w)
	}

	if cfg.Border {
		drawBorderSides(c, w, startY, startY+h-1)
		drawBorderBottom(c, w, startY+h-1)
	}
}

func drawDefault(c Canvas, s *core.State, cfg Config, w, startY, h int) {
	ctx := s.TopCtx()
	y := startY

	borderOffset := 0
	if cfg.Border {
		versionStr := ""
		if s.ShowVersion {
			versionStr = Version
		}
		drawBorderTopWithTitle(c, w, y, cfg.Title, cfg.TitlePos, versionStr, cfg.Label)
		y++
		borderOffset = 1
	}

	for _, hdr := range ctx.Headers {
		hdrStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkCyan).Bold(true)
		hx := borderOffset + 2
		// Name header
		if len(hdr.Fields) > 0 {
			drawText(c, hx, y, hdr.Fields[0], hdrStyle, w-borderOffset*2-2)
			hx += ctx.NameColWidth + ctx.ColGap
		}
		// Skip icon column width if tiered (icon + buffer = 2)
		if cfg.Tiered {
			hx += 2
		}
		// Description header
		if len(hdr.Fields) > 1 {
			drawText(c, hx, y, hdr.Fields[1], hdrStyle, w-borderOffset*2-hx)
		}
		y++
	}

	// Divider line between header and items
	if len(ctx.Headers) > 0 {
		divStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkGray)
		for dx := borderOffset + 1; dx < w-borderOffset-1; dx++ {
			c.SetContent(dx, y, '─', nil, divStyle)
		}
		y++
	}

	promptLines := 2
	itemLines := startY + h - y - promptLines
	if cfg.Border {
		itemLines--
	}
	if itemLines < 0 {
		itemLines = 0
	}

	if ctx.Index >= 0 {
		if ctx.Index < ctx.Offset {
			ctx.Offset = ctx.Index
		}
		if ctx.Index >= ctx.Offset+itemLines {
			ctx.Offset = ctx.Index - itemLines + 1
		}
	} else {
		ctx.Offset = 0
	}

	isSearching := len(ctx.Query) > 0

	for i := 0; i < itemLines && i+ctx.Offset < len(ctx.Filtered); i++ {
		idx := i + ctx.Offset
		item := ctx.Filtered[idx]
		isSelected := idx == ctx.Index
		drawItemRow(c, item, isSelected, isSearching, cfg, ctx, borderOffset, y+i, w)
	}

	bottomY := startY + h - promptLines
	if cfg.Border {
		bottomY--
	}

	scopePath := core.BuildScopePath(s)
	if scopePath != "" {
		bcStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkCyan)
		sepStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkGray)
		bx := borderOffset + 1
		drawText(c, bx, bottomY, "◂ ", sepStyle, w-borderOffset*2)
		bx += 2
		drawText(c, bx, bottomY, scopePath, bcStyle, w-borderOffset*2-bx)
	}

	promptStr := cfg.Prompt
	if promptStr == "" {
		promptStr = "> "
	}
	promptLen := len([]rune(promptStr))

	if len(ctx.Query) > 0 {
		promptStyle := tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true)
		drawText(c, borderOffset, bottomY+1, promptStr, promptStyle, w-borderOffset*2)
		drawText(c, promptLen+borderOffset, bottomY+1, string(ctx.Query), tcell.StyleDefault, w-promptLen-borderOffset*2)
		c.ShowCursor(promptLen+ctx.Cursor+borderOffset, bottomY+1)
	} else if ctx.Index >= 0 && ctx.Index < len(ctx.Filtered) {
		dimPrompt := tcell.StyleDefault.Foreground(tcell.ColorDarkGray)
		drawText(c, borderOffset, bottomY+1, promptStr, dimPrompt, w-borderOffset*2)
		previewText := ctx.Filtered[ctx.Index].Fields[0]
		drawText(c, promptLen+borderOffset, bottomY+1, previewText, tcell.StyleDefault.Foreground(tcell.ColorDarkGray).Italic(true), w-promptLen-borderOffset*2)
		c.HideCursor()
	} else {
		promptStyle := tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true)
		drawText(c, borderOffset, bottomY+1, promptStr, promptStyle, w-borderOffset*2)
		c.ShowCursor(promptLen+borderOffset, bottomY+1)
	}

	if cfg.Border {
		drawBorderSides(c, w, startY, startY+h-1)
		drawBorderBottom(c, w, startY+h-1)
	}
}

// drawFieldText draws text with optional ANSI styles and match highlighting. No column padding.
func drawFieldText(c Canvas, x, y int, field string, styledRunes []core.StyledRune, indices []int, baseStyle tcell.Style, isSelected bool, maxW int) int {
	runes := []rune(field)
	indexSet := make(map[int]bool)
	for _, idx := range indices {
		indexSet[idx] = true
	}

	hlStyle := baseStyle.Foreground(tcell.ColorGreen).Bold(true)
	if isSelected {
		hlStyle = hlStyle.Background(tcell.ColorDarkBlue)
	}

	for i, r := range runes {
		if x >= maxW {
			break
		}
		style := baseStyle
		if styledRunes != nil && i < len(styledRunes) {
			style = styledRunes[i].Style
			if isSelected {
				fg, _, attrs := style.Decompose()
				style = tcell.StyleDefault.Background(tcell.ColorDarkBlue).Foreground(fg).Attributes(attrs)
			}
		}
		if indexSet[i] {
			style = hlStyle
		}
		c.SetContent(x, y, r, nil, style)
		x++
	}
	return x
}

func drawHighlightedField(c Canvas, x, y int, field string, styledRunes []core.StyledRune, indices []int, baseStyle tcell.Style, isSelected bool, widths []int, fieldIdx, gap, maxW int) int {
	runes := []rune(field)
	indexSet := make(map[int]bool)
	for _, idx := range indices {
		indexSet[idx] = true
	}

	for i, r := range runes {
		if x >= maxW {
			break
		}

		style := baseStyle

		// Layer 1: Apply ANSI color if available
		if styledRunes != nil && i < len(styledRunes) {
			style = styledRunes[i].Style
			// If this row is selected, override the background but keep the foreground color
			if isSelected {
				fg, _, attrs := style.Decompose()
				style = tcell.StyleDefault.Background(tcell.ColorDarkBlue).Foreground(fg).Attributes(attrs)
			}
		}

		// Layer 2: Override with match highlight
		if indexSet[i] {
			if isSelected {
				style = style.Foreground(tcell.ColorGreen).Bold(true).Background(tcell.ColorDarkBlue)
			} else {
				style = style.Foreground(tcell.ColorGreen).Bold(true)
			}
		}

		c.SetContent(x, y, r, nil, style)
		x++
	}

	if fieldIdx < len(widths)-1 {
		padTo := widths[fieldIdx]
		for len(runes) < padTo {
			if x >= maxW {
				break
			}
			c.SetContent(x, y, ' ', nil, baseStyle)
			x++
			runes = append(runes, ' ')
		}
		for g := 0; g < gap; g++ {
			if x >= maxW {
				break
			}
			c.SetContent(x, y, ' ', nil, baseStyle)
			x++
		}
	}

	return x
}

func drawText(c Canvas, x, y int, text string, style tcell.Style, maxW int) {
	for i, r := range text {
		if i >= maxW {
			break
		}
		c.SetContent(x+i, y, r, nil, style)
	}
}

func drawBorderTop(c Canvas, w, y int) {
	drawBorderTopWithTitle(c, w, y, "", "", "")
}

func drawBorderTopWithTitle(c Canvas, w, y int, title, pos string, version string, label ...string) {
	borderStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkGray)
	c.SetContent(0, y, '┌', nil, borderStyle)
	for x := 1; x < w-1; x++ {
		c.SetContent(x, y, '─', nil, borderStyle)
	}
	c.SetContent(w-1, y, '┐', nil, borderStyle)

	if title != "" {
		titleRunes := []rune(title)
		maxTitle := w - 6 // leave room for corners + at least one ─ + spaces on each side
		if maxTitle < 1 {
			return
		}
		if len(titleRunes) > maxTitle {
			titleRunes = titleRunes[:maxTitle]
		}
		var startX int
		switch pos {
		case "center":
			startX = (w - len(titleRunes) - 2) / 2
		case "right":
			startX = w - len(titleRunes) - 3 // 1 corner + 1 ─ minimum on right, plus space pad
		default: // "left"
			startX = 2
		}
		if startX < 2 {
			startX = 2
		}
		titleStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkCyan).Bold(true)
		c.SetContent(startX, y, ' ', nil, borderStyle)
		for i, r := range titleRunes {
			c.SetContent(startX+1+i, y, r, nil, titleStyle)
		}
		c.SetContent(startX+1+len(titleRunes), y, ' ', nil, borderStyle)
	}

	// Version pinned to top-right of border (only when enabled)
	if version != "" && version != "UNSET" {
		vRunes := []rune(version)
		vStart := w - len(vRunes) - 3 // 1 corner + 1 ─ + space pad
		if vStart > 2 {
			vStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkGray)
			c.SetContent(vStart, y, ' ', nil, borderStyle)
			for i, r := range vRunes {
				c.SetContent(vStart+1+i, y, r, nil, vStyle)
			}
			c.SetContent(vStart+1+len(vRunes), y, ' ', nil, borderStyle)
		}
	}

	// Label pinned to top-left of border
	if len(label) > 0 && label[0] != "" {
		lRunes := []rune(label[0])
		lStart := 2 // 1 corner + 1 ─
		maxLen := w - 6
		if len(lRunes) > maxLen {
			lRunes = lRunes[:maxLen]
		}
		lStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkGray)
		c.SetContent(lStart, y, ' ', nil, borderStyle)
		for i, r := range lRunes {
			c.SetContent(lStart+1+i, y, r, nil, lStyle)
		}
		c.SetContent(lStart+1+len(lRunes), y, ' ', nil, borderStyle)
	}
}

func drawBorderBottom(c Canvas, w, y int) {
	style := tcell.StyleDefault.Foreground(tcell.ColorDarkGray)
	c.SetContent(0, y, '└', nil, style)
	for x := 1; x < w-1; x++ {
		c.SetContent(x, y, '─', nil, style)
	}
	c.SetContent(w-1, y, '┘', nil, style)
}

func drawBorderSides(c Canvas, w, topY, bottomY int) {
	style := tcell.StyleDefault.Foreground(tcell.ColorDarkGray)
	for y := topY + 1; y < bottomY; y++ {
		c.SetContent(0, y, '│', nil, style)
		c.SetContent(w-1, y, '│', nil, style)
	}
}

func formatOutput(item core.Item, cfg Config) string {
	if len(cfg.AcceptNth) > 0 {
		// Use clean fields for output (ANSI stripped) so downstream consumers get plain text
		var parts []string
		for _, col := range cfg.AcceptNth {
			idx := col - 1
			if idx >= 0 && idx < len(item.Fields) {
				parts = append(parts, item.Fields[idx])
			}
		}
		return strings.Join(parts, "\t")
	}
	// No accept-nth: return the original line (preserves ANSI for piping)
	if item.Original != "" {
		return item.Original
	}
	return strings.Join(item.Fields, "\t")
}

// RunUpdate downloads the latest fzt release from GitHub if a newer version exists.
func RunUpdate() {
	current := Version
	fmt.Fprintf(os.Stderr, "Current: %s\n", current)

	// Get latest release tag
	cmd := exec.Command("gh", "release", "view", "--repo", "nelsong6/fzt", "--json", "tagName", "--jq", ".tagName")
	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to check latest release: %v\n", err)
		return
	}
	latest := strings.TrimSpace(string(out))
	fmt.Fprintf(os.Stderr, "Latest:  %s\n", latest)

	if current == latest {
		fmt.Fprintf(os.Stderr, "Already up to date.\n")
		return
	}

	goos := runtime.GOOS
	goarch := runtime.GOARCH
	asset := fmt.Sprintf("fzt-%s-%s", goos, goarch)
	if goos == "windows" {
		asset += ".exe"
	}

	self, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot determine executable path: %v\n", err)
		return
	}
	dest := filepath.Dir(self)

	fmt.Fprintf(os.Stderr, "Downloading %s...\n", asset)
	dl := exec.Command("gh", "release", "download", "--repo", "nelsong6/fzt", "--pattern", asset, "--dir", dest, "--clobber")
	dl.Stdout = os.Stderr
	dl.Stderr = os.Stderr
	if err := dl.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
		return
	}

	// Rename to just 'fzt' (or 'fzt.exe').
	// On Windows the running exe is locked, but renaming it is allowed.
	// Move the old binary out of the way first, then rename the new one in.
	final := filepath.Join(dest, "fzt")
	if goos == "windows" {
		final += ".exe"
	}
	downloaded := filepath.Join(dest, asset)
	if downloaded != final {
		old := final + ".old"
		os.Remove(old)
		os.Rename(final, old)
		if err := os.Rename(downloaded, final); err != nil {
			fmt.Fprintf(os.Stderr, "Rename failed: %v\n", err)
			os.Rename(old, final) // restore
			return
		}
		os.Remove(old)
	}

	fmt.Fprintf(os.Stderr, "Updated: %s → %s\n", current, latest)
}

// RunFilter runs in non-interactive mode (like fzf --filter).
func RunFilter(items []core.Item, query string, cfg Config) {
	searchCols := cfg.SearchCols
	if len(searchCols) == 0 {
		searchCols = cfg.Nth
	}

	var matched []core.Item
	for _, item := range items {
		ancestors := core.GetAncestorNames(items, item)
		ts, indices := core.ScoreItem(item.Fields, query, searchCols, ancestors)
		if indices != nil {
			if cfg.Tiered {
				ts.Name -= item.Depth * cfg.DepthPenalty
			}
			m := item
			m.Score = ts
			m.MatchIndices = indices
			matched = append(matched, m)
		}
	}

	sort.SliceStable(matched, func(i, j int) bool {
		return matched[j].Score.Less(matched[i].Score)
	})

	for _, item := range matched {
		if cfg.ShowScores {
			fmt.Fprintf(os.Stdout, "[score=N:%d D:%d A:%d] %s\n", item.Score.Name, item.Score.Desc, item.Score.Ancestor, formatOutput(item, cfg))
		} else {
			fmt.Fprintln(os.Stdout, formatOutput(item, cfg))
		}
	}
}
