//go:build !headless

package zenzx

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/zenui"
)

// File > Export...: one row, no submenu, and a form that says what it does, asks for a name,
// and shows the folder, which Browse... may change.

func TestExportIsOneRowWithNoSubmenu(t *testing.T) {
	b, _ := newTestBar(t, nil, "")
	var found bool
	for _, it := range b.widget.ItemsFor(barFile) {
		if strings.HasPrefix(it.Label, "Export") {
			found = true
			if it.Label != "Export..." || len(it.SubItems) != 0 {
				t.Errorf("the Export row is %q with %d sub-items, want %q and none", it.Label, len(it.SubItems), "Export...")
			}
		}
	}
	if !found {
		t.Error("the File menu has no Export row")
	}
}

func TestTheExportFormExplainsAsksForANameAndShowsAFolder(t *testing.T) {
	b, zx := exportBar(t)
	pickExport(b, zx)
	q := b.exportAsk
	if q == nil {
		t.Fatal("no form")
	}
	defer b.cancelExport(zx)
	if q.name != "spectrum-48k" {
		t.Errorf("the name offered is %q", q.name)
	}
	if abs, _ := filepath.Abs(q.dir); q.dir != abs {
		t.Errorf("the folder %q is not absolute", q.dir)
	}
	var text []string
	boxes := 0
	for i := range q.said {
		if q.said[i] == "" {
			boxes++
		} else {
			text = append(text, q.said[i])
		}
	}
	all := strings.Join(text, " ")
	for _, want := range []string{"Export creates a program you can run on its own", "File name:", "Folder:"} {
		if !strings.Contains(all, want) {
			t.Errorf("the form does not say %q: %s", want, all)
		}
	}
	if boxes != 1 {
		t.Errorf("%d boxes to type in, want one: the name", boxes)
	}
}

func TestExportTargetRefusesWhatIsNotAName(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "adir"), 0o755)
	os.WriteFile(filepath.Join(dir, "there"), []byte("x"), 0o644)
	for _, c := range []struct{ name, why string }{
		{"", "Type a name"},
		{"   ", "Type a name"},
		{"a/b", "no slashes"},
		{`a\b`, "no slashes"},
		{"a:b", "none of"},
		{".hidden", "dot"},
		{strings.Repeat("x", 101), "too long"},
		{"adir", "folder"},
	} {
		if p, _, why := exportTarget(dir, c.name); why == "" || !strings.Contains(why, c.why) || p != "" {
			t.Errorf("%q: path %q, why %q, want a refusal mentioning %q", c.name, p, why, c.why)
		}
	}
	if _, _, why := exportTarget(filepath.Join(dir, "nowhere"), "game"); !strings.Contains(why, "folder is not there") {
		t.Errorf("a folder that is not there: %q", why)
	}
	p, exists, why := exportTarget(dir, "  game  ")
	if why != "" || exists || p != filepath.Join(dir, ProgramFileName("game")) {
		t.Errorf("a good name: %q %v %q", p, exists, why)
	}
	if _, exists, why := exportTarget(dir, "there"); why != "" || !exists {
		t.Errorf("a file that is there: exists %v, why %q", exists, why)
	}
}

func TestAFormThatIsRefusedSaysWhyAndKeepsTheMachineStopped(t *testing.T) {
	b, zx := exportBar(t)
	zx.paused = false
	pickExport(b, zx)
	b.exportAsk.name = ""
	b.answerExport(zx)
	if b.exportAsk == nil || !strings.Contains(b.exportAsk.why, "Type a name") {
		t.Fatalf("a refused answer closed the form or said nothing: %+v", b.exportAsk)
	}
	if !zx.paused || b.export == nil {
		t.Error("the machine was let go, or the export dropped, by a refused answer")
	}
	b.cancelExport(zx)
}

func TestAFileThatIsThereIsReplacedOnlyWhenTheSameAnswerIsGivenAgain(t *testing.T) {
	b, zx := exportBar(t)
	pickExport(b, zx)
	dir := t.TempDir()
	path := filepath.Join(dir, ProgramFileName("game"))
	os.WriteFile(path, []byte("old"), 0o644)
	b.exportAsk.dir, b.exportAsk.name = dir, "game"
	b.answerExport(zx)
	if b.exportAsk == nil || !strings.Contains(b.exportAsk.why, "there already") {
		t.Fatalf("the first answer over a file that is there: %+v", b.exportAsk)
	}
	if got, _ := os.ReadFile(path); string(got) != "old" {
		t.Fatal("the file was replaced at the first answer")
	}
	// A different name is a different answer: asking again for the first one is not implied.
	b.exportAsk.name = "game2"
	b.answerExport(zx)
	if b.exportAsk != nil {
		t.Fatalf("a new name was refused: %+v", b.exportAsk)
	}
	if _, err := os.Stat(filepath.Join(dir, ProgramFileName("game2"))); err != nil {
		t.Errorf("game2 was not written: %v", err)
	}

	pickExport(b, zx)
	b.exportAsk.dir, b.exportAsk.name = dir, "game"
	b.answerExport(zx) // told
	b.answerExport(zx) // again: replaced
	if b.exportAsk != nil {
		t.Fatalf("the form is still open after the second answer: %+v", b.exportAsk)
	}
	file, _ := os.ReadFile(path)
	if _, _, err := readProgram(bytes.NewReader(file), int64(len(file))); err != nil {
		t.Errorf("the file that replaced the old one is no program: %v", err)
	}
}

func TestCancellingTheFormLetsTheMachineGo(t *testing.T) {
	for _, was := range []bool{false, true} {
		b, zx := exportBar(t)
		zx.paused = was
		pickExport(b, zx)
		b.updateExport(zx, zenui.Input{Keys: []zenui.Key{zenui.KeyEscape}})
		if b.exportAsk != nil || b.export != nil || zx.paused != was {
			t.Errorf("paused=%v: after Escape the form is %v, the job %v, paused=%v", was, b.exportAsk, b.export, zx.paused)
		}
	}
}

// The browser of the Export form is a Save dialog, and zenui's Save dialog asks before replacing a
// file; this one must not, as the form asks about it itself and the person would be asked twice.
func TestExportBrowserLeavesTheQuestionAboutAFileThatIsThereToTheForm(t *testing.T) {
	b, zx := exportBar(t)
	pickExport(b, zx)
	defer b.cancelExport(zx)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "there"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	b.exportAsk.dir = dir
	b.browseExport()
	d := b.diskDialog
	d.Update(zenui.Input{Chars: []rune("there")})
	if st := d.Update(zenui.Input{Keys: []zenui.Key{zenui.KeyEnter}}); st != zenui.Accepted {
		t.Errorf("Enter on the name of a file that is there gave %v: the browser asked before the form did", st)
	}
}

func TestBrowsingFillsInTheFolderAndTheName(t *testing.T) {
	b, zx := exportBar(t)
	pickExport(b, zx)
	defer b.cancelExport(zx)
	b.browseExport()
	if b.diskDialog == nil || b.dialogKind != dialogExportBrowse || b.exportAsk == nil {
		t.Fatalf("Browse... opened %v (kind %v), the form is %v", b.diskDialog, b.dialogKind, b.exportAsk)
	}
	dir := t.TempDir()
	b.acceptDialog(zx, filepath.Join(dir, "chosen"))
	q := b.exportAsk
	if q == nil || q.dir != dir || q.name != "chosen" || b.diskDialog != nil {
		t.Fatalf("after browsing the form is %+v (browser %v)", q, b.diskDialog)
	}
	if b.export == nil || !zx.paused {
		t.Error("browsing let the machine go, or dropped the export")
	}
}
