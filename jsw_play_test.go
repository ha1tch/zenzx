package zenzx

import (
	"bytes"
	"os"
	"testing"
)

// Jet Set Willy is played, not only loaded: its title screen runs, ENTER starts the game, and the
// keys that are held afterwards are the ones the game reads.
//
// The snapshot is a commercial game and is not distributed with ZenZX: it is read from the path
// in ZENZX_JSW_SNAPSHOT, or from testdata/snapshots, and the test is skipped when neither is
// there. Nothing of the game is kept by the test, only comparisons between screens it has drawn.
//
// The input is checked by comparing two runs that differ in nothing else: the emulator is
// deterministic, so two machines started from the same snapshot, given the same keys, and run
// the same number of frames, draw the same screen; hold a key on one of them and the screens
// differ only if the game read the key.

// jswSnapshot is where the game's snapshot is, or "" when it is not there.
func jswSnapshot() string {
	for _, p := range []string{os.Getenv("ZENZX_JSW_SNAPSHOT"), "testdata/snapshots/jetsetwilly_v1_48k.z80"} {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// jswScreen is the display file and attributes of the machine as they are now.
func jswScreen(zx *ZenZX) []byte {
	b := make([]byte, 0, 0x1B00)
	for a := uint16(0x4000); a < 0x5B00; a++ {
		b = append(b, zx.memory.Read(a))
	}
	return b
}

// jswMachine is a 48K machine with the game loaded and running its title screen.
func jswMachine(t *testing.T, path string) *ZenZX {
	t.Helper()
	zx, err := newMachine("48k", "./rom")
	if err != nil {
		t.Skipf("48k: %v", err)
	}
	if err := zx.LoadSnapshotFormat(path, "z80"); err != nil {
		t.Fatalf("loading %s: %v", path, err)
	}
	zx.paused = false
	return zx
}

// jswStart runs the title screen, holds ENTER until the game starts (the title tune looks at the
// keyboard between its notes, so a short press can be missed, as a person's can), and runs on a
// fixed time into the first room.
func jswStart(zx *ZenZX) (title []byte) {
	resetRun(zx, 100)
	title = jswScreen(zx)
	zx.io.PressKey(6, 0) // ENTER
	for i := 0; i < 500 && bytes.Equal(jswScreen(zx), title); i++ {
		zx.RunFrame()
	}
	zx.io.ReleaseKey(6, 0)
	resetRun(zx, 100)
	return title
}

func TestJetSetWillyTitleStartsAndReadsTheKeys(t *testing.T) {
	path := jswSnapshot()
	if path == "" {
		t.Skip("the Jet Set Willy snapshot is not here (it is not distributed): set ZENZX_JSW_SNAPSHOT to its path")
	}

	// The title screen is drawn, and ENTER takes the game to its first room.
	idle := jswMachine(t, path)
	title := jswStart(idle)
	if lit := bytes.Count(title[:0x1800], []byte{0}); lit > 0x1800-400 {
		t.Fatalf("the title screen is blank: %d of 6144 display bytes are zero", lit)
	}
	room := jswScreen(idle)
	if bytes.Equal(title, room) {
		t.Fatal("the screen is the title screen after ENTER: the game did not start")
	}

	// The same run again is the same screen: the comparison below means something.
	again := jswMachine(t, path)
	jswStart(again)
	resetRun(idle, 90)
	resetRun(again, 90)
	if !bytes.Equal(jswScreen(idle), jswScreen(again)) {
		t.Fatal("two identical runs drew different screens: the emulation is not deterministic, so a held key cannot be told from chance")
	}

	// P and O are Willy's right and left. Held, each changes what is drawn from what the
	// idle machine draws, and the two differ from each other.
	drawn := map[string][]byte{"idle": jswScreen(idle)}
	for name, key := range map[string][2]uint8{"right (P)": {5, 0}, "left (O)": {5, 1}} {
		zx := jswMachine(t, path)
		jswStart(zx)
		zx.io.PressKey(key[0], key[1])
		resetRun(zx, 90)
		zx.io.ReleaseKey(key[0], key[1])
		drawn[name] = jswScreen(zx)
		if bytes.Equal(drawn[name], drawn["idle"]) {
			t.Errorf("holding %s for 90 frames drew the same screen as holding nothing: the game did not read the key", name)
		}
	}
	if bytes.Equal(drawn["right (P)"], drawn["left (O)"]) {
		t.Error("holding right and holding left drew the same screen")
	}
}
