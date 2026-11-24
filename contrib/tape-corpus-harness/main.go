// Command tape-corpus-harness measures zenzx's real-world tape-loading
// coverage and speed against a corpus of actual commercial ZX Spectrum
// releases. See README.md for why this lives in its own module, why it
// downloads its own corpus rather than assuming one exists, and the real
// limits of how it determines success (screenshot-diffing at arm's
// length, not the checksum-level verification register item T-24 in the
// main zenzx repo calls for).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	corpusURL     = "https://archive.org/download/zxspectrum-top-100/ZXSpectrumTop100-noDoc.zip"
	corpusZipName = "ZXSpectrumTop100-noDoc.zip"
)

type modeResult struct {
	Game             string   `json:"game"`
	TapeFile         string   `json:"tape_file"`
	Mode             string   `json:"mode"`
	MilestoneFrame   int      `json:"milestone_frame"`
	MilestoneSeconds float64  `json:"milestone_seconds"`
	RanOutOfTime     bool     `json:"ran_out_of_time"`
	ScriptFailures   []string `json:"script_failures,omitempty"`
	FinalScreenshot  string   `json:"final_screenshot,omitempty"`
	Note             string   `json:"note,omitempty"`
}

func main() {
	zenzxBin := flag.String("zenzx-bin", "", "path to a built zenzx-headless binary (required)")
	cacheDir := flag.String("cache-dir", "./cache", "corpus download/extract dir, or an existing corpus directory")
	gamesFlag := flag.String("games", "Chase H.Q,Cybernoid 2", "comma-separated game directory names, or 'all'")
	modesFlag := flag.String("modes", "fast,accurate,turbo", "comma-separated subset of fast,accurate,turbo")
	model := flag.String("model", "128k", "Spectrum model passed to zenzx-headless for every run (see README: a known simplification)")
	outPath := flag.String("out", "report.json", "output report path")
	maxWait := flag.Duration("max-wait", 15*time.Minute, "per-tape wall-clock timeout")
	shotIntervalFrames := flag.Int("shot-interval", 100, "frames between periodic screenshots used for external completion detection")
	keepWorkDir := flag.Bool("keep-work-dir", false, "don't delete the per-run working directory (screenshots, generated .zen scripts) on exit -- useful for inspecting why a result looks wrong")
	flag.Parse()

	if *zenzxBin == "" {
		fmt.Fprintln(os.Stderr, "tape-corpus-harness: -zenzx-bin is required")
		flag.Usage()
		os.Exit(1)
	}
	if _, err := os.Stat(*zenzxBin); err != nil {
		fmt.Fprintf(os.Stderr, "tape-corpus-harness: -zenzx-bin not found: %v\n", err)
		os.Exit(1)
	}

	if err := ensureCorpus(*cacheDir); err != nil {
		fmt.Fprintf(os.Stderr, "tape-corpus-harness: corpus setup failed: %v\n", err)
		os.Exit(1)
	}

	var games []string
	if strings.EqualFold(strings.TrimSpace(*gamesFlag), "all") {
		var err error
		games, err = listGameDirs(*cacheDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tape-corpus-harness: %v\n", err)
			os.Exit(1)
		}
	} else {
		for _, g := range strings.Split(*gamesFlag, ",") {
			games = append(games, strings.TrimSpace(g))
		}
	}
	var modes []string
	for _, m := range strings.Split(*modesFlag, ",") {
		modes = append(modes, strings.TrimSpace(m))
	}

	workDir, err := os.MkdirTemp("", "tape-corpus-harness-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "tape-corpus-harness: %v\n", err)
		os.Exit(1)
	}
	if *keepWorkDir {
		fmt.Printf("keeping work dir: %s\n", workDir)
	} else {
		defer os.RemoveAll(workDir)
	}

	var results []modeResult
	for _, g := range games {
		tapePath, note, err := findTape(*cacheDir, g)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tape-corpus-harness: %s: %v\n", g, err)
			continue
		}
		for _, m := range modes {
			res, err := runOne(*zenzxBin, tapePath, g, m, *model, *maxWait, *shotIntervalFrames, workDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "tape-corpus-harness: %s/%s: %v\n", g, m, err)
				continue
			}
			res.Note = note
			results = append(results, res)
			fmt.Printf("%-24s %-9s milestone=%7.1fs ran_out_of_time=%-5v %s\n",
				g, m, res.MilestoneSeconds, res.RanOutOfTime, res.TapeFile)
		}
	}

	f, err := os.Create(*outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tape-corpus-harness: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(results); err != nil {
		fmt.Fprintf(os.Stderr, "tape-corpus-harness: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%d results)\n", *outPath, len(results))
}

// ensureCorpus downloads and extracts the archive.org corpus into
// cacheDir if it doesn't already look populated (i.e. cacheDir doesn't
// already contain game subdirectories -- lets a caller point -cache-dir
// at a pre-existing local corpus and skip the download entirely).
func ensureCorpus(cacheDir string) error {
	if entries, err := os.ReadDir(cacheDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				return nil // already populated
			}
		}
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}

	zipPath := filepath.Join(cacheDir, corpusZipName)
	if _, err := os.Stat(zipPath); err != nil {
		fmt.Printf("downloading corpus (~39MB, one-time) from %s ...\n", corpusURL)
		if err := downloadFile(corpusURL, zipPath); err != nil {
			return fmt.Errorf("download: %w", err)
		}
	}

	fmt.Println("extracting corpus...")
	if err := extractZip(zipPath, cacheDir); err != nil {
		return fmt.Errorf("extract: %w", err)
	}
	return nil
}

func downloadFile(url, dest string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}
	tmp := dest + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	out.Close()
	return os.Rename(tmp, dest)
}

func extractZip(zipPath, destDir string) error {
	// Shell out to unzip rather than archive/zip here: several games in
	// this corpus use filenames with characters (accented, bracketed
	// variant markers) that have tripped Go's archive/zip name decoding
	// on some past corpora; system unzip handles them without fuss and
	// this tool already assumes a Unix-like environment (it drives a
	// zenzx-headless binary, which is itself typically built for one).
	cmd := exec.Command("unzip", "-o", "-q", zipPath, "-d", destDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("unzip: %w: %s", err, out)
	}
	return nil
}

func listGameDirs(cacheDir string) ([]string, error) {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return nil, err
	}
	var games []string
	for _, e := range entries {
		if e.IsDir() {
			games = append(games, e.Name())
		}
	}
	sort.Strings(games)
	return games, nil
}

// findTape picks one tape file per game directory. Known simplification
// (see README): many games have several archived variants that differ
// in whether they actually load correctly -- this session's own Chase
// H.Q. investigation found 3 of 7 variants never completed at all. This
// picks the first .tzx, else the first .tap, alphabetically, and
// returns a note flagging that a variant choice was made, not a
// guarantee that it's a working one.
func findTape(cacheDir, game string) (path string, note string, err error) {
	dir := filepath.Join(cacheDir, game)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", "", fmt.Errorf("game directory not found: %w", err)
	}
	var tzxs, taps []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		lower := strings.ToLower(e.Name())
		if strings.HasSuffix(lower, ".tzx") {
			tzxs = append(tzxs, e.Name())
		} else if strings.HasSuffix(lower, ".tap") {
			taps = append(taps, e.Name())
		}
	}
	sort.Strings(tzxs)
	sort.Strings(taps)
	var chosen string
	switch {
	case len(tzxs) > 0:
		chosen = tzxs[0]
	case len(taps) > 0:
		chosen = taps[0]
	default:
		return "", "", fmt.Errorf("no .tap/.tzx file found")
	}
	total := len(tzxs) + len(taps)
	note = fmt.Sprintf("picked %q (1 of %d tape variants archived for this title, not verified as the working one)", chosen, total)
	return filepath.Join(dir, chosen), note, nil
}

// runOne drives one (game, mode) combination via a generated .zen
// script and zenzx-headless as a subprocess, then determines a
// completion milestone externally from the periodic screenshots --
// see README's "How success is determined" section for the honest
// limits of this approach.
func runOne(zenzxBin, tapePath, game, mode, model string, maxWait time.Duration, shotInterval int, workDir string) (modeResult, error) {
	res := modeResult{Game: game, TapeFile: filepath.Base(tapePath), Mode: mode}

	runDir, err := os.MkdirTemp(workDir, "run-")
	if err != nil {
		return res, err
	}
	shotDir := filepath.Join(runDir, "shots")
	if err := os.MkdirAll(shotDir, 0755); err != nil {
		return res, err
	}

	maxFrames := int(maxWait.Seconds() * 50) // 50Hz PAL

	// -shot-interval only applies when NO script drives the run (see
	// docs/zenscript.md: "the script's shot actions are the only
	// source of screenshots" once one is active) -- so periodic
	// capture has to be written into the script itself, not passed as
	// a CLI flag. shotInterval here is the SAME frame-count spacing
	// the caller asked for, just applied by generating one "shot"
	// action per interval instead.
	var sb strings.Builder
	sb.WriteString("wait-boot\n")
	sb.WriteString("0 type \"LOAD \\\"\\\"\"\n")
	sb.WriteString("10 key enter\n")
	for f := shotInterval; f <= maxFrames; f += shotInterval {
		fmt.Fprintf(&sb, "%d shot\n", f)
	}
	scriptPath := filepath.Join(runDir, "load.zen")
	if err := os.WriteFile(scriptPath, []byte(sb.String()), 0644); err != nil {
		return res, err
	}

	ctx, cancel := timeoutContext(maxWait + 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, zenzxBin,
		"-tape", tapePath,
		"-tapemode", mode,
		"-model", model,
		"-script", scriptPath,
		"-frames", fmt.Sprint(maxFrames),
		// NOT -shot-interval: a no-op once -script is active (see the
		// comment above where the script is generated).
		"-shot-dir", shotDir,
		"-shot-prefix", "run",
		"-quiet",
	)
	stderr, err := cmd.CombinedOutput()
	if err != nil && ctx.Err() == nil {
		// A non-zero exit that ISN'T our own timeout is worth surfacing,
		// but not fatal to the whole sweep -- still try to analyse
		// whatever screenshots were produced before the failure.
		res.Note = fmt.Sprintf("zenzx-headless exited with error: %v", err)
	}
	if ctx.Err() != nil {
		res.RanOutOfTime = true
	}
	res.ScriptFailures = extractFailures(string(stderr))

	frame, secs, shotPath, aerr := analyseScreenshots(shotDir)
	if aerr == nil {
		res.MilestoneFrame = frame
		res.MilestoneSeconds = secs
		res.FinalScreenshot = shotPath
	}

	return res, nil
}

func extractFailures(stderrText string) []string {
	var fails []string
	for _, line := range strings.Split(stderrText, "\n") {
		if strings.Contains(line, "FAILED") {
			fails = append(fails, strings.TrimSpace(line))
		}
	}
	return fails
}

// analyseScreenshots finds the last frame at which the screen content
// meaningfully changed, treating that as the real completion milestone
// rather than trusting any zenzx-internal "tape finished" signal --
// see this session's own Chase H.Q. investigation for why that
// distinction matters (docs/TAPE_LOADING_HANDOVER.md in the main repo).
func analyseScreenshots(shotDir string) (frame int, seconds float64, lastPath string, err error) {
	entries, err := os.ReadDir(shotDir)
	if err != nil {
		return 0, 0, "", err
	}
	type shot struct {
		frame int
		path  string
	}
	var shots []shot
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".png") {
			continue
		}
		var f int
		if _, serr := fmt.Sscanf(e.Name(), "run-frame%d.png", &f); serr == nil {
			shots = append(shots, shot{f, filepath.Join(shotDir, e.Name())})
		}
	}
	if len(shots) == 0 {
		return 0, 0, "", fmt.Errorf("no screenshots captured")
	}
	sort.Slice(shots, func(i, j int) bool { return shots[i].frame < shots[j].frame })

	var lastSig string
	lastChange := shots[0].frame
	for _, s := range shots {
		sig, serr := imageSignature(s.path)
		if serr != nil {
			continue
		}
		if sig != lastSig {
			lastChange = s.frame
			lastSig = sig
		}
	}
	last := shots[len(shots)-1]
	return lastChange, float64(lastChange) / 50.0, last.path, nil
}

// imageSignature is a cheap, dependency-free content fingerprint: not
// cryptographic, just enough to detect "this frame differs from the
// last one" across a few hundred screenshots.
func imageSignature(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return "", err
	}
	b := img.Bounds()
	var sum uint64
	for y := b.Min.Y; y < b.Max.Y; y += 3 {
		for x := b.Min.X; x < b.Max.X; x += 3 {
			r, g, bl, _ := colorAt(img, x, y)
			sum = sum*1099511628211 ^ uint64(r)<<16 ^ uint64(g)<<8 ^ uint64(bl)
		}
	}
	return fmt.Sprintf("%x", sum), nil
}

func colorAt(img image.Image, x, y int) (r, g, b, a uint32) {
	return img.At(x, y).RGBA()
}

func timeoutContext(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}
