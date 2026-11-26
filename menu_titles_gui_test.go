//go:build !headless

package zenzx

import (
	"testing"

	"github.com/ha1tch/zenui"
)

// titledSections checks that a menu in sections has a title where each section begins: the first
// row, and the row after each separator.
func titledSections(t *testing.T, name string, items []zenui.Item) {
	t.Helper()
	if len(items) == 0 {
		t.Errorf("%s has no rows", name)
		return
	}
	if !items[0].Title {
		t.Errorf("%s: its first row %q is no title", name, items[0].Label)
	}
	for i, it := range items {
		if it.Separator && (i+1 >= len(items) || !items[i+1].Title) {
			t.Errorf("%s: the section after the separator at row %d has no title", name, i)
		}
	}
}

// The menus that are in sections (a separator between groups of rows) open each section with a
// title, as the Help and Storage menus do: Machine's first group, Tape, Snapshot, View, Theme and
// its Font submenu. The File menu is one group and the Quit row; Custom ROM's rows are the ROMs
// there are, so neither has titles.
func TestMenusInSectionsHaveTitledSections(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	b.refreshMachineItems(zx)
	b.refreshTapeItems(zx)
	b.refreshViewItems(zx)

	titledSections(t, "Tape", b.widget.ItemsFor(barTape))
	titledSections(t, "Snapshot", b.widget.ItemsFor(barSnapshot))
	titledSections(t, "View", b.widget.ItemsFor(barView))
	theme := b.widget.ItemsFor(barTheme)
	titledSections(t, "Theme", theme)
	titledSections(t, "Font", theme[b.themeFontAt].SubItems)
	titledSections(t, "Machine", b.widget.ItemsFor(barMachine)[:1]) // its head; the model groups are titled by machines.json

	// the model groups of the Machine menu keep their own titles, one after each separator
	machine := b.widget.ItemsFor(barMachine)
	for i, it := range machine {
		if it.Separator && (i+1 >= len(machine) || !machine[i+1].Title) {
			t.Errorf("Machine: the group after the separator at row %d has no title", i)
		}
	}
	if !machine[0].Title || machine[0].Label != "Machine" {
		t.Errorf("Machine: its first row is %+v, want the title Machine", machine[0])
	}
}

// The File menu has no separator above Preferences: Open, Export and Preferences are one group,
// and Quit is set apart.
func TestFileMenuHasNoSeparatorAbovePreferences(t *testing.T) {
	b, _ := newTestBar(t, nil, "")
	items := b.widget.ItemsFor(barFile)
	for i, it := range items {
		if it.Label == "Preferences..." && (i == 0 || items[i-1].Separator) {
			t.Errorf("Preferences... is at row %d, below a separator or first", i)
		}
	}
}
