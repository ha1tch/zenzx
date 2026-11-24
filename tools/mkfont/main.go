// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

// Command mkfont extracts the 8x8 ZX Spectrum character set (codes 32-127)
// from a Spectrum 48K ROM and writes it as a BDF font, reproducing the exact
// layout of pkg/fonts/sinclair.bdf.
//
// The character set lives at ROM offset 0x3D00: 96 glyphs, 8 bytes each, one
// byte per 8-pixel row, MSB leftmost. This is the canonical extraction every
// Spectrum emulator uses for its own character ROM.
//
// Use an ORIGINAL UK 48K ROM as the source. Internationalised ROMs (Spanish,
// etc.) substitute locale glyphs into codes 91-93, 96 and 123-125 (the
// [ \ ] ` { | } positions become ¡ Ñ ¿ £ ñ ... and accented variants), which
// is exactly the defect this tool exists to avoid baking into the shipped
// font.
//
// Usage:
//
//	go run ./tools/mkfont -rom rom/48.rom -o pkg/fonts/sinclair.bdf
//
// With -narrow it instead derives the "Sinclair Narrow" 6x8 face from the same ROM glyphs by
// dropping two columns of each 8-wide cell: the far-right column (original
// column 7) and then the fourth column from the left (original column 3). The
// kept columns are the originals 0,1,2,4,5,6, packed into the high 6 bits of
// each row byte (BBX width 6):
//
//	go run ./tools/mkfont -rom rom/48.rom -narrow -o pkg/fonts/sinclair-narrow.bdf
//
// The write is atomic (temp file + rename) and guarded: the tool refuses to
// emit a font whose glyph count is not the expected 96, and prints a per-code
// summary of which glyphs differ from any existing output file so a
// regeneration's effect is visible before it is trusted.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"time"
)

// dropCols lists the original 8x8 columns removed to form the 6x8 face, in the
// order stated in the recipe: far-right (7), then fourth-from-left (3). Column
// numbering is 0 = leftmost .. 7 = rightmost.
var dropCols = []int{7, 3}

// narrowOverrides holds hand-drawn 6-wide replacements for the glyphs the
// mechanical column-drop would otherwise leave blank, broken, or disfigured --
// mostly those whose centre stem or diagonal lived in the dropped column 3.
// Most are taken from the "Sinclair Narrow" face (+, /, 0, 7, <, >, I, Y, [, \,
// ], k, v, w, x and the copyright glyph), which were authored at 6 wide; the
// rest (! : ; T ^ f i l t ) }) and the asterisk are drawn here. A Narrow glyph
// occupying columns 1-5 is shifted one column left to centre it in the 6-wide
// cell; one occupying columns 0-5 (the copyright glyph) is kept as-is.
// Each entry is 8 rows of 6 bits ('#' = ink), packed MSB-first into the high 6
// bits of each byte, matching cellRows' 6-wide output. Glyphs not listed here
// narrow mechanically. These are the only departures from the column-drop rule.
var narrowOverrides = map[int][]string{
	33:  {"......", "..#...", "..#...", "..#...", "..#...", "......", "..#...", "......"}, // !
	39:  {"......", "...#..", "..#...", "......", "......", "......", "......", "......"}, // ' (from "Sinclair Narrow")
	42:  {"......", "......", ".#.#..", "..#...", "#####.", "..#...", ".#.#..", "......"}, // *
	44:  {"......", "......", "......", "......", "......", "...#..", "...#..", "..#..."}, // , (from "Sinclair Narrow")
	43:  {"......", "......", "..#...", "..#...", "#####.", "..#...", "..#...", "......"}, // + (from "Sinclair Narrow")
	47:  {"......", "......", "....#.", "...#..", "..#...", ".#....", "#.....", "......"}, // / (from "Sinclair Narrow")
	48:  {"......", "..###.", ".#...#", ".#..##", ".#.#.#", ".##..#", "..###.", "......"}, // 0 (from "Sinclair Narrow", nudged right to the digit column)
	55:  {"......", ".#####", ".....#", "....#.", "...#..", "..#...", "..#...", "......"}, // 7 (from "Sinclair Narrow", nudged right to the digit column)
	58:  {"......", "......", "..#...", "......", "......", "..#...", "......", "......"}, // :
	59:  {"......", "......", "..#...", "......", "..#...", "..#...", ".#....", "......"}, // ;
	60:  {"......", "......", "...#..", "..#...", ".#....", "..#...", "...#..", "......"}, // < (from "Sinclair Narrow")
	62:  {"......", "......", "#.....", ".#....", "..#...", ".#....", "#.....", "......"}, // > (from "Sinclair Narrow")
	84:  {"......", "#####.", "..#...", "..#...", "..#...", "..#...", "..#...", "......"}, // T
	73:  {"......", ".###..", "..#...", "..#...", "..#...", "..#...", ".###..", "......"}, // I (from "Sinclair Narrow")
	81:  {"......", ".###..", "#...#.", "#...#.", "#.#.#.", "#..#..", ".##.#.", "......"}, // Q (from "Sinclair Narrow")
	86:  {"......", "#...#.", "#...#.", "#...#.", ".#.#..", ".#.#..", "..#...", "......"}, // V (from "Sinclair Narrow")
	89:  {"......", "#...#.", ".#.#..", "..#...", "..#...", "..#...", "..#...", "......"}, // Y (from "Sinclair Narrow")
	94:  {"......", "..#...", ".#.#..", "#...#.", "......", "......", "......", "......"}, // ^
	102: {"......", "...##.", "..#...", ".###..", "..#...", "..#...", "..#...", "......"}, // f
	105: {"......", "....#.", "......", "....#.", "....#.", "....#.", "....#.", "......"}, // i (right edge minus one)
	108: {"......", "...#..", "...#..", "...#..", "...#..", "...#..", "....#.", "......"}, // l (right edge minus one)
	109: {"......", "......", "##.#..", "#.#.#.", "#.#.#.", "#.#.#.", "#.#.#.", "......"}, // m (from "Sinclair Narrow")
	107: {"......", "#.....", "#.#...", "##....", "##....", "#.#...", "#..#..", "......"}, // k (from "Sinclair Narrow")
	116: {"......", "..#...", ".###..", "..#...", "..#...", "..#...", "...#..", "......"}, // t
	118: {"......", "......", ".#...#", ".#...#", "..#.#.", "..#.#.", "...#..", "......"}, // v (from "Sinclair Narrow")
	119: {"......", "......", ".#...#", ".#.#.#", ".#.#.#", ".#.#.#", "..#.#.", "......"}, // w (from "Sinclair Narrow")
	120: {"......", "......", "#...#.", ".#.#..", "..#...", ".#.#..", "#...#.", "......"}, // x (from "Sinclair Narrow")
	122: {"......", "......", "####..", "...#..", "..#...", ".#....", "####..", "......"}, // z (from "Sinclair Narrow")
	126: {"######", "##.##.", "##.##.", "#.##.#", "#.##.#", ".##.##", ".##.##", "######"}, // ~ (from "Sinclair Narrow", a graphic block)
	41:  {"......", ".#....", "..#...", "..#...", "..#...", "..#...", ".#....", "......"}, // )
	91:  {"......", ".###..", ".#....", ".#....", ".#....", ".#....", ".###..", "......"}, // [ (from "Sinclair Narrow")
	92:  {"......", "......", "#.....", ".#....", "..#...", "...#..", "....#.", "......"}, // \ (from "Sinclair Narrow")
	93:  {"......", "###...", "..#...", "..#...", "..#...", "..#...", "###...", "......"}, // ] (from "Sinclair Narrow")
	125: {"......", ".##...", "..#...", "...#..", "..#...", "..#...", ".##...", "......"}, // }
	127: {".####.", "#....#", "#.##.#", "#.#..#", "#.#..#", "#.##.#", "#....#", ".####."}, // (c) (from "Sinclair Narrow")
}

// overrideRows renders an override entry (6-bit row strings) to packed bytes,
// MSB-first, matching the narrowed cell layout.
func overrideRows(spec []string) []byte {
	out := make([]byte, glyphRows)
	for y := 0; y < glyphRows && y < len(spec); y++ {
		var packed byte
		for b := 0; b < 6; b++ {
			if b < len(spec[y]) && spec[y][b] == '#' {
				packed |= 1 << (7 - b)
			}
		}
		out[y] = packed
	}
	return out
}

const (
	charBase  = 0x3D00 // start of the character set in a Spectrum 48K ROM
	firstCode = 32     // space
	lastCode  = 127    // (c) / copyright glyph
	glyphRows = 8      // 8x8 cell
)

// aglNames maps each code point to the Adobe-glyph-list / PostScript name used
// as the BDF STARTCHAR name, matching the names already in sinclair.bdf.
var aglNames = map[int]string{
	32: "space", 33: "exclam", 34: "quotedbl", 35: "numbersign",
	36: "dollar", 37: "percent", 38: "ampersand", 39: "quotesingle",
	40: "parenleft", 41: "parenright", 42: "asterisk", 43: "plus",
	44: "comma", 45: "hyphen", 46: "period", 47: "slash",
	48: "zero", 49: "one", 50: "two", 51: "three", 52: "four",
	53: "five", 54: "six", 55: "seven", 56: "eight", 57: "nine",
	58: "colon", 59: "semicolon", 60: "less", 61: "equal",
	62: "greater", 63: "question", 64: "at",
	65: "A", 66: "B", 67: "C", 68: "D", 69: "E", 70: "F", 71: "G",
	72: "H", 73: "I", 74: "J", 75: "K", 76: "L", 77: "M", 78: "N",
	79: "O", 80: "P", 81: "Q", 82: "R", 83: "S", 84: "T", 85: "U",
	86: "V", 87: "W", 88: "X", 89: "Y", 90: "Z",
	91: "bracketleft", 92: "backslash", 93: "bracketright",
	94: "asciicircum", 95: "underscore", 96: "grave",
	97: "a", 98: "b", 99: "c", 100: "d", 101: "e", 102: "f", 103: "g",
	104: "h", 105: "i", 106: "j", 107: "k", 108: "l", 109: "m", 110: "n",
	111: "o", 112: "p", 113: "q", 114: "r", 115: "s", 116: "t", 117: "u",
	118: "v", 119: "w", 120: "x", 121: "y", 122: "z",
	123: "braceleft", 124: "bar", 125: "braceright", 126: "asciitilde",
	127: "copyright",
}

func main() {
	romPath := flag.String("rom", "rom/48.rom", "source Spectrum 48K ROM (use an original UK ROM)")
	outPath := flag.String("o", "pkg/fonts/sinclair.bdf", "output BDF path")
	narrow := flag.Bool("narrow", false, "derive the 6x8 face (drop columns 7 then 3) instead of the 8x8")
	flag.Parse()

	rom, err := os.ReadFile(*romPath)
	if err != nil {
		die("read rom: %v", err)
	}
	if len(rom) < charBase+(lastCode-firstCode+1)*glyphRows {
		die("rom too small (%d bytes): no character set at 0x%04X", len(rom), charBase)
	}

	width := 8
	kind := "8x8"
	if *narrow {
		width = 8 - len(dropCols)
		kind = fmt.Sprintf("%dx8", width)
	}

	bdf, nGlyphs := buildBDF(rom, *narrow)

	// Guard: the payload must be exactly 96 glyphs. A short read here means a
	// truncated ROM or a wrong offset, and must never reach the output file.
	const want = lastCode - firstCode + 1
	if nGlyphs != want {
		die("guard: produced %d glyphs, expected %d -- refusing to write", nGlyphs, want)
	}

	// Report the effect before writing: which codes differ from the existing
	// output (if any). This is how a regeneration's real change is seen.
	reportDiff(*outPath, rom, *narrow)

	if err := atomicWrite(*outPath, bdf); err != nil {
		die("write %s: %v", *outPath, err)
	}
	fmt.Printf("wrote %s (%s, %d glyphs, codes %d-%d) from %s\n", *outPath, kind, nGlyphs, firstCode, lastCode, *romPath)
}

// glyph returns the 8 row-bytes for a character code, as stored in the ROM.
func glyph(rom []byte, code int) []byte {
	off := charBase + (code-firstCode)*glyphRows
	return rom[off : off+glyphRows]
}

// cellRows returns the glyph's row bytes for the target face: the raw 8-wide
// rows for the 8x8 face, or the narrowed rows for the 6x8 face. In the narrowed
// form the kept columns (originals 0,1,2,4,5,6) are packed into the high bits
// of each byte, leftmost first, so a 6-wide BBX reads them MSB-first exactly as
// the 8-wide font does.
func cellRows(rom []byte, code int, narrow bool) []byte {
	raw := glyph(rom, code)
	if !narrow {
		return raw
	}

	var out []byte
	if spec, ok := narrowOverrides[code]; ok {
		out = overrideRows(spec)
	} else {
		keep := keptCols()
		out = make([]byte, glyphRows)
		for y, row := range raw {
			var packed byte
			for i, c := range keep {
				if row&(1<<(7-c)) != 0 {
					packed |= 1 << (7 - i)
				}
			}
			out[y] = packed
		}
	}

	// Lowercase a-z are right-aligned in the 6-wide cell: the glyph is slid
	// right until its rightmost inked column sits at column 5, so every
	// lowercase letter shares the same right edge regardless of its width.
	// Exceptions: 'i' and 'l' are left one column short of the right edge (set
	// directly in their override specs), so the thin stems get a little air on
	// the right rather than hugging the edge.
	if code >= 'a' && code <= 'z' && code != 'i' && code != 'l' {
		out = rightAlign6(out)
	}
	return out
}

// rightAlign6 slides a 6-wide glyph (packed in the high 6 bits of each byte)
// right so its rightmost inked column is column 5. A blank glyph is unchanged.
func rightAlign6(rows []byte) []byte {
	rightmost := -1
	for c := 5; c >= 0; c-- {
		mask := byte(1 << (7 - c))
		for _, r := range rows {
			if r&mask != 0 {
				rightmost = c
				break
			}
		}
		if rightmost >= 0 {
			break
		}
	}
	if rightmost < 0 || rightmost == 5 {
		return rows
	}
	by := 5 - rightmost
	out := make([]byte, len(rows))
	for i, r := range rows {
		out[i] = (r >> by) & 0xFF
	}
	return out
}

// keptCols is the ordered list of original columns that survive narrowing:
// every column 0..7 except those in dropCols, left to right.
func keptCols() []int {
	dropped := map[int]bool{}
	for _, c := range dropCols {
		dropped[c] = true
	}
	var keep []int
	for c := 0; c < 8; c++ {
		if !dropped[c] {
			keep = append(keep, c)
		}
	}
	return keep
}

// buildBDF renders the whole font and returns it with the glyph count. With
// narrow set it emits the 6x8 derived face; otherwise the raw 8x8 face.
func buildBDF(rom []byte, narrow bool) ([]byte, int) {
	var b bytes.Buffer
	date := time.Now().UTC().Format("2006-01-02 15:04:05")

	width := 8
	swidth := 640 // 1/1000 em per glyph at POINT_SIZE 80, 75 DPI: 80 per px
	if narrow {
		width = 8 - len(dropCols)
		swidth = 80 * width
	}

	fmt.Fprintln(&b, "STARTFONT 2.1")
	if narrow {
		fmt.Fprintf(&b, "FONT -Sinclair-SinclairNarrow-Medium-R-Normal--8-80-75-75-C-%d-ISO8859-1\n", swidth)
	} else {
		fmt.Fprintln(&b, "FONT -Sinclair-ZXSpectrum-Medium-R-Normal--8-80-75-75-C-80-ISO8859-1")
	}
	fmt.Fprintln(&b, "SIZE 8 75 75")
	fmt.Fprintf(&b, "FONTBOUNDINGBOX %d 8 0 -2\n", width)
	if narrow {
		fmt.Fprintln(&b, `COMMENT "Sinclair Narrow -- a 6x8 face derived from the ZX Spectrum 48K character set"`)
		fmt.Fprintf(&b, "COMMENT \"Derived on %s\"\n", date)
		fmt.Fprintln(&b, `COMMENT "Base glyphs: each 8-wide ROM cell loses its far-right column (7), then its fourth-from-left (3)"`)
		fmt.Fprintln(&b, `COMMENT "Kept original columns 0,1,2,4,5,6, packed MSB-first"`)
		fmt.Fprintln(&b, `COMMENT "Lowercase a-z are right-aligned to the cell's right edge (column 5)"`)
		fmt.Fprintln(&b, `COMMENT "Hand-redrawn glyphs (most from the Sinclair Narrow design): ! ' * + , / 0 7 < > I : ; Q T V Y [ \ ] ^ f i k l m t v w x z ~ ) } (c)"`)
	} else {
		fmt.Fprintln(&b, `COMMENT "ZX Spectrum 48K Character Set"`)
		fmt.Fprintf(&b, "COMMENT \"Extracted on %s\"\n", date)
		fmt.Fprintln(&b, `COMMENT "Original character set from Sinclair ZX Spectrum 48K ROM"`)
	}
	fmt.Fprintln(&b, "STARTPROPERTIES 7")
	fmt.Fprintln(&b, `FOUNDRY "Sinclair"`)
	if narrow {
		fmt.Fprintln(&b, `FAMILY_NAME "Sinclair Narrow"`)
	} else {
		fmt.Fprintln(&b, `FAMILY_NAME "ZX Spectrum"`)
	}
	fmt.Fprintln(&b, `WEIGHT_NAME "Medium"`)
	fmt.Fprintln(&b, `SLANT "R"`)
	fmt.Fprintln(&b, `SETWIDTH_NAME "Normal"`)
	fmt.Fprintln(&b, "PIXEL_SIZE 8")
	fmt.Fprintln(&b, "POINT_SIZE 80")
	fmt.Fprintln(&b, "ENDPROPERTIES")
	fmt.Fprintf(&b, "CHARS %d\n", lastCode-firstCode+1)

	n := 0
	for code := firstCode; code <= lastCode; code++ {
		name, ok := aglNames[code]
		if !ok {
			name = fmt.Sprintf("char%d", code)
		}
		fmt.Fprintf(&b, "STARTCHAR %s\n", name)
		fmt.Fprintf(&b, "ENCODING %d\n", code)
		fmt.Fprintf(&b, "SWIDTH %d 0\n", swidth)
		fmt.Fprintf(&b, "DWIDTH %d 0\n", width)
		fmt.Fprintf(&b, "BBX %d 8 0 -2\n", width)
		fmt.Fprintln(&b, "BITMAP")
		for _, row := range cellRows(rom, code, narrow) {
			fmt.Fprintf(&b, "%02X\n", row)
		}
		fmt.Fprintln(&b, "ENDCHAR")
		n++
	}
	fmt.Fprintln(&b, "ENDFONT")
	return b.Bytes(), n
}

// reportDiff prints the codes whose glyph bytes in the source ROM differ from
// those in an existing output file, so a regeneration names exactly what it
// changes. A missing output file is reported as a fresh generation.
func reportDiff(outPath string, rom []byte, narrow bool) {
	old, err := os.ReadFile(outPath)
	if err != nil {
		fmt.Printf("no existing %s -- fresh generation\n", outPath)
		return
	}
	cur := parseBitmaps(old)
	var changed []int
	for code := firstCode; code <= lastCode; code++ {
		want := cellRows(rom, code, narrow)
		if got, ok := cur[code]; !ok || !bytes.Equal(got, want) {
			changed = append(changed, code)
		}
	}
	if len(changed) == 0 {
		fmt.Println("no glyph changes vs existing font")
		return
	}
	fmt.Printf("%d glyph(s) change vs existing font:\n", len(changed))
	for _, code := range changed {
		name := aglNames[code]
		fmt.Printf("  code %3d (%-12s): %s\n", code, name, hexRow(cellRows(rom, code, narrow)))
	}
}

// parseBitmaps reads the ENCODING + BITMAP rows out of an existing BDF.
func parseBitmaps(bdf []byte) map[int][]byte {
	out := map[int][]byte{}
	lines := bytes.Split(bdf, []byte("\n"))
	code := -1
	inBitmap := false
	var rows []byte
	flush := func() {
		if code >= 0 && len(rows) == glyphRows {
			out[code] = append([]byte(nil), rows...)
		}
		rows = rows[:0]
	}
	for _, ln := range lines {
		s := string(bytes.TrimSpace(ln))
		switch {
		case len(s) > 9 && s[:9] == "ENCODING ":
			flush()
			fmt.Sscanf(s, "ENCODING %d", &code)
			inBitmap = false
		case s == "BITMAP":
			inBitmap = true
		case s == "ENDCHAR":
			inBitmap = false
		case inBitmap && len(s) == 2:
			var v int
			if _, err := fmt.Sscanf(s, "%02X", &v); err == nil {
				rows = append(rows, byte(v))
			}
		}
	}
	flush()
	return out
}

func hexRow(b []byte) string {
	s := ""
	for i, v := range b {
		if i > 0 {
			s += " "
		}
		s += fmt.Sprintf("%02X", v)
	}
	return s
}

// atomicWrite writes data to a temp file in the destination directory and
// renames it over the target, so a failure never leaves a partial font.
func atomicWrite(path string, data []byte) error {
	tmp, err := os.CreateTemp(dirOf(path), ".mkfont-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return "."
}

func die(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "mkfont: "+format+"\n", a...)
	os.Exit(1)
}
