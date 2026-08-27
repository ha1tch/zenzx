package zenui

import "testing"

func TestLoadThemeKnownNames(t *testing.T) {
	cases := []struct {
		name ThemeName
		want Theme
	}{
		{ThemeDark, DarkTheme()},
		{ThemeLight, LightTheme()},
		{ThemeSpectrum, SpectrumTheme()},
	}
	for _, c := range cases {
		if got := LoadTheme(c.name); got != c.want {
			t.Errorf("LoadTheme(%s) = %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestLoadThemeUnknownFallsBackToDark(t *testing.T) {
	got := LoadTheme(ThemeName("NotARealTheme"))
	want := DarkTheme()
	if got != want {
		t.Errorf("LoadTheme(unknown) = %+v, want DarkTheme() %+v", got, want)
	}
}

func TestThemesHasNoDuplicates(t *testing.T) {
	seen := make(map[ThemeName]bool)
	for _, name := range Themes {
		if seen[name] {
			t.Errorf("Themes contains %s more than once", name)
		}
		seen[name] = true
	}
}

func TestThemePresetsAreFullyOpaqueExceptBackdrop(t *testing.T) {
	// Backdrop is deliberately translucent (it dims content behind a
	// modal); every other colour should be fully opaque -- a stray
	// A < 0xff elsewhere would mean something drawn with it silently
	// blends with whatever's underneath instead of covering it.
	for _, preset := range []struct {
		name  string
		theme Theme
	}{
		{"Dark", DarkTheme()},
		{"Light", LightTheme()},
		{"Spectrum", SpectrumTheme()},
	} {
		th := preset.theme
		fields := map[string]Colour{
			"Panel": th.Panel, "Sidebar": th.Sidebar, "SideText": th.SideText,
			"Border": th.Border, "Text": th.Text, "DimText": th.DimText,
			"DirText": th.DirText, "SelFill": th.SelFill, "Field": th.Field,
			"Button": th.Button, "ButtonHot": th.ButtonHot,
			"ButtonText": th.ButtonText, "Disabled": th.Disabled,
		}
		for field, c := range fields {
			if c.A != 0xff {
				t.Errorf("%s.%s is not fully opaque: alpha=0x%02x", preset.name, field, c.A)
			}
		}
	}
}

func TestSpectrumThemeMatchesRealMenuColours(t *testing.T) {
	// The load-bearing assertions for "looks like the real 128K menu,"
	// not a generic retro palette: white panel, black text, bright cyan
	// selection -- the actual colours docs/zenzx-model-catalog.pdf shows.
	th := SpectrumTheme()
	white := Colour{0xff, 0xff, 0xff, 0xff}
	black := Colour{0, 0, 0, 0xff}
	brightCyan := Colour{0, 0xff, 0xff, 0xff}

	if th.Panel != white {
		t.Errorf("SpectrumTheme().Panel = %+v, want white %+v", th.Panel, white)
	}
	if th.Text != black {
		t.Errorf("SpectrumTheme().Text = %+v, want black %+v", th.Text, black)
	}
	if th.SelFill != brightCyan {
		t.Errorf("SpectrumTheme().SelFill = %+v, want bright cyan %+v", th.SelFill, brightCyan)
	}
}

func TestDarkAndLightBarSelMatchesSelUnchanged(t *testing.T) {
	// Dark and Light were confirmed correct as shipped -- their bar's
	// open-label styling must stay exactly what SelFill/SelText already
	// were before BarSelFill/BarSelText existed, or the bar's
	// appearance in those two themes would silently change.
	for _, preset := range []struct {
		name  string
		theme Theme
	}{
		{"Dark", DarkTheme()},
		{"Light", LightTheme()},
	} {
		if preset.theme.BarSelFill != preset.theme.SelFill {
			t.Errorf("%s: BarSelFill = %+v, want it to match SelFill %+v (unchanged bar appearance)",
				preset.name, preset.theme.BarSelFill, preset.theme.SelFill)
		}
		if preset.theme.BarSelText != preset.theme.SelText {
			t.Errorf("%s: BarSelText = %+v, want it to match SelText %+v (unchanged bar appearance)",
				preset.name, preset.theme.BarSelText, preset.theme.SelText)
		}
	}
}

func TestDarkAndLightBorderUnchanged(t *testing.T) {
	for _, preset := range []struct {
		name  string
		theme Theme
	}{
		{"Dark", DarkTheme()},
		{"Light", LightTheme()},
	} {
		if preset.theme.BorderThickness != 1 {
			t.Errorf("%s: BorderThickness = %d, want 1 (the flat thickness every dropdown already drew at)", preset.name, preset.theme.BorderThickness)
		}
		if preset.theme.DropdownBorderSkipTop {
			t.Errorf("%s: DropdownBorderSkipTop = true, want false (unchanged 4-sided border)", preset.name)
		}
	}
}

func TestSpectrumThemeBarAndBorderAdjustments(t *testing.T) {
	th := SpectrumTheme()

	black := Colour{0, 0, 0, 0xff}
	white := Colour{0xff, 0xff, 0xff, 0xff}

	if th.BarSelFill != black {
		t.Errorf("SpectrumTheme().BarSelFill = %+v, want black %+v (the open label, not the dropdown's cyan)", th.BarSelFill, black)
	}
	if th.BarSelText != white {
		t.Errorf("SpectrumTheme().BarSelText = %+v, want white %+v (must stay legible over a black fill)", th.BarSelText, white)
	}
	// Sidebar (the bar's resting colour) must be lighter than pure
	// black -- the whole point of the adjustment -- but still dark
	// enough to read as "the title strip", not a mid-grey.
	if th.Sidebar == black {
		t.Error("SpectrumTheme().Sidebar is still pure black -- want it lightened from black, per the requested adjustment")
	}
	if th.Sidebar.R > 0x40 {
		t.Errorf("SpectrumTheme().Sidebar = %+v, want a small lightening from black, not a genuinely grey tone", th.Sidebar)
	}
	if th.BorderThickness <= 1 {
		t.Errorf("SpectrumTheme().BorderThickness = %d, want thicker than the 1px every other theme uses", th.BorderThickness)
	}
	if !th.DropdownBorderSkipTop {
		t.Error("SpectrumTheme().DropdownBorderSkipTop = false, want true (the dropdown always opens directly under the bar)")
	}
}

func TestSpectrumThemeSecondPassAdjustments(t *testing.T) {
	th := SpectrumTheme()

	if th.BorderThickness != 2 {
		t.Errorf("BorderThickness = %d, want 2 (brought down from the first pass's 3)", th.BorderThickness)
	}
	// Sidebar should be visibly lighter than the first pass's 0x1a, but
	// still a near-black, not a genuinely grey tone.
	if th.Sidebar.R <= 0x1a {
		t.Errorf("Sidebar = %+v, want lighter than the first pass's 0x1a (second lightening pass)", th.Sidebar)
	}
	if th.Sidebar.R > 0x50 {
		t.Errorf("Sidebar = %+v, want a modest further lightening, not a genuinely grey tone", th.Sidebar)
	}
	// Padding should be tighter than this package's default (25%/50%)
	// vertically -- a direct pixel comparison against the real menu's
	// own row height in the reference screenshot showed it barely
	// taller than the text itself -- while horizontal padding widened
	// further per direct feedback, not reduced to match.
	if th.ItemPadYPercent <= 0 || th.ItemPadYPercent >= 25 {
		t.Errorf("ItemPadYPercent = %d, want strictly between 0 and the tight default (25) -- the real menu's rows are tighter than even that default", th.ItemPadYPercent)
	}
	if th.ItemPadXPercent <= 65 {
		t.Errorf("ItemPadXPercent = %d, want wider than the first pass's 65", th.ItemPadXPercent)
	}
	if !th.ShowBarRainbow {
		t.Error("ShowBarRainbow = false, want true (the Sinclair rainbow decoration is Spectrum-specific)")
	}
}

func TestDarkAndLightPaddingUnchanged(t *testing.T) {
	for _, preset := range []struct {
		name  string
		theme Theme
	}{
		{"Dark", DarkTheme()},
		{"Light", LightTheme()},
	} {
		if preset.theme.ItemPadYPercent != 25 {
			t.Errorf("%s: ItemPadYPercent = %d, want 25 (the previous fixed lh/4 formula)", preset.name, preset.theme.ItemPadYPercent)
		}
		if preset.theme.ItemPadXPercent != 50 {
			t.Errorf("%s: ItemPadXPercent = %d, want 50 (the previous fixed lh/2 formula)", preset.name, preset.theme.ItemPadXPercent)
		}
		if preset.theme.ShowBarRainbow {
			t.Errorf("%s: ShowBarRainbow = true, want false (the rainbow decoration is Spectrum-specific)", preset.name)
		}
	}
}

func TestSpectrumThemeThirdPassAdjustments(t *testing.T) {
	th := SpectrumTheme()

	if th.ItemPadYPercent != 7 {
		t.Errorf("ItemPadYPercent = %d, want 7 (brought down from the second pass's 14, -2px at default zoom)", th.ItemPadYPercent)
	}
	if th.DropdownBottomPadding != 6 {
		t.Errorf("DropdownBottomPadding = %d, want 6", th.DropdownBottomPadding)
	}
	if !th.UseBarGradient {
		t.Error("UseBarGradient = false, want true")
	}
	wantTop := Colour{0x66, 0x66, 0x66, 0xff}
	if th.BarGradientTop != wantTop {
		t.Errorf("BarGradientTop = %+v, want %+v (40%% grey, one more 15%%-point notch lighter/more contrasted than the previous 25%%)", th.BarGradientTop, wantTop)
	}
	wantBottom := Colour{0, 0, 0, 0xff}
	if th.BarGradientBottom != wantBottom {
		t.Errorf("BarGradientBottom = %+v, want %+v (100%% black)", th.BarGradientBottom, wantBottom)
	}
}

func TestDarkAndLightNoGradientOrBottomPadding(t *testing.T) {
	for _, preset := range []struct {
		name  string
		theme Theme
	}{
		{"Dark", DarkTheme()},
		{"Light", LightTheme()},
	} {
		if preset.theme.UseBarGradient {
			t.Errorf("%s: UseBarGradient = true, want false (flat Sidebar fill unchanged)", preset.name)
		}
		if preset.theme.DropdownBottomPadding != 0 {
			t.Errorf("%s: DropdownBottomPadding = %d, want 0 (unchanged panel geometry)", preset.name, preset.theme.DropdownBottomPadding)
		}
	}
}

func TestSpectrumGradientOverlayValues(t *testing.T) {
	th := SpectrumTheme()
	if !th.UseGradientOverlay {
		t.Fatal("UseGradientOverlay = false, want true")
	}
	wantLeft := Colour{0xff, 0xff, 0xff, 0xff}
	if th.GradientOverlayLeft != wantLeft {
		t.Errorf("GradientOverlayLeft = %+v, want %+v (white, no darkening)", th.GradientOverlayLeft, wantLeft)
	}
	wantRight := Colour{0, 0, 0, 0xff}
	if th.GradientOverlayRight != wantRight {
		t.Errorf("GradientOverlayRight = %+v, want %+v (black, matching the base gradient's own bottom)", th.GradientOverlayRight, wantRight)
	}
	if th.GradientOverlayRight != th.BarGradientBottom {
		t.Error("GradientOverlayRight should match BarGradientBottom, per spec (\"same colours as the bottom bar\")")
	}
}

func TestOtherThemesDoNotUseGradientOverlay(t *testing.T) {
	for name, theme := range map[string]Theme{
		"Default": DefaultTheme(),
		"Dark":    DarkTheme(),
		"Light":   LightTheme(),
	} {
		if theme.UseGradientOverlay {
			t.Errorf("%s: UseGradientOverlay = true, want false", name)
		}
	}
}
