package nextcard

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

// The identity of a card image.
//
// A snapshot of a Next refers to the cards the machine had: where it was on them, which drive
// letters NextZXOS gave their volumes. The cards change while the machine runs, so what they
// hold cannot say which card is which; what a card keeps is its partition table, and in it the
// disk signature, the four bytes at 0x1B8 of the first sector that a disk is told apart by
// (Windows and Linux both use it, and neither a file copied onto the card nor NextZXOS
// writing to it touches it). A card that has none, as the standard card downloaded for the
// Next has not, is given one when a snapshot is first made of a machine that has it
// (Stamp); a card made by ZenZX has one from the start, and a copy of a card is given a new
// one (Resign), so that two files that were one card and are not any more are not taken for
// each other.
//
// The identity also holds the card's size and its partition table: a card the same size, laid
// out the same, with the same signature, is the same card; a card with another signature is
// not, whatever it holds.

const (
	sigOffset   = 0x1B8 // the disk signature: four bytes, little endian
	tableOffset = 0x1BE // the four partition entries, sixteen bytes each
	tableLen    = 64
)

// Identity is what tells a card image from another (see above).
type Identity struct {
	Sig   uint32 // the MBR's disk signature, 0 for a card that has none
	Size  int64  // the image's size in bytes
	Table [tableLen]byte
}

// Keyed reports whether the card has a signature.
func (id Identity) Keyed() bool { return id.Sig != 0 }

// Match is how far one identity is the same card as another.
type Match int

const (
	Different Match = iota // not the same card
	Alike                  // the same size and layout, and a signature that one of them lacks: it may be
	Same                   // the same signature, size and layout
)

// Compare says whether o is the card id is.
func (id Identity) Compare(o Identity) Match {
	switch {
	case id.Size != o.Size || id.Table != o.Table:
		return Different
	case id.Sig != 0 && o.Sig != 0:
		if id.Sig == o.Sig {
			return Same
		}
		return Different
	}
	return Alike
}

// ReadIdentity reads the identity of the card image r, size bytes long.
func ReadIdentity(r io.ReaderAt, size int64) (Identity, error) {
	var mbr [sectorSize]byte
	if _, err := r.ReadAt(mbr[:], 0); err != nil {
		return Identity{}, fmt.Errorf("reading the partition table: %w", err)
	}
	if mbr[510] != 0x55 || mbr[511] != 0xAA {
		return Identity{}, errors.New("the image has no partition table")
	}
	id := Identity{Sig: binary.LittleEndian.Uint32(mbr[sigOffset:]), Size: size}
	copy(id.Table[:], mbr[tableOffset:tableOffset+tableLen])
	return id, nil
}

// IdentityOfFile reads the identity of the card image at path. Only an ordinary file is a card
// image: a pipe is not opened (opening one waits for whoever writes to it), nor is a device.
func IdentityOfFile(path string) (Identity, error) {
	if st, err := os.Stat(path); err != nil {
		return Identity{}, err
	} else if st.IsDir() {
		return Identity{}, errors.New("a folder is not a card image")
	} else if !st.Mode().IsRegular() {
		return Identity{}, errors.New("not an ordinary file")
	}
	f, err := os.Open(path)
	if err != nil {
		return Identity{}, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return Identity{}, err
	}
	if st.IsDir() {
		return Identity{}, errors.New("a folder is not a card image")
	}
	return ReadIdentity(f, st.Size())
}

// NewSignature returns a signature for a card: four random bytes that are not all zero.
func NewSignature() uint32 {
	var b [4]byte
	for {
		if _, err := rand.Read(b[:]); err != nil {
			panic("nextcard: no random numbers: " + err.Error())
		}
		if sig := binary.LittleEndian.Uint32(b[:]); sig != 0 {
			return sig
		}
	}
}

// ReadWriterAt is a card image that can be read and written.
type ReadWriterAt interface {
	io.ReaderAt
	io.WriterAt
}

// writeSignature puts sig in the card's first sector, and nothing else of it.
func writeSignature(f io.WriterAt, sig uint32) error {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], sig)
	_, err := f.WriteAt(b[:], sigOffset)
	return err
}

// Stamp gives the card image f a signature when it has none, and returns its identity as it
// is then; stamped says it was given one. A card that has one is left as it is. An image
// that cannot be written (a card opened read-only) is returned with its identity as it is,
// unkeyed, and the error that says why.
func Stamp(f ReadWriterAt, size int64) (id Identity, stamped bool, err error) {
	id, err = ReadIdentity(f, size)
	if err != nil || id.Keyed() {
		return id, false, err
	}
	sig := NewSignature()
	if err = writeSignature(f, sig); err != nil {
		return id, false, err
	}
	id.Sig = sig
	return id, true, nil
}

// Resign gives the card image at path a new signature, as a copy of a card is given one.
func Resign(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	if _, err = ReadIdentity(f, 0); err == nil {
		err = writeSignature(f, NewSignature())
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}
