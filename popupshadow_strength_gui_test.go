//go:build !headless

package zenzx

import "testing"

// The popup menus' shadow is 30% less dark than the texture; the tint that does it is white with
// that much of its alpha, and 1 leaves the shadow as it was.
func TestMenuShadowIsThirtyPercentLighter(t *testing.T) {
	if c := shadowTint(1); c.A != 255 || c.R != 255 || c.G != 255 || c.B != 255 {
		t.Errorf("full strength: %+v", c)
	}
	if c := shadowTint(menuShadowStrength); c.A < 178 || c.A > 179 {
		t.Errorf("the menus' tint has alpha %d, want 70%% of 255 (178)", c.A)
	}
	if shadowTint(-1).A != 0 || shadowTint(2).A != 255 {
		t.Error("a strength outside 0 to 1 is not held to it")
	}
}
