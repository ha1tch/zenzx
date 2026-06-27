//go:build headless

package main

import (
	"fmt"
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
	if err := fdc.LoadDisk("/mnt/user-data/uploads/artist.dsk"); err != nil {
		t.Skipf("artist.dsk not available: %v", err)
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
	for _, b := range cmd { fdc.WriteData(b) }
	var multi []byte
	for i := 0; i < 4000 && fdc.commandPhase == 1; i++ { multi = append(multi, fdc.ReadData()) }
	for i := 0; i < 7 && fdc.commandPhase == 2; i++ { fdc.ReadData() }
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
