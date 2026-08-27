package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

// embeddedROMs bundles every .rom file in rom/ directly into the
// compiled binary, so a distributed release no longer needs a separate
// rom/ folder shipped alongside it -- the standard ROM set is always
// available. Documentation (.txt), reference hex dumps (.rom.hex), and
// the Z80 CPU exerciser test programs (zexall/zexdoc .com/.z80) are
// deliberately not part of this pattern: they aren't consulted by
// -model's own loading path, so embedding them would only grow the
// binary for no runtime benefit.
//
//go:embed rom/*.rom
var embeddedROMs embed.FS

// resolveROMBytes returns a named ROM's bytes, checking the real
// filesystem first (romDir/name) and falling back to the embedded copy
// only if that file isn't found on disk. This ordering is deliberate,
// not arbitrary: filesystem-first keeps every existing local
// development and test workflow that already points -romdir at a real
// rom/ checkout working completely unchanged, while still making a
// distributed binary with no rom/ folder alongside it self-sufficient
// via the embed fallback.
func resolveROMBytes(name, romDir string) ([]byte, error) {
	diskPath := filepath.Join(romDir, name)
	if data, err := os.ReadFile(diskPath); err == nil {
		return data, nil
	}

	data, err := embeddedROMs.ReadFile("rom/" + name)
	if err != nil {
		return nil, fmt.Errorf("%s: not found on disk at %s, and not in the embedded set", name, diskPath)
	}
	return data, nil
}

// loadPlus3Bytes resolves all four banks of a +3-style ROM set (used by
// plus2a, plus3, and spanishplus3, which all share this 4-bank shape)
// in one call, returning as soon as any one of them fails to resolve
// rather than partially resolving and leaving the caller to notice.
// Lives here (no build tag) rather than in zenzx_headless.go, since
// both zenzx_headless.go (headless) and zenzx_gui.go (!headless) need
// it and their own build tags are mutually exclusive.
func loadPlus3Bytes(romDir, name0, name1, name2, name3 string) ([4][]byte, error) {
	var out [4][]byte
	for i, name := range []string{name0, name1, name2, name3} {
		data, err := resolveROMBytes(name, romDir)
		if err != nil {
			return out, err
		}
		out[i] = data
	}
	return out, nil
}
