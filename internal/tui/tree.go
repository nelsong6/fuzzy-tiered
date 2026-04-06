package tui

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/nelsong6/fzt/core"
)

// handleTreeKey processes a key event when no query is active (tree navigation).
func handleTreeKey(s *core.State, key tcell.Key, ch rune, cfg Config, searchCols []int) (action string, switchToSearch bool) {
	ctx := s.TopCtx()
	visible := core.TreeVisibleItems(s)
	visLen := len(visible)

	switch key {
	case tcell.KeyCtrlC:
		s.Cancelled = true
		return "cancel", false

	case tcell.KeyUp, tcell.KeyCtrlP:
		ctx.NavMode = true
		if visLen > 0 {
			if ctx.TreeCursor <= 0 {
				ctx.TreeCursor = visLen - 1
			} else {
				ctx.TreeCursor--
			}
		}
		return "", false

	case tcell.KeyDown, tcell.KeyCtrlN, tcell.KeyTab:
		ctx.NavMode = true
		if visLen > 0 {
			if ctx.TreeCursor < 0 {
				ctx.TreeCursor = 0
			} else {
				ctx.TreeCursor++
				if ctx.TreeCursor >= visLen {
					ctx.TreeCursor = 0
				}
			}
		}
		return "", false

	case tcell.KeyBacktab:
		ctx.NavMode = true
		if visLen > 0 {
			ctx.TreeCursor--
			if ctx.TreeCursor < 0 {
				ctx.TreeCursor = visLen - 1
			}
		}
		return "", false

	case tcell.KeyEnter:
		if ctx.TreeCursor >= 0 && ctx.TreeCursor < visLen {
			row := visible[ctx.TreeCursor]
			if row.Item.HasChildren {
				core.PushScope(s, row.ItemIdx, cfg, searchCols)
				return "", false
			}
			if ctx.OnLeafSelect != nil {
				return ctx.OnLeafSelect(row.Item), false
			}
			return "select:" + formatOutput(row.Item, cfg), false
		}
		return "", false

	case tcell.KeyRight:
		ctx.NavMode = true
		if ctx.TreeCursor >= 0 && ctx.TreeCursor < visLen {
			row := visible[ctx.TreeCursor]
			if row.Item.HasChildren {
				core.PushScope(s, row.ItemIdx, cfg, searchCols)
			}
		}
		return "", false

	case tcell.KeyLeft:
		ctx.NavMode = true
		if ctx.TreeCursor >= 0 && ctx.TreeCursor < visLen {
			row := visible[ctx.TreeCursor]
			if row.Item.HasChildren && ctx.TreeExpanded[row.ItemIdx] {
				// Collapse expanded folder
				ctx.TreeExpanded[row.ItemIdx] = false
			} else if row.Item.ParentIdx >= 0 {
				// Move cursor to parent
				for vi, vr := range visible {
					if vr.ItemIdx == row.Item.ParentIdx {
						ctx.TreeCursor = vi
						break
					}
				}
			}
		}
		return "", false

	case tcell.KeyBackspace, tcell.KeyBackspace2:
		// Pop scope first, then context
		if len(ctx.Scope) > 1 {
			core.PopScope(s, cfg, searchCols)
			return "", false
		}
		if len(s.Contexts) > 1 {
			s.PopContext()
			return "", false
		}
		return "", false

	case tcell.KeyEscape:
		// Pop scope first, then context
		if len(ctx.Scope) > 1 {
			core.PopScope(s, cfg, searchCols)
			return "", false
		}
		if len(s.Contexts) > 1 {
			s.PopContext()
			return "", false
		}
		// Root context with nothing to clear — exit
		s.Cancelled = true
		return "cancel", false

	case tcell.KeyRune:
		return "", true
	}

	return "", false
}

// handleSearchKey handles all keys when search is active.
// The tree is always the navigation surface — Up/Down move the tree cursor,
// typing edits the query and auto-positions the cursor on the top match.
func handleSearchKey(s *core.State, key tcell.Key, ch rune, cfg Config, searchCols []int) string {
	ctx := s.TopCtx()
	switch key {
	case tcell.KeyCtrlC:
		s.Cancelled = true
		return "cancel"

	case tcell.KeyEscape:
		if len(ctx.Query) > 0 {
			// Clear query, collapse auto-expansions
			ctx.Query = nil
			ctx.Cursor = 0
			ctx.QueryExpanded = make(map[int]bool)
			if len(ctx.Scope) <= 1 {
				ctx.SearchActive = false
				ctx.Filtered = nil
				ctx.TreeCursor = -1
			} else {
				core.FilterItems(s, cfg, searchCols)
			}
			return ""
		}
		if len(ctx.Scope) > 1 {
			core.PopScope(s, cfg, searchCols)
			return ""
		}
		// At root with empty query — pop context if stacked, else exit
		if len(s.Contexts) > 1 {
			s.PopContext()
			return ""
		}
		s.Cancelled = true
		return "cancel"

	case tcell.KeyUp, tcell.KeyCtrlP:
		ctx.NavMode = true
		visible := core.TreeVisibleItems(s)
		if len(visible) > 0 {
			if ctx.TreeCursor <= 0 {
				ctx.TreeCursor = len(visible) - 1
			} else {
				ctx.TreeCursor--
			}
		}
		return ""

	case tcell.KeyDown, tcell.KeyCtrlN:
		ctx.NavMode = true
		visible := core.TreeVisibleItems(s)
		if len(visible) > 0 {
			if ctx.TreeCursor < 0 {
				ctx.TreeCursor = 0
			} else {
				ctx.TreeCursor++
				if ctx.TreeCursor >= len(visible) {
					ctx.TreeCursor = 0
				}
			}
		}
		return ""

	case tcell.KeyTab:
		// Autocomplete: set query to the top match's name.
		// If the match is a folder, push scope (same as typing name + Space).
		if len(ctx.Filtered) > 0 && len(ctx.Filtered[0].Fields) > 0 {
			topMatch := ctx.Filtered[0]
			name := topMatch.Fields[0]
			if !strings.EqualFold(string(ctx.Query), name) {
				// First Tab: autocomplete the name
				ctx.Query = []rune(name)
				ctx.Cursor = len(ctx.Query)
				core.FilterItems(s, cfg, searchCols)
				core.UpdateQueryExpansion(s)
				core.SyncTreeCursorToTopMatch(s)
			}
			// If folder, push scope (same behavior as Space)
			if topMatch.HasChildren {
				idx := core.FindInAll(ctx.AllItems, topMatch)
				if idx >= 0 {
					core.PushScope(s, idx, cfg, searchCols)
				}
			}
		}
		return ""

	case tcell.KeyEnter:
		// Act on tree cursor item
		visible := core.TreeVisibleItems(s)
		if ctx.TreeCursor >= 0 && ctx.TreeCursor < len(visible) {
			row := visible[ctx.TreeCursor]
			if row.Item.HasChildren {
				core.PushScope(s, row.ItemIdx, cfg, searchCols)
				return ""
			}
			if ctx.OnLeafSelect != nil {
				return ctx.OnLeafSelect(row.Item)
			}
			return "select:" + formatOutput(row.Item, cfg)
		}
		// No cursor — act on top match
		if len(ctx.Filtered) > 0 {
			selected := ctx.Filtered[0]
			if selected.HasChildren {
				idx := core.FindInAll(ctx.AllItems, selected)
				if idx >= 0 {
					core.PushScope(s, idx, cfg, searchCols)
				}
				return ""
			}
			if ctx.OnLeafSelect != nil {
				return ctx.OnLeafSelect(selected)
			}
			return "select:" + formatOutput(selected, cfg)
		}
		return ""

	case tcell.KeyBackspace, tcell.KeyBackspace2:
		ctx.NavMode = false
		if len(ctx.Query) == 0 && len(ctx.Scope) > 1 {
			core.PopScope(s, cfg, searchCols)
			return ""
		}
		if len(ctx.Query) == 0 && len(s.Contexts) > 1 {
			s.PopContext()
			return ""
		}
		if len(ctx.Query) > 0 {
			ctx.Query = ctx.Query[:len(ctx.Query)-1]
			ctx.Cursor = len(ctx.Query)
			if len(ctx.Query) == 0 {
				ctx.QueryExpanded = make(map[int]bool)
				ctx.TreeCursor = -1
				if len(ctx.Scope) <= 1 {
					ctx.SearchActive = false
					ctx.Filtered = nil
				} else {
					core.FilterItems(s, cfg, searchCols)
				}
			} else {
				core.FilterItems(s, cfg, searchCols)
				core.UpdateQueryExpansion(s)
				core.SyncTreeCursorToTopMatch(s)
			}
		}
		return ""

	case tcell.KeyLeft:
		// Tree navigation: collapse or move to parent
		visible := core.TreeVisibleItems(s)
		if ctx.TreeCursor >= 0 && ctx.TreeCursor < len(visible) {
			row := visible[ctx.TreeCursor]
			if row.Item.HasChildren && ctx.TreeExpanded[row.ItemIdx] {
				ctx.NavMode = true
				ctx.TreeExpanded[row.ItemIdx] = false
			} else if row.Item.ParentIdx >= 0 {
				ctx.NavMode = true
				for vi, vr := range visible {
					if vr.ItemIdx == row.Item.ParentIdx {
						ctx.TreeCursor = vi
						break
					}
				}
			} else if ctx.NavMode {
				// Already leftmost — exit nav mode, return to search
				ctx.NavMode = false
			}
		}
		return ""

	case tcell.KeyRight:
		ctx.NavMode = true
		// Tree navigation: expand or move to first child
		visible := core.TreeVisibleItems(s)
		if ctx.TreeCursor >= 0 && ctx.TreeCursor < len(visible) {
			row := visible[ctx.TreeCursor]
			if row.Item.HasChildren {
				if !ctx.TreeExpanded[row.ItemIdx] {
					ctx.TreeExpanded[row.ItemIdx] = true
				} else if ctx.TreeCursor+1 < len(visible) {
					ctx.TreeCursor++
				}
			}
		}
		return ""

	case tcell.KeyCtrlU:
		ctx.NavMode = false
		ctx.Query = nil
		ctx.Cursor = 0
		ctx.QueryExpanded = make(map[int]bool)
		if len(ctx.Scope) <= 1 {
			ctx.SearchActive = false
			ctx.Filtered = nil
		} else {
			core.FilterItems(s, cfg, searchCols)
		}
		return ""

	case tcell.KeyCtrlW:
		ctx.NavMode = false
		if len(ctx.Query) > 0 {
			// Delete last word from end
			i := len(ctx.Query) - 1
			for i > 0 && ctx.Query[i-1] == ' ' {
				i--
			}
			for i > 0 && ctx.Query[i-1] != ' ' {
				i--
			}
			ctx.Query = ctx.Query[:i]
			ctx.Cursor = len(ctx.Query)
			if len(ctx.Query) == 0 {
				ctx.QueryExpanded = make(map[int]bool)
				ctx.TreeCursor = -1
				if len(ctx.Scope) <= 1 {
					ctx.SearchActive = false
					ctx.Filtered = nil
				} else {
					core.FilterItems(s, cfg, searchCols)
				}
			} else {
				core.FilterItems(s, cfg, searchCols)
				core.UpdateQueryExpansion(s)
				core.SyncTreeCursorToTopMatch(s)
			}
		}
		return ""

	case tcell.KeyRune:
		ctx.NavMode = false

		// Space on a folder → enter it
		if ch == ' ' {
			visible := core.TreeVisibleItems(s)
			if ctx.TreeCursor >= 0 && ctx.TreeCursor < len(visible) {
				row := visible[ctx.TreeCursor]
				if row.Item.HasChildren {
					core.PushScope(s, row.ItemIdx, cfg, searchCols)
					return ""
				}
			}
			// Not on a folder — insert space in query
		}

		// Append character
		ctx.Query = append(ctx.Query, ch)
		ctx.Cursor = len(ctx.Query)
		core.FilterItems(s, cfg, searchCols)
		core.UpdateQueryExpansion(s)
		core.SyncTreeCursorToTopMatch(s)
		return ""
	}

	return ""
}

// ── Unified renderer ──────────────────────────────────────────

// drawUnified renders the prompt bar and tree. The tree is the single
// navigation surface — no separate results section.
// Within this function, ctx.NavMode affects ONLY the prompt icon.
// All other rendering is mode-independent.
func drawUnified(c Canvas, s *core.State, cfg Config, w, startY, h int) {
	ctx := s.TopCtx()

	borderOffset := 0
	y := startY

	if cfg.Border {
		versionStr := ""
		if s.ShowVersion {
			versionStr = Version
		}
		drawBorderTopWithTitle(c, w, y, cfg.Title, cfg.TitlePos, versionStr, cfg.Label)
		y++
		borderOffset = 1
	}

	hasQuery := len(ctx.Query) > 0
	visible := core.TreeVisibleItems(s)

	// Prompt bar — bordered input field, the primary UI element
	promptBg := tcell.ColorValid + 236 // 256-color: #303030, subtle surface
	borderStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkGray)

	// Mode indicator: search (magnifying glass) vs nav (arrow) — always shown
	var promptIcon rune
	var promptIconStyle tcell.Style
	if ctx.NavMode {
		promptIcon = '\uF0A9'  //
		promptIconStyle = tcell.StyleDefault.Foreground(tcell.ColorDarkCyan).Background(promptBg)
	} else {
		promptIcon = '\uF002'  //
		promptIconStyle = tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true).Background(promptBg)
	}
	promptLen := 2 // icon + space

	// Top border of prompt bar
	c.SetContent(borderOffset, y, '\u250c', nil, borderStyle)     // ┌
	for x := borderOffset + 1; x < w-borderOffset-1; x++ {
		c.SetContent(x, y, '\u2500', nil, borderStyle)             // ─
	}
	c.SetContent(w-borderOffset-1, y, '\u2510', nil, borderStyle) // ┐
	y++

	// Prompt content line with background
	c.SetContent(borderOffset, y, '\u2502', nil, borderStyle) // │
	for x := borderOffset + 1; x < w-borderOffset-1; x++ {
		c.SetContent(x, y, ' ', nil, tcell.StyleDefault.Background(promptBg))
	}
	c.SetContent(w-borderOffset-1, y, '\u2502', nil, borderStyle) // │

	px := borderOffset + 1 // content starts inside the border
	pw := w - borderOffset*2 - 2 // content width inside borders
	// Prompt: [icon] [locked breadcrumb ›] [query or nav preview]
	c.SetContent(px, y, promptIcon, nil, promptIconStyle)
	c.SetContent(px+1, y, ' ', nil, tcell.StyleDefault.Background(promptBg))
	tx := px + promptLen // text position after icon + space

	// Context breadcrumb — ':' when in a pushed context (command mode)
	scopeLen := 0
	if len(s.Contexts) > 1 && ctx.PromptIcon != 0 {
		lockedStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkGray).Background(promptBg)
		c.SetContent(tx+scopeLen, y, ctx.PromptIcon, nil, lockedStyle)
		scopeLen++
		c.SetContent(tx+scopeLen, y, ' ', nil, tcell.StyleDefault.Background(promptBg))
		scopeLen++
	}

	// Scope breadcrumb — just the word greyed out with a space after it.
	if len(ctx.Scope) > 1 {
		lockedStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkGray).Background(promptBg)
		for si := 1; si < len(ctx.Scope); si++ {
			level := ctx.Scope[si]
			if level.ParentIdx >= 0 && level.ParentIdx < len(ctx.AllItems) {
				name := ctx.AllItems[level.ParentIdx].Fields[0]
				drawText(c, tx+scopeLen, y, name, lockedStyle, pw-promptLen-scopeLen)
				scopeLen += len([]rune(name))
				c.SetContent(tx+scopeLen, y, ' ', nil, lockedStyle)
				scopeLen++
			}
		}
	}

	qx := tx + scopeLen // where editable query starts

	contentX := qx // where query or nav preview starts
	contentW := pw - promptLen - scopeLen

	if hasQuery {
		queryStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(promptBg)
		drawText(c, contentX, y, string(ctx.Query), queryStyle, contentW)
		c.ShowCursor(contentX+ctx.Cursor, y)

		// Ghost autocomplete text: show remaining chars of top match if query is a prefix
		if ctx.Cursor == len(ctx.Query) && len(ctx.Filtered) > 0 && len(ctx.Filtered[0].Fields) > 0 {
			name := ctx.Filtered[0].Fields[0]
			nameRunes := []rune(name)
			if len(nameRunes) > len(ctx.Query) && strings.EqualFold(string(nameRunes[:len(ctx.Query)]), string(ctx.Query)) {
				ghost := string(nameRunes[len(ctx.Query):])
				ghostStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkGray).Background(promptBg)
				drawText(c, contentX+len(ctx.Query), y, ghost, ghostStyle, contentW-len(ctx.Query))
			}
		}
	} else if ctx.SearchActive || len(ctx.Scope) > 1 {
		hintStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkGray).Italic(true).Background(promptBg)
		drawText(c, qx, y, "search\u2026", hintStyle, pw-promptLen-scopeLen)
		c.ShowCursor(qx, y)
	} else {
		hintStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkGray).Italic(true).Background(promptBg)
		drawText(c, qx, y, "type to search\u2026", hintStyle, pw-promptLen-scopeLen)
		c.ShowCursor(qx, y)
	}
	y++

	// Bottom border of prompt bar
	c.SetContent(borderOffset, y, '\u2514', nil, borderStyle)     // └
	for x := borderOffset + 1; x < w-borderOffset-1; x++ {
		c.SetContent(x, y, '\u2500', nil, borderStyle)             // ─
	}
	c.SetContent(w-borderOffset-1, y, '\u2518', nil, borderStyle) // ┘
	y++

	// Headers
	if len(ctx.Headers) > 0 {
		hdrStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkCyan).Bold(true)
		x := borderOffset + 2
		for fi, hdr := range ctx.Headers[0].Fields {
			colW := ctx.NameColWidth + ctx.ColGap
			if fi > 0 {
				if cfg.Tiered {
					x += 2
				}
				colW = 0
			}
			drawText(c, x, y, hdr, hdrStyle, w-x-borderOffset)
			x += colW
		}
		y++

		divStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkGray)
		for x := borderOffset + 1; x < w-borderOffset; x++ {
			c.SetContent(x, y, '\u2500', nil, divStyle)
		}
		y++
	}

	// Tree section — the single navigation surface
	totalSpace := h - (y - startY) - borderOffset
	treeSpace := totalSpace

	// When query active, find top match in tree for highlighting
	topMatchIdx := -1
	if hasQuery && len(ctx.Filtered) > 0 {
		topMatchIdx = core.FindInAll(ctx.AllItems, ctx.Filtered[0])
	}

	// Scroll tree to keep cursor visible
	if ctx.TreeCursor >= 0 {
		if ctx.TreeCursor < ctx.TreeOffset {
			ctx.TreeOffset = ctx.TreeCursor
		}
		if ctx.TreeCursor >= ctx.TreeOffset+treeSpace {
			ctx.TreeOffset = ctx.TreeCursor - treeSpace + 1
		}
	}
	if ctx.TreeOffset < 0 {
		ctx.TreeOffset = 0
	}

	for i := 0; i < treeSpace; i++ {
		vi := ctx.TreeOffset + i
		if vi >= len(visible) {
			break
		}
		row := visible[vi]
		isSelected := vi == ctx.TreeCursor
		isTopMatch := hasQuery && row.ItemIdx == topMatchIdx && !isSelected
		drawTreeRow(c, row, isSelected, isTopMatch, ctx, cfg, borderOffset, y+i, w)
	}

	if cfg.Border {
		drawBorderBottom(c, w, startY+h-1)
		drawBorderSides(c, w, startY, startY+h-1)
	}
}

// drawTreeRow renders a single tree item row.
func drawTreeRow(c Canvas, row core.TreeRow, isSelected, isTopMatch bool, ctx *core.TreeContext, cfg Config, borderOffset, y, w int) {
	// Fill background
	if isSelected || isTopMatch {
		bg := tcell.StyleDefault.Background(tcell.ColorDarkBlue)
		for x := borderOffset; x < w-borderOffset; x++ {
			c.SetContent(x, y, ' ', nil, bg)
		}
	}

	x := borderOffset
	hasBg := isSelected || isTopMatch

	// Selection indicator
	if isSelected {
		indStyle := tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true).Background(tcell.ColorDarkBlue)
		drawText(c, x, y, "\u25b8 ", indStyle, 2)
	} else {
		style := tcell.StyleDefault
		if hasBg {
			style = style.Background(tcell.ColorDarkBlue)
		}
		drawText(c, x, y, "  ", style, 2)
	}
	x += 2

	// Indentation
	indent := row.Item.Depth * 2
	for i := 0; i < indent; i++ {
		style := tcell.StyleDefault
		if hasBg {
			style = style.Background(tcell.ColorDarkBlue)
		}
		c.SetContent(x+i, y, ' ', nil, style)
	}
	x += indent

	// Icon
	var iconRune rune
	var iconStyle tcell.Style
	if row.Item.HasChildren {
		iconRune = '\U000F024B'
		iconStyle = tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true)
	} else {
		iconRune = '\uF15B'
		iconStyle = tcell.StyleDefault.Foreground(tcell.ColorWhite)
	}
	if hasBg {
		iconStyle = iconStyle.Background(tcell.ColorDarkBlue)
	}
	c.SetContent(x, y, iconRune, nil, iconStyle)
	x += 2 // wide icon occupies 2 cells
	bufStyle := tcell.StyleDefault
	if hasBg {
		bufStyle = bufStyle.Background(tcell.ColorDarkBlue)
	}
	c.SetContent(x, y, ' ', nil, bufStyle)
	x++

	// Name
	name := ""
	if len(row.Item.Fields) > 0 {
		name = row.Item.Fields[0]
	}
	var nameStyle tcell.Style
	if row.Item.HasChildren {
		nameStyle = tcell.StyleDefault.Foreground(tcell.ColorDarkCyan).Bold(true)
	} else if isSelected {
		nameStyle = tcell.StyleDefault.Foreground(tcell.ColorWhite)
	} else {
		nameStyle = tcell.StyleDefault
	}
	if hasBg {
		nameStyle = nameStyle.Background(tcell.ColorDarkBlue)
	}

	nameWidth := ctx.NameColWidth + ctx.ColGap - indent
	nameRunes := []rune(name)
	if nameWidth < len(nameRunes)+1 {
		nameWidth = len(nameRunes) + 1
	}

	// Highlight matched characters for top match
	if isTopMatch && len(row.Item.MatchIndices) > 0 && len(row.Item.MatchIndices[0]) > 0 {
		drawHighlightedText(c, x, y, name, nameStyle, nameWidth, row.Item.MatchIndices[0], hasBg)
	} else {
		drawText(c, x, y, name, nameStyle, nameWidth)
	}
	x += nameWidth

	// Description
	if len(row.Item.Fields) > 1 {
		desc := row.Item.Fields[1]
		descStyle := tcell.StyleDefault
		if hasBg {
			descStyle = descStyle.Background(tcell.ColorDarkBlue)
		}
		remaining := w - x - borderOffset
		if remaining > 0 {
			drawText(c, x, y, desc, descStyle, remaining)
		}
	}
}

// drawHighlightedText draws text with certain character indices highlighted in green.
func drawHighlightedText(c Canvas, x, y int, text string, baseStyle tcell.Style, maxW int, matchIndices []int, hasBg bool) {
	runes := []rune(text)
	matchSet := make(map[int]bool, len(matchIndices))
	for _, idx := range matchIndices {
		matchSet[idx] = true
	}

	for i, r := range runes {
		if i >= maxW {
			break
		}
		style := baseStyle
		if matchSet[i] {
			style = tcell.StyleDefault.Foreground(tcell.ColorGreen).Bold(true)
			if hasBg {
				style = style.Background(tcell.ColorDarkBlue)
			}
		}
		c.SetContent(x+i, y, r, nil, style)
	}
}

// clickUnifiedRow handles a click on a visual row in the unified view.
func clickUnifiedRow(s *core.State, row int, cfg Config, h int) string {
	ctx := s.TopCtx()
	borderOffset := 0
	if cfg.Border {
		borderOffset = 1
	}

	firstItemRow := borderOffset + 3 // prompt bar (top border + content + bottom border)
	if len(ctx.Headers) > 0 {
		firstItemRow += 2 // header + divider
	}

	visible := core.TreeVisibleItems(s)
	itemRow := row - firstItemRow

	if itemRow < 0 {
		return ""
	}

	vi := ctx.TreeOffset + itemRow
	if vi >= len(visible) {
		return ""
	}
	ctx.TreeCursor = vi
	tr := visible[vi]
	if tr.Item.HasChildren {
		ctx.TreeExpanded[tr.ItemIdx] = !ctx.TreeExpanded[tr.ItemIdx]
		return ""
	}
	if ctx.OnLeafSelect != nil {
		return ctx.OnLeafSelect(tr.Item)
	}
	return "select:" + formatOutput(tr.Item, cfg)
}
