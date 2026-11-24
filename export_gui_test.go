//go:build !headless

package zenzx

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// File > Export...: the machine is stopped the moment the item is picked, the
// name is asked for after, and what is written is the machine as it was at the pick.

func exportBar(t *testing.T) (*appMenuBar, *ZenZX) {
	t.Helper()
	b, zx := newTestBar(t, nil, "")
	if err := zx.LoadSnapshotFormat("testdata/snapshots/z88dk_zx48.sna", "sna"); err != nil {
		t.Fatal(err)
	}
	zx.paused = false
	return b, zx
}

func pickExport(b *appMenuBar, zx *ZenZX) { b.fileMenu.pick(zx, 1, 0) } // File > Export...

// exportAs answers the Export form: the folder and the name typed, and Export pressed.
func exportAs(b *appMenuBar, zx *ZenZX, dir, name string) {
	b.exportAsk.dir, b.exportAsk.name = dir, name
	b.answerExport(zx)
}

func TestExportStopsTheMachineAtThePickAndAsksForAName(t *testing.T) {
	b, zx := exportBar(t)
	resetRun(zx, 50)
	pickExport(b, zx)
	if !zx.paused {
		t.Error("the machine runs while the name is asked for")
	}
	if b.exportAsk == nil || b.diskDialog != nil || b.export == nil {
		t.Fatalf("no name is asked for in the form: form %v, browser %v, job %v", b.exportAsk, b.diskDialog, b.export)
	}
	b.cancelExport(zx)
	if zx.paused {
		t.Error("cancelling left a machine that was running paused")
	}
}

func TestCancellingAnExportLeavesThePauseAsItWas(t *testing.T) {
	for _, was := range []bool{false, true} {
		b, zx := exportBar(t)
		zx.paused = was
		pickExport(b, zx)
		if !zx.paused {
			t.Errorf("paused=%v: not stopped at the pick", was)
		}
		b.cancelExport(zx)
		if zx.paused != was || b.export != nil {
			t.Errorf("paused=%v: after cancelling paused=%v, job=%v", was, zx.paused, b.export)
		}
		b.cancelExport(zx) // nothing to cancel is not an error
	}
}

func TestAcceptingAnExportWritesTheMachineAsItWasAtThePick(t *testing.T) {
	b, zx := exportBar(t)
	resetRun(zx, 50)
	pickExport(b, zx)
	atPick := resetDump(t, zx)

	// The machine does not move while the browser is open, even if the host runs frames.
	if !zx.paused {
		t.Fatal("not paused")
	}
	var note string
	zx.OnNotice = func(m string) { note = m }
	dir := t.TempDir()
	out := filepath.Join(dir, "game")
	exportAs(b, zx, dir, "game")
	if zx.paused {
		t.Error("a machine that was running is paused after the export")
	}
	if !strings.Contains(note, "Exported game") {
		t.Fatalf("notice %q", note)
	}
	file, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	p, _, err := readProgram(bytes.NewReader(file), int64(len(file)))
	if err != nil {
		t.Fatal(err)
	}
	if p.Model != "48k" || p.Title != "game" {
		t.Errorf("the program carries %+v", p)
	}
	back, err := newMachine(p.Model, "./rom")
	if err != nil {
		t.Skip(err)
	}
	if err := back.LoadSnapshotData(p.Snapshot); err != nil {
		t.Fatal(err)
	}
	if d := restoreCompare(atPick, resetDump(t, back), int64(zx.cyclesPerFrame), true); len(d) > 0 {
		t.Errorf("the program is not the machine at the pick:\n  %s", strings.Join(d, "\n  "))
	}
}

func TestExportKeepsAPausedMachinePausedAndRefusesWhatItCannotMake(t *testing.T) {
	b, zx := exportBar(t)
	zx.paused = true
	pickExport(b, zx)
	exportAs(b, zx, t.TempDir(), "p")
	if !zx.paused {
		t.Error("an export unpaused a machine that was paused")
	}

	// A machine that cannot be exported says so and is left alone.
	b2, zx2 := exportBar(t)
	for k := range b2.modelChecked {
		*b2.modelChecked[k] = k == "next"
	}
	var note string
	zx2.OnNotice = func(m string) { note = m }
	pickExport(b2, zx2)
	if zx2.paused || b2.export != nil || b2.diskDialog != nil || note == "" {
		t.Errorf("a Next: paused=%v job=%v dialog=%v notice=%q", zx2.paused, b2.export, b2.diskDialog, note)
	}
}

// consoleDuring runs f and returns what it wrote to the console.
func consoleDuring(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	was := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = was }()
	f()
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

// What the menus say is said in the window: with a toast to show it, the console hears nothing,
// for a Next that cannot be exported and for the Export form's own result alike. A machine
// with no window says it on the console, as the headless build does.
func TestExportMessagesAreToastsAndNotConsoleLines(t *testing.T) {
	b, zx := exportBar(t)
	for k := range b.modelChecked {
		*b.modelChecked[k] = k == "next"
	}
	var toasts []string
	zx.OnNotice = func(m string) { toasts = append(toasts, m) }
	out := consoleDuring(t, func() { pickExport(b, zx) })
	if out != "" {
		t.Errorf("the console was written to: %q", out)
	}
	if len(toasts) != 1 || !strings.Contains(toasts[0], "can be exported from the 48K") {
		t.Errorf("toasts %q", toasts)
	}

	// Nothing running: the same, through the same road.
	zx.stopped = true
	toasts = nil
	out = consoleDuring(t, func() { zx.notice("No machine is running") })
	if out != "" || len(toasts) != 1 {
		t.Errorf("no machine: console %q, toasts %q", out, toasts)
	}

	// Without a window the console has it.
	zx.OnNotice = nil
	out = consoleDuring(t, func() { zx.notice("No machine is running") })
	if !strings.Contains(out, "No machine is running") {
		t.Errorf("with no toast the console has %q", out)
	}
}
