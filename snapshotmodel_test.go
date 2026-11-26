package zenzx

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/ha1tch/zentools/pkg/snapshot"
	"github.com/ha1tch/zentools/pkg/szx"
)

// A snapshot is of a kind of machine, and the kind is read from the file: its header, its size, or
// what its decoder says.

func TestClassOfEveryModel(t *testing.T) {
	want := map[string]machineClass{
		"48k": class48K, "spanish48k": class48K,
		"128k": class128K, "plus2": class128K, "spanish128k": class128K, "spanishplus2": class128K,
		"plus2a": classPlus3, "plus3": classPlus3, "spanishplus3": classPlus3,
		"ts2068": classOther, "next": classNext,
	}
	for _, m := range machineModels {
		w, ok := want[m]
		if !ok {
			t.Errorf("model %q has no kind in this test: a model was added", m)
			continue
		}
		if got := classOfModel(m); got != w {
			t.Errorf("classOfModel(%q) = %v, want %v", m, got, w)
		}
	}
}

// A machine's kind is what it is, not what it was called: a machine built as each model says which
// kind it is, and that is the kind its own model has.
func TestAMachineKnowsItsOwnKind(t *testing.T) {
	for _, m := range machineModels {
		zx, err := newMachine(m, "./rom")
		if err != nil {
			t.Skipf("%s: %v", m, err)
		}
		if got, want := zx.machineClass(), classOfModel(m); got != want {
			t.Errorf("a %s is of kind %v, its model's is %v", m, got, want)
		}
	}
}

// saveAs writes the machine's snapshot in format, and says where.
func saveAs(t *testing.T, zx *ZenZX, name, format string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := zx.SaveSnapshotFormat(path, format); err != nil {
		t.Fatalf("saving %s: %v", name, err)
	}
	return path
}

// The machine a .zxs is of is read from what the machine that saved it was, the +2A being a +3 with
// no disc controller.
func TestAZXSIsOfTheMachineThatSavedIt(t *testing.T) {
	for _, c := range []struct {
		model string
		want  snapshotMachine
	}{
		{"48k", snapshotMachine{class48K, "48k"}},
		{"spanish48k", snapshotMachine{class48K, "48k"}},
		{"128k", snapshotMachine{class128K, "128k"}},
		{"plus2", snapshotMachine{class128K, "128k"}},
		{"plus2a", snapshotMachine{classPlus3, "plus2a"}},
		{"plus3", snapshotMachine{classPlus3, "plus3"}},
	} {
		zx, err := newMachine(c.model, "./rom")
		if err != nil {
			t.Skipf("%s: %v", c.model, err)
		}
		resetRun(zx, 20)
		got, err := readSnapshotMachine(saveAs(t, zx, "s.zxs", "zxs"), "")
		if err != nil || got != c.want {
			t.Errorf("a .zxs saved by a %s is of %+v (%v), want %+v", c.model, got, err, c.want)
		}
	}
}

// .sna by its size, .z80 and .szx by what their decoders say.
func TestTheFilesOfTheOtherFormatsSayWhatTheyAreOf(t *testing.T) {
	const snaps = "testdata/snapshots/"
	for _, c := range []struct {
		file, format string
		want         snapshotMachine
	}{
		{"z88dk_zx48.sna", "sna", snapshotMachine{class48K, "48k"}},
		{"z88dk_zx128.sna", "sna", snapshotMachine{class128K, "128k"}},
		{"manicminer_v2_48k.z80", "z80", snapshotMachine{class48K, "48k"}},
		{"jetsetwilly_v1_48k.z80", "z80", snapshotMachine{class48K, "48k"}},
		{"z80attack_v3_128k.z80", "z80", snapshotMachine{class128K, "128k"}},
	} {
		for _, f := range []string{c.format, ""} { // told the format, and finding it out
			got, err := readSnapshotMachine(snaps+c.file, f)
			if err != nil || got != c.want {
				t.Errorf("%s (format %q) is of %+v (%v), want %+v", c.file, f, got, err, c.want)
			}
		}
	}
	// the files this program writes
	for _, c := range []struct {
		model, name, format string
		want                snapshotMachine
	}{
		{"48k", "a.sna", "sna", snapshotMachine{class48K, "48k"}},
		{"128k", "a.sna", "sna", snapshotMachine{class128K, "128k"}},
		{"48k", "a.z80", "z80", snapshotMachine{class48K, "48k"}},
		{"128k", "a.z80", "z80", snapshotMachine{class128K, "128k"}},
	} {
		zx, err := newMachine(c.model, "./rom")
		if err != nil {
			t.Skipf("%s: %v", c.model, err)
		}
		got, err := readSnapshotMachine(saveAs(t, zx, c.name, c.format), "")
		if err != nil || got != c.want {
			t.Errorf("a %s saved by a %s is of %+v (%v), want %+v", c.format, c.model, got, err, c.want)
		}
	}
}

// .szx says the model of its machine, the +2 and the +2A as well.
func TestAnSZXIsOfItsOwnModel(t *testing.T) {
	zx, err := newMachine("48k", "./rom")
	if err != nil {
		t.Skip(err)
	}
	for _, c := range []struct {
		m    snapshot.Model
		want snapshotMachine
	}{
		{snapshot.Model48K, snapshotMachine{class48K, "48k"}},
		{snapshot.Model128K, snapshotMachine{class128K, "128k"}},
		{snapshot.ModelPlus2, snapshotMachine{class128K, "plus2"}},
		{snapshot.ModelPlus2A, snapshotMachine{classPlus3, "plus2a"}},
		{snapshot.ModelPlus3, snapshotMachine{classPlus3, "plus3"}},
	} {
		s := zx.toMachineState()
		s.Model = c.m
		image, err := szx.Encode(s)
		if err != nil {
			t.Fatalf("encoding .szx of model %v: %v", c.m, err)
		}
		path := filepath.Join(t.TempDir(), "s.szx")
		if err := os.WriteFile(path, image, 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := readSnapshotMachine(path, "")
		if err != nil || got != c.want {
			t.Errorf(".szx of model %v is of %+v (%v), want %+v", c.m, got, err, c.want)
		}
	}
}

func TestWhatIsNotASnapshotIsNotRead(t *testing.T) {
	dir := t.TempDir()
	junk := filepath.Join(dir, "junk.zxs")
	os.WriteFile(junk, []byte("not a snapshot at all"), 0o644)
	for _, p := range []string{junk, filepath.Join(dir, "missing.sna"), filepath.Join(dir, "x.tap")} {
		if _, err := readSnapshotMachine(p, ""); err == nil {
			t.Errorf("%s was read as a snapshot", p)
		}
	}
	if _, err := readSnapshotMachine("testdata/snapshots/z88dk_zx48.sna", "rzx"); err == nil {
		t.Error("a recording was read as a snapshot")
	}
}

// A snapshot loaded into a machine that is not of its kind is refused, with the reason, and the
// machine is as it was; into one of its kind it loads; and a machine that has no kind (a Next, the
// TS2068) is not refused, as it never was.
func TestASnapshotIsLoadedOnlyIntoAMachineOfItsKind(t *testing.T) {
	const snaps = "testdata/snapshots/"
	cases := []struct {
		model, file, format string
		refused             bool
	}{
		{"48k", "z88dk_zx48.sna", "sna", false},
		{"48k", "z88dk_zx128.sna", "sna", true},
		{"48k", "z80attack_v3_128k.z80", "z80", true},
		{"128k", "z88dk_zx128.sna", "sna", false},
		{"128k", "z88dk_zx48.sna", "sna", true},
		{"plus2", "z80attack_v3_128k.z80", "z80", false},
		{"plus3", "z80attack_v3_128k.z80", "z80", true},
		{"plus3", "manicminer_v2_48k.z80", "z80", true},
		{"spanish48k", "manicminer_v2_48k.z80", "z80", false},
		{"ts2068", "manicminer_v2_48k.z80", "z80", false},
		{"ts2068", "z88dk_zx128.sna", "sna", false},
	}
	for _, c := range cases {
		zx, err := newMachine(c.model, "./rom")
		if err != nil {
			t.Skipf("%s: %v", c.model, err)
		}
		pc := zx.cpu.PC
		err = zx.LoadSnapshotFormat(snaps+c.file, c.format)
		switch {
		case c.refused:
			m, ok := snapshotMismatchOf(err)
			if !ok {
				t.Errorf("%s into a %s: %v, want a refusal", c.file, c.model, err)
				continue
			}
			if m.Machine != classOfModel(c.model) || m.Snapshot == m.Machine {
				t.Errorf("%s into a %s: the refusal says %v", c.file, c.model, m)
			}
			if zx.cpu.PC != pc {
				t.Errorf("%s into a %s was refused and the machine changed", c.file, c.model)
			}
		case err != nil:
			t.Errorf("%s into a %s: %v", c.file, c.model, err)
		}
	}
	// The refusal says it in words, for the window and for scripts.
	zx, _ := newMachine("48k", "./rom")
	err := zx.LoadSnapshotFormat(snaps+"z88dk_zx128.sna", "sna")
	if err == nil || err.Error() != "the snapshot is of a 128K and this machine is a 48K" {
		t.Errorf("the refusal reads %q", err)
	}
	if got := snapshotRefusal("g.sna", err); got != "g.sna is a 128K snapshot and this machine is a 48K" {
		t.Errorf("the window says %q", got)
	}
	if got := snapshotRefusal("g.sna", errors.New("boom")); got != "g.sna could not be loaded as a snapshot" {
		t.Errorf("another failure says %q", got)
	}
}

// -snapshot starts the machine the snapshot is of, unless -model was given.
func TestTheStartModelFollowsTheSnapshot(t *testing.T) {
	const snaps = "testdata/snapshots/"
	run := func(args []string, multi, snap string) string {
		fs := flag.NewFlagSet("t", flag.ContinueOnError)
		model := fs.String("model", "48k", "")
		fs.Parse(args)
		startModelFromSnapshot(fs, model, multi, snap, "auto")
		return *model
	}
	for _, c := range []struct {
		name  string
		args  []string
		multi string
		snap  string
		want  string
	}{
		{"128K snapshot, no model", nil, "", snaps + "z88dk_zx128.sna", "128k"},
		{"48K snapshot, no model", nil, "", snaps + "z88dk_zx48.sna", "48k"},
		{"128K snapshot, -model=48k given", []string{"-model=48k"}, "", snaps + "z88dk_zx128.sna", "48k"},
		{"128K snapshot, -model=next given", []string{"-model=next"}, "", snaps + "z88dk_zx128.sna", "next"},
		{"128K snapshot, -multi", nil, "2:48k", snaps + "z88dk_zx128.sna", "48k"},
		{"no snapshot", nil, "", "", "48k"},
		{"a file that is not there", nil, "", snaps + "nothing.sna", "48k"},
		{"the program's own snapshot", nil, "", embeddedSnapshot, "48k"},
	} {
		if got := run(c.args, c.multi, c.snap); got != c.want {
			t.Errorf("%s: model %q, want %q", c.name, got, c.want)
		}
	}
	// a model of the snapshot's kind is kept
	if got := startModelForSnapshot("spanish128k", snaps+"z88dk_zx128.sna", "auto"); got != "spanish128k" {
		t.Errorf("a Spanish 128K was made %q for a 128K snapshot", got)
	}
}
