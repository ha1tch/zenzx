package main

// ============================================================================
// Boot baselines
//
// Measured boot-ready frames per model, at 50 fps (PAL), determined with the
// BootDetector (rainbow-band progress gate + bottom-two-rows final-text signal
// + stabilisation). These serve two purposes:
//
//   1. The wait-boot scheduler action uses the rainbow flag to construct an
//      appropriate detector for the running model.
//   2. The measurement harness asserts the live measurement against these
//      baselines within BootTolerance, catching regressions where a change
//      shifts boot timing.
//
// See bootdetect.go for the +3 drive-B: caveat: these figures assume the
// single-drive "Drives A: and M: available." configuration. If a second drive
// is ever wired into the FDC, the +3 line lengthens and a separate baseline is
// needed for that configuration.
// ============================================================================

// BootTolerance is the fractional band applied to a baseline frame when used
// as a regression guard (+-5%).
const BootTolerance = 0.05

// bootBaseline records the measured ready frame and whether the model's
// startup paints the rainbow band.
type bootBaseline struct {
	frame   int  // measured boot-ready frame at 50 fps
	rainbow bool // startup paints the 128-family rainbow band
}

// bootBaselines maps a model name (as accepted by the -model flag) to its
// measured boot baseline.
var bootBaselines = map[string]bootBaseline{
	"48k":         {frame: 85, rainbow: false},
	"128k":        {frame: 61, rainbow: true},
	"plus2":       {frame: 62, rainbow: true},
	"plus3":       {frame: 135, rainbow: true},
	"spanish48k":  {frame: 85, rainbow: false},
	"spanish128k": {frame: 44, rainbow: false}, // Investronica 1985 ROM: simpler, faster boot
}
