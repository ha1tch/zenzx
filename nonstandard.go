package main

import (
	"fmt"
	"strings"
)

// ============================================================================
// Non-standard features: master switch and gated sub-switches
//
// -non-standard on|off gates every -ns-* flag. When -non-standard is off
// (the default), no -ns-* flag may be set to a non-empty value -- ZenZX
// refuses to start rather than silently ignoring the request. When
// -non-standard is on, each -ns-* flag independently accepts either its
// default (empty: standard behaviour for that subsystem) or one of a fixed
// set of recognised values.
//
// This file only parses and validates the switches; it does not implement
// the modes themselves (see docs/TRACKING.md T-09, T-10).
// ============================================================================

// Recognised -ns-graphics values. Timex/SCLD modes are numbered
// (mode-timex-NNN-name); further ones (hi-res, dual-screen -- see
// docs/timex-modes.md) may be added under the same scheme once designed.
// Only hi-colour is implemented as a flag value so far.
const (
	NSGraphicsTimex001HiColour = "mode-timex-001-hicolour" // Extended Colour Mode: 256x192, 32x192 attribute resolution (8x1 blocks). See docs/timex-modes.md.
	NSGraphicsZenZX01          = "mode-zenzx-01"           // 256x192, 3px/byte, linear framebuffer, no attribute clash
	NSGraphicsZenZX02          = "mode-zenzx-02"           // 512x384, double resolution
)

// Recognised -ns-storage values.
const (
	NSStoragePosix = "storage-zenzx-posix"
)

var nsGraphicsValues = []string{NSGraphicsTimex001HiColour, NSGraphicsZenZX01, NSGraphicsZenZX02}
var nsStorageValues = []string{NSStoragePosix}

// nsGraphicsUsage and nsStorageUsage are pre-joined for use in flag.String
// usage strings in zenzx_headless.go / zenzx_gui.go.
var nsGraphicsUsage = strings.Join(nsGraphicsValues, ", ")
var nsStorageUsage = strings.Join(nsStorageValues, ", ")

// NonStandardConfig holds the validated state of the master switch and its
// sub-switches. An empty Graphics/Storage means "standard behaviour for
// that subsystem" -- Enabled does not imply every sub-switch is engaged.
type NonStandardConfig struct {
	Enabled  bool
	Graphics string
	Storage  string
}

// ParseNonStandardConfig validates the raw flag values together. masterRaw
// must be "on" or "off" (case-sensitive, matching every other enumerated
// flag in this codebase, e.g. -tapemode). graphicsRaw and storageRaw must
// each be "" or one of their recognised values, and must be "" whenever
// masterRaw is "off".
func ParseNonStandardConfig(masterRaw, graphicsRaw, storageRaw string) (NonStandardConfig, error) {
	var cfg NonStandardConfig

	switch masterRaw {
	case "on":
		cfg.Enabled = true
	case "off":
		cfg.Enabled = false
	default:
		return cfg, fmt.Errorf("invalid -non-standard value %q: must be \"on\" or \"off\"", masterRaw)
	}

	if !cfg.Enabled {
		if graphicsRaw != "" {
			return cfg, fmt.Errorf("-ns-graphics requires -non-standard on (got -non-standard off, -ns-graphics=%q)", graphicsRaw)
		}
		if storageRaw != "" {
			return cfg, fmt.Errorf("-ns-storage requires -non-standard on (got -non-standard off, -ns-storage=%q)", storageRaw)
		}
		return cfg, nil
	}

	if graphicsRaw != "" {
		if !isOneOf(graphicsRaw, nsGraphicsValues) {
			return cfg, fmt.Errorf("invalid -ns-graphics value %q: must be one of %v", graphicsRaw, nsGraphicsValues)
		}
		cfg.Graphics = graphicsRaw
	}
	if storageRaw != "" {
		if !isOneOf(storageRaw, nsStorageValues) {
			return cfg, fmt.Errorf("invalid -ns-storage value %q: must be one of %v", storageRaw, nsStorageValues)
		}
		cfg.Storage = storageRaw
	}

	return cfg, nil
}

func isOneOf(s string, values []string) bool {
	for _, v := range values {
		if s == v {
			return true
		}
	}
	return false
}

// Summary returns a one-line human-readable description for startup
// logging, or "" when non-standard features are off.
func (c NonStandardConfig) Summary() string {
	if !c.Enabled {
		return ""
	}
	s := "non-standard features: on"
	if c.Graphics != "" {
		s += fmt.Sprintf(", graphics=%s", c.Graphics)
	}
	if c.Storage != "" {
		s += fmt.Sprintf(", storage=%s", c.Storage)
	}
	if c.Graphics == "" && c.Storage == "" {
		s += " (no sub-features engaged)"
	}
	return s
}
