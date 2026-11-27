// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"fmt"
	"strings"

	"github.com/ha1tch/zentools/pkg/tap"
	"github.com/ha1tch/zentools/pkg/tzx"
)

// What a tape file holds, read without a machine: the blocks it lists, in order, and the first
// screen in it. The Manage Tapes window (tapes_gui.go) shows both for the tape that is chosen; the
// deck (tape_types.go) reads the same files for the pulses it plays.

// tapeScreenLen is the size of a ZX Spectrum screen, which is what a loading screen is.
const tapeScreenLen = 6912

// tapeScreenAddr is where a screen is loaded.
const tapeScreenAddr = 16384

// tapeLine is one block of a tape, as the window lists it.
type tapeLine struct {
	Kind   string // "Program", "Bytes", "Data", "Pause", ...
	Name   string // the name in a header, "" for the rest
	Detail string // how big, where it loads, how long it pauses
}

// text is the line as it is listed, n being the block's number from 1.
func (l tapeLine) text(n int) string {
	s := fmt.Sprintf("%3d  %-9s %-10s %s", n, l.Kind, asciiOnly(l.Name), l.Detail)
	return strings.TrimRight(s, " ")
}

// tapeSummary is what a tape file holds.
type tapeSummary struct {
	Format string     // "TAP" or "TZX"
	Lines  []tapeLine // the blocks, in order
	// Screen is the first screen on the tape: the 6912 bytes of a loading screen, or nil when there
	// is none. ScreenAt is the number of the block it is in (from 1), 0 when there is none.
	Screen   []byte
	ScreenAt int
	// Partial is why the file was read only as far as it was, "" when it was read whole.
	Partial string
}

// asciiOnly is s with anything the typeface does not have shown as "?": a name in a tape's header
// can hold any byte.
func asciiOnly(s string) string {
	out := []rune(s)
	for i, r := range out {
		if r < 0x20 || r > 0x7E {
			out[i] = '?'
		}
	}
	return string(out)
}

// headerLine is the line of a header block.
func headerLine(b tap.Block) tapeLine {
	l := tapeLine{Name: b.Name}
	switch b.Type {
	case tap.TypeProgram:
		l.Kind = "Program"
		l.Detail = fmt.Sprintf("%d bytes", b.DataLength)
		if b.Param1 < 32768 { // 32768 and over say that it does not run by itself
			l.Detail += fmt.Sprintf(", runs from line %d", b.Param1)
		}
	case tap.TypeCode:
		l.Kind = "Bytes"
		l.Detail = fmt.Sprintf("%d bytes at %d", b.DataLength, b.Param1)
	case 1:
		l.Kind = "Numbers"
		l.Detail = fmt.Sprintf("%d bytes", b.DataLength)
	case 2:
		l.Kind = "Characters"
		l.Detail = fmt.Sprintf("%d bytes", b.DataLength)
	default:
		l.Kind = "Header"
		l.Detail = fmt.Sprintf("type %d, %d bytes", b.Type, b.DataLength)
	}
	return l
}

// namesAScreen says a header's name is one that a loading screen has.
func namesAScreen(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "scr") || strings.Contains(n, "screen")
}

// summariser reads the blocks of a tape one by one: a header is remembered until the data block it
// describes comes, which tells a screen from other 6912 bytes.
type summariser struct {
	s    tapeSummary
	head *tap.Block // the header just read, waiting for its data
}

// block takes one tap-format block (flag, payload, checksum), however the file carried it.
func (m *summariser) block(b tap.Block) {
	n := len(m.s.Lines) + 1
	if b.IsHeader {
		m.head = &b
		m.s.Lines = append(m.s.Lines, headerLine(b))
		return
	}
	line := tapeLine{Kind: "Data", Detail: fmt.Sprintf("%d bytes", len(b.Data))}
	if m.head == nil {
		line.Detail += ", no header"
	}
	if b.Flag != 0xFF {
		line.Detail += fmt.Sprintf(", flag %d", b.Flag)
	}
	if !b.ChecksumOK {
		line.Detail += ", checksum wrong"
	}
	if m.s.Screen == nil && len(b.Data) == tapeScreenLen && b.Flag == 0xFF {
		isScreen := m.head == nil || // a loading screen with no header, as loaders read one
			(m.head.Type == tap.TypeCode && m.head.DataLength == tapeScreenLen &&
				(m.head.Param1 == tapeScreenAddr || namesAScreen(m.head.Name)))
		if isScreen {
			m.s.Screen = append([]byte(nil), b.Data...)
			m.s.ScreenAt = n
			line.Detail += ", the screen shown"
		}
	}
	m.head = nil
	m.s.Lines = append(m.s.Lines, line)
}

// other adds a block that is not data: a tone, a pause, a stop.
func (m *summariser) other(l tapeLine) {
	m.head = nil
	m.s.Lines = append(m.s.Lines, l)
}

// summariseTape reads the blocks of a tape file: a TZX when asTZX, a TAP otherwise. A file that
// ends in the middle of a block gives the blocks before it, with Partial saying why it stopped.
func summariseTape(data []byte, asTZX bool) tapeSummary {
	m := &summariser{}
	if !asTZX {
		m.s.Format = "TAP"
		blocks, err := tap.Decode(data)
		for _, b := range blocks {
			m.block(b)
		}
		if err != nil {
			m.s.Partial = err.Error()
		}
		return m.s
	}
	m.s.Format = "TZX"
	blocks, err := tzx.Decode(data)
	for _, b := range blocks {
		switch b.ID {
		case 0x10, 0x11, 0x14: // standard speed, turbo speed and pure data: all carry a tap-format block
			tb, terr := tap.DecodeBlock(b.Data)
			if terr != nil {
				m.other(tapeLine{Kind: "Data", Detail: fmt.Sprintf("%d bytes, too short", len(b.Data))})
				continue
			}
			m.block(tb)
		case 0x12:
			m.other(tapeLine{Kind: "Tone", Detail: fmt.Sprintf("%d pulses", b.PilotLength)})
		case 0x13:
			m.other(tapeLine{Kind: "Pulses", Detail: fmt.Sprintf("%d pulses", len(b.Pulses))})
		case 0x15, 0x18:
			m.other(tapeLine{Kind: "Recording", Detail: fmt.Sprintf("%d bytes", len(b.Data))})
		case 0x19:
			m.other(tapeLine{Kind: "Special", Detail: "generalized data"})
		case 0x20:
			if b.Pause == 0 {
				m.other(tapeLine{Kind: "Stop", Detail: "the tape stops here"})
			} else {
				m.other(tapeLine{Kind: "Pause", Detail: fmt.Sprintf("%d ms", b.Pause)})
			}
		case 0x2A:
			m.other(tapeLine{Kind: "Stop", Detail: "in 48K mode"})
		}
	}
	if err != nil {
		m.s.Partial = err.Error()
	}
	return m.s
}
