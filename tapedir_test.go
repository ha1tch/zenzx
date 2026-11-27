package zenzx

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func putTape(t *testing.T, dir, name string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	abs, _ := filepath.Abs(p)
	return abs
}

// withTapeDir makes the tapes folder a folder of the test's own, and the working directory too.
func withTapeDir(t *testing.T) string {
	t.Helper()
	old := tapeDir
	t.Cleanup(func() { tapeDir = old })
	t.Chdir(t.TempDir())
	tapeDir = TapeDir
	abs, _ := filepath.Abs(TapeDir)
	return abs
}

func TestTheDefaultTapesFolderIsTapes(t *testing.T) {
	if TapeDir != "tapes" || tapeDir != "tapes" {
		t.Errorf("the default tapes folder is %q (in use: %q), want tapes", TapeDir, tapeDir)
	}
}

func TestIsTapePath(t *testing.T) {
	for p, want := range map[string]bool{"a.tap": true, "B.TZX": true, "dir/x.Tap": true, "a.sna": false, "tap": false, "a.tap.txt": false, "": false} {
		if isTapePath(p) != want {
			t.Errorf("isTapePath(%q) = %v, want %v", p, !want, want)
		}
	}
}

func TestAddRecentTapePutsItFirstOnceAndKeepsOnlyTapes(t *testing.T) {
	t.Chdir(t.TempDir())
	list := addRecentTape(nil, "a.tap")
	list = addRecentTape(list, "b.tzx")
	list = addRecentTape(list, "a.tap")
	list = addRecentTape(list, "notes.txt")
	list = addRecentTape(list, "")
	if len(list) != 2 || filepath.Base(list[0]) != "a.tap" || filepath.Base(list[1]) != "b.tzx" || !filepath.IsAbs(list[0]) {
		t.Errorf("the list is %v, want a.tap and b.tzx, absolute", list)
	}
	var many []string
	for i := 0; i < recentTapesMax+10; i++ {
		many = addRecentTape(many, fmt.Sprintf("t%d.tap", i))
	}
	if len(many) != recentTapesMax || filepath.Base(many[0]) != fmt.Sprintf("t%d.tap", recentTapesMax+9) {
		t.Errorf("%d kept, newest %s", len(many), filepath.Base(many[0]))
	}
}

func TestRenameRecentTapeKeepsItsPlace(t *testing.T) {
	got := renameRecentTape([]string{"/a.tap", "/b.tap", "/c.tap"}, "/b.tap", "/z.tap")
	if !reflect.DeepEqual(got, []string{"/a.tap", "/z.tap", "/c.tap"}) {
		t.Errorf("got %v", got)
	}
}

func TestTapeLibraryListsRecentOnesFirstThenTheFolderByName(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "tapes")
	b := putTape(t, dir, "b.tap")
	a := putTape(t, dir, "A.tzx")
	c := putTape(t, dir, "c.tap")
	putTape(t, dir, "notes.txt")
	if err := os.Mkdir(filepath.Join(dir, "sub.tap"), 0o755); err != nil {
		t.Fatal(err)
	}
	far := putTape(t, filepath.Join(root, "elsewhere"), "far.tap")
	gone := filepath.Join(root, "gone.tap")

	lib := tapeLibrary(dir, []string{far, c, gone, c})
	var names []string
	for _, f := range lib {
		names = append(names, f.Name)
	}
	if want := []string{"far.tap", "c.tap", "A.tzx", "b.tap"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("the library is %v, want %v (recent first, the rest by name, nothing twice, nothing that is gone)", names, want)
	}
	here := map[string]bool{}
	for _, f := range lib {
		here[f.Name] = f.Here
	}
	if here["far.tap"] || !here["c.tap"] || !here["A.tzx"] || !here["b.tap"] {
		t.Errorf("which are in the tapes folder: %v", here)
	}
	if lib[2].Path != a || lib[3].Path != b || lib[2].Ext != "tzx" || lib[1].Size != 1 {
		t.Errorf("a tape's path, format and size: %+v", lib[2:])
	}
	if got := tapeLibrary(filepath.Join(root, "nowhere"), []string{far}); len(got) != 1 {
		t.Errorf("a tapes folder that is not there lists %d tapes, want the recent one", len(got))
	}
}

func TestLoadTapeStartsWhereTheLastTapeCameFromElseInTheTapesFolder(t *testing.T) {
	tapes := withTapeDir(t)
	if got := tapeStartDir(nil); got != tapes {
		t.Errorf("with no tape loaded it starts in %q, want the tapes folder %q", got, tapes)
	}
	if st, err := os.Stat(tapes); err != nil || !st.IsDir() {
		t.Error("the tapes folder was not made")
	}
	far := putTape(t, t.TempDir(), "far.tap")
	if got := tapeStartDir([]string{far}); got != filepath.Dir(far) {
		t.Errorf("after a tape from elsewhere it starts in %q, want %q", got, filepath.Dir(far))
	}
	if err := os.RemoveAll(filepath.Dir(far)); err != nil {
		t.Fatal(err)
	}
	if got := tapeStartDir([]string{far}); got != tapes {
		t.Errorf("with that folder gone it starts in %q, want the tapes folder", got)
	}
}
