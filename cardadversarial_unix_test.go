//go:build unix

package zenzx

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// Adversarial tests of the card search that need what only a Unix system has: pipes, devices and
// files that cannot be read (cardadversarial_test.go).

// finishesIn runs f and fails the test if it has not finished in d. A search that opens a pipe waits
// for whoever opens the other end, which no one will.
func finishesIn(t *testing.T, d time.Duration, what string, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { defer close(done); f() }()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("%s did not finish in %v", what, d)
	}
}

// A pipe or a device that is named like a card image is not opened, whether it is found in a folder
// or chosen: opening a pipe waits for a writer, and reading a device may never end.
func TestAPipeOrADeviceNamedAsACardIsNotOpened(t *testing.T) {
	dir := t.TempDir()
	mine := searchCard(t, dir, "next-card.img")
	rec := searchRecord(t, 1, mine)
	pipe := filepath.Join(dir, "a-pipe.img")
	if err := syscall.Mkfifo(pipe, 0o644); err != nil {
		t.Skipf("no pipes here: %v", err)
	}
	t.Cleanup(func() {
		// let a reader that is waiting go, so that a failed test does not leave it behind
		if f, err := os.OpenFile(pipe, os.O_RDWR, 0); err == nil {
			f.Close()
		}
	})
	devices := map[string]string{}
	for _, dev := range []string{"/dev/zero", "/dev/null", "/dev/urandom"} {
		if _, err := os.Stat(dev); err == nil {
			name := filepath.Join(dir, filepath.Base(dev)+".img")
			if err := os.Symlink(dev, name); err == nil {
				devices[name] = dev
			}
		}
	}

	finishesIn(t, 5*time.Second, "looking through a folder with a pipe in it", func() {
		s := &cardSearch{dirs: []string{dir}}
		if files := s.candidates(); len(files) != 1 {
			t.Errorf("a pipe or a device is a candidate: %v", files)
		}
		if got := s.find(0, []cardRecord{rec})[0]; !got.acceptable() || got.Path != mine {
			t.Errorf("%+v", got)
		}
	})
	choices := []string{pipe}
	for name := range devices {
		choices = append(choices, name)
	}
	choices = append(choices, "/dev/zero")
	for _, path := range choices {
		finishesIn(t, 5*time.Second, "a choice of "+filepath.Base(path), func() {
			s := &cardSearch{chosen: map[string]string{needKey(0, 1): path}}
			got := s.find(0, []cardRecord{rec})[0]
			if got.acceptable() || got.Path != "" {
				t.Errorf("%s was taken for a card: %+v", path, got)
			}
		})
	}
}

// A card that cannot be read is passed over in a folder and said so when chosen.
func TestACardThatCannotBeReadIsPassedOver(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: permissions do not stop it reading")
	}
	dir := t.TempDir()
	locked := searchCard(t, dir, "locked.img")
	rec := searchRecord(t, 1, locked)
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o644) })
	s := &cardSearch{dirs: []string{dir}}
	if got := s.find(0, []cardRecord{rec})[0]; got.acceptable() {
		t.Errorf("%+v", got)
	}
	s = &cardSearch{chosen: map[string]string{needKey(0, 1): locked}}
	got := s.find(0, []cardRecord{rec})[0]
	if got.acceptable() || got.why() == "" {
		t.Errorf("%+v", got)
	}
}

// A folder that cannot be read is no folder, and does not stop the others being looked through.
func TestAFolderThatCannotBeReadDoesNotStopTheSearch(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: permissions do not stop it reading")
	}
	closed := filepath.Join(t.TempDir(), "closed")
	if err := os.Mkdir(closed, 0o755); err != nil {
		t.Fatal(err)
	}
	searchCard(t, closed, "hidden.img")
	if err := os.Chmod(closed, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(closed, 0o755) })
	open := t.TempDir()
	mine := searchCard(t, open, "next-card.img")
	rec := searchRecord(t, 1, mine)
	s := &cardSearch{dirs: []string{closed, open}}
	if got := s.find(0, []cardRecord{rec})[0]; !got.acceptable() || got.Path != mine {
		t.Errorf("%+v", got)
	}
}
