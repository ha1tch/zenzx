//go:build !headless

package zenzx

import (
	"testing"

	"github.com/ha1tch/zenui"
)

// The HUD takes its keyboard from the zenui.Input it is given: printable characters go to the
// input line, Backspace takes the last off, and a HUD that is not open takes nothing.
func TestHUDTakesItsKeysFromTheInput(t *testing.T) {
	h := &debugHUD{active: true}
	h.HandleInput(zenui.Input{Chars: []rune("reg a"), Keys: nil})
	if string(h.input) != "reg a" {
		t.Fatalf("typed %q", string(h.input))
	}
	h.HandleInput(zenui.Input{Chars: []rune{0x07, 0x7F, 'x', 0x1b}}) // control characters are not text
	if string(h.input) != "reg ax" {
		t.Errorf("control characters got in: %q", string(h.input))
	}
	h.HandleInput(zenui.Input{Keys: []zenui.Key{zenui.KeyBackspace}})
	if string(h.input) != "reg a" {
		t.Errorf("Backspace left %q", string(h.input))
	}
	h.HandleInput(zenui.Input{Keys: []zenui.Key{zenui.KeyBackspace, zenui.KeyBackspace}}) // one a frame
	if string(h.input) != "reg " {
		t.Errorf("two Backspace keys in a frame left %q", string(h.input))
	}
	closed := &debugHUD{} // not open
	closed.HandleInput(zenui.Input{Chars: []rune("x")})
	if len(closed.input) != 0 {
		t.Errorf("a closed HUD took %q", string(closed.input))
	}
}

// The resize gripper is driven by the input's pointer: a press near the panel's top edge
// starts a drag, the edge follows the pointer (within the limits), and the button coming up
// ends it. A press away from the edge does nothing.
func TestHUDResizeFollowsThePointerInTheInput(t *testing.T) {
	const screenW, screenH = 800, 600
	h := &debugHUD{active: true, anim: 1, heightFrac: hudDefaultFrac}
	top := h.panelTopY(screenH)

	if h.UpdateResize(zenui.Input{MouseY: top - 200, MousePressed: true, MouseDown: true}, screenW, screenH) || h.resizing {
		t.Fatal("a press far from the edge took the pointer")
	}
	if !h.UpdateResize(zenui.Input{MouseY: top + 1, MousePressed: true, MouseDown: true}, screenW, screenH) || !h.resizing {
		t.Fatal("a press on the edge did not start a drag")
	}
	want := float32(screenH-(top-60)) / float32(screenH)
	if !h.UpdateResize(zenui.Input{MouseY: top - 60, MouseDown: true}, screenW, screenH) || h.heightFrac != want {
		t.Errorf("dragging up gave %v (taking the pointer), want %v", h.heightFrac, want)
	}
	h.UpdateResize(zenui.Input{MouseY: -500, MouseDown: true}, screenW, screenH)
	if h.heightFrac != hudMaxFrac {
		t.Errorf("dragged past the top gave %v, want the limit %v", h.heightFrac, hudMaxFrac)
	}
	h.UpdateResize(zenui.Input{MouseY: screenH + 500, MouseDown: true}, screenW, screenH)
	if h.heightFrac != hudMinFrac {
		t.Errorf("dragged past the bottom gave %v, want the limit %v", h.heightFrac, hudMinFrac)
	}
	h.UpdateResize(zenui.Input{MouseY: screenH / 2}, screenW, screenH) // the button is up
	if h.resizing {
		t.Error("the drag did not end when the button came up")
	}
}

// The health of the audio buffer, as the overlay shows it.
func TestAudioHealthWords(t *testing.T) {
	for _, c := range []struct {
		level float32
		word  string
	}{{0, "UNDERRUN WARNING"}, {29.9, "UNDERRUN WARNING"}, {30, "LOW"}, {59.9, "LOW"}, {60, "GOOD"}, {100, "GOOD"}} {
		if _, w := audioHealth(c.level); w != c.word {
			t.Errorf("level %v reads %q, want %q", c.level, w, c.word)
		}
	}
}
