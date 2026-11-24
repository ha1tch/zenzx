package zenzx

import (
	"image"
	"testing"
)

// The software "U" layer both front ends take when a Next shows a Timex mode or the
// tilemap, and the rule that lets a script keep its keys in the GUI.

func TestNextSoftwareULAOnlyWhenNeeded(t *testing.T) {
	io, _, _ := newTilemapTestIO(t)
	calls := 0
	standard := func() *image.RGBA {
		calls++
		return image.NewRGBA(image.Rect(0, 0, ScreenWidth, ScreenHeight))
	}

	// The standard screen: the GUI keeps its own drawing, and nothing is decoded for it.
	if img := io.nextSoftwareULA(standard, nil); img != nil || calls != 0 {
		t.Fatalf("standard screen: picture %v after %d decodes, want none", img != nil, calls)
	}

	// The tilemap over the standard screen: the standard picture is decoded once.
	io.nextRegWriteDirect(0x6B, 0x80)
	img := io.nextSoftwareULA(standard, nil)
	if img == nil || img.Rect.Dx() != ScreenWidth || calls != 1 {
		t.Fatalf("tilemap on: picture %v, %d decodes; want a 256 pixel picture from one decode", img, calls)
	}
	// 80 columns: twice as wide.
	io.nextRegWriteDirect(0x6B, 0xC0)
	if img = io.nextSoftwareULA(standard, nil); img == nil || img.Rect.Dx() != 2*ScreenWidth {
		t.Fatalf("80 column tilemap: picture %v, want 512 pixels wide", img)
	}

	// A Timex mode without the tilemap: the Timex decode replaces the standard one.
	io.nextRegWriteDirect(0x6B, 0x00)
	calls = 0
	io.WritePort(0x00FF, 0x06) // hi-res
	if img = io.nextSoftwareULA(standard, nil); img == nil || img.Rect.Dx() != 512 || calls != 0 {
		t.Fatalf("hi-res: picture %v after %d standard decodes, want 512 pixels and none", img, calls)
	}
}

func TestNextSoftwareULAIsNilOffTheNext(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.Enable128K()
	io := NewSpectrumIO(mem, nil)
	io.nextRegs.regs[0x6B] = 0x80 // a register value that means nothing on a 128K
	if io.nextSoftwareULA(func() *image.RGBA { t.Fatal("decoded"); return nil }, nil) != nil {
		t.Fatal("a 128K got a software ULA layer")
	}
}

func TestSchedulerOwnsKeyboardWhileKeysAreDown(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	s := NewScheduler(&Script{}, ScheduleConfig{})
	if s.OwnsKeyboard() {
		t.Fatal("an idle script owns the keyboard")
	}
	if err := s.doPress(zx, Action{Args: []string{"space"}}); err != nil {
		t.Fatal(err)
	}
	if !s.OwnsKeyboard() {
		t.Fatal("a held key does not give the script the keyboard")
	}
	if err := s.doRelease(zx, Action{Args: []string{"space"}}); err != nil {
		t.Fatal(err)
	}
	if s.OwnsKeyboard() {
		t.Fatal("the script kept the keyboard after releasing its key")
	}
	// A chord, released with "all".
	s.doPress(zx, Action{Args: []string{"edit"}})
	s.doRelease(zx, Action{Args: []string{"all"}})
	if s.OwnsKeyboard() {
		t.Fatal("release all left the script owning the keyboard")
	}
	// Typing owns it until the queue is drained.
	if err := s.doKey(zx, Action{Args: []string{"enter"}}); err != nil {
		t.Fatal(err)
	}
	if !s.OwnsKeyboard() {
		t.Fatal("a queued key does not give the script the keyboard")
	}
	for i := 0; i < 200 && s.OwnsKeyboard(); i++ {
		s.Tick(zx)
	}
	if s.OwnsKeyboard() {
		t.Fatal("the script still owns the keyboard long after its key was typed")
	}
}
