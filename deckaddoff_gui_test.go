//go:build !headless

package zenzx

import (
	"strings"
	"testing"
)

// New on the dock, with adding machines off in Preferences, adds nothing and says so in a toast
// that says where to turn it on. (Adding one when it is on makes a texture, which needs the GL
// context: the GL guards do that.)
func TestDockNewWhileAddingMachinesIsOffSaysSo(t *testing.T) {
	d := testDeck(t, 1)
	var said []string
	front := d.machines[0].zx
	front.OnNotice = func(s string) { said = append(said, s) }
	d.multiOn = func() bool { return false }
	d.choose(len(d.machines), dockLayout{})
	if len(d.machines) != 1 {
		t.Errorf("a machine was added with adding machines off: %d", len(d.machines))
	}
	if len(said) != 1 || !strings.Contains(said[0], "Adding machines is off") || !strings.Contains(said[0], "Preferences") {
		t.Errorf("the toast is %q, want one that says adding machines is off and where to turn it on", said)
	}
	// asked again, it says so again
	d.choose(len(d.machines), dockLayout{})
	if len(said) != 2 || len(d.machines) != 1 {
		t.Errorf("asked twice: %d toasts, %d machines", len(said), len(d.machines))
	}
}
