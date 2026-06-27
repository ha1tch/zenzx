package main

import (
	"fmt"
	"strconv"
	"strings"
)

// ============================================================================
// Screen text recognition (SCREEN$-equivalent)
//
// This reads characters off the rendered display the same way Sinclair BASIC's
// SCREEN$ function does: by matching each 8x8 character cell's bitmap against
// the ROM font and recovering the character code. It is a pure observation --
// it reads the screen bitmap and the ROM font through normal memory reads and
// disturbs no machine state.
//
// The standard font lives in ROM at 0x3D00, storing 8 bytes per glyph for
// character codes 0x20 (space) through 0x7F, i.e. glyph N is at
// 0x3D00 + (N-0x20)*8. A cell matches a glyph if its eight bytes equal the
// glyph's eight bytes; inverse-video cells (INK/PAPER swapped) match the
// bitwise complement, so both are tested.
//
// Limitations: only the standard ROM font is recognised. A program that
// redefines CHARS or draws text with custom routines/UDGs will not be read
// correctly. This is sufficient for ROM messages and standard-font output
// (the SCREEN$ use case).
// ============================================================================

const (
	// fontBase is the ROM address of the standard character set.
	fontBase = 0x3D00
	// fontFirst is the first character code present in the font.
	fontFirst = 0x20
	// fontLast is the last character code present in the font.
	fontLast = 0x7F
	// screenCols and screenRows are the character-cell dimensions.
	screenCols = 32
	screenRows = 24
)

// cellBytes returns the eight bitmap bytes of the character cell at column
// col (0-31) and row (0-23), read top scanline first.
func cellBytes(zx *ZenZX, col, row int) [8]byte {
	var b [8]byte
	for sub := 0; sub < 8; sub++ {
		y := row*8 + sub
		// Use the screen's own bitmap-offset function (verified equivalent to
		// the standard thirds-interleaved layout) and add the 0x4000 base to
		// get the absolute memory address.
		addr := uint16(0x4000 + zx.screen.calcByteOffset(col*8, y))
		b[sub] = zx.memory.Read(addr)
	}
	return b
}

// glyphBytes returns the eight font bytes for character code ch.
func glyphBytes(zx *ZenZX, ch byte) [8]byte {
	var b [8]byte
	base := uint16(fontBase) + uint16(ch-fontFirst)*8
	for i := 0; i < 8; i++ {
		b[i] = zx.memory.Read(base + uint16(i))
	}
	return b
}

// recogniseCell returns the character at the given cell, or 0 if no glyph
// matches. Both normal and inverse-video (complemented) cells are matched.
func recogniseCell(zx *ZenZX, col, row int) byte {
	cell := cellBytes(zx, col, row)

	// Fast path: an all-zero cell is a space.
	allZero := true
	for _, v := range cell {
		if v != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return ' '
	}

	var inv [8]byte
	for i := range cell {
		inv[i] = ^cell[i]
	}

	for ch := byte(fontFirst); ch <= fontLast; ch++ {
		g := glyphBytes(zx, ch)
		if g == cell || g == inv {
			return ch
		}
	}
	return 0 // unrecognised
}

// ReadScreenRow recognises a full character row (0-23) into a string. Cells
// that match no glyph are rendered as '?'.
func ReadScreenRow(zx *ZenZX, row int) string {
	if row < 0 || row >= screenRows {
		return ""
	}
	out := make([]byte, screenCols)
	for col := 0; col < screenCols; col++ {
		ch := recogniseCell(zx, col, row)
		if ch == 0 {
			ch = '?'
		}
		out[col] = ch
	}
	return string(out)
}

// ReadScreen recognises the whole display into 24 rows of text.
func ReadScreen(zx *ZenZX) []string {
	rows := make([]string, screenRows)
	for r := 0; r < screenRows; r++ {
		rows[r] = ReadScreenRow(zx, r)
	}
	return rows
}

// ============================================================================
// Attribute reading
//
// Each character cell has an attribute byte: ink (bits 0-2), paper (bits 3-5),
// bright (bit 6), flash (bit 7). When a screen's text uses a custom font or
// UDGs that the glyph matcher cannot recognise, its *colour* layout is often
// still distinctive -- so menus and title screens can be recognised by their
// attributes even when their characters cannot be read.
// ============================================================================

// spectrum colour names, index = colour code 0-7.
var colourNames = map[string]uint8{
	"black": 0, "blue": 1, "red": 2, "magenta": 3,
	"green": 4, "cyan": 5, "yellow": 6, "white": 7,
}

// attrCell returns the attribute byte at column col, row row.
func attrCell(zx *ZenZX, col, row int) uint8 {
	if col < 0 || col >= screenCols || row < 0 || row >= screenRows {
		return 0
	}
	return zx.screen.attributes[row*screenCols+col]
}

// attrMatch is a colour predicate parsed from a spec string. Any unset field
// is a wildcard. Fields: ink, paper (0-7 or name), bright (0/1).
type attrMatch struct {
	ink, paper   int // -1 = any
	bright       int // -1 = any, else 0/1
}

// parseAttrSpec parses a spec like "ink=yellow paper=blue bright" or
// "ink=6 bright=1" into an attrMatch. Tokens are space-separated; the spec is
// passed already split as args.
func parseAttrSpec(tokens []string) (attrMatch, error) {
	m := attrMatch{ink: -1, paper: -1, bright: -1}
	colour := func(v string) (int, bool) {
		if c, ok := colourNames[strings.ToLower(v)]; ok {
			return int(c), true
		}
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 7 {
			return n, true
		}
		return 0, false
	}
	for _, tok := range tokens {
		switch {
		case tok == "bright":
			m.bright = 1
		case strings.HasPrefix(tok, "ink="):
			c, ok := colour(strings.TrimPrefix(tok, "ink="))
			if !ok {
				return m, fmt.Errorf("bad ink %q", tok)
			}
			m.ink = c
		case strings.HasPrefix(tok, "paper="):
			c, ok := colour(strings.TrimPrefix(tok, "paper="))
			if !ok {
				return m, fmt.Errorf("bad paper %q", tok)
			}
			m.paper = c
		case strings.HasPrefix(tok, "bright="):
			n, err := strconv.Atoi(strings.TrimPrefix(tok, "bright="))
			if err != nil || (n != 0 && n != 1) {
				return m, fmt.Errorf("bad bright %q", tok)
			}
			m.bright = n
		default:
			return m, fmt.Errorf("unknown attribute token %q", tok)
		}
	}
	if m.ink == -1 && m.paper == -1 && m.bright == -1 {
		return m, fmt.Errorf("empty attribute spec")
	}
	return m, nil
}

// matches reports whether an attribute byte satisfies the predicate.
func (m attrMatch) matches(attr uint8) bool {
	if m.ink != -1 && int(attr&0x07) != m.ink {
		return false
	}
	if m.paper != -1 && int((attr>>3)&0x07) != m.paper {
		return false
	}
	if m.bright != -1 && int((attr>>6)&0x01) != m.bright {
		return false
	}
	return true
}

// CountAttrMatches returns how many cells in the given row range (inclusive)
// satisfy the predicate. rowStart/rowEnd of -1 mean the whole screen.
func CountAttrMatches(zx *ZenZX, m attrMatch, rowStart, rowEnd int) int {
	if rowStart < 0 {
		rowStart = 0
	}
	if rowEnd < 0 || rowEnd >= screenRows {
		rowEnd = screenRows - 1
	}
	n := 0
	for row := rowStart; row <= rowEnd; row++ {
		for col := 0; col < screenCols; col++ {
			if m.matches(attrCell(zx, col, row)) {
				n++
			}
		}
	}
	return n
}
