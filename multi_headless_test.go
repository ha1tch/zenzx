//go:build headless

package zenzx

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestShotName(t *testing.T) {
	if got := shotName("zenzx", 0, 1, 100); got != "zenzx-frame000100.png" {
		t.Errorf("one machine: %s; the name must stay what it was", got)
	}
	if got := shotName("run", 2, 4, 7); got != "run-m3-frame000007.png" {
		t.Errorf("third of four machines: %s", got)
	}
}

func TestRealPace(t *testing.T) {
	cases := []struct {
		pace   string
		frames int
		want   bool
	}{{"", 100, false}, {"", 0, true}, {"real", 100, true}, {"MAX", 0, false}, {" max ", 5, false}}
	for _, c := range cases {
		got, err := realPace(c.pace, c.frames)
		if err != nil || got != c.want {
			t.Errorf("realPace(%q, %d) = %v, %v; want %v", c.pace, c.frames, got, err, c.want)
		}
	}
	if _, err := realPace("fast", 0); err == nil {
		t.Error("-pace fast was accepted")
	}
}

func TestFrameClockKeepsFiftyAFrame(t *testing.T) {
	var c frameClock
	start := time.Now()
	for i := 0; i < 5; i++ {
		c.wait()
	}
	if d := time.Since(start); d < 90*time.Millisecond || d > 400*time.Millisecond {
		t.Errorf("five frames took %v, want about 100ms", d)
	}
}

func TestCaptureMachinesWritesOneEach(t *testing.T) {
	a, err := newMachine("48k", "./rom")
	if err != nil {
		t.Fatal(err)
	}
	b, err := newMachine("128k", "./rom")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newMachine("c64", "./rom"); err == nil {
		t.Error("a machine of an unknown model was made")
	}
	for f := 0; f < 5; f++ {
		a.RunFrame()
		b.RunFrame()
	}
	dir := t.TempDir()
	if n := captureMachines([]*ZenZX{a, b}, dir, "t", 5, true); n != 2 {
		t.Fatalf("%d screenshots written, want 2", n)
	}
	for _, name := range []string{"t-m1-frame000005.png", "t-m2-frame000005.png"} {
		if st, err := os.Stat(filepath.Join(dir, name)); err != nil || st.Size() == 0 {
			t.Errorf("%s: %v", name, err)
		}
	}
}
