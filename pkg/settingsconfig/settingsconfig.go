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
	// Toolbar is whether the bar shows the toolbar and not the menu's labels. Optional: a file
	// from before it existed has the labels, as it always did.
	Toolbar bool `json:"toolbar,omitempty"`
	// Dock is whether the machine dock (the tiles that slide up from the bottom edge)
	// may be shown, and Multi whether it may add machines. Both are optional in the file
	// -- a settings.json from before they existed is still valid -- and both default to
	// on, so a missing key behaves as the program always did.
	Dock  *bool `json:"dock,omitempty"`
	Multi *bool `json:"multi,omitempty"`
	// StopAnimation is whether stopping a machine plays the animation (a boot stamps a small
	// Spectrum flat) or the machine just goes. Optional like Dock, and on by default.
	StopAnimation *bool `json:"stopAnimation,omitempty"`
	// TapeAutoload is whether a tape that is loaded is started by typing what loads it (the
	// phantom typist). Optional like Dock, and on by default.
	TapeAutoload *bool `json:"tapeAutoload,omitempty"`
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
	// Escape is what the Escape key does when no menu or window is open to be closed by it --
	// "off" (nothing: it never quits the program), "close" (it closes the debugger too, and
	// never quits), "query" (a second Escape within two seconds quits) or "on" (it quits at
	// once). Optional like TapeDeck, and "off" by default. The -escape switch overrides it for
	// one session.
	Escape string `json:"escape,omitempty"`
	// Microdigital is the position of the language switch of the Microdigital clones (the
	// TK90X and TK95) -- "spanish" or "portuguese". On those machines a diode, fitted or not,
	// set the level of bit 7 of port 0xFE, and some of their ROMs read it before they show a
	// message. Optional like Escape, and "spanish" by default (the bit reads high, as on every
	// other Spectrum). The -language-switch switch overrides it for one session.
	Microdigital string `json:"microdigital,omitempty"`
	// RecentSnapshots are the snapshots that were named (saved with Snapshot > Save As, or loaded
	// with Load Snapshot), newest first, for the Recent Snapshots menu. The ones Save Timestamped
	// and Quick Save make are not in it: they have menus of their own. Optional like Escape.
	RecentSnapshots []string `json:"recentSnapshots,omitempty"`
	// RecentFiles are the files opened with File > Open and Load Disk, newest first, for
	// the Recent place of those dialogs. Optional like RecentSnapshots.
	RecentFiles []string `json:"recentFiles,omitempty"`
	// RecentTapes are the tapes loaded into a deck, newest first, for the Recent Tapes menu and the
	// Manage Tapes window. Optional like RecentFiles.
	RecentTapes []string `json:"recentTapes,omitempty"`
	// OpenFolder and DiskFolder are the folders File > Open and Load Disk start in: the
	// folder of the file opened last with each. Optional: with none, or one that is gone, a
	// dialog starts in the home folder.
	OpenFolder string `json:"openFolder,omitempty"`
	DiskFolder string `json:"diskFolder,omitempty"`
	// FileDialog is how the person last looked at the folders of those dialogs. Optional.
	FileDialog *FileDialogView `json:"fileDialog,omitempty"`
}

// FileDialogView is the choices a person makes in the Open dialogs about how a folder looks: the
// hidden files shown, the details view, and what the list is ordered by (one of FileSortWords) and
// whether it is reversed. It is zenui.View without the file type, which each dialog has its own.
type FileDialogView struct {
	Hidden     bool   `json:"hidden,omitempty"`
	Details    bool   `json:"details,omitempty"`
	Sort       string `json:"sort,omitempty"`
	Descending bool   `json:"descending,omitempty"`
}

// FileSortWords are the words Sort can be, in zenui.SortKey order; "name" is the default.
var FileSortWords = []string{"name", "size", "date", "type"}

// RecentFilePaths is RecentFiles, a copy that can be kept.
func (s *Settings) RecentFilePaths() []string {
	if s == nil {
		return nil
	}
	return append([]string(nil), s.RecentFiles...)
}

// RecentTapePaths is RecentTapes, a copy that can be kept.
func (s *Settings) RecentTapePaths() []string {
	if s == nil {
		return nil
	}
	return append([]string(nil), s.RecentTapes...)
}

// OpenFolderPath is OpenFolder, and DiskFolderPath is DiskFolder: "" when there is none.
func (s *Settings) OpenFolderPath() string {
	if s == nil {
		return ""
	}
	return s.OpenFolder
}

func (s *Settings) DiskFolderPath() string {
	if s == nil {
		return ""
	}
	return s.DiskFolder
}

// FileDialogChoice is FileDialog as a copy that can be kept: nil when none was saved.
func (s *Settings) FileDialogChoice() *FileDialogView {
	if s == nil || s.FileDialog == nil {
		return nil
	}
	v := *s.FileDialog
	return &v
}

// RecentSnapshotPaths is RecentSnapshots, a copy that can be kept.
func (s *Settings) RecentSnapshotPaths() []string {
	if s == nil {
		return nil
	}
	return append([]string(nil), s.RecentSnapshots...)
}

// MicrodigitalWords are the positions of the language switch, in the order the Preferences
// window offers them.
var MicrodigitalWords = []string{"spanish", "portuguese"}

// MicrodigitalWord is Microdigital with its default, "spanish": the switch is left alone unless
// it was turned to Portuguese. A word that is not one of MicrodigitalWords reads as the default.
func (s *Settings) MicrodigitalWord() string {
	if s != nil && s.Microdigital == "portuguese" {
		return "portuguese"
	}
	return "spanish"
}

// EscapeWords are the words Escape can be, in the order the Preferences window offers them.
var EscapeWords = []string{"off", "close", "query", "on"}

// EscapeWord is Escape with its default, "off": the Escape key is disabled unless it was
// turned on. A word that is not one of EscapeWords reads as the default as well.
func (s *Settings) EscapeWord() string {
	if s == nil {
		return "off"
	}
	for _, w := range EscapeWords {
		if s.Escape == w {
			return w
		}
	}
	return "off"
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

// TapeAutoloadOn is TapeAutoload with its default: on unless the file says off.
func (s *Settings) TapeAutoloadOn() bool {
	return s == nil || s.TapeAutoload == nil || *s.TapeAutoload
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
