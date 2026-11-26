//go:build !headless

package zenzx

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ha1tch/zenui"
)

// With a rom-archives.json in custom-roms/, the Download more ROMs menu is what it lists, and its
// rows open those pages.
func TestCustomROMLinksComeFromTheArchivesFile(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	b.romDir = t.TempDir()
	touchROMs(t, b.romDir, "a.rom")
	writeArchives(t, b.romDir, `[
		{"text_label": "First", "url": "https://one.example/"},
		{"text_label": "Second", "url": "https://two.example/page"},
		{"text_label": "Third", "url": "http://three.example/"}
	]`)
	opened := openedURLs(t)
	b.refreshCustomROMItems(zx)

	if got, want := romMenuRows(b), []string{"a.rom", "-", "Download more ROMs"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("the dropdown is %q, want %q", got, want)
	}
	if got, want := subLabels(b.widget.ItemsFor(barCustomROM)[b.romMoreAt]), []string{"First", "Second", "Third"}; !reflect.DeepEqual(got, want) {
		t.Errorf("the submenu is %q, want %q", got, want)
	}
	for i := 0; i < 3; i++ {
		b.pickCustomROM(zx, zenui.MenuBarSelection{BarIndex: barCustomROM, ItemIndex: b.romMoreAt, SubIndex: i})
	}
	if want := []string{"https://one.example/", "https://two.example/page", "http://three.example/"}; !reflect.DeepEqual(*opened, want) {
		t.Errorf("opened %q, want %q", *opened, want)
	}
}

// An empty list takes away the separator and the row; the ROMs, and picking one, are as before.
func TestCustomROMMenuHasNoLinksForAnEmptyArchivesFile(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	b.romDir = t.TempDir()
	touchROMs(t, b.romDir, "a.rom")
	writeArchives(t, b.romDir, "[]")
	opened := openedURLs(t)
	b.refreshCustomROMItems(zx)

	if got, want := romMenuRows(b), []string{"a.rom"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("the dropdown is %q, want %q", got, want)
	}
	if b.romMoreAt != 0 {
		t.Errorf("the links' row is %d with no links", b.romMoreAt)
	}
	// the row of a ROM is not taken for the links, whatever its index
	b.pickCustomROM(zx, zenui.MenuBarSelection{BarIndex: barCustomROM, ItemIndex: 0, SubIndex: 0})
	if len(*opened) != 0 {
		t.Errorf("opened %q", *opened)
	}

	// with no ROMs either, the placeholder alone
	b.romDir = t.TempDir()
	writeArchives(t, b.romDir, "[]")
	b.refreshCustomROMItems(zx)
	if got, want := romMenuRows(b), []string{"No custom ROMs found"}; !reflect.DeepEqual(got, want) {
		t.Errorf("no ROMs and no links: %q, want %q", got, want)
	}
}

// A file that cannot be used leaves the menu as it is without one.
func TestCustomROMLinksFallBackToTheStandardOnes(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	b.romDir = t.TempDir()
	writeArchives(t, b.romDir, "not json at all")
	b.refreshCustomROMItems(zx)
	if got, want := subLabels(b.widget.ItemsFor(barCustomROM)[b.romMoreAt]), []string{"Rodolfo Guerra ROM mods", "Speccy4ever ROM archive"}; !reflect.DeepEqual(got, want) {
		t.Errorf("the submenu is %q, want %q", got, want)
	}
}

// Editing the file shows in the menu the next time it is built; nothing is read again when it has
// not changed.
func TestCustomROMLinksFollowTheFile(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	b.romDir = t.TempDir()
	b.refreshCustomROMItems(zx)
	if n := len(b.widget.ItemsFor(barCustomROM)[b.romMoreAt].SubItems); n != 2 {
		t.Fatalf("with no file: %d links, want 2", n)
	}

	writeArchives(t, b.romDir, `[{"text_label": "Only", "url": "https://only.example/"}]`)
	b.refreshCustomROMItems(zx)
	if got := subLabels(b.widget.ItemsFor(barCustomROM)[b.romMoreAt]); !reflect.DeepEqual(got, []string{"Only"}) {
		t.Errorf("after writing the file: %q", got)
	}

	// not read again while it is unchanged: the same slice comes back
	first := b.archives
	b.refreshCustomROMItems(zx)
	if len(first) == 0 || &first[0] != &b.archives[0] {
		t.Error("an unchanged file was read again")
	}

	// a change of size is seen even within one modification time
	p := filepath.Join(b.romDir, ROMArchivesFile)
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	writeArchives(t, b.romDir, `[{"text_label": "Only", "url": "https://only.example/"}, {"text_label": "More", "url": "https://more.example/"}]`)
	if err := os.Chtimes(p, st.ModTime(), st.ModTime()); err != nil {
		t.Fatal(err)
	}
	b.refreshCustomROMItems(zx)
	if got := subLabels(b.widget.ItemsFor(barCustomROM)[b.romMoreAt]); !reflect.DeepEqual(got, []string{"Only", "More"}) {
		t.Errorf("after growing the file: %q", got)
	}

	// and removing it brings back the standard links
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	b.refreshCustomROMItems(zx)
	if n := len(b.widget.ItemsFor(barCustomROM)[b.romMoreAt].SubItems); n != 2 {
		t.Errorf("after removing the file: %d links, want 2", n)
	}
}

// More links than fit under their row are cut to the ones that do, never fewer than a few.
func TestCustomROMLinksAreCutToWhatFits(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	b.romDir = t.TempDir()
	touchROMs(t, b.romDir, "a.rom")
	text := "["
	for i := 0; i < 40; i++ {
		if i > 0 {
			text += ","
		}
		text += fmt.Sprintf(`{"text_label": "Site %02d", "url": "https://s%02d.example/"}`, i, i)
	}
	writeArchives(t, b.romDir, text+"]")
	opened := openedURLs(t)
	b.refreshCustomROMItems(zx)

	subs := subLabels(b.widget.ItemsFor(barCustomROM)[b.romMoreAt])
	room := b.romRowsFit() - b.romMoreAt
	if room < minROMRows {
		room = minROMRows
	}
	if len(subs) != room || subs[0] != "Site 00" || len(b.romLinks) != room {
		t.Errorf("%d links shown, want %d (first %q)", len(subs), room, subs[0])
	}
	last := len(subs) - 1
	b.pickCustomROM(zx, zenui.MenuBarSelection{BarIndex: barCustomROM, ItemIndex: b.romMoreAt, SubIndex: last})
	if want := fmt.Sprintf("https://s%02d.example/", last); len(*opened) != 1 || (*opened)[0] != want {
		t.Errorf("the last link opened %q, want %q", *opened, want)
	}
}
