package tui

import (
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
)

// parseKey interprets raw terminal bytes and returns the tcell key, rune, and
// number of bytes consumed. Returns consumed=0 if the buffer is empty.
func parseKey(buf []byte, n int) (tcell.Key, rune, int) {
	if n == 0 {
		return 0, 0, 0
	}

	b := buf[0]

	// Single-byte control characters
	switch b {
	case 0x01:
		return tcell.KeyCtrlA, 0, 1
	case 0x03:
		return tcell.KeyCtrlC, 0, 1
	case 0x05:
		return tcell.KeyCtrlE, 0, 1
	case 0x0e:
		return tcell.KeyCtrlN, 0, 1
	case 0x10:
		return tcell.KeyCtrlP, 0, 1
	case 0x15:
		return tcell.KeyCtrlU, 0, 1
	case 0x17:
		return tcell.KeyCtrlW, 0, 1
	case 0x09:
		return tcell.KeyTab, 0, 1
	case 0x0d, 0x0a:
		return tcell.KeyEnter, 0, 1
	case 0x7f:
		return tcell.KeyBackspace2, 0, 1
	case 0x08:
		return tcell.KeyBackspace, 0, 1
	}

	// Escape sequences
	if b == 0x1b {
		if n == 1 {
			// Bare escape — caller should have resolved the timeout already
			return tcell.KeyEscape, 0, 1
		}
		if buf[1] == '[' {
			return parseCSI(buf, n)
		}
		if buf[1] == 'O' && n >= 3 {
			// SS3 sequences (some terminals send these for arrows)
			switch buf[2] {
			case 'A':
				return tcell.KeyUp, 0, 3
			case 'B':
				return tcell.KeyDown, 0, 3
			case 'C':
				return tcell.KeyRight, 0, 3
			case 'D':
				return tcell.KeyLeft, 0, 3
			}
		}
		// Unknown escape sequence — treat as bare escape
		return tcell.KeyEscape, 0, 1
	}

	// UTF-8 printable rune
	r, size := utf8.DecodeRune(buf[:n])
	if r == utf8.RuneError && size <= 1 {
		return 0, 0, 1 // skip invalid byte
	}
	return tcell.KeyRune, r, size
}

// parseCSI handles CSI sequences: \x1b[ ... <final byte>
func parseCSI(buf []byte, n int) (tcell.Key, rune, int) {
	// Minimum: \x1b [ <final> = 3 bytes
	if n < 3 {
		return tcell.KeyEscape, 0, 1
	}

	// Simple CSI: \x1b[X where X is the final byte
	switch buf[2] {
	case 'A':
		return tcell.KeyUp, 0, 3
	case 'B':
		return tcell.KeyDown, 0, 3
	case 'C':
		return tcell.KeyRight, 0, 3
	case 'D':
		return tcell.KeyLeft, 0, 3
	case 'H':
		return tcell.KeyHome, 0, 3
	case 'F':
		return tcell.KeyEnd, 0, 3
	case 'Z':
		return tcell.KeyBacktab, 0, 3
	}

	// Extended CSI: \x1b[<param>~ (e.g. \x1b[3~ = Delete)
	if n >= 4 && buf[3] == '~' {
		switch buf[2] {
		case '3':
			return tcell.KeyDelete, 0, 4
		case '2':
			return tcell.KeyInsert, 0, 4
		case '5':
			return tcell.KeyPgUp, 0, 4
		case '6':
			return tcell.KeyPgDn, 0, 4
		}
	}

	// Skip unrecognized CSI sequences — find the final byte
	for i := 2; i < n; i++ {
		if buf[i] >= 0x40 && buf[i] <= 0x7e {
			return 0, 0, i + 1 // consume the whole sequence, ignore it
		}
	}

	// Incomplete sequence — treat as bare escape
	return tcell.KeyEscape, 0, 1
}
