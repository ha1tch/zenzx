// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// firmwareSandbox points everything that names a folder for the boot files at empty ones, so
// that a test sees only what it makes: NEXT_FILES, the per-user folder, and the replaceable
// checks. It returns the folder to give as -next-files.
func firmwareSandbox(t *testing.T) string {
	t.Helper()
	t.Setenv(nextFilesEnv, "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("AppData", filepath.Join(home, "config"))
	oldUsable, oldVerify, oldRun := nextFilesUsable, nextFirmwareVerify, nextSetupRun
	t.Cleanup(func() { nextFilesUsable, nextFirmwareVerify, nextSetupRun = oldUsable, oldVerify, oldRun })
	return filepath.Join(t.TempDir(), "files")
}

func writeFirmware(t *testing.T, dir string, names ...string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("0123456789"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNextFirmwareAction(t *testing.T) {
	for _, c := range []struct {
		firmware string
		legacy   bool
		want     string
		bad      bool
	}{
		{"", false, "", false},
		{"setup", false, "setup", false},
		{" Check ", false, "check", false},
		{"CLEANUP", false, "cleanup", false},
		{"", true, "setup", false},
		{"setup", true, "setup", false},
		{"check", true, "", true},
		{"download", false, "", true},
	} {
		got, err := nextFirmwareAction(c.firmware, c.legacy)
		if (err != nil) != c.bad || got != c.want {
			t.Errorf("nextFirmwareAction(%q, %v) = %q, %v; want %q, error %v", c.firmware, c.legacy, got, err, c.want, c.bad)
		}
	}
}

// A job keeps its progress where another goroutine can read it, passes the addresses with
// the spaces taken off, and reports the result.
func TestFirmwareJobProgressAndResult(t *testing.T) {
	release := make(chan struct{})
	var got nextcard.Options
	job := newFirmwareJob("/d", " https://a/dist.zip ", "", func(_ context.Context, opts nextcard.Options, progress nextcard.Progress) error {
		got = opts
		progress(nextcard.StageDownload, 30, 120)
		<-release
		return errors.New("no route to host")
	})
	if st := job.State(); st.Stage != nextcard.StageBootROM || st.Finished {
		t.Fatalf("before it starts: %+v", st)
	}
	job.Start()
	deadline := time.Now().Add(2 * time.Second)
	for job.State().Total != 120 {
		if time.Now().After(deadline) {
			t.Fatal("the progress did not arrive")
		}
		time.Sleep(time.Millisecond)
	}
	if st := job.State(); st.Stage != nextcard.StageDownload || st.Done != 30 || st.Finished {
		t.Fatalf("while it runs: %+v", st)
	}
	close(release)
	for !job.State().Finished {
		if time.Now().After(deadline) {
			t.Fatal("the job did not finish")
		}
		time.Sleep(time.Millisecond)
	}
	if st := job.State(); st.Err == nil || st.Err.Error() != "no route to host" {
		t.Fatalf("after it finished: %+v", st)
	}
	if got.Dir != "/d" || got.DistributionURL != "https://a/dist.zip" || got.BootROMURL != "" {
		t.Fatalf("the work was given %+v", got)
	}
}

// Cancel stops a started job, can be called again, and does nothing for one that was not.
func TestFirmwareJobCancel(t *testing.T) {
	newFirmwareJob("/d", "", "", nil).Cancel()
	job := newFirmwareJob("/d", "", "", func(ctx context.Context, _ nextcard.Options, _ nextcard.Progress) error {
		<-ctx.Done()
		return ctx.Err()
	})
	job.Start()
	job.Cancel()
	job.Cancel()
	deadline := time.Now().Add(2 * time.Second)
	for !job.State().Finished {
		if time.Now().After(deadline) {
			t.Fatal("Cancel did not stop the job")
		}
		time.Sleep(time.Millisecond)
	}
	if !errors.Is(job.State().Err, context.Canceled) {
		t.Fatalf("the job ended with %v, want context.Canceled", job.State().Err)
	}
}

// Run does the work on the caller's goroutine and tells onProgress after State is current.
func TestFirmwareJobRunReportsProgress(t *testing.T) {
	var job *firmwareJob
	job = newFirmwareJob("/d", "", "", func(_ context.Context, _ nextcard.Options, progress nextcard.Progress) error {
		progress(nextcard.StageBuild, 5, 10)
		return nil
	})
	seen := 0
	err := job.Run(context.Background(), func(s nextcard.Stage, done, total int64) {
		seen++
		if st := job.State(); st.Stage != s || st.Done != done || st.Total != total {
			t.Errorf("onProgress ran before State was current: %+v for %v %d/%d", st, s, done, total)
		}
	})
	if err != nil || seen != 1 || !job.State().Finished {
		t.Fatalf("Run: err %v, %d progress calls, state %+v", err, seen, job.State())
	}
}

func TestRunNextFirmwareSetup(t *testing.T) {
	dir := firmwareSandbox(t)
	var got nextcard.Options
	nextSetupRun = func(_ context.Context, opts nextcard.Options, progress nextcard.Progress) error {
		got = opts
		progress(nextcard.StageDownload, 50, 100)
		progress(nextcard.StageVerify, 0, 0)
		return nil
	}
	var out, errw bytes.Buffer
	if code := runNextFirmware(firmwareSetup, dir, "", "", &out, &errw); code != 0 {
		t.Fatalf("exit %d: %s", code, errw.String())
	}
	if got.Dir != dir || got.DistributionURL != nextcard.DefaultDistributionURL || got.BootROMURL != nextcard.DefaultBootROMURL {
		t.Errorf("the work was given %+v", got)
	}
	for _, want := range []string{"Downloading the distribution...", "50%", "Verified"} {
		if !strings.Contains(errw.String(), want) {
			t.Errorf("the progress lacks %q:\n%s", want, errw.String())
		}
	}
	nextSetupRun = func(context.Context, nextcard.Options, nextcard.Progress) error {
		return errors.New("the distribution: no route to host")
	}
	errw.Reset()
	if code := runNextFirmware(firmwareSetup, dir, "", "", &out, &errw); code != 1 || !strings.Contains(errw.String(), "Error: the distribution: no route to host") {
		t.Errorf("a failed download: exit %d, %q", code, errw.String())
	}
}

func TestRunNextFirmwareCheck(t *testing.T) {
	dir := firmwareSandbox(t)
	var out, errw bytes.Buffer

	// Nothing there: exit 1, and where a download would go.
	if code := runNextFirmware(firmwareCheck, dir, "", "", &out, &errw); code != 1 {
		t.Fatalf("with no files: exit %d, want 1", code)
	}
	if !strings.Contains(out.String(), "A download would put them in "+dir) || !strings.Contains(errw.String(), "Error:") {
		t.Errorf("with no files:\n%s\n%s", out.String(), errw.String())
	}

	// Two of three: the missing one is named.
	writeFirmware(t, dir, nextcard.BootROMName, nextcard.CardName)
	out.Reset()
	errw.Reset()
	runNextFirmware(firmwareCheck, dir, "", "", &out, &errw)
	if !strings.Contains(out.String(), "missing "+nextcard.DivMMCROMName) {
		t.Errorf("with two files:\n%s", out.String())
	}

	// All three, not usable: exit 1 with the reason.
	writeFirmware(t, dir, nextcard.DivMMCROMName)
	nextFilesUsable = func(string) error { return errors.New("next-card.img has 100 clusters") }
	out.Reset()
	errw.Reset()
	if code := runNextFirmware(firmwareCheck, dir, "", "", &out, &errw); code != 1 || !strings.Contains(out.String(), "all three, not usable: next-card.img has 100 clusters") {
		t.Errorf("with an unusable card: exit %d\n%s", code, out.String())
	}

	// All three, usable: exit 0; the strict check is reported but is not what decides.
	nextFilesUsable = func(string) error { return nil }
	nextFirmwareVerify = func(string) error { return errors.New("the free space record says 5 clusters") }
	out.Reset()
	errw.Reset()
	if code := runNextFirmware(firmwareCheck, dir, "", "", &out, &errw); code != 0 {
		t.Fatalf("with usable files: exit %d: %s", code, errw.String())
	}
	for _, want := range []string{dir + ": all three, usable", "A Next starts from " + dir, "not as a download leaves them: the free space record says 5 clusters"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("usable files: no %q in\n%s", want, out.String())
		}
	}
	nextFirmwareVerify = func(string) error { return nil }
	out.Reset()
	runNextFirmware(firmwareCheck, dir, "", "", &out, &errw)
	if !strings.Contains(out.String(), "as a download leaves them: verified") {
		t.Errorf("verified files:\n%s", out.String())
	}

	// What an unfinished download left is reported, and not counted against the files.
	writeFirmware(t, dir, "distribution.zip")
	out.Reset()
	if code := runNextFirmware(firmwareCheck, dir, "", "", &out, &errw); code != 0 || !strings.Contains(out.String(), "-next-firmware cleanup deletes them") {
		t.Errorf("with a leftover: exit %d\n%s", code, out.String())
	}
}

func TestRunNextFirmwareCleanup(t *testing.T) {
	dir := firmwareSandbox(t)
	var out, errw bytes.Buffer
	if code := runNextFirmware(firmwareCleanup, dir, "", "", &out, &errw); code != 0 || !strings.Contains(errw.String(), "Nothing to clean up in "+dir) {
		t.Fatalf("nothing to clean: exit %d, %q", code, errw.String())
	}
	writeFirmware(t, dir, nextcard.BootROMName, nextcard.DivMMCROMName, nextcard.CardName,
		nextcard.CardName+".part", "distribution.zip", "distribution.img.part")
	errw.Reset()
	if code := runNextFirmware(firmwareCleanup, dir, "", "", &out, &errw); code != 0 {
		t.Fatalf("exit %d: %s", code, errw.String())
	}
	for _, gone := range []string{nextcard.CardName + ".part", "distribution.zip", "distribution.img.part"} {
		if _, err := os.Stat(filepath.Join(dir, gone)); err == nil {
			t.Errorf("%s is still there", gone)
		}
		if !strings.Contains(errw.String(), "Deleted "+filepath.Join(dir, gone)) {
			t.Errorf("the report does not say %s was deleted:\n%s", gone, errw.String())
		}
	}
	for _, kept := range []string{nextcard.BootROMName, nextcard.DivMMCROMName, nextcard.CardName} {
		if _, err := os.Stat(filepath.Join(dir, kept)); err != nil {
			t.Errorf("%s was deleted: %v", kept, err)
		}
	}
	if !strings.Contains(errw.String(), "3 file(s)") || !strings.Contains(errw.String(), "not touched") {
		t.Errorf("the summary:\n%s", errw.String())
	}
}

func TestRunNextFirmwareUnknownAction(t *testing.T) {
	firmwareSandbox(t)
	var out, errw bytes.Buffer
	if code := runNextFirmware("download", "", "", "", &out, &errw); code != 1 || !strings.Contains(errw.String(), "not an action") {
		t.Errorf("exit %d, %q", code, errw.String())
	}
}
