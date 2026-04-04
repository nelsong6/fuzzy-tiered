//go:build windows

package tui

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
	"golang.org/x/term"
)

// openRawTerminal opens CONIN$/CONOUT$ and puts the input handle into raw mode
// with VT input enabled so arrow keys and other specials arrive as ANSI sequences.
func openRawTerminal() (*rawTerminal, error) {
	in, err := os.OpenFile("CONIN$", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open CONIN$: %w", err)
	}

	out, err := os.OpenFile("CONOUT$", os.O_RDWR, 0)
	if err != nil {
		in.Close()
		return nil, fmt.Errorf("open CONOUT$: %w", err)
	}

	// Enable VT processing on the output handle
	outHandle := windows.Handle(out.Fd())
	var outMode uint32
	windows.GetConsoleMode(outHandle, &outMode)
	windows.SetConsoleMode(outHandle, outMode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)

	// Enable VT input on the input handle before going raw
	inHandle := windows.Handle(in.Fd())
	var inMode uint32
	windows.GetConsoleMode(inHandle, &inMode)
	windows.SetConsoleMode(inHandle, inMode|windows.ENABLE_VIRTUAL_TERMINAL_INPUT)

	// Put into raw mode (this further adjusts the console mode)
	oldState, err := term.MakeRaw(int(in.Fd()))
	if err != nil {
		out.Close()
		in.Close()
		return nil, fmt.Errorf("raw mode: %w", err)
	}

	return &rawTerminal{
		in:       in,
		out:      out,
		oldState: oldState,
	}, nil
}
