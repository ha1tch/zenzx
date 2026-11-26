//go:build unix

package nextcard

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// What is not an ordinary file is not a card image, and is not opened to find out: opening a pipe
// waits for whoever writes to it, and reading a device may never end.
func TestIdentityOfAPipeOrADeviceIsRefusedWithoutOpeningIt(t *testing.T) {
	pipe := filepath.Join(t.TempDir(), "pipe.img")
	if err := syscall.Mkfifo(pipe, 0o644); err != nil {
		t.Skipf("no pipes here: %v", err)
	}
	t.Cleanup(func() { // lets go of a reader that is waiting, if the test failed
		if f, err := os.OpenFile(pipe, os.O_RDWR, 0); err == nil {
			f.Close()
		}
	})
	for _, path := range []string{pipe, "/dev/zero", "/dev/null"} {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		done := make(chan error, 1)
		go func(p string) { _, err := IdentityOfFile(p); done <- err }(path)
		select {
		case err := <-done:
			if err == nil {
				t.Errorf("%s was taken for a card image", path)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("looking at %s did not finish", path)
		}
	}
}
