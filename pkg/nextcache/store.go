// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package nextcache

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Stage is how far into its start the machine was when the cache was taken.
type Stage string

const (
	// HandOver is the machine as the firmware leaves it: its ROMs loaded, config mode left,
	// the Z80 reset into them. NextZXOS has not yet read the card, so nothing of the card
	// is in it.
	HandOver Stage = "hand-over"
	// Settled is the machine at rest in NextZXOS, after it has read what it reads at start.
	Settled Stage = "settled"
)

// Stages in the order a start prefers them.
var Stages = []Stage{Settled, HandOver}

// Key is everything a cache depends on. A cache is used only when every part matches.
type Key struct {
	BootROM   string   `json:"boot_rom"`   // SHA-256 of the boot ROM, hex
	DivMMCROM string   `json:"divmmc_rom"` // SHA-256 of the DivMMC ROM, hex
	Firmware  Firmware `json:"firmware"`   // the firmware on the card
	Startup   Firmware `json:"startup"`    // what NextZXOS reads at start: the firmware, and the files it starts without; settled caches only
	Issue     string   `json:"issue"`      // the board issue
	Patches   []string `json:"patches"`    // ROM switches laid over the firmware's ROMs (rom_patches.go), one entry each
	Version   string   `json:"version"`    // ZenZX: a change in the emulation changes the state
	Layout    string   `json:"layout"`     // fingerprint of the machine's state layout, hex
}

// DiffFor is Diff for a cache of stage s. The hand-over is taken before NextZXOS has read
// anything but the firmware, so it does not depend on what else is on the card; the settled
// one does, as far as Startup says.
func (k Key) DiffFor(o Key, s Stage) []string {
	d := k.Diff(o)
	if s == Settled {
		for _, x := range diffFirmware(k.Startup, o.Startup) {
			d = append(d, strings.Replace(x, "firmware file", "start-up file", 1))
		}
	}
	return d
}

// Diff names the parts of the key that differ from o, in a fixed order; empty when none do.
// A firmware file is named by its path, so that `-next-firmware check` can say which. The
// start-up files are not in it: DiffFor adds them for the stage that depends on them.
func (k Key) Diff(o Key) []string {
	var d []string
	if k.BootROM != o.BootROM {
		d = append(d, "the boot ROM")
	}
	if k.DivMMCROM != o.DivMMCROM {
		d = append(d, "the DivMMC ROM")
	}
	d = append(d, diffFirmware(k.Firmware, o.Firmware)...)
	if k.Issue != o.Issue {
		d = append(d, "the board issue")
	}
	if strings.Join(k.Patches, "\x00") != strings.Join(o.Patches, "\x00") {
		d = append(d, "the ROM patches")
	}
	if k.Version != o.Version {
		d = append(d, "the ZenZX version")
	}
	if k.Layout != o.Layout {
		d = append(d, "the layout of the machine's state")
	}
	return d
}

func diffFirmware(a, b Firmware) []string {
	if a.Digest == b.Digest {
		return nil
	}
	have := map[string]FileDigest{}
	for _, f := range a.Files {
		have[f.Path] = f
	}
	var d []string
	seen := map[string]bool{}
	for _, f := range b.Files {
		seen[f.Path] = true
		if g, ok := have[f.Path]; !ok {
			d = append(d, "firmware file "+f.Path+" (not in the cache)")
		} else if g.SHA256 != f.SHA256 || g.Size != f.Size {
			d = append(d, "firmware file "+f.Path)
		}
	}
	for _, f := range a.Files {
		if !seen[f.Path] {
			d = append(d, "firmware file "+f.Path+" (no longer used)")
		}
	}
	if len(d) == 0 {
		d = append(d, "the firmware digest")
	}
	return d
}

// Entry is one cache.
type Entry struct {
	Key     Key       `json:"key"`
	Stage   Stage     `json:"stage"`
	Frames  int       `json:"frames"`  // frames the boot had run when it was taken
	Created time.Time `json:"created"` // for the listing only
	State   []byte    `json:"-"`
}

const fileMagic = "ZXNEXTCACHE1\n"

// header is what precedes the state in a file.
type header struct {
	Entry
	StateLen    int    `json:"state_len"`
	StateSHA256 string `json:"state_sha256"`
}

// Errors Read returns besides *StaleError.
var (
	// ErrAbsent: no cache for this firmware and stage.
	ErrAbsent = errors.New("nextcache: no cache")
	// ErrCorrupt: a file that does not read back as what was written.
	ErrCorrupt = errors.New("nextcache: the cache file is damaged")
)

// StaleError is a cache for this firmware that another part of the key rules out.
type StaleError struct{ Differs []string }

func (e *StaleError) Error() string {
	return "nextcache: the cache is stale: " + strings.Join(e.Differs, ", ")
}

// Ext is the extension of a cache file.
const Ext = ".zxnc"

// Path is where the cache of a firmware and stage lives in dir: one file each, named by the
// firmware digest.
func Path(dir string, fw Firmware, s Stage) string {
	d := fw.Digest
	if len(d) > 24 {
		d = d[:24]
	}
	return filepath.Join(dir, d+"."+string(s)+Ext)
}

// Write stores a cache, whole or not at all.
func Write(dir string, e Entry) error {
	if e.Key.Firmware.Digest == "" {
		return errors.New("nextcache: no firmware digest in the key")
	}
	if e.Created.IsZero() {
		e.Created = time.Now().UTC().Truncate(time.Second)
	}
	sum := sha256.Sum256(e.State)
	js, err := json.Marshal(header{Entry: e, StateLen: len(e.State), StateSHA256: hex.EncodeToString(sum[:])})
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	buf.WriteString(fileMagic)
	var n [4]byte
	binary.BigEndian.PutUint32(n[:], uint32(len(js)))
	buf.Write(n[:])
	buf.Write(js)
	buf.Write(e.State)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := Path(dir, e.Key.Firmware, e.Stage)
	tmp, err := os.CreateTemp(dir, ".nextcache-*.tmp")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(buf.Bytes())
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		os.Remove(tmp.Name())
		return errors.Join(werr, cerr)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}

// load reads and checks one file.
func load(path string) (*Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrAbsent
		}
		return nil, err
	}
	if !bytes.HasPrefix(data, []byte(fileMagic)) || len(data) < len(fileMagic)+4 {
		return nil, ErrCorrupt
	}
	rest := data[len(fileMagic):]
	hl := int(binary.BigEndian.Uint32(rest[:4]))
	rest = rest[4:]
	if hl <= 0 || hl > len(rest) {
		return nil, ErrCorrupt
	}
	var h header
	if err := json.Unmarshal(rest[:hl], &h); err != nil {
		return nil, ErrCorrupt
	}
	state := rest[hl:]
	sum := sha256.Sum256(state)
	if len(state) != h.StateLen || hex.EncodeToString(sum[:]) != h.StateSHA256 {
		return nil, ErrCorrupt
	}
	h.Entry.State = state
	return &h.Entry, nil
}

// Read returns the cache of a stage if it is good for want: *StaleError when it is for the
// same firmware and another part of the key differs, ErrAbsent when there is none, and
// ErrCorrupt when the file does not read back.
func Read(dir string, want Key, s Stage) (*Entry, error) {
	e, err := load(Path(dir, want.Firmware, s))
	if err != nil {
		return nil, err
	}
	if e.Stage != s || e.Key.Firmware.Digest != want.Firmware.Digest {
		return nil, ErrCorrupt
	}
	if d := e.Key.DiffFor(want, s); len(d) > 0 {
		return nil, &StaleError{Differs: d}
	}
	return e, nil
}

// Info describes a cache file without its state.
type Info struct {
	Path    string
	Stage   Stage
	Key     Key
	Frames  int
	Created time.Time
	Size    int64
	Err     error // set when the file does not read back
}

// List describes every cache file in dir, in a fixed order.
func List(dir string) []Info {
	names, _ := filepath.Glob(filepath.Join(dir, "*"+Ext))
	sort.Strings(names)
	var out []Info
	for _, p := range names {
		in := Info{Path: p}
		if st, err := os.Stat(p); err == nil {
			in.Size = st.Size()
		}
		e, err := load(p)
		if err != nil {
			in.Err = err
		} else {
			in.Stage, in.Key, in.Frames, in.Created = e.Stage, e.Key, e.Frames, e.Created
		}
		out = append(out, in)
	}
	return out
}

// Clear removes every cache in dir and says how many. A folder that is not there has none.
func Clear(dir string) (int, error) {
	return remove(dir, func(Info) bool { return true })
}

// Prune removes the caches keep refuses and any that do not read back, and says how many.
func Prune(dir string, keep func(Info) bool) (int, error) {
	return remove(dir, func(in Info) bool { return in.Err != nil || !keep(in) })
}

func remove(dir string, drop func(Info) bool) (int, error) {
	n := 0
	for _, in := range List(dir) {
		if !drop(in) {
			continue
		}
		if err := os.Remove(in.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return n, err
		}
		n++
	}
	// Leftovers of a write that was cut short.
	tmps, _ := filepath.Glob(filepath.Join(dir, ".nextcache-*.tmp"))
	for _, t := range tmps {
		if os.Remove(t) == nil {
			n++
		}
	}
	return n, nil
}
