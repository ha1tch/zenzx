package zenui

// ThemeName identifies one built-in theme preset for selection purposes (a
// menu, a CLI flag, a config file) without callers needing to call each
// preset function themselves just to enumerate what's available.
type ThemeName string

const (
	ThemeDark     ThemeName = "Dark"
	ThemeLight    ThemeName = "Light"
	ThemeSpectrum ThemeName = "Spectrum"
)

// Themes lists every built-in preset name, in display order.
var Themes = []ThemeName{ThemeDark, ThemeLight, ThemeSpectrum}

// LoadTheme returns the named built-in preset. Returns DarkTheme() for an
// unrecognised name -- unlike fonts.Load (which errors on an unknown
// name), a theme always has to resolve to *something* drawable every
// frame, so a UI wiring bug here degrading to a reasonable default is
// preferable to every draw call needing its own error handling.
func LoadTheme(name ThemeName) Theme {
	switch name {
	case ThemeLight:
		return LightTheme()
	case ThemeSpectrum:
		return SpectrumTheme()
	default:
		return DarkTheme()
	}
}

// DarkTheme is the same colour scheme DefaultTheme has always returned,
// with two deliberate adjustments: the dialog panel/field pairing reads
// slightly more distinct from each other, and the selection highlight
// shifts from a flat blue to a slightly richer blue-violet -- a different
// hue, not just a different shade of the same one.
func DarkTheme() Theme {
	return Theme{
		Backdrop:              Colour{0, 0, 0, 0xa0},
		Panel:                 Colour{0x22, 0x22, 0x2c, 0xff},
		Sidebar:               Colour{0x18, 0x18, 0x20, 0xff},
		SideText:              Colour{0xc0, 0xc0, 0xd0, 0xff},
		Border:                Colour{0x6a, 0x6a, 0x78, 0xff},
		Text:                  Colour{0xe0, 0xe0, 0xe8, 0xff},
		DimText:               Colour{0x90, 0x90, 0xa0, 0xff},
		DirText:               Colour{0xff, 0xd0, 0x60, 0xff},
		SelFill:               Colour{0x3a, 0x48, 0x94, 0xff},
		SelText:               Colour{0xf8, 0xf8, 0xfc, 0xff},
		BarSelFill:            Colour{0x3a, 0x48, 0x94, 0xff},
		BarSelText:            Colour{0xf8, 0xf8, 0xfc, 0xff},
		BorderThickness:       1,
		ItemPadYPercent:       25,
		ItemPadXPercent:       50,
		ShowBarRainbow:        false,
		DropdownBottomPadding: 0,
		UseBarGradient:        false,
		UseGradientOverlay:    false,
		ShowZSPLogo:           true,
		CheckboxEmptyColour:   Colour{0x80, 0x80, 0x80, 0xff}, // neutral grey
		SeparatorColour:       Colour{0x6a, 0x6a, 0x78, 0xff}, // same as Border
		UseCheckboxColour:     false,
		Field:                 Colour{0x16, 0x16, 0x1e, 0xff},
		Button:                Colour{0x30, 0x30, 0x40, 0xff},
		ButtonHot:             Colour{0x44, 0x44, 0x58, 0xff},
		ButtonText:            Colour{0xf0, 0xf0, 0xf8, 0xff},
		Disabled:              Colour{0x50, 0x50, 0x5a, 0xff},
	}
}

// LightTheme is a macOS Aqua-inspired light scheme: light grey panels,
// near-white fields, near-black text, and the classic Aqua selection
// blue -- crisp and light rather than a straight colour inversion of
// DarkTheme, matching how Aqua's own selection blue is a specific, fairly
// saturated hue rather than an inverted grey.
func LightTheme() Theme {
	return Theme{
		Backdrop:              Colour{0, 0, 0, 0x50},
		Panel:                 Colour{0xec, 0xec, 0xec, 0xff},
		Sidebar:               Colour{0xe0, 0xe0, 0xe6, 0xff},
		SideText:              Colour{0x40, 0x40, 0x48, 0xff},
		Border:                Colour{0xb0, 0xb0, 0xb8, 0xff},
		Text:                  Colour{0x1a, 0x1a, 0x1e, 0xff},
		DimText:               Colour{0x70, 0x70, 0x78, 0xff},
		DirText:               Colour{0x1a, 0x5f, 0xd6, 0xff},
		SelFill:               Colour{0x3d, 0x7f, 0xe6, 0xff},
		SelText:               Colour{0xff, 0xff, 0xff, 0xff},
		BarSelFill:            Colour{0x3d, 0x7f, 0xe6, 0xff},
		BarSelText:            Colour{0xff, 0xff, 0xff, 0xff},
		BorderThickness:       1,
		ItemPadYPercent:       25,
		ItemPadXPercent:       50,
		ShowBarRainbow:        false,
		DropdownBottomPadding: 0,
		UseBarGradient:        false,
		UseGradientOverlay:    false,
		ShowZSPLogo:           true,
		CheckboxEmptyColour:   Colour{0x80, 0x80, 0x80, 0xff}, // neutral grey
		SeparatorColour:       Colour{0xb0, 0xb0, 0xb8, 0xff}, // same as Border
		UseCheckboxColour:     false,
		Field:                 Colour{0xff, 0xff, 0xff, 0xff},
		Button:                Colour{0xf4, 0xf4, 0xf4, 0xff},
		ButtonHot:             Colour{0xe4, 0xe4, 0xe4, 0xff},
		ButtonText:            Colour{0x1a, 0x1a, 0x1e, 0xff},
		Disabled:              Colour{0xb8, 0xb8, 0xc0, 0xff},
	}
}

// SpectrumTheme mirrors the real Sinclair 128K/+2/+3 boot menu's own
// palette (docs/zenzx-model-catalog.pdf): a white menu panel with black
// text, a solid black field around it, and the menu's own bright cyan
// selection bar -- not a generic "retro" palette, the actual colours of
// the real screen this project's own generated catalog shows.
func SpectrumTheme() Theme {
	return Theme{
		Backdrop: Colour{0, 0, 0, 0xf0},
		Panel:    Colour{0xff, 0xff, 0xff, 0xff},
		Sidebar:  Colour{0x33, 0x33, 0x33, 0xff}, // a further ~10%-of-range lightening on top of the first pass's 0x1a
		SideText: Colour{0xff, 0xff, 0xff, 0xff},
		Border:   Colour{0, 0, 0, 0xff},
		Text:     Colour{0, 0, 0, 0xff},
		DimText:  Colour{0x80, 0x80, 0x80, 0xff},
		DirText:  Colour{0, 0, 0xd7, 0xff},    // Spectrum blue
		SelFill:  Colour{0, 0xff, 0xff, 0xff}, // bright cyan
		SelText:  Colour{0, 0, 0, 0xff},       // black, matching the real menu's own cyan-highlight row
		// The bar's own "which menu is open" indicator is pure black --
		// darker than the bar's own (now twice-lightened) resting
		// Sidebar colour -- not the dropdown's cyan, since the real
		// 128K/+2/+3 title strip never shows that highlight colour,
		// only the menu box below it does. Text stays white (matching
		// the bar's normal SideText), since black text on a black fill
		// would be invisible.
		BarSelFill:            Colour{0, 0, 0, 0xff},
		BarSelText:            Colour{0xff, 0xff, 0xff, 0xff},
		BorderThickness:       2,    // brought down from the first pass's 3px, which read too heavy
		DropdownBorderSkipTop: true, // the dropdown always opens directly under the bar; its top edge shouldn't read as a separate line from the bar's own bottom edge
		// The real menu's own item rows are tight -- barely taller than
		// the text itself. 7% (down from the second pass's 14%) brings
		// row height down a further 2px at the default zoom (padY 2->1,
		// itemH 20->18), per direct feedback that spacing was better
		// but still slightly too tall. Horizontal stays widened per
		// earlier feedback, in the opposite direction from vertical.
		ItemPadYPercent: 7,
		ItemPadXPercent: 75,
		ShowBarRainbow:  true,
		// The panel's own bottom border needs more room below the last
		// row than ItemPadYPercent alone gives it -- left/right padding
		// was confirmed correct as-is, this is the one edge that still
		// read as cramped.
		DropdownBottomPadding: 6,
		// The bar's resting colour is now a vertical gradient rather
		// than the flat, twice-lightened-from-black Sidebar this theme
		// used before -- 70% grey at the top fading to pure black at
		// the bottom, replacing that flat fill outright rather than
		// tuning it further. One further notch more contrasted:
		// 25% -> 40% grey at the top (a 15-percentage-point increase,
		// the same "notch" size used throughout this theme's earlier
		// tuning), the bottom already at pure black, the floor.
		UseBarGradient:    true,
		BarGradientTop:    Colour{0x66, 0x66, 0x66, 0xff}, // 40% grey (25% + 15, ~0.40*255 = 102)
		BarGradientBottom: Colour{0, 0, 0, 0xff},
		// A second, horizontal gradient layered on top of the vertical
		// one via multiply blend mode, over the same right-side region
		// the rainbow occupies -- white (no darkening) at the left
		// edge of that region, black (full darkening, matching the
		// base gradient's own bottom colour) at the window's own
		// right edge, so the darkening increases toward the rainbow's
		// own corner. Drawn before the rainbow itself, so the rainbow
		// stays at full brightness rather than also being darkened.
		UseGradientOverlay:   true,
		GradientOverlayLeft:  Colour{0xff, 0xff, 0xff, 0xff},
		GradientOverlayRight: Colour{0, 0, 0, 0xff},
		ShowZSPLogo:          false, // this theme uses the Sinclair rainbow instead, same bar slot
		// Separators are grey, distinct from the black item text/
		// border, rather than the near-invisible black-on-white
		// Border colour this theme would otherwise use.
		SeparatorColour:     Colour{0x80, 0x80, 0x80, 0xff},
		CheckboxEmptyColour: Colour{0x80, 0x80, 0x80, 0xff}, // neutral grey
		// The checkmark (current-selection tick) reads better as a
		// fixed, recognisably "Spectrum" colour than the item's own
		// computed text/selection colour, which is black for this
		// theme -- standard (not bright) green, a darker shade than
		// the bright green this theme used at first.
		UseCheckmarkColour: true,
		CheckmarkColour:    Colour{0x00, 0xc8, 0x00, 0xff}, // standard Spectrum green
		// Checkboxes (non-mutually-exclusive options) get their own
		// fixed colour too, distinct from the checkmark's green so the
		// two indicator kinds read as visually different at a glance,
		// not just by shape (cross vs tick).
		UseCheckboxColour: true,
		CheckboxColour:    Colour{0x00, 0x00, 0xc8, 0xff}, // standard Spectrum blue
		Field:             Colour{0xff, 0xff, 0xff, 0xff},
		Button:            Colour{0xff, 0xff, 0xff, 0xff},
		ButtonHot:         Colour{0, 0xff, 0xff, 0xff}, // same bright cyan as SelFill
		ButtonText:        Colour{0, 0, 0, 0xff},
		Disabled:          Colour{0xa0, 0xa0, 0xa0, 0xff},
	}
}
