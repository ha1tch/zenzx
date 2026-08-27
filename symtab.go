package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ============================================================================
// Symbol tables
//
// A .sym file (emitted alongside a .bin by both pasmo and zenas) maps label
// names to their assembled addresses, one per line:
//
//	cur_map		EQU 088BAH
//	kn_x		EQU 0C5BBH
//
// zenscript's wait-mem and expect-mem verbs resolve a symbol name against a
// table loaded by the 'sym' verb, so scripts can refer to game-state
// variables by name instead of hardcoding addresses that shift whenever the
// assembled program changes size.
// ============================================================================

// SymbolTable is a name -> address map loaded from a .sym file.
type SymbolTable map[string]uint16

// LoadSymbolTable parses a pasmo/zenas-format .sym file. Each line is
// whitespace-separated fields; a line is a symbol definition only if its
// second field (case-insensitively) is EQU and its third field is a
// hexadecimal value with a trailing H (with or without a leading 0). Lines
// that don't match this shape are skipped, not errored -- a .sym file can
// carry other directives this parser does not need to understand.
func LoadSymbolTable(path string) (SymbolTable, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open symbol file: %w", err)
	}
	defer f.Close()

	tab := SymbolTable{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(strings.ReplaceAll(sc.Text(), "\t", " "))
		if len(fields) < 3 || !strings.EqualFold(fields[1], "EQU") {
			continue
		}
		hex := strings.TrimSuffix(strings.ToUpper(fields[2]), "H")
		v, err := strconv.ParseUint(hex, 16, 32)
		if err != nil || v > 0xFFFF {
			continue // not a plain hex EQU value; not this parser's concern
		}
		tab[fields[0]] = uint16(v)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read symbol file: %w", err)
	}
	return tab, nil
}

// Resolve looks up name in the table. If the table is nil (no 'sym' verb has
// run yet) or name isn't found, it falls back to parsing name as a literal
// numeric address via ParseAddr -- so wait-mem/expect-mem work with either a
// symbol or a raw address, with or without a loaded symbol table.
func (t SymbolTable) Resolve(name string) (uint16, error) {
	if t != nil {
		if addr, ok := t[name]; ok {
			return addr, nil
		}
	}
	addr, err := ParseAddr(name)
	if err != nil {
		return 0, fmt.Errorf("unknown symbol or invalid address %q", name)
	}
	return addr, nil
}
