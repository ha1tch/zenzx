// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/ha1tch/zentools/pkg/zxn"
)

// Screenshots. View > Screenshot saves what the machine shows into the screenshots folder,
// screenshots/ in the folder ZenZX runs in (the way snapshots/ is), under a name that says what
// it is of and when:
//
//	screenshot_<machine>_<YYYYMMDD>_<HHMMSS>.<ext>
//
// The underscore separates the fields, so none of them has one: the machine's name, which is
// the first BASIC program the machine loaded (FirstProgram), or the model before it has loaded
// one, goes through shotMachineField, which turns whatever is not a letter or a digit,
// spaces included, into a hyphen.
//
// What can be saved depends on what the machine is showing, and the formats that do not apply
// are the ones the menu greys:
//
//	PNG      always: the picture as it is drawn, border included
//	GIF      always: the next two seconds, 25 frames a second (screenshot_gif.go)
//	SCR      the ULA's standard screen, 6912 bytes (also the Next's screen 1)
//	Hi-Res   the ULA in the Timex hi-res mode: 12288 bytes and the byte of port 0xFF, a .hires.scr
//	Hi-Colour the ULA in the Timex hi-colour mode: 12288 bytes, a .hicol.scr
//	Layer 2  the Next's Layer 2 as a .nxi: its palette, then its pixels as the Next holds them
//	LoRes    the Next's LoRes as a .nxi, in the same layout
//
// The Next's formats are the layer's own memory, not what the compositor drew from it.

// ScreenshotDir is the screenshots folder's default name, as SnapshotDir is the snapshots'.
const ScreenshotDir = "screenshots"

// screenshotDir is the screenshots folder in use: ScreenshotDir, or what -screenshots-dir says.
var screenshotDir = ScreenshotDir

// screenshotTimeLayout is the date and time in a screenshot's name.
const screenshotTimeLayout = "20060102_150405"

// screenshotPrefix starts every name ZenZX gives a screenshot.
const screenshotPrefix = "screenshot"

// maxMachineField is the most characters the machine's name takes in a file name.
const maxMachineField = 32

// ShotFormat is a way to save the picture.
type ShotFormat int

const (
	ShotPNG      ShotFormat = iota // the picture, border included
	ShotSCR                        // the ULA's standard screen: 6912 bytes
	ShotHiRes                      // the Timex hi-res screen: 12288 bytes and the byte of port 0xFF
	ShotHiColour                   // the Timex hi-colour screen: 12288 bytes
	ShotLayer2                     // the Next's Layer 2: palette and pixels, a .nxi
	ShotLoRes                      // the Next's LoRes: palette and pixels, a .nxi
	ShotGIF                        // the next two seconds, animated
)

// ShotFormats is every format, in the order the menu has them.
var ShotFormats = []ShotFormat{ShotPNG, ShotSCR, ShotHiRes, ShotHiColour, ShotLayer2, ShotLoRes, ShotGIF}

// Ext is the extension of a file of the format.
func (f ShotFormat) Ext() string {
	switch f {
	case ShotPNG:
		return "png"
	case ShotSCR:
		return "scr"
	case ShotHiRes:
		return hiResExt
	case ShotHiColour:
		return hiColExt
	case ShotLayer2, ShotLoRes:
		return "nxi"
	case ShotGIF:
		return "gif"
	}
	return ""
}

// screenshotExts are the extensions the screenshots folder is read for.
var screenshotExts = []string{"png", "scr", hiResExt, hiColExt, "nxi", "gif"}

// The Timex screens are saved with an extension of two words, so that a file says what its 12288
// bytes are (a .scr does not) and a plain .scr stays the ULA's 6912.
const (
	hiResExt = "hires.scr"
	hiColExt = "hicol.scr"
)

// compoundExts are the extensions of two words.
var compoundExts = []string{hiResExt, hiColExt}

// shotExtOf is the extension of a screenshot's file name, in lower case and without its dot:
// "hires.scr" or "hicol.scr" for a Timex screen, otherwise what follows the last dot.
func shotExtOf(name string) string {
	lower := strings.ToLower(name)
	for _, e := range compoundExts {
		if strings.HasSuffix(lower, "."+e) {
			return e
		}
	}
	return strings.TrimPrefix(filepath.Ext(lower), ".")
}

// shotSuffix is the extension of name as it is written in the name, with its dot: ".hires.scr",
// ".PNG". It is "" when there is none.
func shotSuffix(name string) string {
	if ext := shotExtOf(name); ext != "" {
		return name[len(name)-len(ext)-1:]
	}
	return ""
}

// shotStem is name without its extension.
func shotStem(name string) string { return strings.TrimSuffix(name, shotSuffix(name)) }

// isScreenshotExt says whether ext (no dot, any case) is one of a screenshot's.
func isScreenshotExt(ext string) bool {
	ext = strings.ToLower(ext)
	for _, e := range screenshotExts {
		if e == ext {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------------------------
// Names
// ---------------------------------------------------------------------------------------------

// shotMachineField makes a machine's name fit the file name: letters and digits stay, and
// every run of anything else, a space or an underscore among it, becomes one hyphen, with none
// at either end, at most maxMachineField characters long. It is "" when nothing is left.
func shotMachineField(name string) string {
	var b strings.Builder
	hyphen := false
	n := 0
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if hyphen && b.Len() > 0 {
				b.WriteByte('-')
				n++
			}
			hyphen = false
			if n >= maxMachineField {
				break
			}
			b.WriteRune(r)
			n++
			continue
		}
		hyphen = true
	}
	return strings.TrimRight(b.String(), "-")
}

// screenshotName is the name of a screenshot of machine made at t, in a format.
func screenshotName(machine string, t time.Time, ext string) string {
	return screenshotPrefix + "_" + shotMachineField(machine) + "_" + t.Format(screenshotTimeLayout) + "." + ext
}

// parseScreenshotName reads a name made by screenshotName. ok is false for any other name, which
// includes a screenshot that has been renamed.
func parseScreenshotName(name string) (machine string, when time.Time, ext string, ok bool) {
	ext = shotExtOf(name)
	if !isScreenshotExt(ext) {
		return "", time.Time{}, "", false
	}
	parts := strings.Split(shotStem(name), "_")
	if len(parts) != 4 || parts[0] != screenshotPrefix || parts[1] == "" {
		return "", time.Time{}, "", false
	}
	when, err := time.ParseInLocation(screenshotTimeLayout, parts[2]+"_"+parts[3], time.Local)
	if err != nil {
		return "", time.Time{}, "", false
	}
	return parts[1], when, ext, true
}

// ShotMachineName is the machine's name in a screenshot's: the first BASIC program it has loaded,
// and its model while it has loaded none.
func (zx *ZenZX) ShotMachineName() string {
	if n := shotMachineField(zx.FirstProgram()); n != "" {
		return n
	}
	if n := shotMachineField(getModelName(zx)); n != "" {
		return n
	}
	return "machine"
}

// screenshotPath is the path of a file of the screenshots folder.
func screenshotPath(name string) string { return filepath.Join(screenshotDir, name) }

// ensureScreenshotDir makes the folder, when it is not there.
func ensureScreenshotDir() error { return os.MkdirAll(screenshotDir, 0o755) }

// writeScreenshotFile writes data to the screenshots folder as machine's screenshot made at t.
// A name that is taken is not written over: the time moves on a second at a time until the name
// is free, which keeps the name one that parseScreenshotName reads.
func writeScreenshotFile(machine string, t time.Time, ext string, data []byte) (string, error) {
	if err := ensureScreenshotDir(); err != nil {
		return "", err
	}
	for i := 0; i < 600; i++ {
		path := screenshotPath(screenshotName(machine, t.Add(time.Duration(i)*time.Second), ext))
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if _, err = f.Write(data); err != nil {
			f.Close()
			os.Remove(path)
			return "", err
		}
		if err = f.Close(); err != nil {
			os.Remove(path)
			return "", err
		}
		return path, nil
	}
	return "", fmt.Errorf("no free name in %s/ for a screenshot of %s", screenshotDir, machine)
}

// ---------------------------------------------------------------------------------------------
// What the machine is showing
// ---------------------------------------------------------------------------------------------

// ulaShot is the mode the ULA is in, as far as saving its screen goes.
type ulaShot int

const (
	ulaOff      ulaShot = iota // not shown: a Next with the ULA off, or LoRes in its place
	ulaStandard                // the standard screen
	ulaScreen1                 // the Timex's second standard screen, at 0x6000 of bank 5
	ulaHiRes                   // Timex hi-res
	ulaHiColour                // Timex hi-colour
)

// shotULA is the mode the ULA is in now.
func (zx *ZenZX) shotULA() ulaShot {
	io := zx.io
	if io != nil && io.memory != nil && io.memory.isNext {
		if io.nextRegs.regs[0x68]&0x80 != 0 || io.loresEnabled() {
			return ulaOff
		}
		switch io.nextTimexMode() {
		case nextTimexScreen1:
			return ulaScreen1
		case nextTimexHiColour:
			return ulaHiColour
		case nextTimexHiRes:
			return ulaHiRes
		}
		return ulaStandard
	}
	if zx.videoRenderer != nil && zx.videoRenderer.Name() == NSGraphicsTimex001HiColour {
		return ulaHiColour
	}
	return ulaStandard
}

// ShotApplicable says whether the picture on the machine can be saved in the format.
func (zx *ZenZX) ShotApplicable(f ShotFormat) bool {
	switch f {
	case ShotPNG, ShotGIF:
		return true
	case ShotSCR:
		u := zx.shotULA()
		return u == ulaStandard || u == ulaScreen1
	case ShotHiRes:
		return zx.shotULA() == ulaHiRes
	case ShotHiColour:
		return zx.shotULA() == ulaHiColour
	case ShotLayer2:
		return zx.io != nil && zx.io.layer2Enabled()
	case ShotLoRes:
		return zx.io != nil && zx.io.loresEnabled()
	}
	return false
}

// ---------------------------------------------------------------------------------------------
// The pixels
// ---------------------------------------------------------------------------------------------

// ShotImage is the picture of the machine as it is now. A Next's is its whole frame, border
// in it; a classic machine's has the border put round it when border is true, in the colour the
// border has now.
func (zx *ZenZX) ShotImage(border bool) *image.RGBA {
	img := zx.DecodeDisplay()
	if img == nil || !border || zx.videoRenderer == nil || (zx.io != nil && zx.io.nextFrameActive()) {
		return img
	}
	l, r, t, b := zx.videoRenderer.BorderMargins()
	if l+r+t+b == 0 {
		return img
	}
	size := img.Bounds().Size()
	out := image.NewRGBA(image.Rect(0, 0, size.X+l+r, size.Y+t+b))
	var colour uint8
	if zx.io != nil {
		colour = zx.io.borderColor & 7
	}
	draw.Draw(out, out.Bounds(), &image.Uniform{C: zxPalette[colour]}, image.Point{}, draw.Src)
	draw.Draw(out, image.Rect(l, t, l+size.X, t+size.Y), img, img.Bounds().Min, draw.Src)
	return out
}

// shotPNG is img as a PNG file.
func shotPNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ShotBytes is the file of a format (not GIF, which takes frames: screenshot_gif.go) for the
// machine as it is now.
func (zx *ZenZX) ShotBytes(f ShotFormat, border bool) ([]byte, error) {
	if f == ShotGIF {
		return nil, errors.New("a GIF is made over two seconds: use StartGIF")
	}
	if !zx.ShotApplicable(f) {
		return nil, errors.New("the picture is not one that format holds")
	}
	switch f {
	case ShotPNG:
		img := zx.ShotImage(border)
		if img == nil {
			return nil, errors.New("the machine has no picture")
		}
		return shotPNG(img)
	case ShotSCR:
		return zx.shotSCR(), nil
	case ShotHiRes:
		return zx.shotHiRes(), nil
	case ShotHiColour:
		return zx.shotTimexHalves(), nil
	case ShotLayer2:
		return zx.io.shotLayer2(), nil
	case ShotLoRes:
		return zx.io.shotLoRes(), nil
	}
	return nil, errors.New("unknown screenshot format")
}

// scrBitmapLen and scrAttrLen are the two parts of a standard screen.
const (
	scrBitmapLen = 6144
	scrAttrLen   = 768
)

// shotSCR is the standard screen as a .scr holds it: the bitmap, then the attributes.
func (zx *ZenZX) shotSCR() []byte {
	out := make([]byte, 0, scrBitmapLen+scrAttrLen)
	if zx.shotULA() == ulaScreen1 {
		for i := 0; i < scrBitmapLen+scrAttrLen; i++ {
			out = append(out, zx.io.bank5(0x2000+i))
		}
		return out
	}
	out = append(out, zx.screen.bitmap[:scrBitmapLen]...)
	return append(out, zx.screen.attributes[:scrAttrLen]...)
}

// shotHiRes is the Timex hi-res screen as a .hires.scr holds it: the two halves, then the byte of
// port 0xFF, whose bits 5-3 are the ink colour (the paper is its complement), as other tools
// that save the mode add it. Hi-res is a Next's mode here: a classic TS2068 never draws it (KNOWN_ISSUES.md).
func (zx *ZenZX) shotHiRes() []byte {
	out := zx.shotTimexHalves()
	if zx.io != nil {
		out = append(out, zx.io.nextTimexPort)
	}
	return out
}

// shotTimexHalves is a Timex screen, hi-res or hi-colour: 12288 bytes, which are the two
// 6144-byte halves the mode reads at 0x4000 and 0x6000.
func (zx *ZenZX) shotTimexHalves() []byte {
	out := make([]byte, 0, 2*scrBitmapLen)
	if zx.io.memory != nil && zx.io.memory.isNext {
		for i := 0; i < scrBitmapLen; i++ {
			out = append(out, zx.io.bank5(i))
		}
		for i := 0; i < scrBitmapLen; i++ {
			out = append(out, zx.io.bank5(0x2000+i))
		}
		return out
	}
	// A classic machine in hi-colour: the bitmap, and an attribute for each of its bytes at 0x6000.
	out = append(out, zx.screen.bitmap[:scrBitmapLen]...)
	for i := 0; i < scrBitmapLen; i++ {
		out = append(out, zx.memory.Read(uint16(hicolourAttrBase+i)))
	}
	return out
}

// shotPalette is a layer's palette as a .nxp holds it, two bytes a colour, for the colours a
// pixel of depth bits can be, with the palette offset taken into the indices, so that the file
// draws the way the layer does.
func (io *SpectrumIO) shotPalette(layer nextLayer, offset uint8, depth int) []byte {
	n := 256
	if depth == 4 {
		n = 16
	}
	pal := &zxn.Palette{Colors: make([]zxn.Color333, n)}
	for i := range pal.Colors {
		rgb := io.nextPalette.lookup(layer, uint8(resolveNextPixelIndex(uint8(i), offset)))
		pal.Colors[i] = zxn.Color333{R: rgb.R, G: rgb.G, B: rgb.B}
	}
	b, _ := zxn.EncodeNXP(pal) // n is 16 or 256: within the size the format holds
	return b
}

// shotLayer2 is Layer 2 as a .nxi: the palette, then the pixels as the Next's memory has them,
// 49152 bytes for 256x192 and 81920 for the wide modes (320x256 at 8 bits, 640x256 at 4 bits;
// both are stored column by column).
func (io *SpectrumIO) shotLayer2() []byte {
	mode := io.layer2CurrentMode()
	depth := 8
	if mode == layer2Mode640x256x4bpp {
		depth = 4
	}
	pages := int(layer2FramebufferPages(mode))
	base := io.layer2BaseBank()
	out := io.shotPalette(nextLayerLayer2, io.layer2PaletteOffset(), depth)
	for p := 0; p < pages; p++ {
		for off := 0; off < 0x2000; off++ {
			out = append(out, io.memory.nextPageRead(base+uint8(p), uint16(off)))
		}
	}
	return out
}

// shotLoRes is LoRes as a .nxi: the palette, then 12288 bytes of 128x96 pixels, or 6144 for the
// 4-bit Radastan mode, row by row.
func (io *SpectrumIO) shotLoRes() []byte {
	control := io.nextRegs.regs[0x6A]
	offset := control & 0x0F
	if control&0x20 != 0 { // Radastan: one block of bank 5, which port 0xFF and NextREG 0x6A choose
		file := int(io.nextTimexPort&0x01) ^ int(control>>4&0x01)
		out := io.shotPalette(nextLayerULA, offset, 4)
		for i := 0; i < 6144; i++ {
			out = append(out, io.bank5(file*0x2000+i))
		}
		return out
	}
	out := io.shotPalette(nextLayerULA, offset, 8)
	for i := 0; i < 6144; i++ { // the top 48 lines at 0x4000 of the bank, the bottom 48 at 0x6000
		out = append(out, io.bank5(i))
	}
	for i := 0; i < 6144; i++ {
		out = append(out, io.bank5(0x2000+i))
	}
	return out
}

// SaveScreenshot saves the machine's picture in a format (not GIF) into the screenshots folder,
// and returns where it went.
func (zx *ZenZX) SaveScreenshot(f ShotFormat, border bool, now time.Time) (string, error) {
	data, err := zx.ShotBytes(f, border)
	if err != nil {
		return "", err
	}
	return writeScreenshotFile(zx.ShotMachineName(), now, f.Ext(), data)
}
