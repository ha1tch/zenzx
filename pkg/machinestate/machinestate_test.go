// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package machinestate

import (
	"bytes"
	"errors"
	"os"
	"reflect"
	"testing"
)

type leaf struct {
	n    int
	name string
}

type part struct {
	hidden  uint16 // unexported
	Shown   int8
	flags   [4]bool
	mem     [64]byte
	big     [3][5]byte
	slice   []byte
	nilS    []byte
	emptyS  []byte
	ints    []int32
	floats  [2]float64
	table   map[string]int
	leafs   []leaf
	shared  *leaf
	alias   *leaf // the same object as shared
	back    *machine
	extra   *leaf // nil in the machine, set in the saved state
	cb      func() int
	file    *os.File
	iface   any
	skipped int
}

type machine struct {
	a, b  part
	count uint64
	next  *part
}

func policy() *Policy {
	return &Policy{
		SkipFields: map[string]bool{"machinestate.part.skipped": true},
		SkipTypes:  []reflect.Type{reflect.TypeOf(os.File{})},
	}
}

func build() *machine {
	m := &machine{count: 1 << 40}
	m.a = part{hidden: 0xBEEF, Shown: -5, flags: [4]bool{true, false, true, false}, slice: []byte{1, 2, 3},
		nilS: nil, emptyS: []byte{}, ints: []int32{-1, 2, -3}, floats: [2]float64{1.5, -2.25},
		table: map[string]int{"x": 1, "y": -2, "a": 7}, leafs: []leaf{{1, "one"}, {2, "two"}}, skipped: 99}
	for i := range m.a.mem {
		m.a.mem[i] = byte(i * 3)
	}
	m.a.big[2][4] = 0x77
	m.a.shared = &leaf{42, "shared"}
	m.a.alias = m.a.shared
	m.a.back = m
	m.a.extra = &leaf{7, "extra"}
	m.next = &m.b
	m.b.Shown = 9
	return m
}

func encode(t *testing.T, p *Policy, roots ...any) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := p.Encode(&b, roots...); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestRoundTripPutsEveryFieldBack(t *testing.T) {
	p := policy()
	src := build()
	data := encode(t, p, src)

	dst := &machine{}
	dst.a.cb = func() int { return 11 }
	dst.a.skipped = 5
	dst.next = &dst.b
	keep := dst.next
	if err := p.Decode(bytes.NewReader(data), dst); err != nil {
		t.Fatal(err)
	}
	a := &dst.a
	if a.hidden != 0xBEEF || a.Shown != -5 || dst.count != 1<<40 {
		t.Fatalf("scalars: %#x %d %d", a.hidden, a.Shown, dst.count)
	}
	if a.flags != src.a.flags || a.mem != src.a.mem || a.big != src.a.big || a.floats != src.a.floats {
		t.Fatal("arrays differ")
	}
	if !bytes.Equal(a.slice, []byte{1, 2, 3}) || a.nilS != nil || a.emptyS == nil || len(a.emptyS) != 0 {
		t.Fatalf("slices: %v %v %v", a.slice, a.nilS, a.emptyS)
	}
	if !reflect.DeepEqual(a.ints, src.a.ints) || !reflect.DeepEqual(a.table, src.a.table) || !reflect.DeepEqual(a.leafs, src.a.leafs) {
		t.Fatal("ints, map or struct slice differ")
	}
	if a.shared == nil || *a.shared != (leaf{42, "shared"}) {
		t.Fatalf("shared: %+v", a.shared)
	}
	if a.alias != a.shared {
		t.Fatal("two pointers to one object came back as two objects")
	}
	if a.back != dst {
		t.Fatal("the pointer back to the machine is not the machine")
	}
	if a.extra == nil || a.extra.n != 7 {
		t.Fatal("a pointer nil in the machine was not given its object")
	}
	if dst.next != keep || dst.b.Shown != 9 {
		t.Fatal("an object the machine already had was replaced, not filled")
	}
	if a.cb == nil || a.cb() != 11 {
		t.Fatal("a callback was disturbed")
	}
	if a.skipped != 5 {
		t.Fatalf("a skipped field was written: %d", a.skipped)
	}
}

func TestSavingTwiceGivesTheSameBytes(t *testing.T) {
	p := policy()
	a := encode(t, p, build())
	b := encode(t, p, build())
	if !bytes.Equal(a, b) {
		t.Fatal("the same state saved twice differs (a map's order, say)")
	}
}

func TestStateSavedThenRestoredAndSavedAgainIsIdentical(t *testing.T) {
	p := policy()
	first := encode(t, p, build())
	m := &machine{}
	if err := p.Decode(bytes.NewReader(first), m); err != nil {
		t.Fatal(err)
	}
	if again := encode(t, p, m); !bytes.Equal(first, again) {
		t.Fatal("a restored machine does not save as the machine it was saved from")
	}
}

func TestAnotherLayoutIsRefused(t *testing.T) {
	p := policy()
	data := encode(t, p, build())
	type other struct{ x int }
	if err := p.Decode(bytes.NewReader(data), &other{}); !errors.Is(err, ErrLayout) {
		t.Fatalf("got %v, want ErrLayout", err)
	}
	// A skipped field is not part of the layout, a new one is.
	q := &Policy{SkipTypes: []reflect.Type{reflect.TypeOf(os.File{})}}
	if err := q.Decode(bytes.NewReader(data), &machine{}); !errors.Is(err, ErrLayout) {
		t.Fatalf("a policy that skips less: got %v, want ErrLayout", err)
	}
}

func TestNotSavedStateAndTruncatedStateAreRefused(t *testing.T) {
	p := policy()
	if err := p.Decode(bytes.NewReader([]byte("hello")), &machine{}); !errors.Is(err, ErrFormat) {
		t.Fatalf("got %v, want ErrFormat", err)
	}
	data := encode(t, p, build())
	for _, n := range []int{len(magic) + 5, len(magic) + 40, len(data) / 2, len(data) - 1} {
		if err := p.Decode(bytes.NewReader(data[:n]), &machine{}); err == nil {
			t.Fatalf("a state cut at %d of %d bytes was accepted", n, len(data))
		}
	}
}

func TestRootsMustBePointers(t *testing.T) {
	var b bytes.Buffer
	if err := policy().Encode(&b, machine{}); err == nil {
		t.Fatal("a root that is not a pointer was accepted")
	}
	if err := policy().Encode(&b, (*machine)(nil)); err == nil {
		t.Fatal("a nil root was accepted")
	}
}

func TestSeveralRootsKeepTheirPointersBetweenThem(t *testing.T) {
	type owner struct{ v *leaf }
	l := &leaf{1, "x"}
	src1, src2 := &owner{l}, &leaf{}
	src2 = l
	p := &Policy{}
	data := encode(t, p, src1, src2)
	d1 := &owner{}
	d2 := &leaf{}
	// The machine's own root 2 is what the first root's pointer must be given.
	d1.v = d2
	if err := p.Decode(bytes.NewReader(data), d1, d2); err != nil {
		t.Fatal(err)
	}
	if d1.v != d2 || d2.n != 1 || d2.name != "x" {
		t.Fatalf("got %+v %+v", d1.v, d2)
	}
	// A machine whose pointer goes elsewhere does not match the saved one.
	e1, e2 := &owner{&leaf{}}, &leaf{}
	if err := p.Decode(bytes.NewReader(data), e1, e2); err == nil {
		t.Fatal("roots that are other objects than the saved ones were accepted")
	}
}

func TestSkippedListsWhatIsLeftOut(t *testing.T) {
	got := policy().Skipped(&machine{})
	want := map[string]string{
		"machinestate.part.cb":      "kind func",
		"machinestate.part.file":    "policy type",
		"machinestate.part.iface":   "kind interface",
		"machinestate.part.skipped": "policy field",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for _, s := range got {
		if want[s.Field] != s.Reason {
			t.Fatalf("%+v unexpected", s)
		}
	}
}

func TestFingerprintFollowsTheLayout(t *testing.T) {
	p := policy()
	a := p.Fingerprint(&machine{})
	if a != p.Fingerprint(&machine{}) {
		t.Fatal("not stable")
	}
	type x struct{ a int }
	type y struct{ a int8 }
	if p.Fingerprint(&x{}) == p.Fingerprint(&y{}) {
		t.Fatal("a field's type does not show")
	}
}

func TestACycleIsSavedOnce(t *testing.T) {
	type node struct {
		next *node
		v    int
	}
	a := &node{v: 1}
	b := &node{v: 2, next: a}
	a.next = b
	p := &Policy{}
	data := encode(t, p, a)
	d := &node{}
	if err := p.Decode(bytes.NewReader(data), d); err != nil {
		t.Fatal(err)
	}
	if d.v != 1 || d.next == nil || d.next.v != 2 || d.next.next != d {
		t.Fatal("the cycle did not come back as a cycle")
	}
}
