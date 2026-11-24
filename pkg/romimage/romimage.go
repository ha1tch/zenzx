// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

// Package romimage builds a ROM image out of a binary file of any size: the
// binary is placed at an address in a ROM-sized image, and what is around it is
// zeroes or the bytes of an existing ROM, so that the binary acts as an overlay
// on it. The image is always exactly the size of the ROM it is for.
//
// It is the Go form of scripts/mkrom.py, and the two agree byte for byte
// (romimage_python_test.go runs one against the other where python3 exists).
package romimage

import (
	"fmt"
	"strconv"
	"strings"
)

// Size is the size of a ZX Spectrum ROM, 16 KiB. A ROM bank of another size (the
// TS2068's 8 KiB extension ROM) is built the same way from a base of its size.
const Size = 0x4000

// ParseAddress reads an address written as 0x1234, $1234 or decimal. The prefixes
// 0b and 0o are read as binary and octal, and a decimal number keeps its
// leading zeroes as zeroes: 010 is ten, not eight.
func ParseAddress(text string) (int, error) {
	t := strings.TrimSpace(text)
	var n int64
	var err error
	switch {
	case strings.HasPrefix(t, "$"):
		n, err = strconv.ParseInt(t[1:], 16, 64)
	case hasBasePrefix(t):
		n, err = strconv.ParseInt(t, 0, 64)
	default:
		n, err = strconv.ParseInt(t, 10, 64)
	}
	if err != nil {
		return 0, fmt.Errorf("%q is not an address (0x1234, $1234 or decimal)", text)
	}
	return int(n), nil
}

// hasBasePrefix reports whether t starts, after any sign, with 0x, 0b or 0o in
// either case.
func hasBasePrefix(t string) bool {
	t = strings.TrimLeft(t, "+-")
	if len(t) < 2 || t[0] != '0' {
		return false
	}
	switch t[1] {
	case 'x', 'X', 'b', 'B', 'o', 'O':
		return true
	}
	return false
}

// Zeroes is a base of n zero bytes: the image is only the binary and zeroes.
func Zeroes(n int) []byte { return make([]byte, n) }

// Build places payload at address over base and returns the image, which is the
// size of base, and how many bytes of payload were cut off because they would
// have ended past the end of it. The cut is made at the last byte of the image,
// exactly. base is not changed.
//
// An address outside the image (negative, or its size or more) is an error, and
// so is an empty base.
func Build(payload []byte, address int, base []byte) (image []byte, cut int, err error) {
	size := len(base)
	if size == 0 {
		return nil, 0, fmt.Errorf("the base image is empty")
	}
	if address < 0 || address >= size {
		return nil, 0, fmt.Errorf("address 0x%04X is outside the ROM (0x0000 to 0x%04X)", address, size-1)
	}
	kept := payload
	if len(kept) > size-address {
		kept = kept[:size-address]
	}
	image = make([]byte, size)
	copy(image, base)
	copy(image[address:], kept)
	return image, len(payload) - len(kept), nil
}

// CutWarning is the warning for a payload that did not fit: what was placed
// where, where it would have ended, and how much was left out. The same words
// as scripts/mkrom.py gives.
func CutWarning(payloadLen, address, size, cut int) string {
	return fmt.Sprintf("%d bytes at 0x%04X end at 0x%04X, past the %d KiB ROM (0x%04X); cut at 0x%04X, %d bytes left out",
		payloadLen, address, address+payloadLen-1, size/1024, size-1, size-1, cut)
}
