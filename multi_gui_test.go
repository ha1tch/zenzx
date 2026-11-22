//go:build !headless

package zenzx

import "testing"

func TestStartMachinesReturnsTheListedOrder(t *testing.T) {
	// The primary machine is the third of four; the deck starts with it alone.
	d := testDeck(t, 1)
	primaryZX := d.machines[0].zx
	d.machines[0].model = "plus3"
	models := []string{"48k", "128k", "plus3", "48k"}
	all := d.startMachines(models, 2)
	if len(all) != 4 || len(d.machines) != 4 {
		t.Fatalf("%d machines returned, %d in the deck; want 4 and 4", len(all), len(d.machines))
	}
	if all[2] != primaryZX {
		t.Error("the primary machine is not third in the list returned")
	}
	if d.machines[0].zx != primaryZX || d.front != 0 {
		t.Error("the deck should still have the primary machine first and in front")
	}
	seen := map[*ZenZX]bool{}
	for i, zx := range all {
		if zx == nil || seen[zx] {
			t.Fatalf("machine %d of the list is missing or repeated", i+1)
		}
		seen[zx] = true
	}
	// The others, in the deck, in the order they were added: 48k, 128k, 48k.
	for i, want := range []string{"48k", "128k", "48k"} {
		if got := d.machines[i+1].model; got != want {
			t.Errorf("deck place %d holds a %s, want %s", i+2, got, want)
		}
	}
	if all[0] != d.machines[1].zx || all[1] != d.machines[2].zx || all[3] != d.machines[3].zx {
		t.Error("the list returned does not follow the order the models were given in")
	}
}

// A row of machines too long for the window shrinks its pictures until the
// whole dock is inside it: twelve machines in a window at 3x, as -multi
// 12,128k gives.
func TestDockFitsTwelveMachines(t *testing.T) {
	d := testDeck(t, 12)
	d.progress = 1
	for _, w := range []int{960, 640, 1280} {
		l := d.layout(w, 768, 8)
		if l.panel.X < -0.01 || l.panel.X+l.panel.Width > float32(w)+0.01 {
			t.Errorf("in a window %d wide the dock runs from %.1f to %.1f", w, l.panel.X, l.panel.X+l.panel.Width)
		}
		if len(l.tiles) != 13 {
			t.Fatalf("%d tiles, want 12 machines and the New tile", len(l.tiles))
		}
		first, last := l.tiles[0], l.tiles[12]
		if first.X < l.panel.X-0.01 || last.X+last.Width > l.panel.X+l.panel.Width+0.01 {
			t.Errorf("in a window %d wide the tiles run from %.1f to %.1f, outside the panel", w, first.X, last.X+last.Width)
		}
		if w == 960 && l.panel.Width < 900 {
			t.Errorf("the dock is %.1f wide in a window of 960: it shrank more than it had to", l.panel.Width)
		}
	}
}
