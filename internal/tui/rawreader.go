package tui

import (
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
	"golang.org/x/term"
)

// rawTerminal provides platform-abstracted raw TTY input/output for inline rendering.
type rawTerminal struct {
	in       *os.File
	out      *os.File
	oldState *term.State
}

// Close restores the terminal state and closes opened file descriptors.
func (rt *rawTerminal) Close() {
	if rt.oldState != nil {
		term.Restore(int(rt.in.Fd()), rt.oldState)
	}
	// Don't close stderr; only close explicitly opened files
	if rt.in != os.Stdin {
		rt.in.Close()
	}
}

// Write writes to the TTY output.
func (rt *rawTerminal) Write(b []byte) (int, error) {
	return rt.out.Write(b)
}

// WriteString writes a string to the TTY output.
func (rt *rawTerminal) WriteString(s string) (int, error) {
	return rt.out.Write([]byte(s))
}

// Size returns the terminal dimensions.
func (rt *rawTerminal) Size() (width, height int, err error) {
	return term.GetSize(int(rt.out.Fd()))
}

// ReadKey reads raw bytes from the TTY and parses them into a key event.
// Handles the escape key ambiguity with a 50ms timeout.
func (rt *rawTerminal) ReadKey() (tcell.Key, rune, error) {
	buf := make([]byte, 64)
	n, err := rt.in.Read(buf)
	if err != nil {
		return 0, 0, err
	}
	if n == 0 {
		return 0, 0, nil
	}

	// Handle escape ambiguity: if we got exactly \x1b, wait briefly for more
	if n == 1 && buf[0] == 0x1b {
		rt.in.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		n2, _ := rt.in.Read(buf[1:])
		rt.in.SetReadDeadline(time.Time{}) // clear deadline
		n += n2
	}

	key, ch, _ := parseKey(buf, n)
	return key, ch, nil
}
