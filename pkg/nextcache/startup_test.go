// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package nextcache

import (
	"errors"
	"testing"
)

func startupFiles(extra ...FileDigest) Firmware {
	d, sorted, _ := Combine(append(firmwareFiles(), extra...))
	return Firmware{Digest: d, Files: sorted}
}

// What NextZXOS reads at start besides the firmware is in a settled cache and not in a
// hand-over one: a card that differs in it keeps the one and loses the other.
func TestStartupFilesInvalidateOnlyTheSettledStage(t *testing.T) {
	dir := t.TempDir()
	k := testKey(t)
	k.Startup = startupFiles(fd("nextzxos/autoexec.1st", "welcome"))
	for _, s := range Stages {
		if err := Write(dir, Entry{Key: k, Stage: s, State: []byte(s)}); err != nil {
			t.Fatal(err)
		}
	}
	for name, startup := range map[string]Firmware{
		"a start-up file changed": startupFiles(fd("nextzxos/autoexec.1st", "other")),
		"a start-up file gone":    startupFiles(),
		"another one added":       startupFiles(fd("nextzxos/autoexec.1st", "welcome"), fd("nextzxos/browser.cfg", "cfg")),
	} {
		q := k
		q.Startup = startup
		if _, err := Read(dir, q, HandOver); err != nil {
			t.Errorf("%s: the hand-over cache was refused: %v", name, err)
		}
		_, err := Read(dir, q, Settled)
		var se *StaleError
		if !errors.As(err, &se) || len(se.Differs) != 1 {
			t.Errorf("%s: the settled cache: %v", name, err)
			continue
		}
		t.Logf("%s: %v", name, se.Differs)
	}
	if _, err := Read(dir, k, Settled); err != nil {
		t.Fatalf("the same start-up files: %v", err)
	}
}

func TestStartupFilesAreNamedAsStartUpFiles(t *testing.T) {
	k := testKey(t)
	k.Startup = startupFiles(fd("nextzxos/autoexec.1st", "a"))
	q := k
	q.Startup = startupFiles(fd("nextzxos/autoexec.1st", "b"))
	if d := k.Diff(q); len(d) != 0 {
		t.Fatalf("Diff, which is the firmware's, names %v", d)
	}
	d := k.DiffFor(q, Settled)
	if len(d) != 1 || d[0] != "start-up file NEXTZXOS/AUTOEXEC.1ST" {
		t.Fatalf("got %v", d)
	}
	if d := k.DiffFor(q, HandOver); len(d) != 0 {
		t.Fatalf("the hand-over: %v", d)
	}
}
