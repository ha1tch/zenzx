package main

// ============================================================================
// Boot-readiness detection
//
// Determining when a model has finished booting is needed both by the
// measurement harness and by the scheduler's wait-boot action. There is no
// single signal that works for every model, so detection combines two:
//
//   1. The rainbow band. The 128/+2/+3 startup menu paints a BRIGHT spectrum
//      stripe. Its colours are split across the attribute channels: some
//      stripes carry their colour as ink, others as paper, depending on
//      whether the cell's bitmap pixels are set. A row is recognised as the
//      band when a contiguous run of bright cells covers several distinct
//      chroma colours across ink-or-paper. The band is a reliable signal that
//      we are in the menu-paint phase and not an earlier boot plateau.
//
//   2. Stabilisation. The screen content stops changing once a screen has
//      fully painted. For rainbow models this confirms the menu (and, on the
//      +3, the late "Drives available" line that only appears after the FDC
//      drive check) has finished. For the 48K family and the older
//      Investronica Spanish 128K -- which have no rainbow band -- this is the
//      sole signal.
//
// Readiness rule: a rainbow model is ready once the band has appeared AND the
// screen has since been stable for a short confirmation window. A non-rainbow
// model is ready once the screen has been stable for that window following at
// least one change from its initial boot state.
// ============================================================================

// stableConfirmFrames is the number of consecutive unchanged frames required
// to treat the screen as settled. At 50 fps this is a fraction of a second --
// long enough to avoid catching a mid-paint still, short enough to be prompt.
const stableConfirmFrames = 15

// screenContentHash returns a content hash of the current display file
// (bitmap + attributes). Two frames with the same hash render identically.
func screenContentHash(zx *ZenZX) uint64 {
	const prime = 1099511628211
	var h uint64 = 14695981039346656037
	for _, b := range zx.screen.bitmap {
		h = (h ^ uint64(b)) * prime
	}
	for _, b := range zx.screen.attributes {
		h = (h ^ uint64(b)) * prime
	}
	return h
}

// hasRainbowBand reports whether the current attribute area contains the
// 128-family menu's bright rainbow stripe: a row with a contiguous run of
// bright cells whose ink-or-paper colours cover at least minChroma distinct
// non-black, non-white spectrum colours.
func hasRainbowBand(zx *ZenZX) bool {
	const minChroma = 4
	attr := zx.screen.attributes
	for row := 0; row < 24; row++ {
		chroma := map[uint8]bool{}
		brightRun := 0
		maxBrightRun := 0
		for col := 0; col < 32; col++ {
			a := attr[row*32+col]
			if (a>>6)&0x01 != 1 { // not bright -- breaks the run
				brightRun = 0
				continue
			}
			brightRun++
			if brightRun > maxBrightRun {
				maxBrightRun = brightRun
			}
			if ink := a & 0x07; ink != 0 && ink != 7 {
				chroma[ink] = true
			}
			if paper := (a >> 3) & 0x07; paper != 0 && paper != 7 {
				chroma[paper] = true
			}
		}
		if len(chroma) >= minChroma && maxBrightRun >= minChroma {
			return true
		}
	}
	return false
}

// bottomRowsHaveText reports whether character rows 22 and 23 (the bottom two
// text lines, y = 176..191) contain any set bitmap pixels. These rows are the
// last thing the boot routine writes on every model -- the 48K copyright line
// and the 128-family menu's "(c) ... Amstrad" / "Drives available" lines all
// land here -- so a non-empty bottom is a strong "final text has painted"
// signal. It must be paired with stabilisation, because the 48K's pre-CLS
// boot animation also briefly writes noise into these rows.
//
// Caveat re: the +3 drive line. ZenZX currently emulates a single physical
// drive, so the +3 boot line is fixed at "Drives A: and M: available." and
// the frame-135 baseline (and its +-5% regression band) are stable. However,
// the FDC already parses the unit-select bits and the +3 ROM enumerates
// drives from its own probe, so a second drive (B:) is plausibly reachable
// with modest FDC work. If B: support is added, the boot line lengthens, the
// ready frame moves later, and a separate baseline/band is needed for the
// B:-present configuration -- the A:+M: band will otherwise flag a spurious
// "regression". Revisit this if a second drive is wired into the FDC.
func bottomRowsHaveText(zx *ZenZX) bool {
	for y := 176; y < 192; y++ {
		for x := 0; x < 256; x += 8 {
			if zx.screen.bitmap[zx.screen.calcByteOffset(x, y)] != 0 {
				return true
			}
		}
	}
	return false
}

// BootDetector tracks boot progress across frames. Construct one with the
// model's rainbow expectation, call Update(zx) once per emulated frame (after
// RunFrame), and check Ready().
//
// Readiness is "the last content change was at least stableConfirmFrames ago"
// -- i.e. the screen has held steady through a confirmation window. This
// correctly skips the transient boot-animation plateaus, which are each
// followed by further changes. Rainbow-expecting models additionally require
// the menu's rainbow band to have appeared, so that the intermediate clear
// the +3 shows before its FDC drive check completes is not mistaken for the
// finished menu.
type BootDetector struct {
	expectRainbow bool
	lastHash      uint64
	frame         int
	lastChange    int // frame of the most recent content change
	seenChange    bool
	seenRainbow   bool
	ready         bool
	readyFrame    int
}

// NewBootDetector returns a detector. Set expectRainbow true for the
// 128/+2/+3 models, whose startup menu paints a rainbow band; false for the
// 48K family and the older Investronica Spanish 128K, which have none.
func NewBootDetector(expectRainbow bool) *BootDetector {
	return &BootDetector{expectRainbow: expectRainbow, readyFrame: -1, lastChange: -1}
}

// Update advances the detector by one frame. It must be called once per
// emulated frame, after zx.RunFrame().
func (d *BootDetector) Update(zx *ZenZX) {
	d.frame++
	h := screenContentHash(zx)

	if d.frame == 1 {
		d.lastHash = h
		return
	}

	if h != d.lastHash {
		d.seenChange = true
		d.lastChange = d.frame
	}
	d.lastHash = h

	if hasRainbowBand(zx) {
		d.seenRainbow = true
	}

	if d.ready || !d.seenChange {
		return
	}

	// Ready requires three things to coincide:
	//   - the screen has held steady for the confirmation window since the
	//     last change (rejects transient boot plateaus and the 48K's pre-CLS
	//     noise in the bottom rows, which is never stable);
	//   - the bottom two text rows are non-empty (the final text -- copyright
	//     line, or the 128 menu's last line -- has painted);
	//   - for rainbow models, the menu's rainbow band has appeared.
	stableSince := d.frame - d.lastChange
	if stableSince >= stableConfirmFrames && bottomRowsHaveText(zx) {
		if !d.expectRainbow || d.seenRainbow {
			d.ready = true
			d.readyFrame = d.lastChange
		}
	}
}

// Ready reports whether boot has completed.
func (d *BootDetector) Ready() bool { return d.ready }

// ReadyFrame returns the frame of the last content change before the screen
// settled into its ready state, or -1 if not yet ready.
func (d *BootDetector) ReadyFrame() int { return d.readyFrame }
