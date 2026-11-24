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
	// Dock is whether the machine dock (the tiles that slide up from the bottom edge)
	// may be shown, and Multi whether it may add machines. Both are optional in the file
	// -- a settings.json from before they existed is still valid -- and both default to
	// on, so a missing key behaves as the program always did.
	Dock  *bool `json:"dock,omitempty"`
	Multi *bool `json:"multi,omitempty"`
	// StopAnimation is whether stopping a machine plays the animation (a boot stamps a small
	// Spectrum flat) or the machine just goes. Optional like Dock, and on by default.
	StopAnimation *bool `json:"stopAnimation,omitempty"`
	// Drop and DropShift are what a tape, a .scr or a snapshot given to a Next at its menu
	// is done with -- "ask", "copy" (to the SD card) or "run" -- by a plain drop and by a
	// drop with Shift held. Optional like Dock, and "ask" and "run" by default.
	Drop      string `json:"drop,omitempty"`
	DropShift string `json:"dropShift,omitempty"`
	// SnapshotStart is what a snapshot that has been loaded does next -- "paused" (the
	// machine waits for Play or F2) or "play" (it runs at once). Optional like Drop, and
	// "paused" by default.
	SnapshotStart string `json:"snapshotStart,omitempty"`
	// TapeDeck is where the tape deck (the cassette, a bar and the buttons of a recorder,
	// shown while a tape loads in Accurate mode) goes -- "off", "top" or "bottom", of the
	// border, or of the picture when there is no border. Optional like Drop, and "top" by
	// default.
	TapeDeck string `json:"tapeDeck,omitempty"`
	// TapeCassette is the size of the tape deck's cassette -- "small" (beside the buttons),
	// "big" (under them, with the title written large on its label) or "hidden". Optional
	// like TapeDeck, and "big" by default.
	TapeCassette string `json:"tapeCassette,omitempty"`
}

// TapeCassetteWord is TapeCassette with its default, "big".
func (s *Settings) TapeCassetteWord() string {
	if s == nil || s.TapeCassette == "" {
		return "big"
	}
	return s.TapeCassette
}

// TapeDeckWord is TapeDeck with its default, "top".
func (s *Settings) TapeDeckWord() string {
	if s == nil || s.TapeDeck == "" {
		return "top"
	}
	return s.TapeDeck
}

// SnapshotStartAction is SnapshotStart with its default, "paused".
func (s *Settings) SnapshotStartAction() string {
	if s == nil || s.SnapshotStart == "" {
		return "paused"
	}
	return s.SnapshotStart
}

// DropAction is Drop with its default, "ask".
func (s *Settings) DropAction() string {
	if s == nil || s.Drop == "" {
		return "ask"
	}
	return s.Drop
}

// DropShiftAction is DropShift with its default, "run".
func (s *Settings) DropShiftAction() string {
	if s == nil || s.DropShift == "" {
		return "run"
	}
	return s.DropShift
}

// DockOn is Dock with its default: on unless the file says off.
func (s *Settings) DockOn() bool { return s == nil || s.Dock == nil || *s.Dock }

// StopAnimationOn is StopAnimation with its default: on unless the file says off.
func (s *Settings) StopAnimationOn() bool {
	return s == nil || s.StopAnimation == nil || *s.StopAnimation
}

// MultiOn is Multi with its default: on unless the file says off.
func (s *Settings) MultiOn() bool { return s == nil || s.Multi == nil || *s.Multi }

// Result is what Load returns: the resolved settings, which source
// they actually came from, and (if a disk file was present but
// rejected) a human-readable reason -- the same shape
// machineconfig.Result uses, for the same reason: this package
// doesn't write to stderr itself, leaving that to the caller.
type Result struct {
	Settings *Settings
	FromDisk bool
	DiskPath string
	// Warning is a file that could not be used, or could not be written, as the caller should
	// say it. Note is a file with nothing in it that was recreated, which is only worth a line.
	// Recreated is whether the file was written from the default by Load.
	Warning   string
	Note      string
	Recreated bool
}
