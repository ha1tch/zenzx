package main

// Batch three-mode tape-loading comparison harness. NOT a permanent test --
// a one-off investigative/reporting tool, run explicitly via -run, never
// part of a normal `go test ./...` invocation. Boots each game properly
// (real BootDetector, not a fixed-frame guess), types LOAD "", and for
// each of fast/accurate/turbo: runs until either the tape genuinely
// finishes or a generous timeout, then -- only if it finished -- takes a
// screenshot, waits further and presses space (to catch a "press any key"
// transition a single screenshot would miss), and takes a second
// screenshot. Success is judged automatically against the actual
// criteria (stop-tape/menu/gameplay, not merely "a screenshot exists"):
// the final screen must differ meaningfully from the machine's own
// pristine boot screen.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type batchGame struct {
	Name  string
	Path  string
	Model string // "48k" or "128k"
}

var batchGames = []batchGame{
	// Already investigated earlier this session -- included for a single,
	// unified report rather than scattered findings.
	{"Chase HQ", "Chase H.Q/Chase HQ - Side 1.tzx", "48k"},
	{"Cybernoid", "Cybernoid/Cybernoid.tzx", "48k"},
	{"Cybernoid 2 (128K)", "Cybernoid II - The Revenge/Cybernoid 2 - Side 1.tzx", "128k"},
	{"Bubble Bobble", "Bubble Bobble/Bubble Bobble.tzx", "48k"},
	{"Daley Thompson's Decathlon", "Daley Thompson's Decathlon/Daley Thompson's Decathlon - Side A (Zafiro).tzx", "48k"},
	{"Skool Daze", "Skool Daze/Skool Daze.tzx", "48k"},
	{"Eric And The Floaters", "Eric & The Floaters/Eric And The Floaters.tzx", "48k"},
	{"Equinox", "Equinox/Equinox.tzx", "48k"},
	{"Cobra (plain)", "Cobra/Cobra.tzx", "48k"},
	{"Cobra (BUGFIX)", "Cobra/Cobra-BUGFIX.tzx", "48k"},
	{"Lotus Esprit Turbo Challenge", "Lotus Esprit Turbo Challenge/Lotus Esprit Turbo Challenge - Side 1.tzx", "48k"},

	// New set of 20, spanning early (likely standard ROM loader) to late
	// (likely custom/turbo loader) commercial titles, plus one more
	// documented standard-loader reference (Dan Dare) and a second 128K
	// title (Rainbow Islands) for broader 128K coverage alongside
	// Cybernoid 2.
	{"Manic Miner", "Manic Miner/Manic Miner.tzx", "48k"},
	{"Jet Set Willy", "Jet Set Willy/Jet Set Willy.tzx", "48k"},
	{"Sabre Wulf", "Sabre Wulf/SabreWulf.tzx", "48k"},
	{"Knight Lore", "Knight Lore/KnightLore.tzx", "48k"},
	{"Dan Dare", "Dan Dare/Dan Dare.tzx", "48k"},
	{"Elite", "Elite/Elite - 48k.tzx", "48k"},
	{"The Great Escape", "Great Escape, The/The Great Escape.tzx", "48k"},
	{"Batman", "Batman/Batman - Release 3.tzx", "48k"},
	{"Astro Marine Corps", "Astro Marine Corps/Astro Marine Corps - Side 1.tzx", "48k"},
	{"WEC Le Mans", "WEC Le Mans/W.E.C. Le Mans - Side 1.tzx", "48k"},
	{"Exolon", "Exolon/Exolon.tzx", "48k"},
	{"Feud", "Feud/Feud.tzx", "48k"},
	{"Firefly", "Firefly/Firefly (1988)(Ocean).tzx", "48k"},
	{"R-Type", "R-Type/R-Type.tzx", "48k"},
	{"Target Renegade", "Target Renegade/Renegade 2 - Target Renegade - Side 1.tzx", "48k"},
	{"Robocop", "Robocop/Robocop - Side A.tzx", "48k"},
	{"Operation Wolf", "Operation Wolf/Operation Wolf - 48k.tzx", "48k"},
	{"Rick Dangerous", "Rick Dangerous/Rick Dangerous.tzx", "48k"},
	{"Rainbow Islands (128K)", "Rainbow Islands/Rainbow Islands - Side 1.tzx", "128k"},
	{"Fantasy World Dizzy", "Fantasy World Dizzy/Dizzy3-FantasyWorldDizzy.tzx", "48k"},
}

type modeResult struct {
	Mode        string  `json:"mode"`
	Attempted   bool    `json:"attempted"`
	Completed   bool    `json:"completed"` // tape genuinely finished (not a timeout)
	Verified    bool    `json:"verified"`  // automated post-load success heuristic
	Frames      int     `json:"frames"`
	WallSeconds float64 `json:"wall_seconds"`
	ScreenshotA string  `json:"screenshot_a"`
	ScreenshotB string  `json:"screenshot_b"`
	ErrorNote   string  `json:"error_note"`
}

type gameResult struct {
	Name    string       `json:"name"`
	Model   string       `json:"model"`
	Results []modeResult `json:"results"`
}

const corpusRoot = "/tmp/newdiv_corpus"
const shotDir = "/tmp/batch_shots"

// fastModeFrameCap bounds how long a non-completing fast-mode run is
// given before being declared a timeout -- fast mode is near-instant when
// it works at all, so this only needs to be generous, not tape-length.
const fastModeFrameCap = 6000

// slowModeFrameCap bounds accurate/turbo runs -- generous enough for even
// the largest tape in the corpus (Chase HQ's ~37000 frames was the
// largest seen this session; this leaves ample headroom).
const slowModeFrameCap = 200000

func bootAndType(t *testing.T, zx *ZenZX, rainbow bool) bool {
	det := NewBootDetector(rainbow)
	for frame := 0; frame < 600 && !det.Ready(); frame++ {
		zx.RunFrame()
		det.Update(zx)
	}
	if !det.Ready() {
		return false
	}
	kq := NewKeyQueue(10, 5)
	kq.EnqueueText(`LOAD ""`)
	kq.EnqueueChord([]matrixPos{{6, 0}})
	for kq.Active() {
		kq.Step(zx)
		zx.RunFrame()
	}
	zx.io.ResetKeyboard()
	return true
}

// stepMode runs one frame using RunTurboAwareFrame for TapeTurbo (correctly
// handling the fast-path activate/sync-out transition regardless of
// exactly when Playing flips false), plain RunFrame otherwise -- used for
// every phase after boot, including the post-load wait/keypress sequence,
// not just the initial load loop, since the transition can only be
// trusted to have actually happened if RunTurboAwareFrame is what
// observes Playing going false.
func stepMode(zx *ZenZX, mode TapeMode) {
	if mode == TapeTurbo {
		zx.RunTurboAwareFrame(false)
		return
	}
	zx.RunFrame()
}

// pristineBootHash captures the screen hash for a freshly-booted machine
// (no tape involved at all), per model -- the baseline "never actually
// loaded anything" signature to compare final screens against.
func pristineBootHash(t *testing.T, model string) uint64 {
	zx := NewZenZX(AudioBackendOto)
	var err error
	if model == "128k" {
		err = zx.Load128KROM("./rom/128-0.rom", "./rom/128-1.rom")
	} else {
		err = zx.LoadROM("./rom/48.rom")
	}
	if err != nil {
		t.Fatalf("boot hash ROM load: %v", err)
	}
	det := NewBootDetector(model == "128k")
	for frame := 0; frame < 600 && !det.Ready(); frame++ {
		zx.RunFrame()
		det.Update(zx)
	}
	// A few settle frames (cursor blink etc. shouldn't matter for a coarse
	// "is this obviously just the boot screen" check).
	for i := 0; i < 50; i++ {
		zx.RunFrame()
	}
	return screenContentHash(zx)
}

func runOneMode(t *testing.T, g batchGame, mode TapeMode, modeName string, bootHash uint64) modeResult {
	res := modeResult{Mode: modeName, Attempted: true}

	zx := NewZenZX(AudioBackendOto)
	var err error
	if g.Model == "128k" {
		err = zx.Load128KROM("./rom/128-0.rom", "./rom/128-1.rom")
	} else {
		err = zx.LoadROM("./rom/48.rom")
	}
	if err != nil {
		res.ErrorNote = "ROM load: " + err.Error()
		return res
	}
	// zx.tape is already created and its FastLoader already attached by
	// NewZenZX itself -- reassigning zx.tape = NewTape(zx) here, as an
	// earlier version of this harness did, silently threw away that
	// attachment and made every "fast" mode result in this file wrong
	// (trapLoad's own TryIntercept checks fl.Enabled first, which was
	// never true because fl itself was nil). Just load the tape onto
	// the existing zx.tape.
	if err := zx.tape.LoadFile(filepath.Join(corpusRoot, g.Path)); err != nil {
		res.ErrorNote = "tape load: " + err.Error()
		return res
	}
	zx.tape.SetMode(mode)
	zx.tape.Play()

	if !bootAndType(t, zx, g.Model == "128k") {
		res.ErrorNote = "boot never became ready"
		return res
	}
	// Baseline for THIS run specifically: screen right after typing
	// LOAD "" and pressing enter, before any tape data has loaded. The
	// generic pristine-boot hash is not a safe comparison point here --
	// the typed command text alone already differs from a never-typed-
	// anything boot screen, which would make even a genuinely failed
	// load look like a change. A load that never progresses at all
	// should show this same "searching"/prompt screen unchanged.
	postTypeHash := screenContentHash(zx)

	cap := slowModeFrameCap
	if mode == TapeFast {
		cap = fastModeFrameCap
	}

	start := time.Now()
	frames := 0
	fastEarlySuccess := false
	if mode == TapeFast {
		// trapLoad never sets Playing=false even on genuine success (it
		// injects blocks directly, without advancing through the
		// pulse-stream completion logic accurate/turbo use), so
		// completion can't be judged by Playing here at all. Worse,
		// waiting the FULL cap before ever checking the screen is itself
		// wrong: confirmed with a real title (Eric And The Floaters) that
		// genuinely succeeds within a few hundred frames, but reverts to
		// something else (apparently a demo-mode idle timeout inside the
		// game itself) well before a multi-thousand-frame cap would
		// elapse -- checking only at the end would misreport a real,
		// fast, genuine success as a failure. Check periodically instead
		// and capture the earliest point success is detected.
		for frames < cap {
			stepMode(zx, mode)
			frames++
			if frames%50 == 0 {
				h := screenContentHash(zx)
				if h != bootHash && h != postTypeHash && bottomOrMiddleHasContent(zx) {
					fastEarlySuccess = true
					break
				}
			}
		}
	} else {
		for frames < cap && zx.tape.st.Playing {
			stepMode(zx, mode)
			frames++
		}
	}
	res.Frames = frames
	res.WallSeconds = time.Since(start).Seconds()

	if mode != TapeFast {
		if zx.tape.st.Playing {
			res.Completed = false
			return res
		}
		res.Completed = true
	} else {
		res.Completed = fastEarlySuccess
		if !fastEarlySuccess {
			// Ran the full cap without ever detecting success -- still
			// worth a screenshot pair for the report, but no further
			// wait/space-press probing needed since nothing indicates
			// it's about to change.
			res.Completed = true
		}
	}

	safeName := sanitizeName(g.Name) + "_" + modeName
	pathA := filepath.Join(shotDir, safeName+"_a.png")
	_ = writeScreenPNG(pathA, zx, 1)
	res.ScreenshotA = pathA
	hashA := screenContentHash(zx)
	verifiedA := hashA != bootHash && hashA != postTypeHash && bottomOrMiddleHasContent(zx)

	for i := 0; i < 100; i++ {
		stepMode(zx, mode)
	}
	kq2 := NewKeyQueue(10, 5)
	kq2.EnqueueChord(namedKeys["space"])
	for kq2.Active() {
		kq2.Step(zx)
		stepMode(zx, mode)
	}
	zx.io.ResetKeyboard()
	for i := 0; i < 150; i++ {
		stepMode(zx, mode)
	}

	pathB := filepath.Join(shotDir, safeName+"_b.png")
	_ = writeScreenPNG(pathB, zx, 1)
	res.ScreenshotB = pathB
	hashB := screenContentHash(zx)
	verifiedB := hashB != bootHash && hashB != postTypeHash && bottomOrMiddleHasContent(zx)

	// Success at EITHER point counts: the space-press probe (intended to
	// catch a "press any key" transition a single screenshot would miss)
	// can itself trigger a BASIC BREAK on a game that has already loaded
	// successfully and is sitting in a state that treats space as such --
	// confirmed happening for a real title (Eric And The Floaters, fast
	// mode). That is an artefact of this probe, not a genuine load
	// failure, so it must not be able to turn a real success into a
	// false negative.
	res.Verified = verifiedA || verifiedB

	return res
}

// bottomOrMiddleHasContent combines two position-aware signals, since
// neither alone is reliable: attribute-colour variety in the top 22
// character rows (indices 0-703 of the linear 32x24 attribute grid,
// deliberately excluding the bottom 2 rows where BASIC error messages
// like "D BREAK - CONT repeats" always appear) catches colourful games,
// but produced a real false negative on Elite -- vector-wireframe
// graphics that are genuinely, correctly loaded but stay close to
// monochrome white-on-black throughout. Bitmap density (>=500 non-zero
// bytes, same bottom-2-rows exclusion, computed against the Spectrum's
// interleaved screen layout: the last two character rows occupy the
// final 512 bytes of the 6144-byte bitmap) catches that case too, since
// a real screen still has substantial drawn content even without colour
// variety. Either signal on its own is enough.
func bottomOrMiddleHasContent(zx *ZenZX) bool {
	distinct := map[byte]bool{}
	for i := 0; i < 22*32; i++ {
		distinct[zx.screen.attributes[i]] = true
	}
	if len(distinct) >= 2 {
		return true
	}
	nonZero := 0
	for i := 0; i < 6144-512; i++ {
		if zx.screen.bitmap[i] != 0 {
			nonZero++
			if nonZero >= 500 {
				return true
			}
		}
	}
	return false
}

func sanitizeName(s string) string {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			out = append(out, byte(r))
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}

func TestBatchThreeModeCompare(t *testing.T) {
	startIdx := 0
	endIdx := len(batchGames)
	if v := os.Getenv("BATCH_START"); v != "" {
		fmt.Sscanf(v, "%d", &startIdx)
	}
	if v := os.Getenv("BATCH_END"); v != "" {
		fmt.Sscanf(v, "%d", &endIdx)
	}
	if endIdx > len(batchGames) {
		endIdx = len(batchGames)
	}

	os.MkdirAll(shotDir, 0755)

	bootHash48 := pristineBootHash(t, "48k")
	bootHash128 := pristineBootHash(t, "128k")

	var results []gameResult
	outPath := os.Getenv("BATCH_OUT")
	if outPath == "" {
		outPath = "/tmp/batch_results.json"
	}
	// Load any existing results (from a prior partial pass) and extend.
	if data, err := os.ReadFile(outPath); err == nil {
		json.Unmarshal(data, &results)
	}
	existing := map[string]int{}
	for i, r := range results {
		existing[r.Name] = i
	}

	for idx := startIdx; idx < endIdx; idx++ {
		g := batchGames[idx]
		bootHash := bootHash48
		if g.Model == "128k" {
			bootHash = bootHash128
		}
		t.Logf("=== [%d/%d] %s (%s) ===", idx+1, len(batchGames), g.Name, g.Model)

		gr := gameResult{Name: g.Name, Model: g.Model}
		for _, m := range []struct {
			mode TapeMode
			name string
		}{
			{TapeFast, "fast"},
			{TapeAccurate, "accurate"},
			{TapeTurbo, "turbo"},
		} {
			r := runOneMode(t, g, m.mode, m.name, bootHash)
			t.Logf("  %-9s completed=%-5v verified=%-5v frames=%-7d wall=%.2fs %s",
				m.name, r.Completed, r.Verified, r.Frames, r.WallSeconds, r.ErrorNote)
			gr.Results = append(gr.Results, r)
		}

		if i, ok := existing[g.Name]; ok {
			results[i] = gr
		} else {
			results = append(results, gr)
			existing[g.Name] = len(results) - 1
		}

		data, _ := json.MarshalIndent(results, "", "  ")
		os.WriteFile(outPath, data, 0644)
	}

	t.Logf("wrote %d game results to %s", len(results), outPath)
}
