package zenzx

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// A snapshot whose chunk header claims more than the snapshot holds is refused without making room
// for what it claims: the header is the file's word, and a file comes from anywhere.
func TestASnapshotChunkThatClaimsMoreThanTheFileHoldsIsRefusedWithoutRoomForIt(t *testing.T) {
	zx, err := newMachine("next", "./rom")
	if err != nil {
		t.Skipf("next: %v", err)
	}
	r := craftedSnapshot(chunkBytes(ChunkCPU, 0xFFFFFFFF, []byte{1, 2, 3}))
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "hostile.zxs")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	err = zx.LoadSnapshotFormat(path, "zxs")
	runtime.ReadMemStats(&after)
	if err == nil || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("the snapshot was loaded, or refused for another reason: %v", err)
	}
	if got := after.TotalAlloc - before.TotalAlloc; got > 64<<20 {
		t.Errorf("a chunk that is not there was made room for: %d MB", got>>20)
	}
}
