package zenzx

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// A standalone program is the program that exported it followed by the machine (standalone.go).
// These test the file, and then the claim that matters: a machine restored from a program is the
// machine that was exported, down to every field the state codec dumps, and goes on as it would.

func testProgram(model string) *Program {
	return &Program{Model: model, Title: "game", Version: "1.2.3", Snapshot: []byte("a machine, as far as the container knows")}
}

func fakeStub(n int) []byte { return bytes.Repeat([]byte("\x7fELF stub "), n) }

func TestProgramPayloadRoundTrips(t *testing.T) {
	stub := fakeStub(50)
	tail, err := testProgram("128k").payload()
	if err != nil {
		t.Fatal(err)
	}
	file := append(append([]byte{}, stub...), tail...)
	p, programLen, err := readProgram(bytes.NewReader(file), int64(len(file)))
	if err != nil {
		t.Fatal(err)
	}
	if programLen != int64(len(stub)) {
		t.Errorf("the program ends at %d, want %d", programLen, len(stub))
	}
	if p.Model != "128k" || p.Title != "game" || p.Version != "1.2.3" || string(p.Snapshot) != string(testProgram("").Snapshot) {
		t.Errorf("read back %+v", p)
	}
}

func TestAFileWithoutAMachineSaysSo(t *testing.T) {
	for _, file := range [][]byte{nil, []byte("short"), fakeStub(100)} {
		if _, n, err := readProgram(bytes.NewReader(file), int64(len(file))); !errors.Is(err, errNoProgram) || n != int64(len(file)) {
			t.Errorf("%d bytes: got length %d, error %v; want the whole file and errNoProgram", len(file), n, err)
		}
	}
}

func TestADamagedMachineIsRefusedNotMistakenForNone(t *testing.T) {
	tail, _ := testProgram("48k").payload()
	good := append(fakeStub(20), tail...)
	flips := map[string]func([]byte){
		"a byte of the snapshot": func(b []byte) { b[len(b)-trailerSize-sha256Size-3] ^= 0xFF },
		"the checksum":           func(b []byte) { b[len(b)-trailerSize-1] ^= 0xFF },
		"the length":             func(b []byte) { b[len(b)-trailerSize] ^= 0x7F },
		"the beginning":          func(b []byte) { b[len(b)-len(tail)] ^= 0xFF },
	}
	for name, damage := range flips {
		file := append([]byte{}, good...)
		damage(file)
		_, _, err := readProgram(bytes.NewReader(file), int64(len(file)))
		if err == nil || errors.Is(err, errNoProgram) {
			t.Errorf("damage to %s: err = %v, want a refusal that is not errNoProgram", name, err)
		}
	}
	// A machine this ZenZX does not know is refused as well, however well it is packed.
	odd, _ := testProgram("zx81").payload()
	file := append(fakeStub(20), odd...)
	if _, _, err := readProgram(bytes.NewReader(file), int64(len(file))); err == nil || errors.Is(err, errNoProgram) {
		t.Errorf("a zx81: err = %v, want a refusal", err)
	}
}

// sha256Size is the checksum's length, for the damage the test does to it.
const sha256Size = 32

func TestExportMakesTheProgramAndTakesOffTheOldMachine(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "zenzx")
	if err := os.WriteFile(stub, fakeStub(200), 0o755); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(dir, "first")
	if err := ExportProgram(stub, first, testProgram("48k")); err != nil {
		t.Fatal(err)
	}
	// Exporting from a program that carries a machine takes that machine off first.
	second := filepath.Join(dir, "second")
	p2 := testProgram("plus3")
	p2.Snapshot = []byte("another machine")
	if err := ExportProgram(first, second, p2); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{first, second} {
		b, _ := os.ReadFile(f)
		if _, n, err := readProgram(bytes.NewReader(b), int64(len(b))); err != nil || n != int64(len(fakeStub(200))) {
			t.Errorf("%s: program ends at %d (err %v), want the stub's own %d: nothing of an older machine is left", filepath.Base(f), n, err, len(fakeStub(200)))
		}
	}
	b, _ := os.ReadFile(second)
	if p, _, _ := readProgram(bytes.NewReader(b), int64(len(b))); p == nil || p.Model != "plus3" || string(p.Snapshot) != "another machine" {
		t.Errorf("the second program carries %+v", p)
	}
	if st, err := os.Stat(second); err != nil || st.Mode().Perm()&0o100 == 0 {
		t.Errorf("the program is not executable: %v %v", st, err)
	}
	if ents, _ := os.ReadDir(dir); len(ents) != 3 {
		t.Errorf("%d files in the folder, want the stub and the two programs only", len(ents))
	}
}

func TestExportRefusesWhatItCannotMake(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "zenzx")
	os.WriteFile(stub, fakeStub(10), 0o755)
	if err := ExportProgram(stub, stub, testProgram("48k")); err == nil {
		t.Error("a program was written over the one it is made from")
	}
	if err := ExportProgram(stub, filepath.Join(dir, "o"), testProgram("next")); err == nil {
		t.Error("a Next was exported")
	}
	if err := ExportProgram(stub, filepath.Join(dir, "o"), &Program{Model: "48k"}); err == nil {
		t.Error("an empty machine was exported")
	}
	if err := ExportProgram(filepath.Join(dir, "none"), filepath.Join(dir, "o"), testProgram("48k")); err == nil {
		t.Error("a program was made from a stub that is not there")
	}
	if _, err := os.Stat(filepath.Join(dir, "o")); err == nil {
		t.Error("a refused export left a file behind")
	}
}

// restoreCompare lists the fields of the machine dumps that differ, leaving out what is the
// host's and not the machine's: where the frame timeline stands, the border's history, an
// object's identity in the dump, and the counter a renderer's cache is keyed on. The frame and
// cycle counts are the host's tally of time since it started, so they are compared as the
// position inside a frame (cycleCount modulo the frame's length). Straight after the
// restore, the CPU's bookkeeping for the instruction it last ran (how many bytes it read, how
// many cycles it took) is left out: a snapshot is taken between instructions and does not
// carry it, and nothing reads it before the next instruction writes it; the run-on comparison
// shows it makes no difference.
func restoreCompare(a, b map[string]string, frameLen int64, atOnce bool) []string {
	inFrame := func(m map[string]string) string {
		var cycles int64
		fmt.Sscan(m["root0.cycleCount"], &cycles)
		return fmt.Sprint(cycles % frameLen)
	}
	a, b = cloneDump(a), cloneDump(b)
	a["root0.cycleCount"], b["root0.cycleCount"] = inFrame(a), inFrame(b)
	delete(a, "root0.frameCount")
	delete(b, "root0.frameCount")
	if atOnce {
		for _, k := range []string{"accessOffset", "contentionActive", "instrReadIndex", "lastCycles"} {
			delete(a, "root0.cpu."+k)
			delete(b, "root0.cpu."+k)
		}
	}
	var out []string
	keys := map[string]bool{}
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	for k := range keys {
		av, bv := a[k], b[k]
		if av == bv || strings.HasPrefix(av, "#") || strings.HasPrefix(bv, "#") ||
			k == "root0.frameOrigin" || strings.Contains(k, "borderHistory") ||
			strings.Contains(k, "patternGeneration") || k == "root0.io.nextPaletteVersion" {
			continue
		}
		out = append(out, fmt.Sprintf("%s: exported %s, restored %s", k, clip(av), clip(bv)))
	}
	sort.Strings(out)
	return out
}

func cloneDump(m map[string]string) map[string]string {
	c := make(map[string]string, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

func TestARestoredMachineIsTheMachineThatWasExported(t *testing.T) {
	const snaps = "testdata/snapshots/"
	cases := []struct{ model, file, format string }{
		{"48k", "z88dk_zx48.sna", "sna"},
		{"128k", "z88dk_zx128.sna", "sna"},
		{"plus2", "z88dk_zx128.sna", "sna"},
		{"plus2a", "z88dk_zx128.sna", "sna"},
		{"plus3", "z88dk_zx128.sna", "sna"},
		{"spanish48k", "z88dk_zx48.sna", "sna"},
		{"spanish128k", "z88dk_zx128.sna", "sna"},
		{"spanishplus2", "z88dk_zx128.sna", "sna"},
		{"spanishplus3", "z88dk_zx128.sna", "sna"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.model, func(t *testing.T) {
			zx, err := newMachine(c.model, "./rom")
			if err != nil {
				t.Skipf("%s: %v", c.model, err)
			}
			if err := zx.LoadSnapshotFormat(snaps+c.file, c.format); err != nil {
				t.Fatalf("loading %s: %v", c.file, err)
			}
			zx.paused = false
			resetRun(zx, 300)

			p, err := zx.programState(c.model)
			if err != nil {
				t.Fatal(err)
			}
			// What the program carries is read back from a file, as a running program reads it.
			dir := t.TempDir()
			stub := filepath.Join(dir, "stub")
			os.WriteFile(stub, fakeStub(30), 0o755)
			out := filepath.Join(dir, "prog")
			if err := ExportProgram(stub, out, p); err != nil {
				t.Fatal(err)
			}
			file, _ := os.ReadFile(out)
			got, _, err := readProgram(bytes.NewReader(file), int64(len(file)))
			if err != nil {
				t.Fatal(err)
			}

			back, err := newMachine(got.Model, "./rom")
			if err != nil {
				t.Fatal(err)
			}
			if err := back.LoadSnapshotData(got.Snapshot); err != nil {
				t.Fatal(err)
			}
			back.paused = false

			frameLen := int64(zx.cyclesPerFrame)
			now, then := resetDump(t, zx), resetDump(t, back)
			resetRun(zx, 100)
			if d := restoreCompare(now, then, frameLen, true); len(d) > 0 {
				t.Errorf("at once, %d fields differ:\n  %s", len(d), strings.Join(d, "\n  "))
			}
			// And it goes on as the machine would have.
			resetRun(back, 100)
			if d := restoreCompare(resetDump(t, zx), resetDump(t, back), frameLen, false); len(d) > 0 {
				t.Errorf("100 frames on, %d fields differ:\n  %s", len(d), strings.Join(d, "\n  "))
			}
			if !bytes.Equal(jswScreen(zx), jswScreen(back)) {
				t.Error("100 frames on, the screens differ")
			}
		})
	}
}
