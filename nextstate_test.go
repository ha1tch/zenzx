//go:build headless

package zenzx

import (
	"bytes"
	"strings"
	"testing"
)

// The state of a Next, saved and put back (nextstate.go).

func bareNext(t *testing.T) *ZenZX {
	t.Helper()
	zx := NewZenZX(AudioBackendOto)
	if err := loadModelROM(zx, "next"); err != nil {
		t.Fatal(err)
	}
	zx.running = true
	return zx
}

func dumpOf(t *testing.T, zx *ZenZX) string {
	t.Helper()
	var b bytes.Buffer
	if err := nextStatePolicy().Dump(&b, zx.nextStateRoots()...); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// firstDifference is the first line two dumps disagree on, for a failure message.
func firstDifference(a, b string) string {
	la, lb := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := 0; i < len(la) && i < len(lb); i++ {
		if la[i] != lb[i] {
			return la[i] + "   |   " + lb[i]
		}
	}
	return "the dumps are of different lengths"
}

// pokeNext puts the machine in a state no fresh one is in: registers of several kinds,
// memory in several places, a palette entry, a sprite, the DMA, and a few frames of running.
func pokeNext(zx *ZenZX) {
	zx.io.SetNextRegister(0x07, 2)
	zx.io.SetNextRegister(0x15, 0x21)
	zx.io.SetNextRegister(0x43, 0x30)
	zx.io.SetNextRegister(0x40, 5)
	zx.io.SetNextRegister(0x41, 0xE3)
	for page := 0; page < nextTotalPages; page += 17 {
		zx.memory.nextPageWrite(uint8(page), 0x123, uint8(page)+1)
	}
	zx.memory.nextPageWrite(0xDF, 0x1FFF, 0x99) // the last page of the 1792K
	zx.memory.rom[2][0x777] ^= 0xFF
	zx.cpu.A, zx.cpu.B, zx.cpu.SP, zx.cpu.PC = 0x12, 0x34, 0xFF00, 0x8000
	zx.io.ayRegisters[3] = 0x55
	for i := 0; i < 3; i++ {
		zx.RunFrame()
	}
}

func TestNextStateRoundTrip(t *testing.T) {
	a := bareNext(t)
	pokeNext(a)
	data, err := a.NextStateBytes()
	if err != nil {
		t.Fatal(err)
	}

	b := bareNext(t)
	if dumpOf(t, a) == dumpOf(t, b) {
		t.Fatal("the poking changed nothing, so the test proves nothing")
	}
	if err := b.RestoreNextState(bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	if da, db := dumpOf(t, a), dumpOf(t, b); da != db {
		t.Fatalf("a restored machine differs: %s", firstDifference(da, db))
	}
	if again, _ := b.NextStateBytes(); !bytes.Equal(data, again) {
		t.Fatal("a restored machine does not save as the machine it was saved from")
	}
	// And they go on alike.
	for i := 0; i < 5; i++ {
		a.RunFrame()
		b.RunFrame()
	}
	if da, db := dumpOf(t, a), dumpOf(t, b); da != db {
		t.Fatalf("five frames later they differ: %s", firstDifference(da, db))
	}
}

func TestNextStateKeepsWhatIsWiredAtConstruction(t *testing.T) {
	a := bareNext(t)
	pokeNext(a)
	data, _ := a.NextStateBytes()
	b := bareNext(t)
	cpu, mem, io, screen := b.cpu, b.memory, b.io, b.screen
	rasterFn, readFn := b.io.rasterTimingFn, b.memory.layer2AccessReadFn
	if err := b.RestoreNextState(bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	if b.cpu != cpu || b.memory != mem || b.io != io || b.screen != screen {
		t.Fatal("restoring replaced an object the machine was made of")
	}
	if b.io.memory != b.memory || b.io.rasterScreen != b.screen {
		t.Fatal("the pointers between the parts no longer lead to the parts")
	}
	if (rasterFn == nil) != (b.io.rasterTimingFn == nil) || (readFn == nil) != (b.memory.layer2AccessReadFn == nil) {
		t.Fatal("a callback was lost")
	}
}

func TestNextStateRefusesWhatIsNotANextAtRest(t *testing.T) {
	classic := NewZenZX(AudioBackendOto)
	if _, err := classic.NextStateBytes(); err == nil {
		t.Fatal("a machine that is not a Next was saved")
	}
	if err := classic.RestoreNextState(bytes.NewReader(nil)); err == nil {
		t.Fatal("a machine that is not a Next was restored")
	}
	zx := bareNext(t)
	zx.turboActive = true
	if _, err := zx.NextStateBytes(); err == nil {
		t.Fatal("a machine running the tape loader's fast path was saved")
	}
}

func TestNextStateRefusesWhatIsNotAState(t *testing.T) {
	zx := bareNext(t)
	pokeNext(zx)
	before := dumpOf(t, zx)
	for name, data := range map[string][]byte{"nothing": nil, "text": []byte("not a state at all, but long enough to have a header"), "short": []byte("ZXMSTATE1\n")} {
		if err := zx.RestoreNextState(bytes.NewReader(data)); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	if dumpOf(t, zx) != before {
		t.Fatal("a refused state changed the machine")
	}
	data, _ := zx.NextStateBytes()
	if err := zx.RestoreNextState(bytes.NewReader(data[:len(data)/2])); err == nil {
		t.Fatal("half a state was accepted")
	}
}

// The list of what a state leaves out is pinned: a callback, a file or a device added to the
// machine shows up here, and has to be decided one way or the other.
func TestNextStateLeavesOutExactlyThese(t *testing.T) {
	got := map[string]bool{}
	for _, s := range nextStatePolicy().Skipped(NewZenZX(AudioBackendOto)) {
		got[s.Field+" ("+s.Reason+")"] = true
	}
	want := strings.Split(strings.TrimSpace(`
z80.Z80.ContendedIODelay (kind func)
z80.Z80.ContendedMemDelay (kind func)
z80.Z80.FastPortReadIn (kind func)
z80.Z80.FastPortWriteOut (kind func)
z80.Z80.IO (kind interface)
z80.Z80.M1Hook (kind func)
z80.Z80.Memory (kind interface)
z80.Z80.NextregWrite (kind func)
zenzx.DivMMC.Debug (policy field)
zenzx.DivMMC.pcFn (kind func)
zenzx.DivMMC.rom3Fn (kind func)
zenzx.SDCard.Debug (policy field)
zenzx.SDCard.img (kind interface)
zenzx.SDCard.wr (policy field)
zenzx.SpectrumIO.audio (policy type)
zenzx.SpectrumIO.cpu (kind interface)
zenzx.SpectrumIO.fdc (policy type)
zenzx.SpectrumIO.floatingBusFn (kind func)
zenzx.SpectrumIO.link (policy type)
zenzx.SpectrumIO.nextRegLog (policy field)
zenzx.SpectrumIO.nextVideoLineFn (kind func)
zenzx.SpectrumIO.onTS2068VideoModeChange (kind func)
zenzx.SpectrumIO.rasterTimingFn (kind func)
zenzx.SpectrumIO.tapePlayingFn (kind func)
zenzx.SpectrumMemory.divMMCReadFn (kind func)
zenzx.SpectrumMemory.divMMCWriteFn (kind func)
zenzx.SpectrumMemory.layer2AccessReadFn (kind func)
zenzx.SpectrumMemory.layer2AccessWriteFn (kind func)
zenzx.SpectrumScreen.lastFlashTime (policy field)
zenzx.ZenZX.NEXAskReset (kind func)
zenzx.ZenZX.OnNEXRun (kind func)
zenzx.ZenZX.audio (policy field)
zenzx.ZenZX.auto (policy field)
zenzx.ZenZX.card2 (policy field)
zenzx.ZenZX.display (kind interface)
zenzx.ZenZX.escape (policy field)
zenzx.ZenZX.firstProgram (policy field)
zenzx.ZenZX.nexRun (policy field)
zenzx.ZenZX.nexScratch (policy field)
zenzx.ZenZX.next (policy field)
zenzx.ZenZX.nextSettle (policy field)
zenzx.ZenZX.nonStandard (policy field)
zenzx.ZenZX.onNextHandOver (kind func)
zenzx.ZenZX.paused (policy field)
zenzx.ZenZX.rendererSelected (policy field)
zenzx.ZenZX.romPatches (policy field)
zenzx.ZenZX.running (policy field)
zenzx.ZenZX.scriptKeys (policy field)
zenzx.ZenZX.snapshotFormat (policy field)
zenzx.ZenZX.tape (policy field)
zenzx.ZenZX.turboActive (policy field)
zenzx.ZenZX.turboNextBoundary (policy field)
zenzx.ZenZX.turboPageFlags (policy field)
zenzx.ZenZX.turboPages (policy field)
zenzx.ZenZX.turboPort7FFD (policy field)
zenzx.ZenZX.turboSlotPage (policy field)
zenzx.ZenZX.uiOwnsEscape (policy field)
zenzx.ZenZX.videoRenderer (kind interface)
zenzx.nextConfigState.divRAM (policy field)
zenzx.nextConfigState.divROM (policy field)
zenzx.nextRaster.img (policy type)
zenzx.nextRaster.sprites (policy type)
zenzx.nextRaster.timex (policy type)
`), "\n")
	wanted := map[string]bool{}
	for _, w := range want {
		wanted[w] = true
		if !got[w] {
			t.Errorf("no longer left out: %s", w)
		}
	}
	for g := range got {
		if !wanted[g] {
			t.Errorf("newly left out, and not decided here: %s", g)
		}
	}
}

func TestNextStateFingerprintIsStable(t *testing.T) {
	a := nextStatePolicy().Fingerprint(NewZenZX(AudioBackendOto))
	b := nextStatePolicy().Fingerprint(NewZenZX(AudioBackendOto))
	if a != b {
		t.Fatal("the layout fingerprint of two machines differs")
	}
}
