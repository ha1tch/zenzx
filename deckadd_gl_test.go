//go:build !headless

package zenzx

import (
	"os"
	"runtime"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// TestGLDockNewAddsAMachine: New on the dock, with adding machines on in Preferences, adds a machine
// (it makes the picture of it, a texture, so it needs the GL context) and says nothing; with it off,
// it adds none and says so, then and the next time. The off half is tested without a window, in
// deckaddoff_gui_test.go.
//
// A dormant guard, G-30 (docs/KNOWN_ISSUES.md): it opens a window.
//
//	Xvfb :99 -screen 0 1280x1024x24 &
//	DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 go test -run TestGLDockNew -v .
func TestGLDockNewAddsAMachine(t *testing.T) {
	if os.Getenv("ZENZX_GL_TESTS") == "" {
		t.Skip("ZENZX_GL_TESTS is not set: this test opens a window (dormant guard G-30)")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogNone)
	rl.InitWindow(640, 480, "ZenZX dock")
	defer rl.CloseWindow()

	d := testDeck(t, 1)
	var said []string
	d.machines[0].zx.OnNotice = func(s string) { said = append(said, s) }
	d.multiOn = func() bool { return true }
	d.choose(len(d.machines), dockLayout{tiles: make([]rl.Rectangle, 2)})
	if len(d.machines) != 2 || len(said) != 0 {
		t.Errorf("adding machines on: %d machines, said %q", len(d.machines), said)
	}
	if !d.flying {
		t.Errorf("the new machine is not flying to the front")
	}
	d.multiOn = func() bool { return false }
	d.choose(len(d.machines), dockLayout{tiles: make([]rl.Rectangle, 3)})
	if len(d.machines) != 2 || len(said) != 1 {
		t.Errorf("adding machines off: %d machines, %d toasts, want 2 and 1", len(d.machines), len(said))
	}
}
