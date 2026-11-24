// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"math"
	"testing"
)

// Every new tape is a cassette of another colour; the same tape stays the colour it is.
func TestDeckCassetteChangesWithEveryNewTape(t *testing.T) {
	d := tapeDeck{place: tapeDeckTop}
	tp := deckTape([]int{100, 100}, "One")
	first := d.Update(tp, 0.01).Variant
	for i := 0; i < 20; i++ {
		if v := d.Update(tp, 0.01).Variant; v != first {
			t.Fatalf("the cassette changed colour with no new tape: %d -> %d", first, v)
		}
	}
	prev := first
	for i := 0; i < 200; i++ {
		tp.st.Filename = "/tapes/next" + deckItoa(i) + ".tap"
		v := d.Update(tp, 0.01).Variant
		if v == prev {
			t.Fatalf("tape %d got the colour of the one before (%d)", i, v)
		}
		if v < 0 || v >= cassetteVariants {
			t.Fatalf("colour %d out of range", v)
		}
		prev = v
	}
}

func TestDeckCassetteUsesEveryColour(t *testing.T) {
	d := tapeDeck{place: tapeDeckBottom}
	tp := deckTape([]int{100}, "X")
	seen := map[int]bool{}
	for i := 0; i < 500; i++ {
		tp.st.Filename = "t" + deckItoa(i)
		seen[d.Update(tp, 0.01).Variant] = true
	}
	if len(seen) != cassetteVariants {
		t.Errorf("only %d of %d colours turned up in 500 tapes", len(seen), cassetteVariants)
	}
}

func TestDeckCassettePickIsInjectable(t *testing.T) {
	var calls [][2]int
	d := tapeDeck{place: tapeDeckTop, pick: func(n, prev int) int {
		calls = append(calls, [2]int{n, prev})
		return (prev + 5 + n) % n
	}}
	tp := deckTape([]int{100}, "X")
	d.Update(tp, 0.01)
	tp.st.Filename = "other"
	d.Update(tp, 0.01)
	d.Update(tp, 0.01)
	// The first tape is asked for with no colour before it, the second with the first's (5).
	if len(calls) != 2 || calls[0] != [2]int{cassetteVariants, -1} || calls[1] != [2]int{cassetteVariants, 4} {
		t.Errorf("pick calls %v", calls)
	}
}

func TestNextCassette(t *testing.T) {
	for prev := -1; prev < cassetteVariants; prev++ {
		for i := 0; i < 50; i++ {
			v := nextCassette(cassetteVariants, prev)
			if v < 0 || v >= cassetteVariants || v == prev {
				t.Fatalf("nextCassette(%d) = %d", prev, v)
			}
		}
	}
	if nextCassette(1, 0) != 0 || nextCassette(0, 0) != 0 {
		t.Error("a deck with one colour must keep it")
	}
}

func TestDeckReelsTurnOnlyWhilePlaying(t *testing.T) {
	d := tapeDeck{place: tapeDeckTop}
	tp := deckTape([]int{1_000_000, 1_000_000}, "X")
	if v := d.Update(tp, 0.1); v.Spin != 0 {
		t.Errorf("stopped reels turned: %v", v.Spin)
	}
	tp.st.Playing = true
	v := d.Update(tp, 0.1)
	if math.Abs(v.Spin-0.1*cassetteSpinRate) > 1e-9 {
		t.Errorf("spin %v after 0.1 s", v.Spin)
	}
	tp.st.Playing, tp.st.Paused = false, true
	if v2 := d.Update(tp, 0.1); v2.Spin != v.Spin {
		t.Error("held reels turned")
	}
	tp.st.Playing, tp.st.Paused = true, false
	for i := 0; i < 1000; i++ {
		if v = d.Update(tp, 0.1); v.Spin < 0 || v.Spin >= 2*math.Pi {
			t.Fatalf("spin %v out of one turn", v.Spin)
		}
	}
	tp.st.Position = 2
	before := d.spin
	if d.Update(tp, 0.1); d.spin != before {
		t.Error("the reels turn at the end of the tape")
	}
}
