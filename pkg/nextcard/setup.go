package nextcard

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The three files a Next boots from, by the names they have in the folder that holds them.
const (
	BootROMName   = "nextboot.rom" // the 8K boot ROM: it loads TBBLUE.FW from the card
	DivMMCROMName = "enNxtmmc.rom" // the DivMMC ROM of the distribution the card came from
	CardName      = "next-card.img"

	romSize = 8192
)

// Where the files come from when nobody says otherwise, and the SHA-256 each download must
// have. A download from another address is not checked against these, only for what it
// holds.
const (
	DefaultDistributionURL = "https://www.specnext.com/distro/26.11/sn-emulator-26.11.zip"
	DefaultBootROMURL      = "https://raw.githubusercontent.com/jorgegv/jnext/7aace3a23c5636d7fc64262e7f3e1bbfd75fd713/roms/nextboot.rom"

	distributionSHA256 = "81fe97d37ef4e373643b4d399c22ab2acca631c02a778e8d01e136e0c7c5d2d6"
	bootROMSHA256      = "ee0b99c56b2c4cf72152662288f274483f8380865df0cbdcf3774f7974494798"
)

// Files names the three files in a folder.
type Files struct {
	Dir, BootROM, DivMMCROM, Card string
}

// In returns the three file names in dir.
func In(dir string) Files {
	return Files{Dir: dir, BootROM: filepath.Join(dir, BootROMName),
		DivMMCROM: filepath.Join(dir, DivMMCROMName), Card: filepath.Join(dir, CardName)}
}

// Present reports whether all three files exist.
func (f Files) Present() bool {
	for _, p := range []string{f.BootROM, f.DivMMCROM, f.Card} {
		if st, err := os.Stat(p); err != nil || st.IsDir() {
			return false
		}
	}
	return true
}

// DefaultDir is the folder the files are downloaded to: zenzx/next under the user's
// configuration folder.
func DefaultDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "zenzx", "next"), nil
}

// Stage is a step of Setup, for the progress display.
type Stage int

const (
	StageBootROM  Stage = iota // downloading the boot ROM
	StageDownload              // downloading the distribution
	StageUnpack                // taking the card image out of the zip
	StageBuild                 // writing the new card
	StageVerify                // checking the result
)

// String is the stage as a line of the progress display.
func (s Stage) String() string {
	return [...]string{"Downloading the boot ROM", "Downloading the distribution", "Unpacking the card image",
		"Building the SD card", "Verifying"}[s]
}

// Progress is told how far a stage has got. total is 0 when it is not known.
type Progress func(stage Stage, done, total int64)

// Options are what Setup needs to know.
type Options struct {
	Dir             string // where the files go
	DistributionURL string // empty means DefaultDistributionURL
	BootROMURL      string // empty means DefaultBootROMURL
	Client          *http.Client
}

// Setup makes the three files in opts.Dir: it downloads what is missing, builds the card
// and verifies the result. A file that is already there is kept, the card above all, since
// it may hold what the user has saved. The distribution zip is deleted once the card is
// made. Setup can be stopped through ctx; what it leaves behind is then only ".part" files,
// which the next run replaces.
func Setup(ctx context.Context, opts Options, progress Progress) error {
	if progress == nil {
		progress = func(Stage, int64, int64) {}
	}
	if opts.DistributionURL == "" {
		opts.DistributionURL = DefaultDistributionURL
	}
	if opts.BootROMURL == "" {
		opts.BootROMURL = DefaultBootROMURL
	}
	if opts.Client == nil {
		opts.Client = http.DefaultClient
	}
	if err := os.MkdirAll(opts.Dir, 0o755); err != nil {
		return err
	}
	f := In(opts.Dir)

	if !exists(f.BootROM) {
		sum, err := download(ctx, opts.Client, opts.BootROMURL, f.BootROM, func(done, total int64) { progress(StageBootROM, done, total) })
		if err != nil {
			return fmt.Errorf("the boot ROM: %w", err)
		}
		if opts.BootROMURL == DefaultBootROMURL && sum != bootROMSHA256 {
			os.Remove(f.BootROM)
			return fmt.Errorf("the boot ROM that was downloaded is not the expected file (SHA-256 %s)", sum)
		}
	}
	if !exists(f.Card) || !exists(f.DivMMCROM) {
		zipPath := filepath.Join(opts.Dir, "distribution.zip")
		sum, err := download(ctx, opts.Client, opts.DistributionURL, zipPath, func(done, total int64) { progress(StageDownload, done, total) })
		if err != nil {
			return fmt.Errorf("the distribution: %w", err)
		}
		if opts.DistributionURL == DefaultDistributionURL && sum != distributionSHA256 {
			os.Remove(zipPath)
			return fmt.Errorf("the distribution that was downloaded is not the expected file (SHA-256 %s)", sum)
		}
		if _, err := FromZip(ctx, zipPath, opts.Dir, progress); err != nil {
			return err
		}
		os.Remove(zipPath)
	}
	progress(StageVerify, 0, 0)
	return Verify(opts.Dir)
}

func exists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// download fetches url into path through a ".part" file and returns the SHA-256 of what
// it wrote.
func download(ctx context.Context, client *http.Client, url, path string, progress func(done, total int64)) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "zenzx")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s answered %s", url, resp.Status)
	}
	part := path + ".part"
	out, err := os.Create(part)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	total := resp.ContentLength
	if total < 0 {
		total = 0
	}
	var done int64
	buf := make([]byte, 256*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := out.Write(buf[:n]); err != nil {
				out.Close()
				os.Remove(part)
				return "", err
			}
			hash.Write(buf[:n])
			done += int64(n)
			progress(done, total)
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			out.Close()
			os.Remove(part)
			return "", rerr
		}
	}
	if err := out.Close(); err != nil {
		os.Remove(part)
		return "", err
	}
	if total > 0 && done != total {
		os.Remove(part)
		return "", fmt.Errorf("the download stopped at %d of %d bytes", done, total)
	}
	if err := os.Rename(part, path); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

var timingLine = regexp.MustCompile(`(?im)^timing=\d+`)

// FromZip makes the card and the DivMMC ROM in dir from a distribution zip. A card that is
// already in dir is replaced: Setup, which keeps an existing card, is what callers that
// must not lose one use.
func FromZip(ctx context.Context, zipPath, dir string, progress Progress) (Layout, error) {
	if progress == nil {
		progress = func(Stage, int64, int64) {}
	}
	z, err := zip.OpenReader(zipPath)
	if err != nil {
		return Layout{}, fmt.Errorf("the distribution is not a zip file: %w", err)
	}
	defer z.Close()
	var image, rom *zip.File
	for _, e := range z.File {
		switch name := strings.ToLower(filepath.Base(e.Name)); {
		case strings.HasSuffix(name, ".img"):
			image = e
		case name == strings.ToLower(DivMMCROMName):
			rom = e
		}
	}
	if image == nil || rom == nil {
		return Layout{}, errors.New("the distribution holds no card image or no " + DivMMCROMName)
	}
	f := In(dir)

	r, err := rom.Open()
	if err != nil {
		return Layout{}, err
	}
	romData, err := io.ReadAll(io.LimitReader(r, romSize+1))
	r.Close()
	if err != nil || len(romData) != romSize {
		return Layout{}, fmt.Errorf("%s in the distribution is not an 8K ROM", DivMMCROMName)
	}
	if err := os.WriteFile(f.DivMMCROM, romData, 0o644); err != nil {
		return Layout{}, err
	}

	// The image has to be read at random, so it is unpacked first.
	source := filepath.Join(dir, "distribution.img.part")
	defer os.Remove(source)
	if err := unpack(ctx, image, source, func(done, total int64) { progress(StageUnpack, done, total) }); err != nil {
		return Layout{}, err
	}
	src, err := os.Open(source)
	if err != nil {
		return Layout{}, err
	}
	defer src.Close()
	st, err := src.Stat()
	if err != nil {
		return Layout{}, err
	}

	part := f.Card + ".part"
	defer os.Remove(part)
	lay, err := Rebuild(ctx, src, st.Size(), part, func(done, total int64) { progress(StageBuild, done, total) })
	if err != nil {
		return lay, err
	}
	return lay, os.Rename(part, f.Card)
}

func unpack(ctx context.Context, e *zip.File, path string, progress func(done, total int64)) error {
	in, err := e.Open()
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	total := int64(e.UncompressedSize64)
	buf := make([]byte, 1<<20)
	var done int64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, rerr := in.Read(buf)
		if n > 0 {
			if _, err := out.Write(buf[:n]); err != nil {
				return err
			}
			done += int64(n)
			progress(done, total)
		}
		if rerr == io.EOF {
			return nil
		}
		if rerr != nil {
			return rerr
		}
	}
}

// Rebuild writes to path a card with the files of the image src, as the package comment
// describes.
func Rebuild(ctx context.Context, src io.ReaderAt, size int64, path string, progress func(done, total int64)) (Layout, error) {
	mbr, start, sectors, err := partition(src)
	if err != nil {
		return Layout{}, err
	}
	vol, err := Open(src)
	if err != nil {
		return Layout{}, err
	}
	if cfg := vol.Root.Find("machines/next/config.ini"); cfg != nil {
		data, err := vol.ReadFile(cfg)
		if err != nil {
			return Layout{}, err
		}
		if len(timingLine.FindAll(data, -1)) != 1 {
			return Layout{}, errors.New("config.ini on the card does not have exactly one timing line")
		}
		cfg.SetData(timingLine.ReplaceAll(data, []byte("timing=0")))
	} else {
		return Layout{}, errors.New("the card has no /machines/next/config.ini")
	}
	if rom := vol.Root.Find("machines/next/zx81.rom"); rom != nil && rom.Size == 16384 {
		data, err := vol.ReadFile(rom)
		if err != nil {
			return Layout{}, err
		}
		rom.SetData(append(append([]byte{}, data...), data...))
	}
	// Short names are made again from the long ones: see the package comment.
	vol.Root.Walk(func(_ string, n *Node) {
		n.Short, n.NTRes = [11]byte{}, 0
	})
	out, err := os.Create(path)
	if err != nil {
		return Layout{}, err
	}
	if err := out.Truncate(size); err != nil {
		out.Close()
		return Layout{}, err
	}
	lay, err := Build(out, mbr, start, sectors, vol.Root, func(n *Node) ([]byte, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return vol.ReadFile(n)
	}, progress)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return lay, err
}

// Verify checks the three files in dir: the ROMs for their size, and the card for being a
// FAT32 volume the Next's firmware accepts, with its two FAT copies equal, its free space
// record in step with the FAT, the firmware on it and timing=0 in config.ini.
func Verify(dir string) error {
	f := In(dir)
	for _, p := range []string{f.BootROM, f.DivMMCROM} {
		st, err := os.Stat(p)
		if err != nil {
			return fmt.Errorf("%s is missing", filepath.Base(p))
		}
		if st.Size() != romSize {
			return fmt.Errorf("%s is %d bytes, not the 8192 of an 8K ROM", filepath.Base(p), st.Size())
		}
	}
	card, err := os.Open(f.Card)
	if err != nil {
		return fmt.Errorf("%s is missing", CardName)
	}
	defer card.Close()
	return VerifyCard(card)
}

// Usable reports whether the three files in dir are ones a Next can be started from: both
// ROMs 8K, and the card a FAT32 volume the firmware accepts with the firmware on it. It is
// the check made at every start, so it asks for less than Verify does after a download: a
// card that has been in use, or was made by other means, is not turned away for its free
// space record or its video setting.
func Usable(dir string) error {
	f := In(dir)
	for _, p := range []string{f.BootROM, f.DivMMCROM} {
		st, err := os.Stat(p)
		if err != nil {
			return fmt.Errorf("%s is missing", filepath.Base(p))
		}
		if st.Size() != romSize {
			return fmt.Errorf("%s is %d bytes, not the 8192 of an 8K ROM", filepath.Base(p), st.Size())
		}
	}
	card, err := os.Open(f.Card)
	if err != nil {
		return fmt.Errorf("%s is missing", CardName)
	}
	defer card.Close()
	vol, err := Open(card)
	if err != nil {
		return fmt.Errorf("%s: %w", CardName, err)
	}
	if vol.geo.clusterCount < MinFAT32Clusters {
		return fmt.Errorf("%s has %d clusters; the firmware wants at least %d", CardName, vol.geo.clusterCount, MinFAT32Clusters)
	}
	if fw := vol.Root.Find("TBBLUE.FW"); fw == nil || fw.Size == 0 {
		return fmt.Errorf("%s has no TBBLUE.FW, the firmware", CardName)
	}
	return nil
}

// VerifyCard is the card part of Verify.
func VerifyCard(card io.ReaderAt) error {
	vol, err := Open(card)
	if err != nil {
		return fmt.Errorf("the card: %w", err)
	}
	g := vol.geo
	if g.clusterCount < MinFAT32Clusters {
		return fmt.Errorf("the card has %d clusters; the firmware wants at least %d", g.clusterCount, MinFAT32Clusters)
	}
	first := make([]byte, (int(g.clusterCount)+2)*4)
	second := make([]byte, len(first))
	if _, err := card.ReadAt(first, g.fatOffset(0)); err != nil {
		return err
	}
	if g.fats > 1 {
		if _, err := card.ReadAt(second, g.fatOffset(1)); err != nil {
			return err
		}
		if !bytes.Equal(first, second) {
			return errors.New("the card's two FAT copies differ")
		}
	}
	free := uint32(0)
	for _, e := range vol.fat[2:] {
		if e == 0 {
			free++
		}
	}
	info := make([]byte, sectorSize)
	if _, err := card.ReadAt(info, (g.start+int64(g.fsInfo))*sectorSize); err != nil {
		return err
	}
	if string(info[0:4]) != "RRaA" || string(info[484:488]) != "rrAa" {
		return errors.New("the card has no free space record")
	}
	if rec := uint32(info[488]) | uint32(info[489])<<8 | uint32(info[490])<<16 | uint32(info[491])<<24; rec != 0xFFFFFFFF && rec != free {
		return fmt.Errorf("the card's free space record says %d clusters and the FAT has %d free", rec, free)
	}
	fw := vol.Root.Find("TBBLUE.FW")
	if fw == nil || fw.Size == 0 {
		return errors.New("the card has no TBBLUE.FW, the firmware")
	}
	cfg := vol.Root.Find("machines/next/config.ini")
	if cfg == nil {
		return errors.New("the card has no /machines/next/config.ini")
	}
	data, err := vol.ReadFile(cfg)
	if err != nil {
		return err
	}
	if !regexp.MustCompile(`(?im)^timing=0\s*$`).Match(data) {
		return errors.New("config.ini on the card does not say timing=0, so the firmware would stop at its video-mode test")
	}
	return nil
}
