package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// CustomROMDir is the default location scanned by -custom-roms-menu.
// Intended for language-translated and other alternate ROM variants
// for international users -- distinct from rom/, which holds the
// standard, embedded set (embedded_roms.go).
const CustomROMDir = "custom-roms"

// listCustomROMs returns every .rom file directly inside dir, sorted by
// name. A missing or empty directory returns an empty slice, not an
// error -- custom-roms/ having nothing in it yet is a perfectly normal
// state, not a failure.
func listCustomROMs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(e.Name()), ".rom") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// runCustomROMMenu lists every ROM in dir, prompts (via in) for a
// selection and which bank to place it in, and applies it through
// zx.OverrideROMBank -- the interactive counterpart to already knowing
// the exact filename and bank number up front via -rom0..-rom3.
// Writes all prompts and messages to out. Returns with an explanatory
// message rather than an error when there's nothing to select, the
// user skips, or the input given isn't valid -- none of these should
// abort the emulator, they should just leave the standard ROM set in
// place.
func runCustomROMMenu(zx *ZenZX, dir string, in *bufio.Reader, out *os.File) {
	names := listCustomROMs(dir)
	if len(names) == 0 {
		fmt.Fprintf(out, "No .rom files found in %s -- nothing to select, keeping the standard ROM set.\n", dir)
		return
	}

	fmt.Fprintf(out, "\nCustom ROMs available in %s:\n", dir)
	for i, name := range names {
		size := "?"
		if info, err := os.Stat(filepath.Join(dir, name)); err == nil {
			size = fmt.Sprintf("%d bytes", info.Size())
		}
		fmt.Fprintf(out, "  %d. %s (%s)\n", i+1, name, size)
	}
	fmt.Fprintln(out, "  0. Skip -- keep the standard ROM set")
	fmt.Fprint(out, "Select a custom ROM: ")

	choice, ok := readInt(in, 0, len(names))
	if !ok {
		fmt.Fprintln(out, "Not a valid selection -- keeping the standard ROM set.")
		return
	}
	if choice == 0 {
		fmt.Fprintln(out, "Skipped -- keeping the standard ROM set.")
		return
	}

	selected := names[choice-1]
	path := filepath.Join(dir, selected)

	bank := 0
	if maxBank := zx.maxROMBank(); maxBank > 0 {
		fmt.Fprintf(out, "Which ROM bank should %s replace? (0-%d): ", selected, maxBank)
		b, ok := readInt(in, 0, maxBank)
		if !ok {
			fmt.Fprintln(out, "Not a valid bank -- defaulting to bank 0.")
		} else {
			bank = b
		}
	}

	if err := zx.OverrideROMBank(bank, path); err != nil {
		fmt.Fprintf(out, "Error applying %s to bank %d: %v\n", selected, bank, err)
		return
	}
	fmt.Fprintf(out, "Loaded %s into ROM bank %d.\n", selected, bank)
}

// readInt reads one line from in, parsing it as an integer in
// [min, max] inclusive. The bool is false for anything that isn't a
// valid integer in range, letting the caller give a specific message
// rather than readInt guessing what the caller wants said.
func readInt(in *bufio.Reader, min, max int) (int, bool) {
	line, _ := in.ReadString('\n')
	line = strings.TrimSpace(line)
	n, err := strconv.Atoi(line)
	if err != nil || n < min || n > max {
		return 0, false
	}
	return n, true
}
