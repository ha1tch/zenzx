//go:build !headless

package zenzx

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// What a program is exported with: the border, the menu bar and the other preferences as the
// window had them at the pick, whatever they are changed to while the name is asked for.
func TestAnExportCarriesTheWindowAsItWasAtThePick(t *testing.T) {
	b, zx := exportBar(t)
	zx.window().showBorder = false
	zx.window().showFPS = true
	b.fixed = true
	b.stopAnimation = false
	pickExport(b, zx)

	// Changed while the browser is open: not what is exported.
	zx.window().showBorder = true
	b.fixed = false

	dir := t.TempDir()
	out := filepath.Join(dir, "game")
	exportAs(b, zx, dir, "game")
	file, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	p, _, err := readProgram(bytes.NewReader(file), int64(len(file)))
	if err != nil {
		t.Fatal(err)
	}
	v := p.View
	if v == nil || v.Settings == nil {
		t.Fatalf("the program carries no window: %+v", p)
	}
	if v.Border {
		t.Error("the border was hidden at the pick, and the program shows it")
	}
	if !v.FPS {
		t.Error("the FPS counter was shown at the pick, and the program hides it")
	}
	if !v.Settings.FixedMenuBar {
		t.Error("the menu bar was fixed at the pick, and the program lets it hide")
	}
	if v.Settings.StopAnimationOn() {
		t.Error("the stop animation was off at the pick, and the program has it on")
	}
	if v.Settings.DisplayScale != zx.window().screen.multiplier {
		t.Errorf("the scale is %d, want %d", v.Settings.DisplayScale, zx.window().screen.multiplier)
	}
	if v.checked(validThemeNames, validFontNames) == nil {
		t.Errorf("the settings the program carries are not valid ones: %+v", v.Settings)
	}
}

// And the other way: a window with its border and a bar that hides exports as that.
func TestAnExportOfADefaultWindowShowsTheBorderAndLetsTheBarHide(t *testing.T) {
	b, zx := exportBar(t)
	pickExport(b, zx)
	v := b.export.program.View
	b.cancelExport(zx)
	if v == nil || !v.Border || v.FPS || v.Settings.FixedMenuBar {
		t.Errorf("a default window exports as %+v", v)
	}
}
