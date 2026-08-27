// Package settingsconfig loads, validates, and saves the user's
// menu-configured preferences -- theme, font, font zoom, display
// scale, and whether the menu bar is pinned shown -- to settings.json.
// Unlike machineconfig (read-only, embedded default with an optional
// disk override), this package's file is written to as well as read:
// every time one of these preferences changes via a menu, the current
// state is saved back to disk so the next launch picks up where the
// last one left off.
//
// Deliberately not persisted here: the running -model (an explicit
// CLI flag, or the emulator's own hardcoded default, should always win
// over a stale persisted model rather than silently booting into
// whatever was last used), and FPS/border visibility (session-level
// diagnostic toggles, not the kind of thing most applications persist
// across restarts).
package settingsconfig

// Settings is the top-level shape of settings.json.
type Settings struct {
	Version int `json:"version"`
	// Theme is a zenui.ThemeName's own string value (Dark, Light, or
	// Spectrum, matching zenui.ThemeSpectrum's own constant) -- kept
	// as a plain string here rather than importing zenui, so this
	// package has no GUI-layer dependency of its own.
	Theme string `json:"theme"`
	// Font is a fonts.Name's own string value (Sinclair, TomThumb,
	// Spleen, Cozette, Creep, or HaxorMedium) -- same reasoning as
	// Theme above.
	Font string `json:"font"`
	// FontZoom is the dropdown text's own magnification (1-3),
	// distinct from DisplayScale below.
	FontZoom int `json:"fontZoom"`
	// DisplayScale is the emulated Spectrum display's own window
	// multiplier.
	DisplayScale int `json:"displayScale"`
	// FixedMenuBar mirrors appMenuBar.fixed -- whether the bar is
	// pinned permanently shown.
	FixedMenuBar bool `json:"fixedMenuBar"`
}

// Result is what Load returns: the resolved settings, which source
// they actually came from, and (if a disk file was present but
// rejected) a human-readable reason -- the same shape
// machineconfig.Result uses, for the same reason: this package
// doesn't write to stderr itself, leaving that to the caller.
type Result struct {
	Settings *Settings
	FromDisk bool
	DiskPath string
	Warning  string
}
