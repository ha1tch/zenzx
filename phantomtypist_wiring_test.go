// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"strings"
	"testing"
)

// What the typist leaves alone, how it ends, and how the host paces it.

func typistBasicTape(t *testing.T) []byte {
	t.Helper()
	image, err := BasicTape("hello", "10 POKE 32768,171\n")
	if err != nil {
		t.Fatal(err)
	}
	return image
}

// A Next, a machine with a ROM of its own, a machine with no tape and one with an empty deck are
// refused, and a refusal changes nothing: no reset, no typing, the tape where it was.
func TestAutoloadRefusedLeavesTheMachineAlone(t *testing.T) {
	image := typistBasicTape(t)

	refuse := func(name string, zx *ZenZX) {
		t.Helper()
		zx.memory.Write(0x8000, 0x5A) // survives a refusal, not a reset
		if zx.TapeAutoloads() {
			t.Errorf("%s: TapeAutoloads says it would", name)
		}
		if zx.AutoloadTape() {
			t.Errorf("%s: AutoloadTape started", name)
		}
		if zx.TypistActive() {
			t.Errorf("%s: the typist is active after a refusal", name)
		}
		if got := zx.memory.Read(0x8000); got != 0x5A {
			t.Errorf("%s: a refusal reset the machine (0x8000 is %02X)", name, got)
		}
	}

	custom := typistMachine(t, "48k")
	insertTape(t, custom, image)
	custom.customROM = true
	refuse("custom ROM", custom)
	if custom.tape.st.Position != 0 || !custom.tape.st.Loaded {
		t.Error("custom ROM: the tape was disturbed")
	}

	next := typistMachine(t, "48k")
	insertTape(t, next, image)
	next.memory.isNext = true
	refuse("Next", next)

	empty := typistMachine(t, "48k")
	refuse("empty deck", empty)

	notape := typistMachine(t, "48k")
	notape.tape = nil
	refuse("no tape deck", notape)

	stopped := typistMachine(t, "48k")
	insertTape(t, stopped, image)
	stopped.stopped = true
	refuse("stopped machine", stopped)
}

// A reset gives the typing up and lets the keys go.
func TestResetCancelsTheTypist(t *testing.T) {
	zx := typistMachine(t, "48k")
	for i := 0; i < 100; i++ {
		zx.RunFrame()
	}
	insertTape(t, zx, typistBasicTape(t))
	if !zx.AutoloadTape() {
		t.Fatal("no autoload")
	}
	for i := 0; i < 120 && zx.io.typistKeys == [8]uint8{}; i++ {
		zx.RunFrame()
	}
	if zx.io.typistKeys == [8]uint8{} {
		t.Fatal("the typist never pressed a key")
	}
	zx.Reset()
	if zx.TypistActive() {
		t.Error("the typist survived a reset")
	}
	if zx.io.typistKeys != [8]uint8{} {
		t.Errorf("a reset left typist keys down: %v", zx.io.typistKeys)
	}
}

// A key of the person's takes the keyboard from the typist; with none down it carries on.
func TestAPersonsKeyCancelsTheTypist(t *testing.T) {
	zx := typistMachine(t, "48k")
	for i := 0; i < 100; i++ {
		zx.RunFrame()
	}
	insertTape(t, zx, typistBasicTape(t))
	if !zx.AutoloadTape() {
		t.Fatal("no autoload")
	}
	zx.cancelTypistOnKey()
	if !zx.TypistActive() {
		t.Fatal("the typist stopped though no key was down")
	}
	zx.io.PressKey(1, 0) // the A key, held by a person
	zx.cancelTypistOnKey()
	if zx.TypistActive() {
		t.Error("the typist carried on over a key of the person's")
	}
	if zx.io.typistKeys != [8]uint8{} {
		t.Errorf("the typist's keys were left down: %v", zx.io.typistKeys)
	}
	if zx.tape.st.Playing {
		t.Error("the tape was played though the typing was given up")
	}
}

// In Accurate mode the tape stays stopped while the typing goes on, plays after the last ENTER, and
// the program comes in through the pulses; when it is over the typist has let every key go.
func TestTypistPlaysTheTapeOnlyAfterTheLastEnter(t *testing.T) {
	zx := typistMachine(t, "48k")
	zx.tape.SetMode(TapeAccurate)
	for i := 0; i < 100; i++ {
		zx.RunFrame()
	}
	insertTape(t, zx, typistBasicTape(t))
	if !zx.AutoloadTape() {
		t.Fatal("no autoload")
	}
	playedWhileTyping := false
	frames := -1
	for f := 0; f < 8000; f++ {
		zx.RunFrame()
		if zx.TypistActive() && zx.tape.st.Playing {
			playedWhileTyping = true
		}
		if zx.memory.Read(0x8000) == typistMark {
			frames = f
			break
		}
	}
	if frames < 0 {
		t.Fatalf("the program never loaded in Accurate mode:\n%s", screenText(zx))
	}
	if playedWhileTyping {
		t.Error("the tape played while the typist was still typing")
	}
	if zx.TypistActive() {
		t.Error("the typist is still active after the program ran")
	}
	if zx.io.typistKeys != [8]uint8{} {
		t.Errorf("the typist left keys down: %v", zx.io.typistKeys)
	}
	t.Logf("Accurate mode: the program ran after %d frames", frames)
}

// With a run-ahead the host's frames do more than one machine frame each while the typist types,
// so the program is in sooner by the host's count; with none the clock is exact.
func TestTypistRunsAheadOfTheHostClock(t *testing.T) {
	hostFrames := func(runAhead int) int {
		zx := typistMachine(t, "48k")
		for i := 0; i < 100; i++ {
			zx.RunFrame()
		}
		insertTape(t, zx, typistBasicTape(t))
		zx.typistRunAhead = runAhead
		if !zx.AutoloadTape() {
			t.Fatal("no autoload")
		}
		for f := 0; f < 5000; f++ {
			zx.RunTurboAwareFrame(false)
			if zx.memory.Read(0x8000) == typistMark {
				return f
			}
		}
		t.Fatalf("run-ahead %d: the program never ran", runAhead)
		return -1
	}
	exact, fast := hostFrames(0), hostFrames(5)
	t.Logf("host frames to the program: %d exact, %d with run-ahead", exact, fast)
	if fast*2 >= exact {
		t.Errorf("run-ahead took %d host frames against %d without: it did not accelerate", fast, exact)
	}
	// A front end's default reaches a machine that has none of its own.
	defer func(v int) { defaultTypistRunAhead = v }(defaultTypistRunAhead)
	defaultTypistRunAhead = 3
	zx := typistMachine(t, "48k")
	insertTape(t, zx, typistBasicTape(t))
	if !zx.AutoloadTape() || zx.typistRunAhead != 3 {
		t.Errorf("the default run-ahead was not given: %d", zx.typistRunAhead)
	}
}

// The toast says what happened, or what to do by hand, in the machine's own terms.
func TestTapeInsertedNote(t *testing.T) {
	image := typistBasicTape(t)
	for _, c := range []struct {
		model string
		hand  string // what must be in the note when it is not autoloaded
	}{
		{"48k", `LOAD ""`},
		{"ts2068", `LOAD ""`},
		{"spanish128k", `LOAD ""`},
		{"128k", "Tape Loader"},
		{"plus2", "Tape Loader"},
		{"plus3", "Tape Loader"},
	} {
		zx := typistMachine(t, c.model)
		insertTape(t, zx, image)
		off := zx.TapeInsertedNote("hello.tap", 2, false)
		if !strings.Contains(off, c.hand) || !strings.Contains(off, "loaded and rewound (2 blocks)") {
			t.Errorf("%s with autoload off: %q", c.model, off)
		}
		if zx.TypistActive() {
			t.Errorf("%s: the note started the typist with autoload off", c.model)
		}
		zx.paused = true
		on := zx.TapeInsertedNote("hello.tap", 1, true)
		if zx.paused {
			t.Errorf("%s: a tape that loads by itself left the machine paused", c.model)
		}
		if !strings.Contains(on, "loading by itself") || !strings.Contains(on, "(1 block)") || !zx.TypistActive() {
			t.Errorf("%s with autoload on: %q (active %v)", c.model, on, zx.TypistActive())
		}
	}

	// Where nothing can be typed the note says so plainly, with no hint for a Next.
	next := typistMachine(t, "48k")
	insertTape(t, next, image)
	next.memory.isNext = true
	got := next.TapeInsertedNote("hello.tap", 2, true)
	if got != "The tape hello.tap is loaded and rewound (2 blocks)" {
		t.Errorf("a Next: %q", got)
	}
	custom := typistMachine(t, "48k")
	insertTape(t, custom, image)
	custom.customROM = true
	if got := custom.TapeInsertedNote("hello.tap", 2, true); !strings.Contains(got, "ROM's own command") || custom.TypistActive() {
		t.Errorf("a custom ROM: %q", got)
	}
}
