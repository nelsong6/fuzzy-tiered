//go:build js && wasm

package main

import (
	"strings"
	"syscall/js"

	"github.com/gdamore/tcell/v2"
	"github.com/nelsong6/fzt/core"
	"github.com/nelsong6/fzt/internal/tui"
)

var (
	currentItems     []core.Item
	session          *tui.Session
	pendingCommands  []core.CommandItem
	pendingFrontend  struct{ name, version string }
)

func main() {
	js.Global().Set("fzt", js.ValueOf(map[string]interface{}{
		"init":        js.FuncOf(initSession),
		"handleKey":   js.FuncOf(handleKey),
		"clickRow":    js.FuncOf(clickRow),
		"resize":      js.FuncOf(resize),
		"loadYAML":    js.FuncOf(loadYAML),
		"setLabel":    js.FuncOf(setLabel),
		"addCommands": js.FuncOf(addCommands),
		"setFrontend":    js.FuncOf(setFrontend),
		"getVisibleRows": js.FuncOf(getVisibleRows),
		"getPromptState": js.FuncOf(getPromptState),
		"getUIState":     js.FuncOf(getUIState),
	}))
	select {}
}

// setLabel sets a label string displayed on the top-left border.
// Args: label (string)
// Returns: SessionFrame if session exists, null otherwise
func setLabel(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return jsError("setLabel requires a label string")
	}
	label := args[0].String()
	if session != nil {
		session.SetLabel(label)
		frame := session.Render()
		return frameToJS(frame)
	}
	// Store for later — will be applied when session is created
	pendingLabel = label
	return js.Null()
}

var pendingLabel string

// addCommands registers frontend-specific commands for the `:` palette.
// Args: commands (array of {name: string, description: string, action: string})
// Must be called before init().
func addCommands(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return jsError("addCommands requires an array of command objects")
	}
	arr := args[0]
	length := arr.Length()
	pendingCommands = make([]core.CommandItem, 0, length)
	for i := 0; i < length; i++ {
		obj := arr.Index(i)
		pendingCommands = append(pendingCommands, core.CommandItem{
			Name:        obj.Get("name").String(),
			Description: obj.Get("description").String(),
			Action:      obj.Get("action").String(),
		})
	}
	return js.Null()
}

// setFrontend registers the frontend name and version.
// Args: {name: string, version: string}
// Must be called before init().
func setFrontend(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return jsError("setFrontend requires an object with name and version")
	}
	obj := args[0]
	pendingFrontend.name = obj.Get("name").String()
	pendingFrontend.version = obj.Get("version").String()
	return js.Null()
}

// loadYAML parses YAML and stores items, but does not create a session.
func loadYAML(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return jsError("loadYAML requires a YAML string argument")
	}
	items, err := core.LoadYAMLFromString(args[0].String())
	if err != nil {
		return jsError(err.Error())
	}
	currentItems = items
	return js.Null()
}

// initSession creates a new headless TUI session in tree view mode.
// Args: cols (int), rows (int)
// Returns: {ansi: string, cursorX: int, cursorY: int}
func initSession(this js.Value, args []js.Value) interface{} {
	if len(args) < 2 {
		return jsError("init requires (cols, rows)")
	}
	cols := args[0].Int()
	rows := args[1].Int()

	if len(currentItems) == 0 {
		return jsError("no items loaded — call loadYAML first")
	}

	headerItem := core.Item{Fields: []string{"Name", "Description"}, Depth: -1}
	items := append([]core.Item{headerItem}, currentItems...)

	cfg := tui.Config{
		Layout:       "reverse",
		Border:       true,
		Tiered:       true,
		DepthPenalty: 5,
		HeaderLines:  1,
	}

	session = tui.NewTreeSession(items, cfg, cols, rows)
	if pendingFrontend.name != "" {
		session.SetFrontendInfo(pendingFrontend.name, pendingFrontend.version)
	}
	if len(pendingCommands) > 0 {
		session.SetFrontendCommands(pendingCommands)
	}
	if pendingLabel != "" {
		session.SetLabel(pendingLabel)
		pendingLabel = ""
	}
	frame := session.Render()
	return frameToJS(frame)
}

// clickRow handles a mouse click on a visual row in tree mode.
// Args: row (int, 0-based visual row)
// Returns: {ansi: string, cursorX: int, cursorY: int, action: string, url: string}
func clickRow(this js.Value, args []js.Value) interface{} {
	if session == nil {
		return jsError("session not initialized")
	}
	if len(args) < 1 {
		return jsError("clickRow requires (row)")
	}
	row := args[0].Int()

	frame, action := session.ClickRow(row)

	obj := js.Global().Get("Object").New()
	obj.Set("ansi", frame.ANSI)
	obj.Set("cursorX", frame.CursorX)
	obj.Set("cursorY", frame.CursorY)
	obj.Set("action", action)
	if strings.HasPrefix(action, "select:") {
		obj.Set("url", session.SelectedURL())
	}
	return obj
}

// handleKey processes a keyboard event.
// Args: key (string, e.g. "ArrowUp", "Enter", "a"), ctrl (bool), shift (bool)
// Returns: {ansi: string, cursorX: int, cursorY: int, action: string}
func handleKey(this js.Value, args []js.Value) interface{} {
	if session == nil {
		return jsError("session not initialized")
	}
	if len(args) < 3 {
		return jsError("handleKey requires (key, ctrl, shift)")
	}

	keyStr := args[0].String()
	ctrl := args[1].Bool()
	shift := args[2].Bool()

	key, ch := translateKey(keyStr, ctrl, shift)
	if key == tcell.KeyRune && ch == 0 {
		// Unrecognized key — ignore
		return js.Null()
	}

	frame, action := session.HandleKey(key, ch)

	obj := js.Global().Get("Object").New()
	obj.Set("ansi", frame.ANSI)
	obj.Set("cursorX", frame.CursorX)
	obj.Set("cursorY", frame.CursorY)
	obj.Set("action", action)
	if strings.HasPrefix(action, "select:") {
		obj.Set("url", session.SelectedURL())
	}
	return obj
}

// resize changes the terminal dimensions.
// Args: cols (int), rows (int)
// Returns: {ansi: string, cursorX: int, cursorY: int}
func resize(this js.Value, args []js.Value) interface{} {
	if session == nil {
		return jsError("session not initialized")
	}
	if len(args) < 2 {
		return jsError("resize requires (cols, rows)")
	}
	cols := args[0].Int()
	rows := args[1].Int()

	frame := session.Resize(cols, rows)
	return frameToJS(frame)
}

// translateKey maps browser key event properties to tcell key + rune.
func translateKey(key string, ctrl, shift bool) (tcell.Key, rune) {
	switch key {
	case "ArrowUp":
		return tcell.KeyUp, 0
	case "ArrowDown":
		return tcell.KeyDown, 0
	case "ArrowLeft":
		return tcell.KeyLeft, 0
	case "ArrowRight":
		return tcell.KeyRight, 0
	case "Enter":
		return tcell.KeyEnter, 0
	case "Escape":
		return tcell.KeyEscape, 0
	case "Backspace":
		return tcell.KeyBackspace2, 0
	case "Delete":
		return tcell.KeyDelete, 0
	case "Tab":
		if shift {
			return tcell.KeyBacktab, 0
		}
		return tcell.KeyTab, 0
	case "Home":
		return tcell.KeyCtrlA, 0
	case "End":
		return tcell.KeyCtrlE, 0
	}

	// Single character keys
	if len(key) == 1 {
		r := rune(key[0])
		if ctrl {
			switch r {
			case 'a', 'A':
				return tcell.KeyCtrlA, 0
			case 'e', 'E':
				return tcell.KeyCtrlE, 0
			case 'u', 'U':
				return tcell.KeyCtrlU, 0
			case 'w', 'W':
				return tcell.KeyCtrlW, 0
			case 'p', 'P':
				return tcell.KeyCtrlP, 0
			case 'n', 'N':
				return tcell.KeyCtrlN, 0
			case 'c', 'C':
				return tcell.KeyCtrlC, 0
			}
			return tcell.KeyRune, 0 // unknown ctrl combo — ignore
		}
		return tcell.KeyRune, r
	}

	// Multi-character key name we don't handle (Shift, Control, etc.)
	return tcell.KeyRune, 0
}

// getVisibleRows returns structured data for all visible tree rows.
func getVisibleRows(this js.Value, args []js.Value) interface{} {
	if session == nil {
		return jsError("session not initialized")
	}
	rows := session.GetVisibleRows()
	arr := js.Global().Get("Array").New(len(rows))
	for i, row := range rows {
		obj := js.Global().Get("Object").New()
		obj.Set("name", row.Name)
		obj.Set("description", row.Description)
		obj.Set("depth", row.Depth)
		obj.Set("isFolder", row.IsFolder)
		obj.Set("isSelected", row.IsSelected)
		obj.Set("isTopMatch", row.IsTopMatch)
		obj.Set("nameMatchIndices", intsToJS(row.NameMatchIndices))
		obj.Set("descMatchIndices", intsToJS(row.DescMatchIndices))
		arr.SetIndex(i, obj)
	}
	return arr
}

// getPromptState returns structured prompt bar state.
func getPromptState(this js.Value, args []js.Value) interface{} {
	if session == nil {
		return jsError("session not initialized")
	}
	ps := session.GetPromptState()
	obj := js.Global().Get("Object").New()
	obj.Set("mode", ps.Mode)
	scopeArr := js.Global().Get("Array").New(len(ps.ScopePath))
	for i, s := range ps.ScopePath {
		scopeArr.SetIndex(i, s)
	}
	obj.Set("scopePath", scopeArr)
	obj.Set("query", ps.Query)
	obj.Set("cursor", ps.Cursor)
	obj.Set("ghost", ps.Ghost)
	obj.Set("hint", ps.Hint)
	return obj
}

// getUIState returns structured chrome/metadata state.
func getUIState(this js.Value, args []js.Value) interface{} {
	if session == nil {
		return jsError("session not initialized")
	}
	ui := session.GetUIState()
	obj := js.Global().Get("Object").New()
	obj.Set("title", ui.Title)
	obj.Set("titlePos", ui.TitlePos)
	obj.Set("version", ui.Version)
	obj.Set("label", ui.Label)
	obj.Set("border", ui.Border)
	obj.Set("treeOffset", ui.TreeOffset)
	obj.Set("totalVisible", ui.TotalVisible)
	return obj
}

func intsToJS(ints []int) interface{} {
	if ints == nil {
		return js.Null()
	}
	arr := js.Global().Get("Array").New(len(ints))
	for i, v := range ints {
		arr.SetIndex(i, v)
	}
	return arr
}

func frameToJS(frame tui.SessionFrame) interface{} {
	obj := js.Global().Get("Object").New()
	obj.Set("ansi", frame.ANSI)
	obj.Set("cursorX", frame.CursorX)
	obj.Set("cursorY", frame.CursorY)
	return obj
}

func jsError(msg string) interface{} {
	return js.Global().Get("Error").New(msg)
}
