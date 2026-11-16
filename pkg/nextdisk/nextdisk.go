// Package nextdisk is the read side of zenzx's future DivMMC/SD-card
// bridge: opening a FAT16/FAT32 disk image and reading named files out
// of it, the same shape a real ZX Spectrum Next's boot ROM and
// TBBLUE.FW use to find config.ini, menu.def, and the machine/
// peripheral ROM images during the real boot sequence (see
// docs/NEXT_BOOT_REFERENCE.md, stages 1-2).
//
// The FAT16/FAT32 driver itself is github.com/ha1tch/blipper/fatfs
// (Apache 2.0, same author as zenzx) rather than a second
// implementation written here -- see docs/NEXT_FIRMWARE_LICENSING.md
// for the firmware-sourcing decision this supports and NOTICE for
// this dependency's own attribution.
//
// Nothing in this package is wired to the Z80 bus yet. It answers one
// question only -- "given a FAT image, what files does it hold and
// what are their bytes" -- which is the filesystem half of T-36's
// Tier 2 boot chain (docs/TRACKING.md). The other half, the DivMMC
// hardware bridge itself (the Z80-visible automap/register interface
// that makes an SD card reachable from Z80 code in the first place),
// does not exist yet and is tracked separately there.
package nextdisk

import (
	"fmt"
	"io"

	"github.com/ha1tch/blipper/fatfs"
)

// Image is a read-only FAT16/FAT32 image, opened from whatever backs
// r -- a file on disk, an embedded []byte via bytes.NewReader, or any
// other io.ReadSeeker.
//
// Read-only deliberately: the real boot chain (config.ini, menu.def,
// ROM images) only ever reads its SD card during boot. A read-write
// Image is not exposed here; add one alongside this, following the
// same pattern and fatfs.OpenImageRW, if and when a write path is
// actually needed.
type Image struct {
	vol *fatfs.Volume
}

// OpenImage opens a FAT16/FAT32 image read-only. Long filenames are
// enabled: real Next SD-card paths (docs/NEXT_BOOT_REFERENCE.md's
// /MACHINES/NEXT/... convention) are long, mixed-case names, not 8.3
// short names, even though fatfs itself defaults long-name support to
// off.
func OpenImage(r io.ReadSeeker) (*Image, error) {
	vol, err := fatfs.OpenImage(r, fatfs.WithLongNames(true))
	if err != nil {
		return nil, fmt.Errorf("nextdisk: %w", err)
	}
	return &Image{vol: vol}, nil
}

// Type reports which FAT variant the image uses. A real Next SD card
// is FAT32; FAT16 is accepted here only because fatfs supports it
// too, not because any real Next firmware needs it.
func (img *Image) Type() fatfs.FATType {
	return img.vol.Type()
}

// List returns every regular file name in the image's root directory,
// in on-disk order.
func (img *Image) List() []string {
	return img.vol.List()
}

// Exists reports whether a named file is present.
func (img *Image) Exists(name string) bool {
	return img.vol.Exists(name)
}

// Stat returns the size, in bytes, of a named file.
func (img *Image) Stat(name string) (uint32, error) {
	size, err := img.vol.Stat(name)
	if err != nil {
		return 0, fmt.Errorf("nextdisk: stat %q: %w", name, err)
	}
	return size, nil
}

// ReadFile returns the full contents of a named file. Boot-chain
// files (config.ini, menu.def, individual machine/peripheral ROM
// images) are all kilobytes, not megabytes, so reading a whole file
// at once is the right shape here; a streaming Open, returning
// fatfs's own *fatfs.File directly, belongs alongside this rather
// than replacing it if a large file is ever the reason to touch this
// package again.
func (img *Image) ReadFile(name string) ([]byte, error) {
	f, err := img.vol.Open(name)
	if err != nil {
		return nil, fmt.Errorf("nextdisk: open %q: %w", name, err)
	}
	buf := make([]byte, f.Size())
	if _, err := io.ReadFull(f, buf); err != nil {
		return nil, fmt.Errorf("nextdisk: read %q: %w", name, err)
	}
	return buf, nil
}
