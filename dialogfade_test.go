package zenzx

import (
	"testing"
	"time"
)

func TestDialogFadeStartsWhole(t *testing.T) {
	if a, s, done := dialogFadeAt(0); a != 1 || s != 1 || done {
		t.Fatalf("at the start: alpha %v scale %v done %v", a, s, done)
	}
	if a, s, done := dialogFadeAt(-time.Second); a != 1 || s != 1 || done {
		t.Fatalf("before the start: alpha %v scale %v done %v", a, s, done)
	}
}

func TestDialogFadeEndsGone(t *testing.T) {
	for _, e := range []time.Duration{dialogFadeTime, dialogFadeTime + time.Millisecond, time.Hour} {
		if a, s, done := dialogFadeAt(e); a != 0 || s != dialogFadeEnd || !done {
			t.Errorf("at %v: alpha %v scale %v done %v, want 0, %v and done", e, a, s, done, dialogFadeEnd)
		}
	}
}

// Every step is fainter and smaller than the one before, and stays between the two ends.
func TestDialogFadeOnlyGoesDown(t *testing.T) {
	pa, ps := float32(1), float32(1)
	for e := time.Duration(0); e < dialogFadeTime; e += time.Millisecond {
		a, s, done := dialogFadeAt(e)
		if done {
			t.Fatalf("done at %v, before %v", e, dialogFadeTime)
		}
		if a > pa || s > ps || a <= 0 || a > 1 || s < dialogFadeEnd || s > 1 {
			t.Fatalf("at %v: alpha %v (was %v), scale %v (was %v)", e, a, pa, s, ps)
		}
		pa, ps = a, s
	}
}

// It is short enough that no one waits for it, and long enough to be a fade and not a flicker.
func TestDialogFadeTimeIsASensibleLength(t *testing.T) {
	if dialogFadeTime < 80*time.Millisecond || dialogFadeTime > 300*time.Millisecond {
		t.Errorf("dialogFadeTime = %v", dialogFadeTime)
	}
}
