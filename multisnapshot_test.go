package zenzx

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func sampleManifest(n int) (multiManifest, [][]byte) {
	m := multiManifest{Layout: "Side by Side", Front: n - 1, Zoom: 2}
	var snaps [][]byte
	for i := 0; i < n; i++ {
		m.Machines = append(m.Machines, multiMachineEntry{Model: "48k", Name: "m" + string(rune('A'+i)), Renamed: i == 0})
		snaps = append(snaps, bytes.Repeat([]byte{byte(i + 1)}, 100+i))
	}
	return m, snaps
}

// A multisnapshot is read back as it was written: the layout, the front machine, the names and each
// machine's snapshot, in order.
func TestMultiSnapshotRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "two.zms")
	man, snaps := sampleManifest(3)
	if err := writeMultiSnapshot(path, man, snaps); err != nil {
		t.Fatal(err)
	}
	got, gotSnaps, err := readMultiSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Layout != "Side by Side" || got.Front != 2 || got.Zoom != 2 || len(got.Machines) != 3 {
		t.Errorf("manifest %+v", got)
	}
	if got.Machines[0].Name != "mA" || !got.Machines[0].Renamed || got.Machines[1].Renamed {
		t.Errorf("names %+v", got.Machines)
	}
	if !reflect.DeepEqual(gotSnaps, snaps) {
		t.Error("the snapshots are not what was written")
	}
	if m, err := peekMultiSnapshot(path); err != nil || len(m.Machines) != 3 {
		t.Errorf("peek: %+v %v", m, err)
	}
	// nothing but the file is left in the folder
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".zms-*")); len(left) != 0 {
		t.Errorf("temporary files left: %v", left)
	}
}

// A file that is not a multisnapshot, or one that says what cannot be loaded, is refused with a
// reason, and one that is read is never given more than it says.
func TestMultiSnapshotRefusals(t *testing.T) {
	dir := t.TempDir()
	zipOf := func(entries map[string]string) string {
		p := filepath.Join(dir, "x.zms")
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		for name, body := range entries {
			w, _ := zw.Create(name)
			w.Write([]byte(body))
		}
		zw.Close()
		os.WriteFile(p, buf.Bytes(), 0o644)
		return p
	}
	ok := func(front int, files ...string) string {
		s := `{"format":"zenzx-multisnapshot","version":1,"layout":"Grid","front":` + string(rune('0'+front)) + `,"machines":[`
		for i, f := range files {
			if i > 0 {
				s += ","
			}
			s += `{"file":"` + f + `","model":"48k"}`
		}
		return s + "]}"
	}
	cases := []struct {
		name    string
		entries map[string]string
		want    string
	}{
		{"no manifest", map[string]string{"a.zxs": "x"}, "no manifest"},
		{"not json", map[string]string{"manifest.json": "{"}, "cannot be read"},
		{"another format", map[string]string{"manifest.json": `{"format":"other","version":1}`}, "not a ZenZX multisnapshot"},
		{"newer version", map[string]string{"manifest.json": `{"format":"zenzx-multisnapshot","version":9,"machines":[{"file":"a"}]}`}, "version 9"},
		{"no machines", map[string]string{"manifest.json": `{"format":"zenzx-multisnapshot","version":1,"machines":[]}`}, "no machines"},
		{"front out of range", map[string]string{"manifest.json": ok(3, "a.zxs")}, "front machine"},
		{"path in a name", map[string]string{"manifest.json": ok(0, "../a.zxs"), "a.zxs": "x"}, "not one of the zip's"},
		{"a name with a slash", map[string]string{"manifest.json": ok(0, "d/a.zxs")}, "not one of the zip's"},
		{"two machines, one file", map[string]string{"manifest.json": ok(0, "a.zxs", "a.zxs"), "a.zxs": "x"}, "of one file"},
		{"a snapshot missing", map[string]string{"manifest.json": ok(0, "a.zxs")}, "no snapshot"},
	}
	for _, c := range cases {
		_, _, err := readMultiSnapshot(zipOf(c.entries))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %v, want one saying %q", c.name, err, c.want)
		}
	}
	// not a zip at all, and not there
	notZip := filepath.Join(dir, "plain.zms")
	os.WriteFile(notZip, []byte("hello"), 0o644)
	if _, _, err := readMultiSnapshot(notZip); err == nil || !strings.Contains(err.Error(), "not a multisnapshot") {
		t.Errorf("plain file: %v", err)
	}
	if _, _, err := readMultiSnapshot(filepath.Join(dir, "gone.zms")); err == nil {
		t.Error("a file that is not there was read")
	}
	// too many machines to write, and a count that does not match
	man, snaps := sampleManifest(multiMax + 1)
	if err := writeMultiSnapshot(filepath.Join(dir, "big.zms"), man, snaps); err == nil {
		t.Error("more machines than the most were written")
	}
	man, snaps = sampleManifest(2)
	if err := writeMultiSnapshot(filepath.Join(dir, "odd.zms"), man, snaps[:1]); err == nil {
		t.Error("one snapshot for two machines was written")
	}
	if _, err := os.Stat(filepath.Join(dir, "odd.zms")); err == nil {
		t.Error("a refused write left a file")
	}
}

// An entry that holds more than the limit is not unpacked.
func TestReadZipEntryLimit(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("big")
	w.Write(bytes.Repeat([]byte{0}, 5000))
	zw.Close()
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := readZipEntry(zr.File[0], 4000); err == nil {
		t.Error("an entry over the limit was read")
	}
	if b, err := readZipEntry(zr.File[0], 5000); err != nil || len(b) != 5000 {
		t.Errorf("an entry within it: %d bytes, %v", len(b), err)
	}
}

func TestIsMultiSnapshotPath(t *testing.T) {
	for p, want := range map[string]bool{"/a/x.zms": true, "X.ZMS": true, "x.zxs": false, "zms": false, "x.zms.bak": false} {
		if got := isMultiSnapshotPath(p); got != want {
			t.Errorf("isMultiSnapshotPath(%q) = %v", p, got)
		}
	}
}
