// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package romimage

import (
	"bytes"
	"strings"
	"testing"
)

func TestBuildPlacesAtAddressOverZeroes(t *testing.T) {
	payload := []byte{1, 2, 3}
	for _, addr := range []int{0, 1, 0x1234, Size - 3} {
		img, cut, err := Build(payload, addr, Zeroes(Size))
		if err != nil || cut != 0 {
			t.Fatalf("addr 0x%X: cut %d, err %v", addr, cut, err)
		}
		if len(img) != Size {
			t.Fatalf("addr 0x%X: image is %d bytes, want %d", addr, len(img), Size)
		}
		if !bytes.Equal(img[addr:addr+3], payload) {
			t.Errorf("addr 0x%X: payload not where asked", addr)
		}
		for i, b := range img {
			if (i < addr || i >= addr+3) && b != 0 {
				t.Fatalf("addr 0x%X: byte 0x%04X is 0x%02X, want zero around the payload", addr, i, b)
			}
		}
	}
}

func TestBuildOverlayKeepsEverythingElseOfTheBase(t *testing.T) {
	base := make([]byte, Size)
	for i := range base {
		base[i] = byte(i*7 + 3)
	}
	keep := append([]byte(nil), base...)
	payload := bytes.Repeat([]byte{0xFF}, 0x20)
	img, cut, err := Build(payload, 0x38, base)
	if err != nil || cut != 0 {
		t.Fatalf("cut %d, err %v", cut, err)
	}
	if !bytes.Equal(img[:0x38], keep[:0x38]) || !bytes.Equal(img[0x58:], keep[0x58:]) {
		t.Error("the overlay changed bytes outside the payload")
	}
	if !bytes.Equal(img[0x38:0x58], payload) {
		t.Error("the payload is not in the image")
	}
	if !bytes.Equal(base, keep) {
		t.Error("Build changed the base it was given")
	}
}

func TestBuildCutsAtExactlyTheEndOfTheROM(t *testing.T) {
	payload := bytes.Repeat([]byte{0x55}, 0x300)
	img, cut, err := Build(payload, 0x3F00, Zeroes(Size))
	if err != nil {
		t.Fatal(err)
	}
	if cut != 0x200 {
		t.Errorf("cut = %d, want 512", cut)
	}
	if len(img) != Size {
		t.Fatalf("image is %d bytes, want exactly %d", len(img), Size)
	}
	if !bytes.Equal(img[0x3F00:], bytes.Repeat([]byte{0x55}, 0x100)) {
		t.Error("the kept part of the payload is wrong")
	}

	// Bigger than the whole ROM, at zero: the first 16 KiB of it.
	big := make([]byte, 51200)
	for i := range big {
		big[i] = byte(i)
	}
	img, cut, err = Build(big, 0, Zeroes(Size))
	if err != nil || cut != 51200-Size || !bytes.Equal(img, big[:Size]) {
		t.Errorf("a payload larger than the ROM: cut %d, err %v", cut, err)
	}
}

func TestBuildExactFitAndLastByteAreNotCut(t *testing.T) {
	if _, cut, err := Build(make([]byte, 0x100), 0x3F00, Zeroes(Size)); err != nil || cut != 0 {
		t.Errorf("exact fit: cut %d, err %v", cut, err)
	}
	img, cut, err := Build([]byte{0xEE}, Size-1, Zeroes(Size))
	if err != nil || cut != 0 || img[Size-1] != 0xEE {
		t.Errorf("last byte: cut %d, err %v, byte 0x%02X", cut, err, img[Size-1])
	}
	img, cut, err = Build([]byte{1, 2}, Size-1, Zeroes(Size))
	if err != nil || cut != 1 || img[Size-1] != 1 {
		t.Errorf("two bytes at the last address: cut %d, err %v, byte 0x%02X", cut, err, img[Size-1])
	}
}

func TestBuildAddressOutsideTheROMIsAnError(t *testing.T) {
	for _, addr := range []int{-1, Size, 0xFFFF, 1 << 40} {
		if img, _, err := Build([]byte{1}, addr, Zeroes(Size)); err == nil || img != nil {
			t.Errorf("address %d: want an error and no image, got %v", addr, err)
		}
	}
}

func TestBuildForABankOfAnotherSize(t *testing.T) {
	img, cut, err := Build(bytes.Repeat([]byte{9}, 0x100), 0x1F80, Zeroes(0x2000))
	if err != nil || len(img) != 0x2000 || cut != 0x80 {
		t.Fatalf("8 KiB bank: len %d, cut %d, err %v", len(img), cut, err)
	}
	if _, _, err := Build([]byte{1}, 0x2000, Zeroes(0x2000)); err == nil {
		t.Error("0x2000 is outside an 8 KiB bank")
	}
	if _, _, err := Build([]byte{1}, 0, nil); err == nil {
		t.Error("an empty base must be refused")
	}
}

func TestBuildEmptyPayloadIsTheBase(t *testing.T) {
	base := bytes.Repeat([]byte{4}, Size)
	img, cut, err := Build(nil, 0x100, base)
	if err != nil || cut != 0 || !bytes.Equal(img, base) {
		t.Errorf("empty payload: cut %d, err %v", cut, err)
	}
}

func TestParseAddress(t *testing.T) {
	good := map[string]int{
		"0": 0, "0x1234": 0x1234, "0X1234": 0x1234, "$1234": 0x1234, "4660": 0x1234,
		"010": 10, "0b101": 5, "0o17": 15, " 0x38 ": 0x38, "-1": -1,
	}
	for in, want := range good {
		got, err := ParseAddress(in)
		if err != nil || got != want {
			t.Errorf("ParseAddress(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", "banana", "0x", "$", "12ab", "0xZZ", "1.5"} {
		if _, err := ParseAddress(in); err == nil {
			t.Errorf("ParseAddress(%q) should fail", in)
		}
	}
}

func TestCutWarningSaysWhatHappened(t *testing.T) {
	got := CutWarning(0x300, 0x3F00, Size, 0x200)
	for _, want := range []string{"768 bytes", "0x3F00", "0x41FF", "16 KiB", "0x3FFF", "512 bytes left out"} {
		if !strings.Contains(got, want) {
			t.Errorf("warning %q lacks %q", got, want)
		}
	}
}
