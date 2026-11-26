//go:build !headless

package zenzx

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ha1tch/zenui"
)

// romMenuRows is the Custom ROM dropdown as it is shown: a label per row, "-" for a separator.
func romMenuRows(b *appMenuBar) []string {
	var got []string
	for _, it := range b.widget.ItemsFor(barCustomROM) {
		if it.Separator {
			got = append(got, "-")
		} else {
			got = append(got, it.Label)
		}
	}
	return got
}

// openedURLs replaces openURL for the length of a test and says what was opened.
func openedURLs(t *testing.T) *[]string {
	t.Helper()
	var opened []string
	was := openURL
	openURL = func(u string) { opened = append(opened, u) }
	t.Cleanup(func() { openURL = was })
	return &opened
}

func TestCustomROMMenuEndsWithASeparatorAndDownloadMoreROMs(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	b.romDir = t.TempDir()

	// no ROMs: the placeholder, then the row of links
	b.refreshCustomROMItems(zx)
	if got, want := romMenuRows(b), []string{"No custom ROMs found", "-", "Download more ROMs"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("with no ROMs: %q, want %q", got, want)
	}
	if b.romMoreAt != 2 {
		t.Errorf("the links' row is %d, want 2", b.romMoreAt)
	}

	// ROMs: them, then the separator and the row, which stays last
	for _, n := range []string{"b.rom", "a.rom"} {
		if err := os.WriteFile(filepath.Join(b.romDir, n), []byte{0}, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	b.refreshCustomROMItems(zx)
	rows := romMenuRows(b)
	if len(rows) != 4 || rows[2] != "-" || rows[3] != "Download more ROMs" || b.romMoreAt != 3 {
		t.Fatalf("with two ROMs: %q, links' row %d", rows, b.romMoreAt)
	}
	more := b.widget.ItemsFor(barCustomROM)[3]
	var subs []string
	for _, s := range more.SubItems {
		subs = append(subs, s.Label)
	}
	if want := []string{"Rodolfo Guerra ROM mods", "Speccy4ever ROM archive"}; !reflect.DeepEqual(subs, want) {
		t.Errorf("the submenu is %q, want %q", subs, want)
	}
}

func TestCustomROMMenuLinksOpenTheirPages(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	b.romDir = t.TempDir()
	opened := openedURLs(t)
	b.refreshCustomROMItems(zx)

	b.pickCustomROM(zx, zenui.MenuBarSelection{BarIndex: barCustomROM, ItemIndex: b.romMoreAt, SubIndex: 0})
	b.pickCustomROM(zx, zenui.MenuBarSelection{BarIndex: barCustomROM, ItemIndex: b.romMoreAt, SubIndex: 1})
	want := []string{"https://sites.google.com/view/rodolfoguerra/", "https://speccy4ever.speccy.org/"}
	if !reflect.DeepEqual(*opened, want) {
		t.Errorf("opened %q, want %q", *opened, want)
	}

	// the row itself, and a submenu choice that is not one, open nothing
	*opened = nil
	b.pickCustomROM(zx, zenui.MenuBarSelection{BarIndex: barCustomROM, ItemIndex: b.romMoreAt, SubIndex: -1})
	b.pickCustomROM(zx, zenui.MenuBarSelection{BarIndex: barCustomROM, ItemIndex: b.romMoreAt, SubIndex: 2})
	// and the placeholder is not a link
	b.pickCustomROM(zx, zenui.MenuBarSelection{BarIndex: barCustomROM, ItemIndex: 0, SubIndex: -1})
	if len(*opened) != 0 {
		t.Errorf("opened %q for what is no link", *opened)
	}
	if b.bankMode || b.pendingROM != "" {
		t.Error("a link started a ROM pick")
	}
}

func TestCustomROMLinksNeedNoMachineButROMsDo(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	b.romDir = t.TempDir()
	if err := os.WriteFile(filepath.Join(b.romDir, "a.rom"), []byte{0}, 0o644); err != nil {
		t.Fatal(err)
	}
	b.refreshCustomROMItems(zx)
	link := zenui.MenuBarSelection{BarIndex: barCustomROM, ItemIndex: b.romMoreAt, SubIndex: 1}
	rom := zenui.MenuBarSelection{BarIndex: barCustomROM, ItemIndex: 0, SubIndex: -1}
	if b.needsMachine(link) {
		t.Error("a link to a page of ROMs is refused with no machine running")
	}
	if !b.needsMachine(rom) {
		t.Error("a ROM is allowed with no machine running")
	}
}

func TestCustomROMBankChoiceHasNoLinks(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	b.romDir = t.TempDir()
	b.refreshCustomROMItems(zx)
	b.bankMode, b.pendingROM = true, "a.rom"
	b.refreshCustomROMItems(zx)
	for _, r := range romMenuRows(b) {
		if r == "-" || r == "Download more ROMs" {
			t.Errorf("the bank choice has %q among its rows", r)
		}
	}
	if b.romMoreAt != 0 {
		t.Errorf("a bank row would be taken for the links: %d", b.romMoreAt)
	}
	// and a bank numbered like the old row of links is a bank
	opened := openedURLs(t)
	b.pickCustomROM(zx, zenui.MenuBarSelection{BarIndex: barCustomROM, ItemIndex: 2, SubIndex: 0})
	if len(*opened) != 0 {
		t.Errorf("a bank choice opened %q", *opened)
	}
}
