// Package fonts embeds the bundled BDF bitmap fonts and decodes them
// through pkg/bdf.
//
// Bundled faces:
//
//	Sinclair    8x8   — the ZX Spectrum 48K ROM character set (period-correct)
//	Cozette     6x13  — a modern, legible programming bitmap face
//	TomThumb    4x6   — extremely small, for space-constrained UI
//	Spleen      5x8   — small, clean, modern bitmap face
//	Creep       7x11  — a distinctive, decorative face
//	HaxorMedium 8x14  — a stylised "hacker" face
//
// See LICENSE-Sinclair.txt for Sinclair's own provenance and the Amstrad/
// Cliff Lawson permission it carries -- the identical basis already relied
// on for the ROM images in rom/. See LICENSE-Cozette.txt for Cozette's MIT
// licence. TomThumb, Spleen, Creep, and HaxorMedium come from
// github.com/ha1tch/bdf-fonts, whose own README states its fonts are
// public domain except for a short, explicitly named list (tom-thumb,
// cozette, sinclair, tahoma) -- TomThumb carries its own MIT notice
// embedded in its BDF COPYRIGHT property.
package fonts

import (
	"bytes"
	_ "embed"

	"github.com/ha1tch/zenzx/pkg/bdf"
)

//go:embed sinclair.bdf
var sinclairBDF []byte

//go:embed cozette.bdf
var cozetteBDF []byte

//go:embed tom-thumb.bdf
var tomThumbBDF []byte

//go:embed spleen-5x8.bdf
var spleenBDF []byte

//go:embed creep.bdf
var creepBDF []byte

//go:embed haxor-medium-12.bdf
var haxorMediumBDF []byte

// Sinclair decodes the bundled ZX Spectrum 8x8 face.
func Sinclair() (*bdf.Font, error) { return bdf.Parse(bytes.NewReader(sinclairBDF)) }

// Cozette decodes the bundled 6x13 programming face.
func Cozette() (*bdf.Font, error) { return bdf.Parse(bytes.NewReader(cozetteBDF)) }

// TomThumb decodes the bundled 4x6 face.
func TomThumb() (*bdf.Font, error) { return bdf.Parse(bytes.NewReader(tomThumbBDF)) }

// Spleen decodes the bundled 5x8 face.
func Spleen() (*bdf.Font, error) { return bdf.Parse(bytes.NewReader(spleenBDF)) }

// Creep decodes the bundled 7x11 decorative face.
func Creep() (*bdf.Font, error) { return bdf.Parse(bytes.NewReader(creepBDF)) }

// HaxorMedium decodes the bundled 8x14 face.
func HaxorMedium() (*bdf.Font, error) { return bdf.Parse(bytes.NewReader(haxorMediumBDF)) }

// SinclairBytes exposes the raw embedded BDF data (a defensive copy, not the
// original slice), for callers that want to parse or persist the source
// themselves.
func SinclairBytes() []byte { return defensiveCopy(sinclairBDF) }

func defensiveCopy(b []byte) []byte {
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

// Name identifies one bundled face for selection purposes (a menu, a CLI
// flag, a config file) without callers needing to import the Font types
// themselves just to enumerate what's available.
type Name string

const (
	NameSinclair    Name = "Sinclair"
	NameCozette     Name = "Cozette"
	NameTomThumb    Name = "TomThumb"
	NameSpleen      Name = "Spleen"
	NameCreep       Name = "Creep"
	NameHaxorMedium Name = "HaxorMedium"
)

// All lists every bundled face name, in a fixed, deliberate display order
// (roughly smallest/most utilitarian to largest/most decorative) --
// callers building a selection menu can range over this directly rather
// than maintaining their own parallel list.
var All = []Name{NameSinclair, NameTomThumb, NameSpleen, NameCozette, NameCreep, NameHaxorMedium}

// Load decodes the named bundled face. Returns an error for a name not in
// All -- there is no silent fallback to a default face, since a caller
// building a selection UI from All should never be able to pass an
// unrecognised name in the first place.
func Load(name Name) (*bdf.Font, error) {
	switch name {
	case NameSinclair:
		return Sinclair()
	case NameCozette:
		return Cozette()
	case NameTomThumb:
		return TomThumb()
	case NameSpleen:
		return Spleen()
	case NameCreep:
		return Creep()
	case NameHaxorMedium:
		return HaxorMedium()
	default:
		return nil, &UnknownFontError{Name: name}
	}
}

// UnknownFontError is returned by Load for a name not in All.
type UnknownFontError struct{ Name Name }

func (e *UnknownFontError) Error() string { return "fonts: unknown font name " + string(e.Name) }
