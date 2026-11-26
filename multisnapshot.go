// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// A multisnapshot (.zms) is the state of every machine of a window, and how the window shows them.
// It is a zip file: manifest.json, which says which machines there are and how they are laid out,
// and one snapshot (.zxs, which every kind of machine has, the Next's included) for each machine, in
// the order the machines are in the dock. Saving takes the machines at one moment (multisnapshot_gui.go);
// loading closes the machines that are running and starts these in their place.

// MultiSnapshotExt is a multisnapshot's extension, without the dot.
const MultiSnapshotExt = "zms"

const (
	multiSnapshotFormat  = "zenzx-multisnapshot"
	multiSnapshotVersion = 1
	multiManifestName    = "manifest.json"

	// What a multisnapshot may hold when it is read: a file that says more is refused before it is
	// unpacked, so that a damaged or hostile one cannot fill the memory.
	multiSnapshotMaxEntry = 256 << 20
	multiSnapshotMaxTotal = 1 << 30
)

// multiManifest is manifest.json.
type multiManifest struct {
	Format   string              `json:"format"`
	Version  int                 `json:"version"`
	Layout   string              `json:"layout"` // the View menu's name for how the machines are shown
	Front    int                 `json:"front"`  // the machine that has the keyboard, and the main tile of the layouts that have one
	Zoom     int                 `json:"zoom,omitempty"`
	Machines []multiMachineEntry `json:"machines"`
}

// multiMachineEntry is one machine of the manifest.
type multiMachineEntry struct {
	File    string `json:"file"`              // its snapshot, a name in the zip
	Model   string `json:"model"`             // the model it was
	Name    string `json:"name,omitempty"`    // the name it has in the dock
	Renamed bool   `json:"renamed,omitempty"` // the person gave it
}

// isMultiSnapshotPath says the file at path is named as a multisnapshot.
func isMultiSnapshotPath(path string) bool {
	return strings.EqualFold(filepath.Ext(path), "."+MultiSnapshotExt)
}

// multiFileName is the name of machine i's snapshot in the zip.
func multiFileName(i int) string { return fmt.Sprintf("machine%02d.zxs", i+1) }

// check says whether a manifest can be loaded.
func (m *multiManifest) check() error {
	switch {
	case m.Format != multiSnapshotFormat:
		return errors.New("it is not a ZenZX multisnapshot")
	case m.Version < 1 || m.Version > multiSnapshotVersion:
		return fmt.Errorf("it is of version %d, which this ZenZX does not read", m.Version)
	case len(m.Machines) == 0:
		return errors.New("it has no machines in it")
	case len(m.Machines) > multiMax:
		return fmt.Errorf("it has %d machines, and the most is %d", len(m.Machines), multiMax)
	case m.Front < 0 || m.Front >= len(m.Machines):
		return errors.New("its front machine is not one of its machines")
	}
	seen := map[string]bool{}
	for i, e := range m.Machines {
		if e.File == "" || e.File != filepath.Base(e.File) || strings.ContainsAny(e.File, "/\\") || e.File == multiManifestName {
			return fmt.Errorf("machine %d names a file that is not one of the zip's", i+1)
		}
		if seen[e.File] {
			return fmt.Errorf("machines %d and another are of one file", i+1)
		}
		seen[e.File] = true
	}
	return nil
}

// writeMultiSnapshot writes a multisnapshot to path: the manifest and the snapshots, snaps[i] being
// machine i's. The file is written whole or not at all.
func writeMultiSnapshot(path string, man multiManifest, snaps [][]byte) error {
	man.Format, man.Version = multiSnapshotFormat, multiSnapshotVersion
	if len(snaps) != len(man.Machines) {
		return fmt.Errorf("%d snapshots for %d machines", len(snaps), len(man.Machines))
	}
	for i := range man.Machines {
		man.Machines[i].File = multiFileName(i)
	}
	if err := man.check(); err != nil {
		return err
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	put := func(name string, data []byte) error {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}
	js, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return err
	}
	if err := put(multiManifestName, append(js, '\n')); err != nil {
		return err
	}
	for i, s := range snaps {
		if err := put(man.Machines[i].File, s); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".zms-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, werr := tmp.Write(buf.Bytes())
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

// openMultiSnapshot opens the zip at path and reads its manifest.
func openMultiSnapshot(path string) (*zip.ReadCloser, multiManifest, error) {
	var man multiManifest
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, man, fmt.Errorf("it is not a multisnapshot: %w", err)
	}
	fail := func(err error) (*zip.ReadCloser, multiManifest, error) {
		zr.Close()
		return nil, multiManifest{}, err
	}
	f := findZipEntry(zr.File, multiManifestName)
	if f == nil {
		return fail(errors.New("it has no manifest"))
	}
	data, err := readZipEntry(f, 1<<20)
	if err != nil {
		return fail(err)
	}
	if err := json.Unmarshal(data, &man); err != nil {
		return fail(fmt.Errorf("its manifest cannot be read: %w", err))
	}
	if err := man.check(); err != nil {
		return fail(err)
	}
	return zr, man, nil
}

// peekMultiSnapshot reads a multisnapshot's manifest only.
func peekMultiSnapshot(path string) (multiManifest, error) {
	zr, man, err := openMultiSnapshot(path)
	if err != nil {
		return man, err
	}
	zr.Close()
	return man, nil
}

// readMultiSnapshot reads a multisnapshot: its manifest and machine i's snapshot as snaps[i].
func readMultiSnapshot(path string) (multiManifest, [][]byte, error) {
	zr, man, err := openMultiSnapshot(path)
	if err != nil {
		return man, nil, err
	}
	defer zr.Close()
	snaps := make([][]byte, len(man.Machines))
	var total int64
	for i, e := range man.Machines {
		f := findZipEntry(zr.File, e.File)
		if f == nil {
			return man, nil, fmt.Errorf("machine %d has no snapshot in it", i+1)
		}
		if total += int64(f.UncompressedSize64); total > multiSnapshotMaxTotal {
			return man, nil, errors.New("it is too big to be a multisnapshot")
		}
		if snaps[i], err = readZipEntry(f, multiSnapshotMaxEntry); err != nil {
			return man, nil, fmt.Errorf("machine %d: %w", i+1, err)
		}
	}
	return man, snaps, nil
}

// findZipEntry is the entry of files called name, or nil.
func findZipEntry(files []*zip.File, name string) *zip.File {
	for _, f := range files {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// readZipEntry reads a zip entry, refusing one that holds more than limit.
func readZipEntry(f *zip.File, limit int64) ([]byte, error) {
	if int64(f.UncompressedSize64) > limit {
		return nil, fmt.Errorf("%s is too big", f.Name)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s is too big", f.Name)
	}
	return data, nil
}
