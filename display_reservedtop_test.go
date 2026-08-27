//go:build !headless

package main

import "testing"

func TestSetReservedTopHeightAddsToTargetHeight(t *testing.T) {
	zx := testZX()
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}
	dm := zx.display

	baseTarget := dm.targetHeight

	dm.SetReservedTopHeight(24)
	if dm.targetHeight != baseTarget+24 {
		t.Errorf("targetHeight = %d, want %d (base + 24)", dm.targetHeight, baseTarget+24)
	}
	if !dm.isAnimating {
		t.Error("SetReservedTopHeight should trigger the animation (isAnimating=true)")
	}
}

func TestSetReservedTopHeightNoopWhenUnchanged(t *testing.T) {
	zx := testZX()
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}
	dm := zx.display

	dm.SetReservedTopHeight(0) // already 0
	if dm.isAnimating {
		t.Error("setting reservedTopHeight to its own current value should not trigger an animation")
	}
}

func TestSetReservedTopHeightBackToZeroRestoresTarget(t *testing.T) {
	zx := testZX()
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}
	dm := zx.display

	baseTarget := dm.targetHeight
	dm.SetReservedTopHeight(24)
	dm.SetReservedTopHeight(0)

	if dm.targetHeight != baseTarget {
		t.Errorf("targetHeight = %d after reserving then releasing, want back to %d", dm.targetHeight, baseTarget)
	}
}

func TestSetScaleValidValue(t *testing.T) {
	zx := testZX()
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}
	dm := zx.display

	if !dm.SetScale(2) {
		t.Fatal("SetScale(2) = false, want true (2 should be a valid scale)")
	}
	if dm.screen.multiplier != 2 {
		t.Errorf("multiplier = %d, want 2", dm.screen.multiplier)
	}
}

func TestSetScaleInvalidValueRejected(t *testing.T) {
	zx := testZX()
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}
	dm := zx.display

	if dm.SetScale(0) {
		t.Error("SetScale(0) = true, want false (0 is out of range)")
	}
	if dm.SetScale(-1) {
		t.Error("SetScale(-1) = true, want false (negative is out of range)")
	}
}

func TestSetScaleSameValueIsNoopButReturnsTrue(t *testing.T) {
	zx := testZX()
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}
	dm := zx.display
	dm.SetScale(2)

	dm.isAnimating = false // reset to detect whether SetScale triggers a fresh one

	if !dm.SetScale(2) {
		t.Error("SetScale to the already-current value should still return true")
	}
	if dm.isAnimating {
		t.Error("SetScale to the already-current value should not trigger a new animation")
	}
}
