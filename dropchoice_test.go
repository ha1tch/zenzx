//go:build !headless

package zenzx

import "testing"

// TestAnyDropChoiceExts checks which dropped-file types the Next drop routing
// applies to: tapes, screen dumps and snapshots can be copied or run; a .nex can
// only be copied (run-on-the-fly is unimplemented); a +3 disc or a plain file is
// routed elsewhere entirely.
func TestAnyDropChoiceExts(t *testing.T) {
	for _, f := range []string{"/x/g.tap", "/x/g.tzx", "/x/p.scr", "/x/s.sna", "/x/s.z80", "/x/s.szx", "/x/p.nex", "/x/d.dsk"} {
		if !anyDropChoice([]string{f}) {
			t.Errorf("%s should be handled by the Next drop routing", f)
		}
	}
	for _, f := range []string{"/x/other.txt", "/x/pic.png"} {
		if anyDropChoice([]string{f}) {
			t.Errorf("%s should not be handled by the Next drop routing", f)
		}
	}
	// A mixed drop is handled if any file qualifies.
	if !anyDropChoice([]string{"/x/readme.txt", "/x/game.tap"}) {
		t.Error("a drop containing a tape should be handled")
	}
}

// TestSplitDropChoice checks the runnable/copy-only split: tapes, .scr and
// snapshots and Next programs offer the copy-or-run choice; a .dsk can only be copied on
// a Next (no run path there); anything else is dropped from both lists.
func TestSplitDropChoice(t *testing.T) {
	runnable, copyOnly := splitDropChoice([]string{
		"/x/game.tap", "/x/pic.scr", "/x/demo.nex", "/x/notes.txt", "/x/disc.dsk",
	})
	if len(runnable) != 3 {
		t.Errorf("runnable = %v, want the tape, the .scr and the .nex", runnable)
	}
	if len(copyOnly) != 1 {
		t.Errorf("copyOnly = %v, want the .dsk", copyOnly)
	}
}

// TestDropChoiceFlow drives the dialog's decision without the GUI: open it, then
// confirm each result path reports the right decision and hands back the files.
func TestDropChoiceFlow(t *testing.T) {
	d := &dropChoice{}
	if d.Active() {
		t.Error("a fresh dialog should not be active")
	}
	d.open([]string{"/x/a.tap", "/x/b.tap"}, 120)
	if !d.Active() {
		t.Error("open should make the dialog active")
	}
	if got := d.message(); got != "two files" {
		t.Errorf("message = %q, want %q", got, "two files")
	}
	files := d.takeFiles()
	if len(files) != 2 {
		t.Errorf("takeFiles returned %d files, want 2", len(files))
	}
	d.close()
	if d.Active() {
		t.Error("close should deactivate the dialog")
	}
}
