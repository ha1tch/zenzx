package main

import (
	"bufio"
	"strings"
	"testing"
)

func parse(t *testing.T, src string) (*Script, error) {
	t.Helper()
	return parseScript(bufio.NewScanner(strings.NewReader(src)), "test.zen")
}

func TestParseOffsetUnits(t *testing.T) {
	cases := []struct {
		tok  string
		want int
	}{
		{"0", 0},
		{"100", 100},
		{"100f", 100},
		{"2s", 100},
		{"500ms", 25},
		{"1s", 50},
		{"20ms", 1},
		{"10ms", 1},  // 0.5 frame rounds to 1
		{"9ms", 0},   // 0.45 frame rounds to 0
		{"1.5s", 75}, // fractional seconds
	}
	for _, c := range cases {
		got, err := parseOffset(c.tok)
		if err != nil {
			t.Errorf("parseOffset(%q) unexpected error: %v", c.tok, err)
			continue
		}
		if got != c.want {
			t.Errorf("parseOffset(%q) = %d, want %d", c.tok, got, c.want)
		}
	}
}

func TestParseOffsetErrors(t *testing.T) {
	for _, tok := range []string{"-5", "abc", "", "12x", "-1s"} {
		if _, err := parseOffset(tok); err == nil {
			t.Errorf("parseOffset(%q) expected error, got nil", tok)
		}
	}
}

func TestParseCommentsAndBlanks(t *testing.T) {
	src := `
# leading comment
   # indented comment

0   shot
   # another comment
50  reset
`
	s, err := parse(t, src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s.Actions) != 2 {
		t.Fatalf("got %d actions, want 2", len(s.Actions))
	}
	if s.Actions[0].Verb != "shot" || s.Actions[1].Verb != "reset" {
		t.Errorf("verbs = %q, %q; want shot, reset", s.Actions[0].Verb, s.Actions[1].Verb)
	}
}

func TestParseArgs(t *testing.T) {
	s, err := parse(t, "10 snapshot game.z80 auto")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a := s.Actions[0]
	if a.Verb != "snapshot" || len(a.Args) != 2 || a.Args[0] != "game.z80" || a.Args[1] != "auto" {
		t.Errorf("got verb=%q args=%v", a.Verb, a.Args)
	}
	if a.Line != 1 {
		t.Errorf("line = %d, want 1", a.Line)
	}
}

func TestParseSortStableByFrame(t *testing.T) {
	// Out-of-order input with a same-frame pair; expect ascending frame and
	// preserved source order within the equal-frame group.
	src := `
200 reset
50  shot first
50  shot second
0   reset
`
	s, err := parse(t, src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantFrames := []int{0, 50, 50, 200}
	if len(s.Actions) != len(wantFrames) {
		t.Fatalf("got %d actions, want %d", len(s.Actions), len(wantFrames))
	}
	for i, wf := range wantFrames {
		if s.Actions[i].Frame != wf {
			t.Errorf("action[%d].Frame = %d, want %d", i, s.Actions[i].Frame, wf)
		}
	}
	// Same-frame order preserved: "first" before "second".
	if s.Actions[1].Args[0] != "first" || s.Actions[2].Args[0] != "second" {
		t.Errorf("same-frame order not preserved: %v, %v", s.Actions[1].Args, s.Actions[2].Args)
	}
}

func TestParseUnknownVerbRejected(t *testing.T) {
	_, err := parse(t, "0 frobnicate")
	if err == nil {
		t.Fatal("expected error for unknown verb, got nil")
	}
	if !strings.Contains(err.Error(), "unknown verb") {
		t.Errorf("error = %q, want it to mention unknown verb", err)
	}
}

func TestParsePhase2VerbsKnown(t *testing.T) {
	// Phase 2 keyboard verbs must parse (known) even though they are not yet
	// wired for execution.
	for _, v := range []string{"key", "type", "press", "release"} {
		if _, err := parse(t, "0 "+v+" x"); err != nil {
			t.Errorf("verb %q should parse as known, got error: %v", v, err)
		}
	}
}

func TestParseMalformedLine(t *testing.T) {
	// A line with an offset but no verb is malformed.
	if _, err := parse(t, "100"); err == nil {
		t.Error("expected error for offset-only line, got nil")
	}
}

func TestParseVerbCaseInsensitive(t *testing.T) {
	s, err := parse(t, "0 SHOT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Actions[0].Verb != "shot" {
		t.Errorf("verb = %q, want canonical lower-case shot", s.Actions[0].Verb)
	}
}

func TestParseWaitBootLoneToken(t *testing.T) {
	s, err := parse(t, "wait-boot")
	if err != nil {
		t.Fatalf("wait-boot as lone token should parse, got: %v", err)
	}
	if len(s.Actions) != 1 || s.Actions[0].Verb != "wait-boot" {
		t.Fatalf("got %d actions, first verb %q", len(s.Actions), s.Actions[0].Verb)
	}
	if s.Actions[0].Frame != 0 {
		t.Errorf("wait-boot frame = %d, want 0", s.Actions[0].Frame)
	}
}

func TestParseWaitBootIsSortBarrier(t *testing.T) {
	// Offsets after wait-boot are boot-relative; the sort must not pull a
	// large pre-barrier absolute offset past the barrier, nor reorder across
	// it. Source order across the barrier must be preserved.
	src := `
200 shot pre-late
50  shot pre-early
wait-boot
100 shot post-late
10  shot post-early
`
	s, err := parse(t, src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantVerbsArgs := []struct {
		verb string
		arg  string
	}{
		{"shot", "pre-early"}, // 50 sorted before 200 within pre-partition
		{"shot", "pre-late"},  // 200
		{"wait-boot", ""},     // barrier stays in place
		{"shot", "post-early"}, // 10 sorted before 100 within post-partition
		{"shot", "post-late"},  // 100
	}
	if len(s.Actions) != len(wantVerbsArgs) {
		t.Fatalf("got %d actions, want %d", len(s.Actions), len(wantVerbsArgs))
	}
	for i, w := range wantVerbsArgs {
		a := s.Actions[i]
		if a.Verb != w.verb {
			t.Errorf("action[%d] verb = %q, want %q", i, a.Verb, w.verb)
		}
		if w.arg != "" && (len(a.Args) == 0 || a.Args[0] != w.arg) {
			t.Errorf("action[%d] arg = %v, want %q", i, a.Args, w.arg)
		}
	}
}

func TestParseMultipleWaitBoot(t *testing.T) {
	// Two barriers create three partitions, each independently sorted.
	src := `
wait-boot
30 shot a
10 shot b
wait-boot
40 shot c
20 shot d
`
	s, err := parse(t, src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantOrder := []string{"wait-boot", "b", "a", "wait-boot", "d", "c"}
	if len(s.Actions) != len(wantOrder) {
		t.Fatalf("got %d actions, want %d", len(s.Actions), len(wantOrder))
	}
	for i, w := range wantOrder {
		a := s.Actions[i]
		got := a.Verb
		if a.Verb == "shot" {
			got = a.Args[0]
		}
		if got != w {
			t.Errorf("action[%d] = %q, want %q", i, got, w)
		}
	}
}
