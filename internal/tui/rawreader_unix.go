//go:build !windows

package tui

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

// openRawTerminal opens /dev/tty and puts it into raw mode.
func openRawTerminal() (*rawTerminal, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open /dev/tty: %w", err)
	}

	oldState, err := term.MakeRaw(int(tty.Fd()))
	if err != nil {
		tty.Close()
		return nil, fmt.Errorf("raw mode: %w", err)
	}

	return &rawTerminal{
		in:       tty,
		out:      tty,
		oldState: oldState,
	}, nil
}
