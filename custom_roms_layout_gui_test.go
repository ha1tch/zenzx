//go:build !headless

package zenzx

import (
	"reflect"
	"testing"
)

// The ROMs that ship in custom-roms/ are in two folders, each a submenu of the Custom ROM menu, and
// none is loose in the folder.
func TestShippedCustomROMsAreInTheirFolders(t *testing.T) {
	want := []string{
		"Microdigital/TK90X-es-UY.rom",
		"Microdigital/TK90X-pt-BR.rom",
		"Microdigital/TK90X.rom",
		"Microdigital/TK95-es.rom",
		"Microdigital/TK95-pt.rom",
		"Zenpoint Project/zenpoint-0.1.3-en.rom",
		"Zenpoint Project/zenpoint-0.1.3-es.rom",
	}
	got := listCustomROMs(CustomROMDir)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("custom-roms/ lists %q, want %q", got, want)
	}

	b, zx := newTestBar(t, nil, "")
	b.refreshCustomROMItems(zx)
	subs := map[string]int{}
	for _, it := range b.widget.ItemsFor(barCustomROM) {
		if len(it.SubItems) > 0 {
			subs[it.Label] = len(it.SubItems)
		}
	}
	if subs["Microdigital"] != 5 || subs["Zenpoint Project"] != 2 {
		t.Errorf("the Custom ROM menu's submenus (and their rows) are %v, want Microdigital 5 and Zenpoint Project 2", subs)
	}
}
