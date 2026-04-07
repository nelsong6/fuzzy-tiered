package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/nelsong6/fzt/render"
)

// tcellCanvas wraps a real tcell.Screen to satisfy render.Canvas.
type tcellCanvas struct {
	screen tcell.Screen
}

func (c *tcellCanvas) SetContent(x, y int, primary rune, combining []rune, style tcell.Style) {
	c.screen.SetContent(x, y, primary, combining, style)
}

func (c *tcellCanvas) Size() (int, int) {
	return c.screen.Size()
}

func (c *tcellCanvas) ShowCursor(x, y int) {
	c.screen.ShowCursor(x, y)
}

func (c *tcellCanvas) HideCursor() {
	c.screen.HideCursor()
}

// Ensure tcellCanvas satisfies render.Canvas at compile time.
var _ render.Canvas = (*tcellCanvas)(nil)
