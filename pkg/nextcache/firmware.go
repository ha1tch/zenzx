// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

// Package nextcache keeps the state of a Next that has booted, so that the next start does
// not have to boot (docs/proposals/NEXT_BOOT_CACHE.md).
//
// A cache is used only for the firmware it was made from. What identifies that firmware to
// the cache is Firmware: one digest over the files the Next boots from, defined here so
// that it comes out the same however the files are met.
package nextcache

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// digestTag begins the digest, and is its version: a change to the definition below is a
// change of tag, and a cache made under the old one is then invalid, not misread.
const digestTag = "zenzx-next-firmware-v1"

// FileDigest is one firmware file as the cache knows it.
type FileDigest struct {
	Path   string `json:"path"` // from the card's root, '/' as the separator, upper case
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"` // of the file's contents, hex
}

// Firmware is the firmware on a card: the files, and the one digest over them.
type Firmware struct {
	Digest string       `json:"digest"` // hex SHA-256, see Combine
	Files  []FileDigest `json:"files"`  // sorted by Path
}

// canonicalPath names a file by its place on the card: from the root, with '/' for the
// separator and the letters upper-cased, because FAT does not tell Tbblue.fw from TBBLUE.FW.
func canonicalPath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.Trim(p, "/")
	return strings.ToUpper(p)
}

// Combine is the digest of a set of files, and does not depend on the order they are given
// in. Each file is a record: the length of its path as 4 bytes, big-endian; the path; its
// size as 8 bytes, big-endian; the 32 bytes of the SHA-256 of its contents. The lengths
// come first so that no two different sets can be written as the same run of bytes. The
// records are sorted by path, as bytes, and joined; the digest is the SHA-256 of digestTag
// followed by the joined records. Files are returned in the order the digest takes them.
func Combine(files []FileDigest) (digest string, sorted []FileDigest, err error) {
	sorted = make([]FileDigest, len(files))
	for i, f := range files {
		f.Path = canonicalPath(f.Path)
		sorted[i] = f
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	h := sha256.New()
	h.Write([]byte(digestTag))
	for i, f := range sorted {
		if i > 0 && sorted[i-1].Path == f.Path {
			return "", nil, fmt.Errorf("nextcache: %s is listed twice", f.Path)
		}
		sum, err := hex.DecodeString(f.SHA256)
		if err != nil || len(sum) != sha256.Size {
			return "", nil, fmt.Errorf("nextcache: %s has no SHA-256", f.Path)
		}
		var n [8]byte
		binary.BigEndian.PutUint32(n[:4], uint32(len(f.Path)))
		h.Write(n[:4])
		h.Write([]byte(f.Path))
		binary.BigEndian.PutUint64(n[:], uint64(f.Size))
		h.Write(n[:])
		h.Write(sum)
	}
	return hex.EncodeToString(h.Sum(nil)), sorted, nil
}

// ReadFirmware reads the files at paths from the card image and returns their digests. A
// file the card lacks is an error and nothing is returned: a digest over fewer files would
// match no card, and some card by accident.
func ReadFirmware(card io.ReaderAt, paths []string) (Firmware, error) {
	return ReadFiles(card, paths, nil)
}

// ReadFiles is ReadFirmware with files that may be missing: those of optional the card has
// are in the digest, with the rest the digest says nothing of, so that a card without one
// and a card with it come out different, and two cards without it the same.
func ReadFiles(card io.ReaderAt, required, optional []string) (Firmware, error) {
	vol, err := nextcard.Open(card)
	if err != nil {
		return Firmware{}, fmt.Errorf("nextcache: the card cannot be read: %w", err)
	}
	var files []FileDigest
	paths := append(append([]string{}, required...), optional...)
	for i, p := range paths {
		n := vol.Root.Find(p)
		if n == nil || n.IsDir() {
			if i >= len(required) {
				continue
			}
			return Firmware{}, fmt.Errorf("nextcache: the card has no %s", p)
		}
		data, err := vol.ReadFile(n)
		if err != nil {
			return Firmware{}, fmt.Errorf("nextcache: %s cannot be read: %w", p, err)
		}
		sum := sha256.Sum256(data)
		files = append(files, FileDigest{Path: p, Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:])})
	}
	digest, sorted, err := Combine(files)
	if err != nil {
		return Firmware{}, err
	}
	return Firmware{Digest: digest, Files: sorted}, nil
}
