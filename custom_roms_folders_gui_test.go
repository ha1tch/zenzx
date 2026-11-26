//go:build !headless

package zenzx

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ha1tch/zenui"
)

// subLabels is the labels of a dropdown row's submenu.
func subLabels(it zenui.Item) []string {
	var out []string
	for _, s := range it.SubItems {
		out = append(out, s.Label)
	}
	return out
}

// The folders of custom-roms/ are submenus of the Custom ROM dropdown, two folders down, ahead of
// its own ROMs and the row of links, and a ROM chosen in one is the one that loads.
func TestCustomROMMenuHasTheFoldersAsSubmenus(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	b.romDir = t.TempDir()
	touchROMs(t, b.romDir, "top.rom", "mods/a.rom", "mods/deep/b.rom", "mods/deep/deeper/c.rom", ".git/x.rom", "docs/readme.txt")
	marked := make([]byte, 16384)
	marked[0] = 0xAB
	if err := os.WriteFile(filepath.Join(b.romDir, "mods", "deep", "b.rom"), marked, 0o644); err != nil {
		t.Fatal(err)
	}

	b.refreshCustomROMItems(zx)
	if got, want := romMenuRows(b), []string{"mods", "top.rom", "-", "Download more ROMs"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("the dropdown is %q, want %q", got, want)
	}
	items := b.widget.ItemsFor(barCustomROM)
	if got, want := subLabels(items[0]), []string{"a.rom", "deep/b.rom"}; !reflect.DeepEqual(got, want) {
		t.Errorf("the folder's submenu is %q, want %q", got, want)
	}
	if b.romMoreAt != 3 {
		t.Errorf("the links' row is %d, want 3", b.romMoreAt)
	}

	// the ROM two folders down loads, by its path under custom-roms/
	b.pickCustomROM(zx, zenui.MenuBarSelection{BarIndex: barCustomROM, ItemIndex: 0, SubIndex: 1})
	if zx.memory.rom[0][0] != 0xAB {
		t.Errorf("the ROM of mods/deep/b.rom did not load: first byte %#x", zx.memory.rom[0][0])
	}

	// the folder as a whole is no ROM, and starts nothing
	was := zx.memory.rom[0][0]
	b.pickCustomROM(zx, zenui.MenuBarSelection{BarIndex: barCustomROM, ItemIndex: 0, SubIndex: -1})
	if b.bankMode || b.pendingROM != "" || zx.memory.rom[0][0] != was {
		t.Error("choosing a folder as a whole did something")
	}
}

// A folder's ROMs need a machine, and so do the ROMs of the dropdown; the links do not.
func TestCustomROMFolderChoicesNeedAMachine(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	b.romDir = t.TempDir()
	touchROMs(t, b.romDir, "mods/a.rom")
	b.refreshCustomROMItems(zx)
	if !b.needsMachine(zenui.MenuBarSelection{BarIndex: barCustomROM, ItemIndex: 0, SubIndex: 0}) {
		t.Error("a ROM in a folder is allowed with no machine running")
	}
}

// Too many ROMs for the window put the rest in "More...", which still reaches each of them.
func TestCustomROMMenuPutsTheRestUnderMore(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	b.romDir = t.TempDir()
	var names []string
	for _, n := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		names = append(names, n+".rom")
	}
	touchROMs(t, b.romDir, names...)
	b.refreshCustomROMItems(zx)

	// with no window the dropdown has a dozen rows: half of them are the most it shows
	rows := romMenuRows(b)
	if len(rows) != 5+1+2 || rows[5] != "More..." || rows[6] != "-" || rows[7] != "Download more ROMs" {
		t.Fatalf("the dropdown is %q", rows)
	}
	if got, want := subLabels(b.widget.ItemsFor(barCustomROM)[5]), []string{"f.rom", "g.rom", "h.rom"}; !reflect.DeepEqual(got, want) {
		t.Errorf("More... holds %q, want %q", got, want)
	}
	path, ok := romAt(b.romRows, 5, 2)
	if !ok || path != "h.rom" {
		t.Errorf("the last ROM under More... is %q, %v", path, ok)
	}
	if b.romMoreAt != 7 {
		t.Errorf("the links' row is %d, want 7", b.romMoreAt)
	}
}
