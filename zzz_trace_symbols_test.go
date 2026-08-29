//go:build headless

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// symTable maps address to label name, loaded from a pasmo-format
// symbol file -- the format zenas emits via --sym[=path], confirmed
// directly against a real zenas build this session:
//
//	label\t\tEQU 0XXXXH
//
// one symbol per line. Malformed individual lines are skipped rather
// than failing the whole load -- a hand-edited or partially-generated
// .sym file shouldn't make annotation unavailable for every symbol it
// does parse correctly.
type symTable map[uint16]string

func loadSymFile(path string) (symTable, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	tbl := make(symTable)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 || !strings.EqualFold(fields[1], "EQU") {
			continue
		}
		hexPart := strings.TrimSuffix(strings.TrimSuffix(fields[2], "H"), "h")
		v, err := strconv.ParseUint(hexPart, 16, 16)
		if err != nil {
			continue
		}
		tbl[uint16(v)] = fields[0]
	}
	return tbl, nil
}

// annotate formats an address as plain hex, or hex plus a bracketed
// label when the symbol table has one at exactly that address -- the
// raw hex is never replaced, only appended to, so a consumer that
// doesn't care about symbols still gets the same field it always did
// as a prefix.
func (t symTable) annotate(addr uint16) string {
	if t != nil {
		if name, ok := t[addr]; ok {
			return fmt.Sprintf("%04X[%s]", addr, name)
		}
	}
	return fmt.Sprintf("%04X", addr)
}
