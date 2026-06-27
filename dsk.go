package main

// ============================================================================
// DSK disk image parsing (standard and extended CPC/+3 format)
//
// A DSK image models a floppy as a Disk Information Block followed by a series
// of tracks, each a Track Information Block followed by its sectors. Unlike a
// flat sector dump, a DSK records each sector's CHRN address (cylinder, head,
// record/ID, size code) and FDC status, so the controller must locate a sector
// by matching the requested ID rather than by a computed offset. This is what
// real +3DOS disks -- including over-formatted ones with non-standard track
// counts or sector IDs -- require.
//
// Two header variants exist:
//   - Standard:  magic "MV - CPC...", one track size for the whole disk at
//                offset 0x32 (little-endian).
//   - Extended:  magic "EXTENDED...", a per-track size table from offset 0x34,
//                each byte giving the track size in units of 256 bytes (0 =
//                unformatted/absent track).
//
// This is an independent Go implementation. Its structure follows the public
// DSK format specification (and was cross-checked for correctness against the
// GPL-3.0 SpecIde emulator's parser), but no code is copied.
// ============================================================================

import (
	"fmt"
	"os"
)

// DiskSector is one sector: its CHRN address, recorded FDC status bytes, and
// data. SizeCode is the N value (actual bytes = 128 << N).
type DiskSector struct {
	C, H, R, N uint8  // cylinder, head, record (sector ID), size code
	ST1, ST2   uint8  // FDC status register 1/2 as recorded in the image
	Data       []byte // sector contents
}

// DiskTrack is one physical track: its number/side and ordered sector list.
type DiskTrack struct {
	Track, Side uint8
	SizeCode    uint8 // track-level sector size code
	GapLength   uint8
	Filler      uint8
	Sectors     []DiskSector
}

// Disk is a parsed DSK image.
type Disk struct {
	NumTracks uint8
	NumSides  uint8
	Extended  bool
	Tracks    []DiskTrack // indexed track-major: idx = track*NumSides + side
	Modified  bool
}

// trackInfoMagic is the leading text of a Track Information Block.
const trackInfoMagic = "Track-Info"

// LoadDSK reads and parses a DSK file (standard or extended).
func LoadDSK(path string) (*Disk, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseDSK(data)
}

// ParseDSK parses DSK bytes into a Disk.
func ParseDSK(data []byte) (*Disk, error) {
	if len(data) < 0x100 {
		return nil, fmt.Errorf("DSK too small: %d bytes", len(data))
	}

	header := string(data[0:8])
	var extended bool
	switch {
	case header == "EXTENDED":
		extended = true
	case header == "MV - CPC":
		extended = false
	default:
		return nil, fmt.Errorf("not a DSK image (bad magic %q)", header)
	}

	d := &Disk{
		NumTracks: data[0x30],
		NumSides:  data[0x31],
		Extended:  extended,
	}
	total := int(d.NumTracks) * int(d.NumSides)
	if total == 0 {
		return nil, fmt.Errorf("DSK reports zero tracks")
	}

	// Build the per-track size table.
	sizes := make([]int, total)
	if extended {
		if 0x34+total > 0x100 {
			return nil, fmt.Errorf("extended DSK track table overflows info block")
		}
		for i := 0; i < total; i++ {
			sizes[i] = int(data[0x34+i]) * 256
		}
	} else {
		ts := int(data[0x32]) | int(data[0x33])<<8
		for i := range sizes {
			sizes[i] = ts
		}
	}

	// Parse each track at its running offset.
	offset := 0x100
	d.Tracks = make([]DiskTrack, total)
	for tt := 0; tt < total; tt++ {
		if sizes[tt] == 0 {
			// Unformatted/absent track: leave an empty DiskTrack in place.
			continue
		}
		trk, err := parseTrack(data, offset)
		if err != nil {
			return nil, fmt.Errorf("track %d: %v", tt, err)
		}
		d.Tracks[tt] = trk
		offset += sizes[tt]
	}

	return d, nil
}

// parseTrack parses a Track Information Block and its sectors at offset.
func parseTrack(data []byte, offset int) (DiskTrack, error) {
	var trk DiskTrack
	if offset+0x100 > len(data) {
		return trk, fmt.Errorf("track info block past end of file")
	}
	if string(data[offset:offset+len(trackInfoMagic)]) != trackInfoMagic {
		return trk, fmt.Errorf("bad track magic at 0x%X", offset)
	}

	trk.Track = data[offset+0x10]
	trk.Side = data[offset+0x11]
	trk.SizeCode = data[offset+0x14]
	numSectors := int(data[offset+0x15])
	trk.GapLength = data[offset+0x16]
	trk.Filler = data[offset+0x17]

	if 0x18+8*numSectors > 0x100 {
		return trk, fmt.Errorf("sector descriptor table overflows info block")
	}

	// Sector data begins one info-block (0x100) past the track start.
	dataOffset := offset + 0x100
	trk.Sectors = make([]DiskSector, 0, numSectors)
	for ss := 0; ss < numSectors; ss++ {
		e := offset + 0x18 + 8*ss
		sec := DiskSector{
			C:   data[e],
			H:   data[e+1],
			R:   data[e+2],
			N:   data[e+3],
			ST1: data[e+4],
			ST2: data[e+5],
		}
		// Actual data length: the extended format stores it at +6/+7
		// (little-endian); if zero, fall back to 128<<N.
		length := int(data[e+6]) | int(data[e+7])<<8
		if length == 0 {
			length = 0x80 << sec.N
		}
		if dataOffset+length <= len(data) {
			sec.Data = make([]byte, length)
			copy(sec.Data, data[dataOffset:dataOffset+length])
			dataOffset += length
		} else {
			sec.Data = make([]byte, length) // truncated image: zero-fill
		}
		trk.Sectors = append(trk.Sectors, sec)
	}
	return trk, nil
}

// trackIndex returns the index into Disk.Tracks for a physical (track, side).
func (d *Disk) trackIndex(track, side uint8) int {
	if d.NumSides == 0 {
		return -1
	}
	idx := int(track)*int(d.NumSides) + int(side)
	if idx < 0 || idx >= len(d.Tracks) {
		return -1
	}
	return idx
}

// FindSector locates a sector by its physical track/side and record ID (R).
// It returns a pointer into the disk's data so writes are reflected. Returns
// nil if no such sector exists.
func (d *Disk) FindSector(track, side, r uint8) *DiskSector {
	idx := d.trackIndex(track, side)
	if idx < 0 {
		return nil
	}
	trk := &d.Tracks[idx]
	for i := range trk.Sectors {
		if trk.Sectors[i].R == r {
			return &trk.Sectors[i]
		}
	}
	return nil
}

// Track returns the parsed track for a physical track/side, or nil.
func (d *Disk) Track(track, side uint8) *DiskTrack {
	idx := d.trackIndex(track, side)
	if idx < 0 {
		return nil
	}
	return &d.Tracks[idx]
}

// Serialize encodes the disk back into extended-DSK bytes. The extended format
// is used unconditionally on save: it represents per-track sizes (including
// unformatted tracks) without loss, which the standard format cannot.
func (d *Disk) Serialize() []byte {
	total := int(d.NumTracks) * int(d.NumSides)

	// Disk Information Block (0x100 bytes).
	dib := make([]byte, 0x100)
	copy(dib, []byte("EXTENDED CPC DSK File\r\nDisk-Info\r\n"))
	copy(dib[0x22:], []byte("ZenZX\x00"))
	dib[0x30] = d.NumTracks
	dib[0x31] = d.NumSides

	// Compute each track's on-disk size (track info block + sector data),
	// rounded up to a multiple of 256, and fill the size table at 0x34.
	trackBytes := make([][]byte, total)
	for i := 0; i < total && i < len(d.Tracks); i++ {
		trk := &d.Tracks[i]
		if len(trk.Sectors) == 0 {
			dib[0x34+i] = 0 // unformatted track
			continue
		}
		tb := serializeTrack(trk)
		trackBytes[i] = tb
		dib[0x34+i] = byte(len(tb) / 256)
	}

	out := make([]byte, 0, 0x100*(total+1))
	out = append(out, dib...)
	for i := 0; i < total; i++ {
		if trackBytes[i] != nil {
			out = append(out, trackBytes[i]...)
		}
	}
	return out
}

// serializeTrack encodes one track: a 0x100-byte Track Information Block
// followed by its sector data, the whole padded to a 256-byte multiple.
func serializeTrack(trk *DiskTrack) []byte {
	tib := make([]byte, 0x100)
	copy(tib, []byte(trackInfoMagic))
	tib[0x10] = trk.Track
	tib[0x11] = trk.Side
	tib[0x14] = trk.SizeCode
	tib[0x15] = byte(len(trk.Sectors))
	tib[0x16] = trk.GapLength
	tib[0x17] = trk.Filler

	var data []byte
	for i, s := range trk.Sectors {
		e := 0x18 + 8*i
		tib[e] = s.C
		tib[e+1] = s.H
		tib[e+2] = s.R
		tib[e+3] = s.N
		tib[e+4] = s.ST1
		tib[e+5] = s.ST2
		tib[e+6] = byte(len(s.Data) & 0xFF)
		tib[e+7] = byte(len(s.Data) >> 8)
		data = append(data, s.Data...)
	}

	out := append(tib, data...)
	// Pad to a 256-byte boundary.
	if rem := len(out) % 256; rem != 0 {
		out = append(out, make([]byte, 256-rem)...)
	}
	return out
}

// SaveDSK serializes the disk and writes it to path.
func (d *Disk) SaveDSK(path string) error {
	return os.WriteFile(path, d.Serialize(), 0644)
}
