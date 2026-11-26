package zenzx

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// touchROMs makes the named files (paths with slashes, under dir), each a byte long.
func touchROMs(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, n := range names {
		p := filepath.Join(dir, filepath.FromSlash(n))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte{0}, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// The scan goes two folders down and no further, leaves out dot folders, folders with no ROM
// and files that are not ROMs, and sorts at every level.
func TestScanCustomROMsGoesTwoFoldersDown(t *testing.T) {
	dir := t.TempDir()
	touchROMs(t, dir,
		"b.rom", "a.ROM", "notes.txt",
		"sub1/x.rom", "sub1/inner/y.rom", "sub1/inner/deeper/z.rom", "sub1/inner/w.rom",
		"sub2/q.rom",
		".hidden/h.rom",
		"onlytext/readme.txt", "emptyinner/none/readme.txt")
	if err := os.MkdirAll(filepath.Join(dir, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}

	roms, folders := scanCustomROMs(dir)
	if want := []string{"a.ROM", "b.rom"}; !reflect.DeepEqual(roms, want) {
		t.Errorf("the ROMs of the folder itself: %q, want %q", roms, want)
	}
	want := []romFolder{
		{"sub1", []string{"sub1/x.rom", "sub1/inner/w.rom", "sub1/inner/y.rom"}},
		{"sub2", []string{"sub2/q.rom"}},
	}
	if !reflect.DeepEqual(folders, want) {
		t.Errorf("the folders: %+v, want %+v", folders, want)
	}

	flat := listCustomROMs(dir)
	wantFlat := []string{"a.ROM", "b.rom", "sub1/x.rom", "sub1/inner/w.rom", "sub1/inner/y.rom", "sub2/q.rom"}
	if !reflect.DeepEqual(flat, wantFlat) {
		t.Errorf("listCustomROMs: %q, want %q", flat, wantFlat)
	}
}

func TestScanCustomROMsOfNothing(t *testing.T) {
	roms, folders := scanCustomROMs(filepath.Join(t.TempDir(), "missing"))
	if len(roms) != 0 || len(folders) != 0 {
		t.Errorf("a missing folder has %q and %+v", roms, folders)
	}
	if got := listCustomROMs(t.TempDir()); len(got) != 0 {
		t.Errorf("an empty folder lists %q", got)
	}
}

// With no folders the rows are the ROMs, as they were; a folder is a submenu of its own, ahead of
// the ROMs, with its ROMs named by what is under the folder.
func TestBuildROMRowsFoldersAreSubmenusAheadOfROMs(t *testing.T) {
	rows := buildROMRows([]string{"a.rom", "b.rom"}, nil, 40)
	if len(rows) != 2 || rows[0].Path != "a.rom" || rows[1].Label != "b.rom" || rows[0].isMenu() {
		t.Fatalf("with no folders: %+v", rows)
	}

	rows = buildROMRows([]string{"a.rom"},
		[]romFolder{{"sub", []string{"sub/x.rom", "sub/inner/y.rom"}}}, 40)
	if len(rows) != 2 || !rows[0].isMenu() || rows[0].Label != "sub" || rows[1].Path != "a.rom" {
		t.Fatalf("a folder and a ROM: %+v", rows)
	}
	got := []string{rows[0].Subs[0].Label, rows[0].Subs[1].Label}
	if want := []string{"x.rom", "inner/y.rom"}; !reflect.DeepEqual(got, want) {
		t.Errorf("the folder's rows are %q, want %q", got, want)
	}
	if rows[0].Subs[1].Path != "sub/inner/y.rom" {
		t.Errorf("a ROM two folders down has the path %q", rows[0].Subs[1].Path)
	}
}

// leavesOf is every ROM path in rows, submenus opened, in order.
func leavesOf(rows []romRow) []string {
	var out []string
	for _, r := range rows {
		if r.isMenu() {
			for _, s := range r.Subs {
				out = append(out, s.Path)
			}
		} else {
			out = append(out, r.Path)
		}
	}
	return out
}

// A long folder goes in pages, each a submenu of its own that fits under its row; too many rows
// put the rest in "More..." submenus; and whatever the sizes, every ROM is reachable once, no
// submenu is longer than the room below its row, and the dropdown stops at half the screen.
func TestBuildROMRowsPagesAndMore(t *testing.T) {
	var many []string
	for i := 0; i < 30; i++ {
		many = append(many, fmt.Sprintf("r%02d.rom", i))
	}
	rows := buildROMRows(many, nil, 20)
	// half of 20 is 10: nine ROMs, then "More..." rows with the other 21, the room under a row
	// at 9 being 11 and at 10 being 10
	if len(rows) != 11 || rows[9].Label != "More..." || rows[10].Label != "More... (2)" {
		t.Fatalf("30 ROMs on 20 rows: %d rows, %q then %q", len(rows), rows[9].Label, rows[len(rows)-1].Label)
	}
	if len(rows[9].Subs) != 11 || len(rows[10].Subs) != 10 {
		t.Errorf("the More... pages hold %d and %d", len(rows[9].Subs), len(rows[10].Subs))
	}
	if !reflect.DeepEqual(leavesOf(rows), many) {
		t.Error("the ROMs are not all there once, in order")
	}

	var big romFolder
	big.Name = "big"
	for i := 0; i < 30; i++ {
		big.ROMs = append(big.ROMs, fmt.Sprintf("big/r%02d.rom", i))
	}
	rows = buildROMRows(nil, []romFolder{big}, 20)
	if len(rows) != 2 || rows[0].Label != "big" || rows[1].Label != "big (2)" || len(rows[0].Subs) != 20 || len(rows[1].Subs) != 10 {
		t.Errorf("a folder of 30 on 20 rows: %+v", rows)
	}

	for _, fit := range []int{0, 5, 8, 12, 20, 60} {
		for _, nFolders := range []int{0, 1, 3, 9} {
			for _, nRoms := range []int{0, 1, 7, 25, 70} {
				var folders []romFolder
				var want []string
				for f := 0; f < nFolders; f++ {
					fo := romFolder{Name: fmt.Sprintf("f%d", f)}
					for i := 0; i < 3+f*5; i++ {
						fo.ROMs = append(fo.ROMs, fmt.Sprintf("f%d/r%d.rom", f, i))
					}
					folders = append(folders, fo)
					want = append(want, fo.ROMs...)
				}
				var roms []string
				for i := 0; i < nRoms; i++ {
					roms = append(roms, fmt.Sprintf("n%02d.rom", i))
				}
				want = append(want, roms...)
				rows := buildROMRows(roms, folders, fit)
				name := fmt.Sprintf("fit %d, %d folders, %d ROMs", fit, nFolders, nRoms)

				got := leavesOf(rows)
				if len(got) != len(want) {
					t.Errorf("%s: %d ROMs reachable, want %d", name, len(got), len(want))
					continue
				}
				seen := map[string]int{}
				for _, g := range got {
					seen[g]++
				}
				for _, w := range want {
					if seen[w] != 1 {
						t.Errorf("%s: %s reachable %d times", name, w, seen[w])
					}
				}
				eff := fit
				if eff < 2*minROMRows {
					eff = 2 * minROMRows
				}
				for i, r := range rows {
					room := eff - i
					if room < minROMRows {
						room = minROMRows
					}
					if r.isMenu() && len(r.Subs) > room {
						t.Errorf("%s: row %d (%s) holds %d under room for %d", name, i, r.Label, len(r.Subs), room)
					}
					if strings.HasPrefix(r.Label, "More...") && i < eff/2-1 {
						t.Errorf("%s: a More... row at %d, before half the screen", name, i)
					}
				}
			}
		}
	}
}

func TestROMAt(t *testing.T) {
	rows := buildROMRows([]string{"a.rom"}, []romFolder{{"sub", []string{"sub/x.rom", "sub/in/y.rom"}}}, 40)
	for _, c := range []struct {
		item, sub int
		want      string
		ok        bool
	}{
		{0, 1, "sub/in/y.rom", true},
		{0, 0, "sub/x.rom", true},
		{0, -1, "", false}, // the folder as a whole
		{0, 2, "", false},
		{1, -1, "a.rom", true},
		{1, 0, "a.rom", true}, // a ROM has no submenu: the index is not looked at
		{2, -1, "", false},
		{-1, -1, "", false},
	} {
		got, ok := romAt(rows, c.item, c.sub)
		if got != c.want || ok != c.ok {
			t.Errorf("romAt(%d, %d) = %q, %v; want %q, %v", c.item, c.sub, got, ok, c.want, c.ok)
		}
	}
	if _, ok := romAt(nil, 0, -1); ok {
		t.Error("a ROM out of no rows")
	}
}
