package zenzx

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// customROMDepth is how many levels of subfolders of custom-roms/ are looked into: the folders
// of custom-roms/ (the first level) and the folders of those (the second). A ROM deeper than that
// is not listed, and a folder with no ROM in it, down to that depth, is not shown.
const customROMDepth = 2

// romFolder is a folder of custom-roms/ that has ROMs in it, down to customROMDepth.
type romFolder struct {
	// Name is the folder's name.
	Name string
	// ROMs are the paths of its ROMs under custom-roms/, with slashes whatever the system: the
	// folder's own, then those of its folders, each sorted.
	ROMs []string
}

// readROMDir is the .rom files and the folders directly inside dir, each sorted by name. A
// folder whose name begins with a dot is left out (.git, say), and a link to a folder is a
// folder. A directory that cannot be read has nothing in it.
func readROMDir(dir string) (roms, folders []string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}
	for _, e := range entries {
		name := e.Name()
		isDir := e.IsDir()
		if !isDir && e.Type()&fs.ModeSymlink != 0 {
			if st, err := os.Stat(filepath.Join(dir, name)); err == nil && st.IsDir() {
				isDir = true
			}
		}
		switch {
		case isDir:
			if !strings.HasPrefix(name, ".") {
				folders = append(folders, name)
			}
		case strings.EqualFold(filepath.Ext(name), ".rom"):
			roms = append(roms, name)
		}
	}
	sort.Strings(roms)
	sort.Strings(folders)
	return roms, folders
}

// scanCustomROMs reads custom-roms/: the .rom files directly in dir, and its folders that have
// ROMs in them, as far down as customROMDepth. A missing or empty dir has nothing.
func scanCustomROMs(dir string) (roms []string, folders []romFolder) {
	roms, dirs := readROMDir(dir)
	for _, d := range dirs {
		f := romFolder{Name: d}
		walkROMFolder(filepath.Join(dir, d), d, 1, &f)
		if len(f.ROMs) > 0 {
			folders = append(folders, f)
		}
	}
	return roms, folders
}

// walkROMFolder adds to f the ROMs of the folder at path (rel under custom-roms/, level deep),
// then those of its folders, while level is short of customROMDepth.
func walkROMFolder(path, rel string, level int, f *romFolder) {
	roms, subs := readROMDir(path)
	for _, r := range roms {
		f.ROMs = append(f.ROMs, rel+"/"+r)
	}
	if level >= customROMDepth {
		return
	}
	for _, s := range subs {
		walkROMFolder(filepath.Join(path, s), rel+"/"+s, level+1, f)
	}
}

// romRow is a row of the Custom ROM dropdown: a ROM, or a submenu of ROMs.
type romRow struct {
	// Label is what the row says.
	Label string
	// Path is a ROM's path under custom-roms/, with slashes; empty for a submenu.
	Path string
	// Folder is, for a submenu that is a folder's, the folder's name.
	Folder string
	// Subs are a submenu's rows, all of them ROMs.
	Subs []romRow
}

// isMenu is whether the row opens a submenu rather than being a ROM.
func (r romRow) isMenu() bool { return r.Path == "" }

// minROMRows is the fewest rows a page of a submenu is cut to, whatever the screen.
const minROMRows = 4

// buildROMRows makes the rows of the Custom ROM dropdown from what scanCustomROMs found: a submenu
// for each folder, then the ROMs of custom-roms/ itself. fit is how many rows of the dropdown's
// height the screen has below the bar. A submenu opens level with its row and goes down from there,
// so a folder's ROMs are cut into pages of the rows that are left below its row, each page a
// submenu of its own ("Name", "Name (2)"); and when the dropdown has more than half of fit rows, the
// rows from there on go into "More..." submenus (a folder's ROMs among them with the folder's name
// before each: a submenu holds ROMs, not submenus), so that a "More..." row, which comes last, still
// has room under it.
func buildROMRows(roms []string, folders []romFolder, fit int) []romRow {
	if fit < 2*minROMRows {
		fit = 2 * minROMRows
	}
	room := func(at int) int { // the rows a submenu opened by the row at index at can hold
		if n := fit - at; n > minROMRows {
			return n
		}
		return minROMRows
	}
	var rows []romRow
	for _, f := range folders {
		leaves := make([]romRow, len(f.ROMs))
		for i, p := range f.ROMs {
			leaves[i] = romRow{Label: strings.TrimPrefix(p, f.Name+"/"), Path: p}
		}
		for page := 1; len(leaves) > 0; page++ {
			n := room(len(rows))
			if n > len(leaves) {
				n = len(leaves)
			}
			label := f.Name
			if page > 1 {
				label = fmt.Sprintf("%s (%d)", f.Name, page)
			}
			rows = append(rows, romRow{Label: label, Folder: f.Name, Subs: leaves[:n]})
			leaves = leaves[n:]
		}
	}
	for _, r := range roms {
		rows = append(rows, romRow{Label: r, Path: r})
	}

	top := fit / 2
	if top < minROMRows+2 {
		top = minROMRows + 2
	}
	if len(rows) <= top {
		return rows
	}
	keep := top - 1
	var rest []romRow
	for _, r := range rows[keep:] {
		if !r.isMenu() {
			rest = append(rest, r)
			continue
		}
		for _, l := range r.Subs {
			rest = append(rest, romRow{Label: r.Folder + "/" + l.Label, Path: l.Path})
		}
	}
	out := append([]romRow(nil), rows[:keep]...)
	for page := 1; len(rest) > 0; page++ {
		n := room(len(out))
		if n > len(rest) {
			n = len(rest)
		}
		label := "More..."
		if page > 1 {
			label = fmt.Sprintf("More... (%d)", page)
		}
		out = append(out, romRow{Label: label, Subs: rest[:n]})
		rest = rest[n:]
	}
	return out
}

// romAt is the ROM that a choice in the Custom ROM dropdown (an index among rows, and an index in
// the submenu of a row that has one, else -1) names; ok is false for a row that is no ROM, a
// submenu chosen as a whole, or an index out of range.
func romAt(rows []romRow, item, sub int) (path string, ok bool) {
	if item < 0 || item >= len(rows) {
		return "", false
	}
	r := rows[item]
	if r.isMenu() {
		if sub < 0 || sub >= len(r.Subs) {
			return "", false
		}
		r = r.Subs[sub]
	}
	return r.Path, r.Path != ""
}
