//go:build headless

package main

import (
	"fmt"
	"os"
	"testing"
)

// drive a READ DATA command through the FDC and collect the data bytes
func fdcReadSector(fdc *FDC765, c, h, r uint8) ([]byte, []uint8) {
	// Command phase: READ DATA = 0x46 (MFM+0x06). Params:
	// cmd, (H<<2|drive), C, H, R, N, EOT, GPL, DTL
	cmd := []uint8{0x46, (h << 2), c, h, r, 0x02, r, 0x2A, 0xFF}
	for _, b := range cmd {
		fdc.WriteData(b)
	}
	// Execution phase: drain data bytes until phase leaves 1
	var data []byte
	for i := 0; i < 600 && fdc.commandPhase == 1; i++ {
		data = append(data, fdc.ReadData())
	}
	// Result phase: read 7 result bytes
	var res []uint8
	for i := 0; i < 7 && fdc.commandPhase == 2; i++ {
		res = append(res, fdc.ReadData())
	}
	return data, res
}

func TestFDCReadMatchesDisk(t *testing.T) {
	fdc := NewFDC765()
	path := os.Getenv("ZENZX_TEST_DSK")
	usingOverride := path != ""
	if !usingOverride {
		path = "testdata/synthetic.dsk"
	}
	if err := fdc.LoadDisk(path); err != nil {
		if usingOverride {
			t.Skipf("ZENZX_TEST_DSK=%s not usable: %v", path, err)
		}
		t.Fatalf("loading checked-in fixture %s: %v", path, err)
	}
	disk := fdc.disk
	if disk == nil {
		t.Fatal("disk not parsed")
	}
	// Read track 0 sector 1 via the controller, compare with parsed data.
	data, res := fdcReadSector(fdc, 0, 0, 1)
	want := disk.FindSector(0, 0, 1).Data
	fmt.Printf("read %d bytes, result=% X\n", len(data), res)
	if len(data) != len(want) {
		t.Fatalf("length mismatch: got %d want %d", len(data), len(want))
	}
	for i := range want {
		if data[i] != want[i] {
			t.Fatalf("byte %d mismatch: got %02X want %02X", i, data[i], want[i])
		}
	}
	fmt.Println("single-sector read OK")

	// Multi-sector read: R=1, EOT=4 should yield 4 sectors back-to-back.
	cmd := []uint8{0x46, 0, 5, 0, 1, 0x02, 4, 0x2A, 0xFF} // track 5, R=1..4
	for _, b := range cmd {
		fdc.WriteData(b)
	}
	var multi []byte
	for i := 0; i < 4000 && fdc.commandPhase == 1; i++ {
		multi = append(multi, fdc.ReadData())
	}
	for i := 0; i < 7 && fdc.commandPhase == 2; i++ {
		fdc.ReadData()
	}
	fmt.Printf("multi-sector read returned %d bytes (expected %d)\n", len(multi), 4*512)
	if len(multi) != 4*512 {
		t.Errorf("multi-sector: got %d bytes, want %d", len(multi), 4*512)
	}
	// Verify the 4 sectors concatenate the right data.
	for s := uint8(1); s <= 4; s++ {
		secWant := disk.FindSector(5, 0, s).Data
		seg := multi[int(s-1)*512 : int(s)*512]
		for i := range secWant {
			if seg[i] != secWant[i] {
				t.Fatalf("multi sector R=%d byte %d mismatch", s, i)
			}
		}
	}
	fmt.Println("multi-sector read OK")
}

const (
	synthDSKNumTracks       = 6
	synthDSKSectorsPerTrack = 4
	synthDSKSizeCode        = 0x02 // 512 bytes/sector, matches BytesPerSector
)

// buildSyntheticDisk formats and writes testdata/synthetic.dsk's 6 tracks
// entirely through the FDC765 controller's own command interface -- SEEK,
// FORMAT TRACK, then WRITE DATA per sector -- the same sequence real +3DOS
// format/write software would issue. The fixture is therefore produced (and
// incidentally exercised) by zenzx's own +3 disk-write path, not assembled
// by hand.
func buildSyntheticDisk(t *testing.T) *FDC765 {
	t.Helper()
	fdc := NewFDC765()
	fdc.diskPresent = true
	fdc.disk = &Disk{
		NumTracks: synthDSKNumTracks,
		NumSides:  1,
		Tracks:    make([]DiskTrack, synthDSKNumTracks),
	}

	for track := uint8(0); track < synthDSKNumTracks; track++ {
		// Format Track reads the destination track from fdc.currentCylinder,
		// which only a prior Seek sets -- it carries no track byte of its own.
		fdc.WriteData(0x0F) // Seek
		fdc.WriteData(0)    // (H<<2|drive)
		fdc.WriteData(track)

		// Format Track: cmd, (H<<2|drive), N, SC, GPL, filler.
		fdc.WriteData(0x4D)
		fdc.WriteData(0)
		fdc.WriteData(synthDSKSizeCode)
		fdc.WriteData(synthDSKSectorsPerTrack)
		fdc.WriteData(0x2A)
		fdc.WriteData(0xE5)
		for r := uint8(1); r <= synthDSKSectorsPerTrack; r++ {
			fdc.WriteData(track) // C
			fdc.WriteData(0)     // H
			fdc.WriteData(r)     // R
			fdc.WriteData(synthDSKSizeCode)
		}
		drainFDCResult(t, fdc, "format track", track, 0)

		for r := uint8(1); r <= synthDSKSectorsPerTrack; r++ {
			cmd := []uint8{0x45, 0, track, 0, r, synthDSKSizeCode, r, 0x2A, 0xFF}
			for _, b := range cmd {
				fdc.WriteData(b)
			}
			for i := 0; i < BytesPerSector; i++ {
				fdc.WriteData(syntheticSectorByte(track, r, i))
			}
			drainFDCResult(t, fdc, "write sector", track, r)
		}
	}

	return fdc
}

// drainFDCResult reads the 7-byte result phase following an FDC command and
// fails the test if the controller is left mid-command afterward.
func drainFDCResult(t *testing.T, fdc *FDC765, what string, track, sector uint8) {
	t.Helper()
	for i := 0; i < 7 && fdc.commandPhase == 2; i++ {
		fdc.ReadData()
	}
	if fdc.commandPhase != 0 {
		t.Fatalf("%s track=%d sector=%d: controller left in phase %d, not idle", what, track, sector, fdc.commandPhase)
	}
}

// syntheticSectorByte deterministically derives sector content from its
// position, so TestFDCReadMatchesDisk compares against real, non-degenerate
// per-sector data rather than a uniform filler byte.
func syntheticSectorByte(track, sector uint8, i int) byte {
	return byte((int(track)*97 + int(sector)*31 + i) & 0xFF)
}

// TestGenerateSyntheticDSK (re)writes testdata/synthetic.dsk. Skipped by
// default -- it is a golden-file generator, not a check -- run it explicitly
// with ZENZX_REGEN_TESTDATA=1 after a deliberate change to the format above.
func TestGenerateSyntheticDSK(t *testing.T) {
	if os.Getenv("ZENZX_REGEN_TESTDATA") == "" {
		t.Skip("set ZENZX_REGEN_TESTDATA=1 to regenerate testdata/synthetic.dsk")
	}
	fdc := buildSyntheticDisk(t)
	fdc.diskFilename = "testdata/synthetic.dsk"
	fdc.diskModified = true
	if err := fdc.SaveDisk(); err != nil {
		t.Fatalf("saving synthetic disk: %v", err)
	}
	t.Logf("wrote %s", fdc.diskFilename)
}
