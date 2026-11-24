package nextcard

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// memImage is a card image in memory.
type memImage []byte

func (m memImage) ReadAt(p []byte, off int64) (int, error)  { return copy(p, m[off:]), nil }
func (m memImage) WriteAt(p []byte, off int64) (int, error) { return copy(m[off:], p), nil }

const (
	testStart   = 63
	testSectors = 600000 // about 290 MB: enough for FAT32's minimum with 4K clusters
)

func testMBR() []byte {
	mbr := make([]byte, sectorSize)
	mbr[446+4] = 0x0C
	binary.LittleEndian.PutUint32(mbr[446+8:], testStart)
	binary.LittleEndian.PutUint32(mbr[446+12:], testSectors)
	mbr[510], mbr[511] = 0x55, 0xAA
	return mbr
}

// testTree is a small tree with the kinds of name a real card has.
func testTree() (*Node, map[string][]byte) {
	big := bytes.Repeat([]byte("0123456789abcdef"), 1000) // 16000 bytes: four clusters
	files := map[string][]byte{
		"/TBBLUE.FW":                                          []byte("firmware"),
		"/README.md":                                          []byte("mixed case, fits 8.3"),
		"/machines/next/config.ini":                           []byte("scandoubler=1\r\ntiming=8\r\nmenu=1\r\n"),
		"/machines/next/zx81.rom":                             bytes.Repeat([]byte{0x81}, 16384),
		"/demos/A long folder name/big.bin":                   big,
		"/demos/A long folder name/empty":                     nil,
		"/demos/Daybreak with spaces.nex":                     []byte("nex"),
		"/demos/daybreak with spaces 2.nex":                   []byte("nex2"),
		"/docs/ünïcödé and more than thirteen characters.txt": []byte("long"),
	}
	root := NewDir("")
	root.Add("TBBLUE.FW", files["/TBBLUE.FW"])
	root.Add("README.md", files["/README.md"])
	next := root.AddDir("machines").AddDir("next")
	next.Add("config.ini", files["/machines/next/config.ini"])
	next.Add("zx81.rom", files["/machines/next/zx81.rom"])
	demos := root.AddDir("demos")
	long := demos.AddDir("A long folder name")
	long.Add("big.bin", big)
	long.Add("empty", nil)
	demos.Add("Daybreak with spaces.nex", files["/demos/Daybreak with spaces.nex"])
	demos.Add("daybreak with spaces 2.nex", files["/demos/daybreak with spaces 2.nex"])
	root.AddDir("docs").Add("ünïcödé and more than thirteen characters.txt", []byte("long"))
	return root, files
}

func buildTestImage(t *testing.T) (memImage, Layout, map[string][]byte) {
	t.Helper()
	img := make(memImage, (testStart+testSectors)*sectorSize)
	root, files := testTree()
	lay, err := Build(img, testMBR(), testStart, testSectors, root, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return img, lay, files
}

func TestBuildThenReadBack(t *testing.T) {
	img, lay, files := buildTestImage(t)
	if lay.Files != len(files) || lay.Dirs != 5 {
		t.Fatalf("layout counts %d files and %d folders, want %d and 5", lay.Files, lay.Dirs, len(files))
	}
	vol, err := Open(img)
	if err != nil {
		t.Fatal(err)
	}
	if vol.ClusterCount() < MinFAT32Clusters || vol.SectorsPerCluster() != 8 {
		t.Fatalf("%d clusters of %d sectors", vol.ClusterCount(), vol.SectorsPerCluster())
	}
	seen := 0
	vol.Root.Walk(func(path string, n *Node) {
		if n.IsDir() {
			return
		}
		want, ok := files[path]
		if !ok {
			t.Errorf("unexpected file %q", path)
			return
		}
		got, err := vol.ReadFile(n)
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s: read back %d bytes (%v), want %d", path, len(got), err, len(want))
		}
		seen++
	})
	if seen != len(files) {
		t.Fatalf("read back %d files, want %d", seen, len(files))
	}
	// Names are found whatever their case, and by their short form too.
	for _, p := range []string{"tbblue.fw", "MACHINES/NEXT/CONFIG.INI", "demos/ALONGF~1/BIG.BIN"} {
		if vol.Root.Find(p) == nil {
			t.Errorf("Find(%q) found nothing", p)
		}
	}
	if vol.Root.Find("demos/nothing") != nil {
		t.Error("Find found a file that is not there")
	}
}

func TestBuildWritesAConsistentVolume(t *testing.T) {
	img, lay, _ := buildTestImage(t)
	if err := func() error { _, err := Open(img); return err }(); err != nil {
		t.Fatal(err)
	}
	g, err := readGeometry(img, testStart)
	if err != nil {
		t.Fatal(err)
	}
	// The backup boot sector and both FSInfo sectors.
	sector := func(n int64) []byte { return img[(testStart+n)*sectorSize : (testStart+n+1)*sectorSize] }
	if !bytes.Equal(sector(0), sector(6)) || !bytes.Equal(sector(1), sector(7)) {
		t.Fatal("the backup boot sector or its FSInfo differs from the first")
	}
	free, next := binary.LittleEndian.Uint32(sector(1)[488:]), binary.LittleEndian.Uint32(sector(1)[492:])
	if free != lay.FreeClusters || next != lay.FirstFree || free != g.clusterCount-lay.Used {
		t.Fatalf("FSInfo says %d free from %d; layout %d free from %d of %d", free, next, lay.FreeClusters, lay.FirstFree, g.clusterCount)
	}
	// The FAT: the media entries, then exactly the used clusters, then nothing.
	fat := img[g.fatOffset(0):]
	if binary.LittleEndian.Uint32(fat[0:]) != 0x0FFFFFF8 || binary.LittleEndian.Uint32(fat[4:]) != fatEOC {
		t.Fatal("the FAT's first two entries are wrong")
	}
	for c := uint32(2); c < g.clusterCount+2; c++ {
		used := binary.LittleEndian.Uint32(fat[c*4:]) != 0
		if used != (c < 2+lay.Used) {
			t.Fatalf("cluster %d: used=%v with %d clusters in use", c, used, lay.Used)
		}
	}
	if !bytes.Equal(fat[:g.fatSectors*sectorSize], img[g.fatOffset(1):g.fatOffset(1)+int64(g.fatSectors)*sectorSize]) {
		t.Fatal("the two FAT copies differ")
	}
	if err := VerifyCard(img); err == nil || !strings.Contains(err.Error(), "timing=0") {
		t.Fatalf("a card with timing=8 passed verification: %v", err)
	}
}

func TestShortNames(t *testing.T) {
	root := NewDir("")
	for _, name := range []string{"README.md", "TBBLUE.FW", "A long folder name", "a long folder name 2", "x.y.z", "nöt ascii"} {
		root.Add(name, nil)
	}
	if err := assignShortNames(root); err != nil {
		t.Fatal(err)
	}
	want := map[string]struct {
		short string
		long  bool
	}{
		"README.md":            {"README  MD ", true}, // lower case letters need the long name
		"TBBLUE.FW":            {"TBBLUE  FW ", false},
		"A long folder name":   {"ALONGF~1   ", true},
		"a long folder name 2": {"ALONGF~2   ", true},
		"x.y.z":                {"XY~1    Z  ", true},
		"nöt ascii":            {"N_TASC~1   ", true},
	}
	seen := map[string]bool{}
	for _, c := range root.Children {
		w := want[c.Name]
		if string(c.Short[:]) != w.short || c.hasLong != w.long {
			t.Errorf("%q: short %q long=%v, want %q long=%v", c.Name, c.Short[:], c.hasLong, w.short, w.long)
		}
		if seen[string(c.Short[:])] {
			t.Errorf("short name %q used twice", c.Short[:])
		}
		seen[string(c.Short[:])] = true
	}
	// The checksum of the specification's own example.
	var example [11]byte
	copy(example[:], "FOO     BAR")
	sum := byte(0)
	for _, c := range example {
		sum = ((sum & 1) << 7) + (sum >> 1) + c
	}
	if shortChecksum(example) != sum {
		t.Fatal("shortChecksum disagrees with the specification's formula")
	}
}

func TestBuildRefusesAPartitionTooSmallForFAT32(t *testing.T) {
	img := make(memImage, 70000*sectorSize)
	if _, err := Build(img, testMBR(), testStart, 60000, NewDir(""), nil, nil); err == nil {
		t.Fatal("a 30 MB partition was accepted; it cannot hold 65,525 clusters of 4K")
	}
}

// distributionZip makes a zip like the real distribution's from a source image.
func distributionZip(t *testing.T, img []byte, rom []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for name, data := range map[string][]byte{"cspect-next-1gb.img": img, "enNxtmmc.rom": rom, "enNextZX.rom": make([]byte, 65536)} {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(data)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestSetupDownloadsBuildsAndVerifies(t *testing.T) {
	// The source has the short names the real distribution has: numeric tails on folders
	// whose names would fit 8.3.
	src := make(memImage, (testStart+testSectors)*sectorSize)
	tree, _ := testTree()
	for path, short := range map[string]string{"machines": "MACHIN~1   ", "machines/next": "NEXT~1     ", "machines/next/config.ini": "CONFIG~1INI"} {
		n := tree.Find(path)
		copy(n.Short[:], short)
		n.hasLong = true
	}
	if _, err := Build(src, testMBR(), testStart, testSectors, tree, nil, nil); err != nil {
		t.Fatal(err)
	}
	rom := bytes.Repeat([]byte{0xC9}, romSize)
	boot := bytes.Repeat([]byte{0xF3}, romSize)
	zipData := distributionZip(t, src, rom)
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch r.URL.Path {
		case "/dist.zip":
			w.Header().Set("Content-Length", fmt.Sprint(len(zipData)))
			w.Write(zipData)
		case "/boot.rom":
			w.Write(boot)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	stages := map[Stage]int64{}
	opts := Options{Dir: dir, DistributionURL: srv.URL + "/dist.zip", BootROMURL: srv.URL + "/boot.rom"}
	err := Setup(context.Background(), opts, func(s Stage, done, total int64) {
		if done < stages[s] {
			t.Errorf("%v went backwards: %d after %d", s, done, stages[s])
		}
		stages[s] = done
	})
	if err != nil {
		t.Fatal(err)
	}
	if stages[StageDownload] != int64(len(zipData)) || stages[StageUnpack] != int64(len(src)) || stages[StageBuild] == 0 {
		t.Fatalf("progress reached: %v", stages)
	}
	f := In(dir)
	if !f.Present() {
		t.Fatal("Setup succeeded and the files are not all there")
	}
	if got, _ := os.ReadFile(f.DivMMCROM); !bytes.Equal(got, rom) {
		t.Fatal("the DivMMC ROM is not the one in the zip")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 3 {
		t.Fatalf("%d files left in the folder, want the three: %v", len(entries), entries)
	}

	// The card: what Rebuild changes, and everything else as it was.
	card, err := os.Open(f.Card)
	if err != nil {
		t.Fatal(err)
	}
	defer card.Close()
	vol, err := Open(card)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := vol.ReadFile(vol.Root.Find("machines/next/config.ini"))
	if string(cfg) != "scandoubler=1\r\ntiming=0\r\nmenu=1\r\n" {
		t.Fatalf("config.ini on the card is %q", cfg)
	}
	for path, want := range map[string]string{"machines": "MACHINES   ", "machines/next": "NEXT       ", "machines/next/config.ini": "CONFIG  INI"} {
		if n := vol.Root.Find(path); string(n.Short[:]) != want || n.Name != filepath.Base(path) {
			t.Errorf("%s has short name %q and long name %q, want %q and the long name kept", path, n.Short[:], n.Name, want)
		}
	}
	if n := vol.Root.Find("machines/next/zx81.rom"); n.Size != 32768 {
		t.Fatalf("zx81.rom is %d bytes, want it doubled to 32768", n.Size)
	}
	if n := vol.Root.Find("demos/A long folder name/big.bin"); n == nil || n.Size != 16000 {
		t.Fatal("a file in a long-named folder did not come across")
	}

	// A second Setup finds everything and downloads nothing.
	before := requests
	if err := Setup(context.Background(), opts, nil); err != nil || requests != before {
		t.Fatalf("second Setup: %v, %d more requests", err, requests-before)
	}
	// A saved file must survive: a missing ROM is fetched without touching the card.
	stamp, _ := os.Stat(f.Card)
	os.Remove(f.BootROM)
	if err := Setup(context.Background(), opts, nil); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.Stat(f.Card); !after.ModTime().Equal(stamp.ModTime()) {
		t.Fatal("Setup rewrote an existing card to fetch a missing ROM")
	}
}

func TestSetupFailures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/boot.rom":
			w.Write(make([]byte, romSize))
		case "/notzip":
			w.Write([]byte("this is not a zip file"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	for name, opts := range map[string]Options{
		"missing distribution": {DistributionURL: srv.URL + "/gone.zip", BootROMURL: srv.URL + "/boot.rom"},
		"not a zip":            {DistributionURL: srv.URL + "/notzip", BootROMURL: srv.URL + "/boot.rom"},
		"missing boot ROM":     {DistributionURL: srv.URL + "/notzip", BootROMURL: srv.URL + "/gone.rom"},
	} {
		opts.Dir = t.TempDir()
		if err := Setup(context.Background(), opts, nil); err == nil {
			t.Errorf("%s: Setup succeeded", name)
		}
		if In(opts.Dir).Present() {
			t.Errorf("%s: the files are reported present", name)
		}
		left, _ := filepath.Glob(filepath.Join(opts.Dir, "*.part"))
		if len(left) != 0 {
			t.Errorf("%s: left %v behind", name, left)
		}
	}
	// A stopped Setup.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Setup(ctx, Options{Dir: t.TempDir(), DistributionURL: srv.URL + "/notzip", BootROMURL: srv.URL + "/boot.rom"}, nil); err == nil {
		t.Error("a cancelled Setup succeeded")
	}
}

func TestDefaultDownloadsAreChecked(t *testing.T) {
	// The recorded sums are those of 64 hex digits, and a default address is held to them:
	// served something else, download succeeds and Setup refuses the file.
	for _, s := range []string{distributionSHA256, bootROMSHA256} {
		if b, err := hex.DecodeString(s); err != nil || len(b) != sha256.Size {
			t.Fatalf("%q is not a SHA-256", s)
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(make([]byte, romSize)) }))
	defer srv.Close()
	client := srv.Client()
	client.Transport = rewrite{srv.URL}
	dir := t.TempDir()
	err := Setup(context.Background(), Options{Dir: dir, Client: client}, nil)
	if err == nil || !strings.Contains(err.Error(), "not the expected file") {
		t.Fatalf("a wrong boot ROM from the default address gave: %v", err)
	}
	if exists(In(dir).BootROM) {
		t.Fatal("the refused boot ROM was kept")
	}
}

// rewrite sends every request to one test server, whatever host it names.
type rewrite struct{ to string }

func (r rewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	u := *req.URL
	u.Scheme, u.Host = "http", strings.TrimPrefix(r.to, "http://")
	clone := req.Clone(req.Context())
	clone.URL = &u
	return http.DefaultTransport.RoundTrip(clone)
}

// TestRealDistribution rebuilds the real card when ZENZX_NEXT_ZIP names the distribution
// zip, and verifies it. It writes about 2 GB to a temporary folder.
func TestUsableAsksLessThanVerify(t *testing.T) {
	img, _, _ := buildTestImage(t)
	dir := t.TempDir()
	write := func(name string, data []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(BootROMName, make([]byte, romSize))
	write(DivMMCROMName, make([]byte, romSize))
	write(CardName, img)
	if err := Usable(dir); err != nil {
		t.Fatalf("a freshly built card is not usable: %v", err)
	}

	// A card that has been in use, with a free space record that is no longer exact: fine
	// to start from, though Verify would not pass it after a download.
	used := append(memImage(nil), img...)
	used[(testStart+1)*sectorSize+488]++
	write(CardName, used)
	if err := Usable(dir); err != nil {
		t.Fatalf("a card with a stale free space record was turned away: %v", err)
	}
	if err := Verify(dir); err == nil {
		t.Fatal("Verify passed a card whose free space record is wrong")
	}

	write(CardName, make([]byte, len(img)))
	if err := Usable(dir); err == nil {
		t.Fatal("an empty image passed as a card")
	}
	write(CardName, img)
	write(BootROMName, make([]byte, 100))
	if err := Usable(dir); err == nil || !strings.Contains(err.Error(), BootROMName) {
		t.Fatalf("a 100 byte boot ROM gave %v", err)
	}
	os.Remove(filepath.Join(dir, BootROMName))
	if err := Usable(dir); err == nil {
		t.Fatal("a folder without the boot ROM passed")
	}
}

func TestRealDistribution(t *testing.T) {
	path := os.Getenv("ZENZX_NEXT_ZIP")
	if path == "" {
		t.Skip("ZENZX_NEXT_ZIP not set")
	}
	dir := os.Getenv("ZENZX_NEXT_OUT")
	if dir == "" {
		dir = t.TempDir()
	}
	lay, err := FromZip(context.Background(), path, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d files, %d folders, %d bytes; %d of %d clusters used", lay.Files, lay.Dirs, lay.Bytes, lay.Used, lay.Clusters)
	card, err := os.Open(In(dir).Card)
	if err != nil {
		t.Fatal(err)
	}
	defer card.Close()
	if err := VerifyCard(card); err != nil {
		t.Fatal(err)
	}
}
