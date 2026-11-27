//go:build !headless

package zenzx

import (
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenui/zenuiraylib"
	"github.com/ha1tch/zenzx/pkg/fonts"
	"github.com/ha1tch/zenzx/pkg/machineconfig"
	"github.com/ha1tch/zenzx/pkg/nextcard"
	"github.com/ha1tch/zenzx/pkg/settingsconfig"
)

//go:embed machines.json
var defaultMachinesJSON []byte

//go:embed settings.json
var defaultSettingsJSON []byte

// validThemeNames and validFontNames are string forms of
// zenui.Themes/fonts.All, for settingsconfig.Load's own schema
// validation -- that package deliberately has no dependency on either
// zenui or fonts (see pkg/settingsconfig's own doc comment), so the
// closed sets of valid values are passed in as plain strings from
// here instead.
var (
	validThemeNames = func() []string {
		s := make([]string, len(zenui.Themes))
		for i, t := range zenui.Themes {
			s[i] = string(t)
		}
		return s
	}()
	validFontNames = func() []string {
		s := make([]string, len(fonts.All))
		for i, f := range fonts.All {
			s[i] = string(f)
		}
		return s
	}()
)

// validModelIDs lists every -model flag value zenzx_headless.go
// actually understands. machines.json's own "id" fields are validated
// against this set: the file controls how the menu presents and
// groups models, never which -model identifiers exist, so a typo or
// an invented identifier in a user-edited file is caught during
// loading rather than producing a menu item that can never actually
// switch models.
var validModelIDs = []string{
	"48k", "128k", "plus2", "plus2a", "plus3",
	"spanish48k", "spanish128k", "spanishplus2", "spanishplus3",
	"ts2068", "next",
}

const (
	menuStripHeight     = 24 // px, the strip's height once fully shown
	menuBarSlideTime    = 220 * time.Millisecond
	menuBarIdleDelay    = 100 * time.Millisecond
	menuBarEdgeDistance = 10 // px from the top border that counts as "at the edge"
	menuBarHideMargin   = 14 // extra px below the bar before we start hiding
)

// The Machine menu is built from machineconfig.Config now, not a
// hardcoded var here -- loaded via machineconfig.Load from
// machines.json (a disk file if one exists and validates, otherwise
// the copy embedded in the binary via defaultMachinesJSON below). See
// pkg/machineconfig for the node schema (separator/title/model/
// submenu) and validation pipeline, and machines.json at the project
// root for the current grouping -- "In Spanish" is a flat group of
// four models under one title, not broken out by manufacturer the way
// an earlier revision of this file had it.

// zoomLevels are the magnification factors the Font menu's "Font zoom
// X_" items offer -- applied to every dropdown's text/layout, never to
// the bar strip itself, whose text is fitted to the window's width
// instead (menubarscale.go). X2 is the default, matching what the
// dropdowns rendered at before this setting existed, so picking nothing
// changes nothing.
var zoomLevels = []int{1, 2, 3}

// viewRotateLabel is the View menu's item that moves every machine to the
// next place in the layout.
const viewRotateLabel = "Rotate Machines"

// barState is the slide animation's lifecycle.
type barState int

const (
	barHidden barState = iota
	barSlidingIn
	barShown
	barSlidingOut
)

// dialogKind is what the bar's file browser is open for.
type dialogKind int

const (
	dialogDisk         dialogKind = iota // Floppy Disk > Load Disk...
	dialogOpenFile                       // File > Open...
	dialogInsertCard                     // SD Card > Insert Card Image...
	dialogNewCard                        // SD Card > New Empty Card: names the new image
	dialogSaveSnapshot                   // Snapshot > Save As: names the snapshot file
	dialogLoadSnapshot                   // Snapshot > Load Snapshot...: chooses the snapshot file
	dialogSaveMulti                      // Snapshot > Save Multisnapshot...: names the multisnapshot file
	dialogLoadMulti                      // Snapshot > Load Multisnapshot...: chooses it
	dialogExportBrowse                   // File > Export...: the Browse button's file browser, whose folder and name go into the form
	dialogChooseCard                     // the cards of a snapshot: chooses the file of one that is not to be found
	dialogLoadTape                       // Tape > Load Tape...: chooses the tape file
)

// bar index constants -- Custom ROM (barCustomROM) gets dedicated handling
// in Update since picking a ROM is a two-step flow (file, then maybe bank);
// every other menu dispatches through the uniform actions table.
const (
	barFile = iota
	barMachine
	barCustomROM
	barTape
	barFloppyDisk
	barSnapshot
	barView
	barTheme
)

// appMenuBar owns the slide-in-on-idle presentation policy and all six
// menus' contents/actions, driving a zenui.MenuBar (the general, reusable
// widget) underneath. The widget itself has no opinion about how or when
// it appears -- that policy lives entirely here, in the host.
type appMenuBar struct {
	widget         *zenui.MenuBar
	warnings       []string // what was found wrong while it was built, for the window to say once it is up
	state          barState
	progress       float32 // 0 (fully hidden) .. 1 (fully shown), eased
	stateChangedAt time.Time

	// idle-at-top-edge tracking. Only meaningful while hidden or sliding
	// out -- once shown there's nothing left to trigger.
	lastMouseX, lastMouseY int32
	idling                 bool
	idleSince              time.Time

	// actions[barIndex][itemIndex] runs when that item is picked, for
	// every bar except Custom ROM (barCustomROM), which needs its own
	// two-step (ROM, then bank) handling below instead of a flat action.
	actions map[int][]func(zx *ZenZX)

	romDir  string
	romRows []romRow // snapshot of custom-roms/ taken when its dropdown opened: the rows it shows
	// archives are the pages of "Download more ROMs" (rom_archives.go), as last read, with the
	// stamp of the file they came from; romLinks are the ones the dropdown shows, the first that fit.
	archives      []romArchive
	archivesStamp string
	archivesRead  bool
	romLinks      []romArchive
	pendingROM    string // ROM chosen in step one, awaiting a bank pick in step two
	bankMode      bool   // true while the Custom ROM dropdown is showing bank choices, not ROM names
	romMoreAt     int    // the index of its "Download more ROMs" row, while the dropdown lists ROMs; 0 when it has none

	// diskDialog is the Load Disk file browser, non-nil only while
	// open. Unlike Custom ROM's two-step flow (which reuses the bar's
	// own dropdown), this is a separate zenui.Dialog instance -- a file
	// browser, not a flat item list -- drawn and updated independently
	// of b.widget.
	diskDialog *zenui.Dialog
	// dialogKind says what diskDialog is for while it is open: the Floppy Disk menu's
	// disk, File > Open's file of any kind, or the SD Card menu's card image to put in the
	// second slot or to name a new one (sdcardmenu_gui.go). newCardSize is, for dialogNewCard,
	// the size in bytes of the card to be made.
	dialogKind  dialogKind
	newCardSize int64
	saveFormat  string     // for dialogSaveSnapshot, the format chosen (snapshotsaveas.go)
	export      *exportJob // File > Export...: the machine as it was when the item was picked (export_gui.go)
	exportAsk   *exportAsk // the Export form, while it is open (export_dialog_gui.go)
	exportDir   string     // the folder the last export went to, where the next is offered
	fileMenu    menuSpec   // the File menu's rows, with Export's submenu
	// openFiles, set by the host, carries out File > Open: the files chosen go the way
	// files dropped on the window go. quit, also the host's, ends the program the way
	// closing its window does. Either is nil in a bar with no host (the tests), and the
	// File menu rows that need one are disabled.
	openFiles func(files []string)
	quit      func()
	// prefs is the Preferences window while it is open (preferences_gui.go). dockOn and
	// multiOn are two of its settings, kept here with the others: whether the machine dock
	// may be shown, and whether it may add machines. Both default to on.
	prefs   *prefsWindow
	dockOn  bool
	multiOn bool
	// tapeAutoload is whether a tape that is loaded starts by itself: the machine types what
	// loads it (phantomtypist.go).
	tapeAutoload bool
	// stopAnimation is whether Machine > Stop plays the animation of a boot stamping the
	// machine flat, or lets the machine go at once. On by default.
	stopAnimation bool
	// dropPlain and dropShift are what a tape, .scr or snapshot dropped on a Next at its
	// menu is done with, without Shift and with it (planDrop, dropchoice_gui.go).
	dropPlain, dropShift dropAction
	// snapshotPlay is whether a snapshot, once loaded, runs at once rather than waiting
	// paused (the default). It is read through snapshotStartsPlaying (input.go).
	snapshotPlay bool
	// tapeDeck is where the tape deck goes, or off (tapedeck.go). Off by default.
	tapeDeck tapeDeckPlace
	// tapeCassette is the size of its cassette: small by default.
	tapeCassette tapeCassetteSize
	// escape is the saved Escape key setting, which the Preferences window changes. escapeFlag
	// is the -escape switch of this session, if one was given: it wins until the window changes
	// the setting, and it is never written to the settings file.
	escape     escapeMode
	escapeFlag *escapeMode
	// microdigitalPT is the saved position of the Microdigital language switch (microdigital.go),
	// which the Preferences window changes: true is Portuguese. microdigitalFlag is the
	// -language-switch switch of this session, if one was given: it wins until the window
	// changes the setting, and it is never written to the settings file.
	microdigitalPT   bool
	microdigitalFlag *bool
	// The bar's fourth menu is Storage: the +3's floppy drive, or the SD cards of a Next
	// that boots NextZXOS. diskItems is the first's items, storageSpec the second's rows
	// (items and actions both, see menuspec_gui.go), storageIsSD which it is now, nextCard2
	// the -sdcard2 option.
	barK        int // the scale of the bar's own text (menubarscale.go), 0 until it has been drawn
	storageSpec menuSpec
	snapSpec    menuSpec // the Snapshot menu's rows, with Load Timestamped's and Save As's submenus
	viewMenu    menuSpec // the View menu's rows, with Screenshot's submenu (screenshot_gui.go)
	snapSig     string   // the snapshots folder and day it showed when its items were last set
	snapStopped bool     // no machine was running when it was (the rows that need one are greyed)
	recentSnaps []string // the named snapshots lately saved or loaded, newest first (snapshotrecent.go)
	// The tapes (tapedir.go): those lately loaded, newest first; the Tape menu's rows, with Recent
	// Tapes's submenu; which of those are still there, looked at again every two seconds.
	recentTapes   []string
	tapeMenu      menuSpec
	tapeRecentKey string
	tapeRecentAt  time.Time
	tapeRecentOK  []string
	// Recent Disks (discmenu_recent_gui.go): the recent files that are disks and are still there, looked at
	// again every two seconds.
	diskRecentKey string
	diskRecentAt  time.Time
	diskRecentOK  []string
	// The Open browsers (fileopen_gui.go): the files lately opened, newest first; the folders File >
	// Open and Load Disk start in; how the person last looked at a folder; and which of the two
	// diskDialog is while it is one of them.
	recentFiles []string
	openFolder  string
	diskFolder  string
	fileView    *settingsconfig.FileDialogView
	fileDlg     fileDialogKind
	storageIsSD bool
	storageSig  string // what the menu showed when its items were last set
	nextCard2   string

	// logoMenu is the dropdown of the rainbow (Spectrum theme) / ZSP logo (Dark/Light)
	// decoration on the right of the bar: the Help menu, in titled sections -- the bar's
	// own preferences, help and about, and the developer tools. The decoration is not a
	// bar label, so the menu is opened by hand (openLogoMenu), but it is made from a
	// menuSpec like every other. logoSpec is that spec while the menu is open.
	logoMenu *zenui.Menu

	// popupShadow, when set, draws the drop shadow of a popup menu's box (popupshadow_gui.go): the
	// window sets it to the picture frame's shadow texture. Unset, the menus have none.
	popupShadow func(box rl.Rectangle)

	// binHand is true while the pointer is a hand because it is over a tab of the Load Binary
	// window (binary_gui.go).
	binHand bool

	// logoHand is true while the pointer is a hand because it is over a button of the bar or a
	// toolbar button (barbuttons_gui.go).
	logoHand bool
	// toolbarOn is true while the bar shows the toolbar and not the menu's labels; toolbarT is how
	// far the slide between the two has gone, 0 for the labels to 1 for the toolbar; and
	// toolbarCur is the toolbar's buttons as the machine was at the last Update.
	toolbarOn  bool
	toolbarT   float32
	toolbarCur []toolbarItem
	// overflowOpen is true while logoMenu is the toolbar's overflow menu and not the Help menu.
	overflowOpen bool
	// tipKey is what the pointer is over that has a hover hint, and tipAt when it got there.
	tipKey string
	tipAt  float64
	// prefsHand is true while the pointer is a hand because it is over a tab of the
	// Preferences window (preferences_tips_gui.go).
	prefsHand bool
	logoSpec  menuSpec
	// openDebugger, set by the host, opens the debugger HUD (hud_gui.go) on the front
	// machine; nil while there is no debugger, which leaves the Developer item disabled.
	openDebugger func()
	// savedFixed is the fixed-bar choice the settings file holds, so that the file is
	// written when the choice changes however it changed, and not otherwise.
	savedFixed bool
	// activeModal is whichever of Help/About is currently open (nil
	// otherwise) -- a single field, not two, since only one can ever
	// be open at a time (both are only reachable through logoMenu,
	// which itself can't be open at the same time as either).
	activeModal *zenui.TextModal
	// fade is what a window that has just closed fades out with (dialogfade_gui.go).
	fade dialogFader
	// modalHand is whether the pointer is a hand because it is over a link in activeModal.
	modalHand bool

	// nextSetup is the first-start dialog of the Next while it is open (nextsetup_gui.go).
	// The four settings beside it come from the command line: where the boot files are
	// looked for, whether the card is write-protected, DivMMC logging, and -next-classic.
	nextSetup    *nextSetup
	nextFilesDir string
	nextReadOnly bool
	nextDebug    bool
	nextClassic  bool
	// fixed keeps the bar permanently shown, bypassing the normal
	// idle-at-edge auto-hide entirely, when true.
	fixed bool

	// settingsPath, if non-empty, is where saveSettings persists the
	// current theme/font/zoom/scale/fixed state after each change --
	// left empty by newAppMenuBar itself (set separately by main,
	// after construction, from the -settings flag) so every existing
	// test that constructs an appMenuBar directly never touches disk:
	// saveSettings is a no-op whenever this is "".
	settingsPath string

	// Checkmark state for Theme/Font/Zoom/Machine -- each menu's items
	// hold a *bool pointing into the matching map, built once at
	// construction and updated in place (never replaced) whenever the
	// underlying setting changes, so a menu re-opened later still shows
	// the correct current selection without rebuilding its item list.
	themeChecked map[zenui.ThemeName]*bool
	fontChecked  map[fonts.Name]*bool
	zoomChecked  map[int]*bool // dropdown text zoom (Font menu's own "Font zoom X_"), not the display scale below
	modelChecked map[string]*bool

	// displayScaleChecked is the View menu's X1/X2/X3 items' own
	// checkmark state -- distinct from zoomChecked (that one is the
	// dropdown text's own zoom, this one is the emulated display's).
	// Unlike the maps above, this one's pointed-to values are replaced
	// each frame by refreshViewItems rather than mutated in place,
	// since Item.Disabled (which also needs refreshing here, as
	// maxMultiplierThatFits can change if the window moves to a
	// different monitor) has no pointer-based live-update mechanism
	// the way Checked does -- the whole item list is rebuilt via
	// SetItems, the same pattern refreshCustomROMItems already uses.
	displayScaleChecked map[int]*bool

	// fontActions is Font's own action list -- separate from b.actions
	// since Font is a submenu nested inside Theme's own dropdown now
	// (themeFontAt identifies which of Theme's items it is),
	// not a top-level bar label with its own b.actions[barX] entry the
	// way every other menu still is.
	fontActions []func(zx *ZenZX)
	// themeFontAt is the index of the Font row among Theme's items, after its titles and themes.
	themeFontAt int

	// machineModelFlags[i] is the -model flag value for machineItems'
	// item i (Update dispatches on this directly), or "" for a Reset/
	// Pause/Title/Separator/Submenu row that isn't a model switch at
	// this level -- parallel to machineItems since zenui.Item itself
	// has no room for data only the host cares about.
	machineModelFlags []string
	// machineSubmenuFlags[i], present only when machineItems[i] has
	// SubItems set (a "submenu" node in machines.json), is the
	// parallel flag list for that submenu's own nested items --
	// sel.SubIndex indexes into this the same way sel.ItemIndex
	// indexes into machineModelFlags. machines.json's own schema caps
	// nesting at one level (see pkg/machineconfig's Submenu doc
	// comment for why), so this never needs a third level of its own.
	machineSubmenuFlags map[int][]string
	// machineGroupItems are the model rows that follow the Machine menu's head rows
	// (machinemenu_gui.go).
	machineGroupItems []zenui.Item
	// stopMachine stops the front machine, which the deck does with its animation
	// (machinedeck_gui.go); nil when there is no deck.
	stopMachine func()
	// startMachine starts a machine of a model when none is running, and returns it; nil when
	// one is (the deck's start).
	startMachine func(model string) *ZenZX

	// cards is the SD Cards window while it is open (sdcards_gui.go).
	// cardUse tells where a card image is among the window's machines and
	// machineNumber a machine's number in the dock (machinedeck_gui.go);
	// with neither set there is one machine, number 1.
	cards         *cardsWindow
	snaps         *snapshotsWindow     // the Snapshots window while it is open (snapshots_gui.go)
	shots         *screenshotsWindow   // the Screenshots window while it is open (screenshots_gui.go)
	tapes         *tapesWindow         // the Tapes window while it is open (tapes_gui.go)
	monoText      *zenuiraylib.BDFText // the monospaced face for dumps, when the chosen font is not one (monofont_gui.go)
	monoName      fonts.Name           // which face monoText is
	bin           *binaryWindow        // the Binary window (Load file, Peek and Poke) while it is open (binary_gui.go)
	cardUse       func(path string) cardUse
	machineNumber func(zx *ZenZX) int
	// disks is the Floppy Disks window while it is open (floppies_gui.go);
	// diskUse tells which machine has a disk file in its drive and machineAt
	// returns the machine with a number, both the dock's.
	// layoutState and setLayout are the dock's tiled layouts for the View
	// menu: the layout in use and the number of machines, and choosing one.
	layoutState func() (layout deckLayout, machines int)
	setLayout   func(l deckLayout)
	// rotateMachines moves every machine to the next place in the layout.
	rotateMachines func()
	// saveMulti and loadMulti save every machine of the window to a multisnapshot and load one in
	// their place (multisnapshot_gui.go); replaceAsk is the question that comes before a load that
	// closes machines that are running, and snapMachines how many there were when the Snapshot menu was
	// last made, which says whether Save Multisnapshot can be chosen.
	saveMulti func(path string) error
	loadMulti func(path string, plan cardPlan) (int, error)
	// multiCards finds the cards of the machines of a multisnapshot that had them, so that the person is
	// asked for the ones that are not to be found before anything is closed.
	multiCards   func(path string) ([]cardNeed, *cardSearch, error)
	replaceAsk   *cardQuestion
	snapMachines int

	// cardsAsk is the question about the cards of a snapshot of a Next that are not to be found
	// (cardsask_gui.go).
	cardsAsk *cardsAsk

	disks     *disksWindow
	diskUse   func(path string) int
	machineAt func(n int) *ZenZX

	text      *zenuiraylib.BDFText
	renderer  zenuiraylib.Renderer
	theme     zenui.Theme
	themeName zenui.ThemeName
	fontName  fonts.Name
	scale     int // applied to every dropdown's text/layout; the bar strip itself always draws at 1 regardless -- see zoomLevels
}

// parseThemeFlag resolves the -theme flag's raw string into a
// zenui.ThemeName, case-insensitively (so "-theme=spectrum" and
// "-theme=Spectrum" both work; the space-stripping in normalize below is
// a harmless leftover from when the Spectrum theme's own name was
// "Spectrum 128" with a space -- kept as general case-insensitivity
// tolerance rather than removed, since it costs nothing now that no
// valid theme name actually contains a space). Falls back to
// zenui.ThemeDark -- with a warning, so a typo is visible rather than
// silently defaulting -- for anything that doesn't match one of
// zenui.Themes.
func parseThemeFlag(raw string) zenui.ThemeName {
	normalize := func(s string) string {
		return strings.ToLower(strings.ReplaceAll(s, " ", ""))
	}
	want := normalize(raw)
	for _, name := range zenui.Themes {
		if normalize(string(name)) == want {
			return name
		}
	}
	fmt.Printf("Warning: unrecognised -theme value %q, using Dark. Valid values: Dark, Light, Spectrum\n", raw)
	return zenui.ThemeDark
}

// newAppMenuBar builds a hidden bar with all six menus populated, loading
// the Sinclair face once so the bar and every dropdown share the same
// glyph texture cache. initialTheme is the theme active from the very
// first frame -- see zenui.LoadTheme for how an unrecognised name
// resolves. initialModel is the -model flag's own value (available well
// before the actual ROM-loading switch in zenzx_gui.go's main runs, since
// flag parsing happens first) -- used only to seed the Machine menu's
// checkmark correctly; the caller is still responsible for actually
// loading that model's ROM set. zx is used to wire the View menu's
// FPS/Border checkboxes directly to the display manager's own fields
// (see the View item construction below for why).
func newAppMenuBar(zx *ZenZX, romDir string, initialTheme zenui.ThemeName, initialModel string, initialSettings *settingsconfig.Settings, settingsPath string) (*appMenuBar, error) {
	// initialSettings may be nil (most tests, and any construction
	// that doesn't care about persisted preferences) -- fall back to
	// the same hardcoded defaults this constructor always used before
	// settings.json existed, so a nil caller gets identical behaviour
	// to before this feature existed. When non-nil, its fields are
	// already guaranteed valid (settingsconfig.Load never returns a
	// partially-invalid *Settings), so no re-validation is needed here.
	initialFontName := fonts.NameSpleen
	initialFontZoom := 2
	initialFixed := false
	if initialSettings != nil {
		initialFontName = fonts.Name(initialSettings.Font)
		initialFontZoom = initialSettings.FontZoom
		initialFixed = initialSettings.FixedMenuBar
	}
	// A nil *Settings, or a file with no dock/multi key, is on: the way it always was.
	initialDock, initialMulti := initialSettings.DockOn(), initialSettings.MultiOn()
	initialDrop := parseDropAction(initialSettings.DropAction(), dropAsk)
	initialDropShift := parseDropAction(initialSettings.DropShiftAction(), dropRun)
	initialEscape, _ := parseEscapeMode(initialSettings.EscapeWord()) // anything unknown is off

	face, err := fonts.Load(initialFontName)
	if err != nil {
		return nil, fmt.Errorf("loading %s font for menu bar: %w", initialFontName, err)
	}
	theme := zenui.LoadTheme(initialTheme)
	text := zenuiraylib.NewBDFText(face)

	b := &appMenuBar{
		theme:               theme,
		themeName:           initialTheme,
		text:                text,
		renderer:            zenuiraylib.Renderer{Text: text},
		fontName:            initialFontName,
		scale:               initialFontZoom, // matches dlgScale's own default (2) when initialSettings is nil -- unchanged visual size until the user picks a different zoom
		fixed:               initialFixed,
		toolbarOn:           initialSettings != nil && initialSettings.Toolbar,
		toolbarT:            toolbarRest(initialSettings != nil && initialSettings.Toolbar),
		savedFixed:          initialFixed,
		dockOn:              initialDock,
		multiOn:             initialMulti,
		stopAnimation:       initialSettings.StopAnimationOn(),
		tapeAutoload:        initialSettings.TapeAutoloadOn(),
		recentSnaps:         initialSettings.RecentSnapshotPaths(),
		recentFiles:         initialSettings.RecentFilePaths(),
		recentTapes:         initialSettings.RecentTapePaths(),
		openFolder:          initialSettings.OpenFolderPath(),
		diskFolder:          initialSettings.DiskFolderPath(),
		fileView:            initialSettings.FileDialogChoice(),
		dropPlain:           initialDrop,
		dropShift:           initialDropShift,
		snapshotPlay:        initialSettings.SnapshotStartAction() == "play",
		tapeDeck:            parseTapeDeckPlace(initialSettings.TapeDeckWord(), tapeDeckTop),
		tapeCassette:        parseTapeCassetteSize(initialSettings.TapeCassetteWord(), cassetteBig),
		escape:              initialEscape,
		microdigitalPT:      initialSettings.MicrodigitalWord() == "portuguese",
		settingsPath:        settingsPath,
		romDir:              romDir,
		actions:             make(map[int][]func(zx *ZenZX)),
		themeChecked:        make(map[zenui.ThemeName]*bool),
		fontChecked:         make(map[fonts.Name]*bool),
		zoomChecked:         make(map[int]*bool),
		modelChecked:        make(map[string]*bool),
		displayScaleChecked: make(map[int]*bool),
	}

	// The File menu, first on the bar: what a person coming from another program looks for
	// there. Open takes any file the program loads, the way a drop does; Quit is for a
	// system with no application menu to quit from.
	fileSpec := b.fileSpec()
	b.fileMenu = fileSpec
	fileItems := fileSpec.items()
	b.actions[barFile] = fileSpec.actions()

	machineActions := []func(zx *ZenZX){
		func(*ZenZX) {}, // the title
		func(zx *ZenZX) { zx.Reset(); fmt.Println("Reset") },
		func(zx *ZenZX) { b.playMachine(zx) },
		func(zx *ZenZX) {
			if !zx.paused {
				zx.TogglePause()
			}
		},
		func(zx *ZenZX) {
			if b.stopMachine != nil {
				b.stopMachine()
			}
		},
	}
	b.actions[barMachine] = machineActions

	machinesRes, err := machineconfig.Load("machines.json", defaultMachinesJSON, validModelIDs)
	if err != nil {
		// The embedded default failing validation is a genuine bug in
		// this binary, not a user configuration problem -- there is no
		// sensible Machine menu to build without it.
		return nil, fmt.Errorf("embedded machines.json is invalid: %w", err)
	}
	if machinesRes.FromDisk {
		fmt.Printf("Machine menu: loaded %s\n", machinesRes.DiskPath)
	} else if machinesRes.Warning != "" {
		fmt.Println("Warning:", machinesRes.Warning)
		b.warnings = append(b.warnings, machinesRes.Warning)
	}

	groupItems, groupFlags, groupSubFlags := b.buildMachineNodes(machinesRes.Config.Items, initialModel)
	b.machineGroupItems = groupItems
	machineItems := append(b.machineHead(zx), groupItems...)
	machineModelFlags := append(make([]string, machineHeadRows), groupFlags...)
	b.machineModelFlags = machineModelFlags
	// groupSubFlags is indexed relative to groupItems (0-based); shift
	// every key by machineHeadRows (the title, Reset, Play, Pause, Stop) to match machineItems' own
	// indices, which is what sel.ItemIndex actually reports.
	b.machineSubmenuFlags = make(map[int][]string, len(groupSubFlags))
	for i, flags := range groupSubFlags {
		b.machineSubmenuFlags[i+machineHeadRows] = flags
	}

	// The Tape menu is made by tapeSpec (tapemenu_gui.go) and made again each frame the bar is
	// shown, since which of Play and Stop can be chosen follows the tape.
	tapeSpec := b.tapeSpec(zx)
	tapeItems := tapeSpec.items()
	b.actions[barTape] = tapeSpec.actions()
	b.tapeMenu = tapeSpec

	// The storage menu: the same two sections on every machine, Floppy Disk and SD Card,
	// live where the model has the hardware and disabled, with the reason, where it has not
	// (storageMenuSpec, sdcardmenu_gui.go). It is made again as the machine changes.
	b.storageSpec = b.storageMenuSpec(zx)
	diskItems := b.storageSpec.items()
	b.actions[barFloppyDisk] = b.storageSpec.actions()

	// The Snapshot menu is made by snapshotSpec (snapshotmenu_gui.go) and made again when the
	// snapshots folder changes.
	now := time.Now()
	b.snapStopped = zx.stopped
	b.snapSpec = b.snapshotSpec(now)
	b.snapSig = b.snapshotSig(now)
	snapItems := b.snapSpec.items()
	b.actions[barSnapshot] = b.snapSpec.actions()

	// The View menu is made by viewSpec, and made again by refreshViewItems every frame
	// the bar is shown, so what its checkboxes and zoom items show is never older than the
	// machine. This is its first build.
	viewSpec := b.viewSpec(zx)
	viewItems := viewSpec.items()
	b.actions[barView] = viewSpec.actions()
	b.viewMenu = viewSpec

	// Theme's dropdown is in two titled sections, the themes and the text (Font), so its
	// items, and the actions that go with them, begin with a title; none of them has an action.
	noAction := func(*ZenZX) {}
	themeItems := []zenui.Item{{Label: "Theme", Title: true}}
	themeActions := []func(zx *ZenZX){noAction}
	for _, name := range zenui.Themes {
		checked := name == initialTheme
		b.themeChecked[name] = &checked
		themeItems = append(themeItems, zenui.Item{Label: string(name), Checked: b.themeChecked[name]})
		themeName := name // capture per-iteration
		themeActions = append(themeActions, func(zx *ZenZX) { b.ApplyTheme(themeName); b.saveSettings(zx) })
	}

	fontItems := append(make([]zenui.Item, 0, len(fonts.All)+len(zoomLevels)+3), zenui.Item{Label: "Font", Title: true})
	fontActions := append(make([]func(zx *ZenZX), 0, len(fonts.All)+len(zoomLevels)+3), noAction)
	for _, name := range fonts.All {
		checked := name == b.fontName // b.fontName already reflects the real initial value (initialFontName above)
		b.fontChecked[name] = &checked
		fontItems = append(fontItems, zenui.Item{Label: string(name), Checked: b.fontChecked[name]})
		fontName := name // capture per-iteration
		fontActions = append(fontActions, func(zx *ZenZX) {
			if err := b.ApplyFont(fontName); err != nil {
				fmt.Printf("Warning: could not switch to font %s: %v\n", fontName, err)
				zx.notice("The font %s could not be used: %v", fontName, err)
				return
			}
			b.saveSettings(zx)
		})
	}
	fontItems = append(fontItems, zenui.Item{Separator: true}, zenui.Item{Label: "Zoom", Title: true})
	fontActions = append(fontActions, noAction, noAction) // the separator's and the title's own indices -- never actually called, placeholders to keep the zoom entries below aligned
	for _, z := range zoomLevels {
		checked := z == b.scale // b.scale is already set to 2 above, the default zoom
		b.zoomChecked[z] = &checked
		fontItems = append(fontItems, zenui.Item{Label: fmt.Sprintf("Zoom X%d", z), Checked: b.zoomChecked[z]})
		zoom := z // capture per-iteration
		fontActions = append(fontActions, func(zx *ZenZX) { b.ApplyZoom(zoom); b.saveSettings(zx) })
	}
	b.fontActions = fontActions

	// Font folds into Theme's own dropdown as a nested submenu (an
	// earlier version had it as its own top-level bar label) -- the
	// two together read as one "appearance" menu rather than two
	// separate ones. themeFontAt, the index of the Font row, is set here
	// and read by Update's own dispatch, rather than worked out twice.
	themeItems = append(themeItems, zenui.Item{Separator: true}, zenui.Item{Label: "Text", Title: true})
	themeActions = append(themeActions, noAction, noAction)
	b.themeFontAt = len(themeItems)
	themeItems = append(themeItems, zenui.Item{Label: "Font", SubItems: fontItems})
	themeActions = append(themeActions, noAction) // Font's own row: its submenu is dispatched through fontActions
	b.actions[barTheme] = themeActions

	// Custom ROM's items are populated fresh each time it's opened
	// (refreshCustomROMItems), since the directory's contents can't be
	// known up front the way every other menu's fixed item list can.
	b.widget = zenui.NewMenuBar(zenui.MenuBarConfig{
		Items: []zenui.MenuBarItem{
			{Label: "File", Items: fileItems},
			{Label: "Machine", Items: machineItems},
			{Label: "Custom ROM", Items: nil},
			{Label: "Tape", Items: tapeItems},
			{Label: storageLabel, Items: diskItems},
			{Label: "Snapshot", Items: snapItems},
			{Label: "View", Items: viewItems},
			{Label: "Theme", Items: themeItems},
		},
		Scale: b.scale,
	})
	return b, nil
}

// Unload frees the bar's glyph textures. Call once, before rl.CloseWindow.
func (b *appMenuBar) Unload() { b.text.Unload() }

// ApplyTheme switches the bar's (and every dropdown's) colour scheme.
// Every draw call already reads b.theme fresh each frame, so this takes
// effect on the very next frame with no further propagation needed.
func (b *appMenuBar) ApplyTheme(name zenui.ThemeName) {
	b.themeName = name
	b.theme = zenui.LoadTheme(name)
	for k, checked := range b.themeChecked {
		*checked = k == name
	}
}

// ApplyFont switches the bar's (and every dropdown's) typeface. Unlike
// ApplyTheme, this needs a new glyph texture cache built and the old
// one's GPU textures freed -- text is drawn through b.renderer, which
// every draw call also reads fresh each frame, so replacing it here is
// enough for the change to take effect everywhere on the next frame.
// Leaves the current font and theme untouched if name fails to load.
func (b *appMenuBar) ApplyFont(name fonts.Name) error {
	face, err := fonts.Load(name)
	if err != nil {
		return err
	}
	old := b.text
	b.text = zenuiraylib.NewBDFText(face)
	b.renderer = zenuiraylib.Renderer{Text: b.text}
	b.fontName = name
	old.Unload()
	for k, checked := range b.fontChecked {
		*checked = k == name
	}
	return nil
}

// ApplyZoom switches the magnification every dropdown's text/layout
// draws at -- never the bar strip itself, whose scale follows the
// window's width (barScale) regardless of this setting. Takes effect
// the next time a dropdown is opened (zenui.MenuBar.SetScale's own
// doc comment explains why an already-open one isn't resized out from
// under an in-progress interaction).
func (b *appMenuBar) ApplyZoom(scale int) {
	b.scale = scale
	b.widget.SetScale(scale)
	for k, checked := range b.zoomChecked {
		*checked = k == scale
	}
}

// Active reports whether the bar currently owns input -- an open
// dropdown, disk dialog, logo menu, or modal. Deliberately does NOT
// include the bar's own visibility (b.state != barHidden): the bar
// being shown or fixed, with nothing open, must not withhold keyboard
// input from the emulated machine -- only an actually-open interactive
// element does that. An earlier version included the visibility check,
// meaning the emulator lost keyboard input any time the mouse merely
// drifted near the top edge, whether or not anything was actually
// open, and permanently while the bar was fixed.
func (b *appMenuBar) Active() bool {
	return b.widget.Active() || b.logoMenu != nil || b.modalOpen()
}

// modalOpen reports that a window or dialog has the whole frame: the file browser, the
// Help/About/info readers, the Next's first-start dialog, the SD Cards and Floppy Disks
// windows.
func (b *appMenuBar) modalOpen() bool {
	return b.diskDialog != nil || b.exportAsk != nil || b.activeModal != nil || b.nextSetup != nil || b.cards != nil || b.snaps != nil || b.shots != nil || b.tapes != nil || b.replaceAsk != nil || b.cardsAsk != nil || b.bin != nil || b.disks != nil || b.prefs != nil
}

// closeMenus closes every dropdown of the bar, the Help menu included, and abandons a
// Custom ROM pick that was waiting for its bank.
func (b *appMenuBar) closeMenus() {
	b.widget.Close()
	b.logoMenu = nil
	b.bankMode, b.pendingROM = false, ""
}

// enforceSingleSurface is the rule that the bar shows one thing at a time: a window or
// dialog has the frame to itself, so no dropdown stays open under it, and the Help menu
// and a bar dropdown are never both open. Update runs it first thing each frame, whichever
// path opened what -- the opening paths do not each have to remember to close the others.
func (b *appMenuBar) enforceSingleSurface() {
	switch {
	case b.modalOpen():
		b.closeMenus()
	case b.logoMenu != nil && b.widget.Active():
		b.widget.Close()
		b.bankMode, b.pendingROM = false, ""
	}
}

func (b *appMenuBar) show() {
	if b.state == barShown || b.state == barSlidingIn {
		return
	}
	b.state = barSlidingIn
	b.stateChangedAt = time.Now()
}

func (b *appMenuBar) hide() {
	if b.state == barHidden || b.state == barSlidingOut {
		return
	}
	b.state = barSlidingOut
	b.stateChangedAt = time.Now()
	b.widget.Close()
	b.bankMode = false
	b.pendingROM = ""
}

// buildMachineNodes converts a flat list of machineconfig.Node into
// parallel zenui.Item/flag slices, recursing one level into any
// Submenu node's own Items (never deeper -- machines.json's own
// schema already rejects a second level of submenu nesting, so the
// inner call's own submenu-flags map is always empty and safely
// discarded; see pkg/machineconfig's Submenu doc comment for why one
// level is the limit). Also populates b.modelChecked for every Model
// node encountered, at any depth, the same direct-pointer live-update
// pattern every other checkmarked menu in this file already uses.
func (b *appMenuBar) buildMachineNodes(nodes []machineconfig.Node, initialModel string) (items []zenui.Item, flags []string, subFlags map[int][]string) {
	items = make([]zenui.Item, 0, len(nodes))
	flags = make([]string, 0, len(nodes))
	subFlags = map[int][]string{}

	for _, node := range nodes {
		indent := strings.Repeat("  ", node.Indent)
		switch node.Type {
		case machineconfig.Separator:
			items = append(items, zenui.Item{Separator: true})
			flags = append(flags, "")
		case machineconfig.Title:
			items = append(items, zenui.Item{Title: true, Label: indent + node.Label})
			flags = append(flags, "")
		case machineconfig.Model:
			checked := node.ID == initialModel
			b.modelChecked[node.ID] = &checked
			items = append(items, zenui.Item{
				Label:   indent + node.Label,
				Checked: b.modelChecked[node.ID],
			})
			flags = append(flags, node.ID)
		case machineconfig.Submenu:
			subItems, subItemFlags, _ := b.buildMachineNodes(node.Items, initialModel)
			items = append(items, zenui.Item{Label: node.Label, SubItems: subItems})
			flags = append(flags, "")
			subFlags[len(items)-1] = subItemFlags
		}
	}
	return items, flags, subFlags
}

// romArchives is the pages Custom ROM > Download more ROMs opens, as of now: those of
// custom-roms/rom-archives.json when it is there and usable, else the standard ones
// (rom_archives.go). The file is read again when it changes.
func (b *appMenuBar) romArchives() []romArchive {
	stamp := romArchivesStamp(b.romDir)
	if !b.archivesRead || stamp != b.archivesStamp {
		list, warning := loadROMArchives(b.romDir)
		if warning != "" {
			fmt.Printf("Warning: %s\n", warning)
		}
		b.archives, b.archivesStamp, b.archivesRead = list, stamp, true
	}
	return b.archives
}

// isRomSites says the selection is the Custom ROM dropdown's "Download more ROMs" row (or a row
// of its submenu), which only opens a page and so needs no machine.
func (b *appMenuBar) isRomSites(sel zenui.MenuBarSelection) bool {
	return sel.BarIndex == barCustomROM && !b.bankMode && b.romMoreAt > 0 && sel.ItemIndex == b.romMoreAt
}

// zenZXHomepageURL is the ZenZX project's own page, opened by the logo
// menu's "View ZenZX Homepage" item.
const zenZXHomepageURL = "https://ha1tch.github.io/zsp/projects/zenzx"

// openURL hands a URL to the system's browser. It is a variable so that the tests can
// replace it: run on a machine with a display, a test that reached rl.OpenURL started a
// real browser, which then kept the test run from ending.
var openURL = rl.OpenURL

// helpSpec is the Help menu, in sections: help and about, and the developer tools. (The
// bar's preferences are File > Preferences.) Each section opens with a title. The Developer item is a submenu so
// that more tools can join the debugger there.
func (b *appMenuBar) helpSpec() menuSpec {
	return menuSpec{
		menuTitle("Help"),
		menuRow("Help", func(*ZenZX) { b.activeModal = newHelpModal() }),
		menuRow("ZenZX Manual", func(*ZenZX) { openManual(version, manualDirs(), openURL) }),
		menuRow("About...", func(*ZenZX) {
			b.activeModal = newAboutModal(strings.ReplaceAll(aboutTextTemplate, "__VERSION__", versionLine()))
		}),
		menuRow("Acknowledgements...", func(*ZenZX) { b.activeModal = newAcknowledgementsModal() }),
		menuRow("ZenZX website", func(*ZenZX) { openURL(zenZXHomepageURL) }),
		menuSep(),
		menuTitle("Tools"),
		{label: "Developer", sub: menuSpec{
			{label: "Debugger", disabled: b.openDebugger == nil, run: func(*ZenZX) { b.openDebugger() }},
		}},
	}
}

// fileSpec is the File menu.
func (b *appMenuBar) fileSpec() menuSpec {
	return menuSpec{
		menuRow("Open...", func(zx *ZenZX) { b.openFileDialog() }),
		menuRow("Export...", func(zx *ZenZX) { b.beginExport(zx) }), // the machine as a program of its own: export_gui.go
		menuRow("Preferences...", func(zx *ZenZX) { b.openPrefs(zx) }),
		menuSep(),
		{label: "Quit", run: func(*ZenZX) {
			if b.quit != nil {
				b.quit()
			}
		}},
	}
}

// openFileDialog opens the file browser for File > Open. What is chosen goes where the
// same file dropped on the window would (openFiles).
func (b *appMenuBar) openFileDialog() {
	cfg := b.openDialogConfig("Open", b.openFolder)
	cfg.FilterSets = fileOpenSets()
	b.diskDialog = zenui.NewDialog(cfg)
	b.dialogKind, b.fileDlg = dialogOpenFile, fileDlgOpen
}

// openLogoMenu opens the Help menu, anchored at the burger button. Whatever else of the
// bar is open is closed first: one menu at a time (closeMenus).
func (b *appMenuBar) openLogoMenu(screenW int) {
	b.closeMenus()
	// A zero-width anchor at the desired right edge forces Menu's own layout to
	// right-align regardless of the menu's own width -- the same trick openSubmenu already
	// uses for its own positioning. The edge is the burger button's, so the menu's right edge
	// lines up with it.
	hb := b.helpButton(screenW)
	b.logoSpec = b.helpSpec()
	b.overflowOpen = false
	// The Help menu sits at the right edge, so a submenu has no room on its right: zenui
	// opens it to the left of the row, beside the menu rather than over it.
	b.logoMenu = zenui.NewMenu(zenui.MenuConfig{
		Items:  b.logoSpec.items(),
		Anchor: zenui.Rect{X: hb.X + hb.W, Y: 0, W: 0, H: b.stripH()},
		Scale:  b.scale,
	})
}

// reconcileFixed makes the window agree with the bar's fixed choice, whichever way that
// choice was made or restored: the space the fixed bar occupies is reserved at the top
// of the window exactly while the bar is fixed, a fixed bar is on show, and the settings
// file holds the choice. It is idempotent and runs every frame (Update), so the choice and
// the window cannot be left disagreeing -- as they were when settings.json said fixed and
// nothing had yet asked the window to make room.
func (b *appMenuBar) reconcileFixed(zx *ZenZX) {
	want, drawn := 0, 0
	if b.fixed {
		want, drawn = b.reserveH(zx.window()), b.stripH()
		b.show()
	}
	zx.window().barOverlay = drawn
	zx.window().SetReservedTopHeight(want) // a no-op when it is already so
	if b.fixed != b.savedFixed {
		b.savedFixed = b.fixed
		b.saveSettings(zx)
	}
}

// applyFixedState is reconcileFixed, for the callers that have just changed b.fixed.
func (b *appMenuBar) applyFixedState(zx *ZenZX) { b.reconcileFixed(zx) }

// currentSettings is the preferences as they stand now: what saveSettings writes, and what a
// standalone program exported now carries.
func (b *appMenuBar) currentSettings(zx *ZenZX) *settingsconfig.Settings {
	dock, multi, stop, autoload := b.dockOn, b.multiOn, b.stopAnimation, b.tapeAutoload // copies: the result is kept
	return &settingsconfig.Settings{
		Version:       1,
		Theme:         string(b.themeName),
		Font:          string(b.fontName),
		FontZoom:      b.scale,
		DisplayScale:  zx.window().chosenMultiplier(),
		FixedMenuBar:  b.fixed,
		Toolbar:       b.toolbarOn,
		Dock:          &dock,
		Multi:         &multi,
		StopAnimation: &stop,
		TapeAutoload:  &autoload,
		Drop:          b.dropPlain.String(),
		DropShift:     b.dropShift.String(),
		SnapshotStart: snapshotStartWord(b.snapshotPlay),
		TapeDeck:      b.tapeDeck.String(),
		TapeCassette:  b.tapeCassette.String(),
		Escape:        b.escape.String(),
		Microdigital:  microdigitalWord(b.microdigitalPT),

		RecentSnapshots: append([]string(nil), b.recentSnaps...),
		RecentFiles:     append([]string(nil), b.recentFiles...),
		RecentTapes:     append([]string(nil), b.recentTapes...),
		OpenFolder:      b.openFolder,
		DiskFolder:      b.diskFolder,
		FileDialog:      b.fileView,
	}
}

// microdigitalNow is where the Microdigital language switch stands at this moment (true is
// Portuguese): the session's -language-switch switch while it stands, otherwise the saved
// setting.
func (b *appMenuBar) microdigitalNow() bool {
	if b == nil {
		return false
	}
	if b.microdigitalFlag != nil {
		return *b.microdigitalFlag
	}
	return b.microdigitalPT
}

// setMicrodigital is what the Preferences window does: it turns the switch, on every machine,
// ending the -language-switch switch's say for the rest of the session.
func (b *appMenuBar) setMicrodigital(portuguese bool) {
	b.microdigitalPT, b.microdigitalFlag = portuguese, nil
	SetMicrodigitalLanguage(portuguese)
}

// escapeNow is how the Escape key behaves at this moment: the session's -escape switch while it
// stands, otherwise the saved setting.
func (b *appMenuBar) escapeNow() escapeMode {
	if b == nil {
		return escapeOff
	}
	if b.escapeFlag != nil {
		return *b.escapeFlag
	}
	return b.escape
}

// setEscape is what the Preferences window does: it makes the choice the one in force, ending
// the -escape switch's say for the rest of the session.
func (b *appMenuBar) setEscape(m escapeMode) {
	b.escape, b.escapeFlag = m, nil
}

// saveSettings persists the current theme/font/zoom/scale/fixed state
// to b.settingsPath -- called after every menu action that changes
// one of them (ApplyTheme, ApplyFont, ApplyZoom, the View menu's own
// X1/X2/X3 dispatch, applyFixedState), so the next launch picks up
// where this session left off. A no-op whenever b.settingsPath is ""
// (the default for any appMenuBar not explicitly wired to persist,
// including every existing test's own construction), and a save
// failure is reported but not fatal -- losing the ability to persist
// a preference shouldn't crash a running emulator over it.
func (b *appMenuBar) saveSettings(zx *ZenZX) {
	if b.settingsPath == "" {
		return
	}
	s := b.currentSettings(zx)
	b.savedFixed = b.fixed // the file now holds this choice
	if err := settingsconfig.Save(b.settingsPath, s); err != nil {
		fmt.Printf("Warning: could not save %s: %v\n", b.settingsPath, err)
		zx.notice("The settings could not be saved to %s: %v", b.settingsPath, err)
	}
}

// handleLogoMenuResult carries out a choice from the Help menu: the row at index, or at
// sub within the row's submenu. A title, a divider and a disabled row are never carried
// out, whatever the index.
func (b *appMenuBar) handleLogoMenuResult(zx *ZenZX, index, sub int) {
	b.logoSpec.pick(zx, index, sub)
}

// switchModel is the Machine menu's choice of a model: the live switch, and for a Next its
// boot chain.
func (b *appMenuBar) switchModel(zx *ZenZX, model string) {
	zx.detachNextBootChain()
	switchModelLive(zx, model, b.romDir)
	if model == "next" {
		b.startNext(zx)
	}
}

// startNext makes a machine that is already a Next boot NextZXOS: from its boot files when
// they are there, and otherwise by opening the first-start dialog, which downloads them.
// With -next-classic it does neither.
func (b *appMenuBar) startNext(zx *ZenZX) {
	if b.nextClassic {
		return
	}
	f, ok, note := locateNextFiles(b.nextFilesDir)
	if ok {
		b.attachNext(zx, f)
		return
	}
	if note != "" {
		fmt.Printf("Next: boot files found but not usable: %s\n", note)
	}
	dir, err := nextDownloadDir(b.nextFilesDir)
	if err != nil {
		fmt.Printf("Next: no folder to keep the boot files in: %v\n", err)
		zx.notice("The Next has no folder to keep its boot files in: %v", err)
		return
	}
	b.nextSetup = newNextSetup(dir, note)
}

// attachNext attaches the boot chain and starts the machine from its boot ROM.
func (b *appMenuBar) attachNext(zx *ZenZX, f nextcard.Files) {
	protected, err := zx.attachNextBootChain(f, b.nextReadOnly, b.nextDebug)
	if err != nil {
		fmt.Printf("Next: the boot files in %s could not be used: %v\n", f.Dir, err)
		zx.notice("The Next boot files in %s could not be used: %v", f.Dir, err)
		return
	}
	zx.Reset()
	zx.io.nextResetRequest = nextResetHard
	fmt.Println(describeNextChain(f, protected))
	if protected && !b.nextReadOnly {
		// the card could not be opened for writing: what the Next saves to it is lost, and was not asked for
		zx.notice("The Next's SD card is write-protected, as it could not be opened for writing: nothing saved to it is kept")
	}
	card2 := b.nextCard2
	if card2 != "none" {
		// The card this Next would get in its second slot may be in another
		// machine already.
		path := card2
		if path == "" {
			path = filepath.Join(f.Dir, ownCardName)
		}
		if u := b.whereIs(path); u.machine != 0 && u.machine != b.numberOf(zx) {
			fmt.Printf("Next: %s is in machine %d; slot 2 of this machine is left empty\n", filepath.Base(path), u.machine)
			zx.notice("%s is in machine %d, so slot 2 of this Next is left empty", filepath.Base(path), u.machine)
			card2 = "none"
		}
	}
	if line, err := zx.insertOwnOrGivenCard(card2, b.nextReadOnly); err != nil {
		fmt.Printf("Next: the second SD card could not be used: %v\n", err)
		zx.notice("The second SD card could not be used: %v", err)
	} else if line != "" {
		fmt.Println(line)
	}
}

// easeInOutCubic is the standard ease-in-ease-out curve: slow start, fast
// middle, slow finish. t is a linear 0..1 progress fraction; the return
// value is the eased 0..1 fraction to actually use for the slide offset.
func easeInOutCubic(t float32) float32 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	f := -2*t + 2
	return 1 - f*f*f/2
}

// openDiskDialog opens the "Load Disk..." file browser, gated on the
// current model actually having a floppy controller (zx.io.hasFDC tracks
// this live -- switchModelLive enables it only for plus3/spanishplus3, so
// checking it here is equivalent to checking the model directly, without
// this file needing its own copy of that model list).
func (b *appMenuBar) openDiskDialog(zx *ZenZX) {
	if !zx.io.hasFDC {
		zx.notice("This model has no disk drive: switch to a +3 first, from the Machine menu")
		return
	}
	cfg := b.openDialogConfig("Load Disk", b.diskFolder)
	cfg.Filters = []string{"dsk"}
	b.diskDialog = zenui.NewDialog(cfg)
	b.dialogKind, b.fileDlg = dialogDisk, fileDlgDisk
}

// viewSpec is the View menu as the machine and the window stand now. The rows that
// depend on state take it from here, on every build, rather than being patched in place:
//   - Border asks borderView. When the border is the user's to choose it is a box on
//     showBorder itself; when it is fixed (a tiled layout, a Layer 2 mode that
//     fills the frame, a mode with no border) it is a disabled row showing what is true.
//   - X 1..X 3 are disabled when they do not fit the monitor or the mode, or when more
//     than one machine is on the screen (the zoom is for one machine filling the window).
//   - The layouts are disabled when the number of machines gives them no meaning.
func (b *appMenuBar) viewSpec(zx *ZenZX) menuSpec {
	dm := zx.window()
	spec := menuSpec{menuTitle("Display"), menuCheckbox("FPS Counter", &dm.showFPS, nil)}
	if v := dm.borderView(); v.Choosable {
		// The window changes size with the border, so the menu closes: the row is a box that does not
		// toggle (a toggling checkbox keeps the menu open, and it would be left where the window was);
		// UpdateWindowSize re-sizes the window to match.
		spec = append(spec, menuEntry{label: "Border", checked: &dm.showBorder, box: true, run: func(zx *ZenZX) {
			dm := zx.window()
			dm.showBorder = !dm.showBorder
		}})
	} else {
		shown := v.Shown
		spec = append(spec, menuEntry{label: "Border", checked: &shown, box: true, disabled: true})
	}
	fullscreen := dm.fullscreen
	spec = append(spec,
		menuEntry{label: "Full Screen", checked: &fullscreen, box: true, run: func(zx *ZenZX) { zx.window().ToggleFullscreen() }},
		menuSep(),
		menuTitle("Scale"),
	)
	layout, machines := layoutSingle, 1
	if b.layoutState != nil {
		layout, machines = b.layoutState()
	}
	limit := dm.maxMultiplierThatFits()
	// Layer 2's 640x256x4bpp mode gates out 1x (see DisplayManager.minMultiplierForMode),
	// so X 1 is disabled the way an X above the monitor-fit limit is, rather than silently
	// doing nothing when clicked.
	min := dm.MinMultiplierForMode()
	for n := 1; n <= 3; n++ {
		n := n
		checked := dm.screen.multiplier == n
		b.displayScaleChecked[n] = &checked
		spec = append(spec, menuEntry{
			label:    fmt.Sprintf("X %d", n),
			checked:  b.displayScaleChecked[n],
			disabled: n > limit || n < min || layout != layoutSingle || dm.fullscreen,
			run:      func(zx *ZenZX) { zx.window().SetScale(n); b.saveSettings(zx) },
		})
	}
	spec = append(spec, menuSep(), menuTitle("Layout"))
	for i, name := range deckLayoutNames {
		l := deckLayout(i)
		checked := l == layout
		spec = append(spec, menuEntry{
			label:    name,
			checked:  &checked,
			disabled: !layoutAllowed(l, machines),
			run: func(zx *ZenZX) {
				if b.setLayout != nil {
					b.setLayout(l)
				}
			},
		})
	}
	spec = append(spec, menuEntry{
		label:    viewRotateLabel,
		disabled: machines < 2,
		run: func(zx *ZenZX) {
			if b.rotateMachines != nil {
				b.rotateMachines()
			}
		},
	})
	// Screenshot: the formats the picture can be saved in, and the window of what was (screenshot_gui.go)
	return append(spec, menuSep(), menuTitle("Capture"), menuEntry{label: "Screenshot", sub: b.screenshotSpec(zx)})
}

// refreshViewItems rebuilds the View menu each frame the bar is shown, items and actions
// together from viewSpec. Item.Disabled has no pointer-based live-update mechanism the way
// Checked does, and the scale can change outside the menu entirely (PgUp/PgDn), so
// rebuilding via SetItems is what keeps both correct.
//
// This is where a real MenuBar.SetItems bug was actually discovered: an earlier version
// recreated the open dropdown's Menu from scratch on every SetItems call, so calling this
// every frame made the whole View menu feel unresponsive (empty itemRects until the next
// Draw) the entire time it was open. Fixed in pkg/zenui/menubar.go itself.
func (b *appMenuBar) refreshViewItems(zx *ZenZX) {
	spec := b.viewSpec(zx)
	b.actions[barView] = spec.actions()
	b.viewMenu = spec
	b.widget.SetItems(barView, spec.items())
}

// refreshCustomROMItems rebuilds the Custom ROM dropdown's contents --
// called every frame the bar is shown, since the directory's contents (or
// whether we're mid-flow picking a bank instead of a ROM) can change
// between opens. Calling SetItems every frame while this dropdown might
// be open depends on MenuBar.SetItems updating an already-open Menu's
// items in place rather than recreating it -- an earlier version of
// SetItems recreated the Menu instead, wiping itemRects until the next
// Draw and making whichever dropdown this pattern was applied to
// (discovered via the View menu, once it gained the same per-frame
// refresh for its own zoom items) feel completely unresponsive the
// entire time it was open. Fixed at the source in pkg/zenui/menubar.go
// rather than worked around here.
func (b *appMenuBar) refreshCustomROMItems(zx *ZenZX) {
	var items []zenui.Item
	b.romMoreAt = 0
	if b.bankMode {
		maxBank := zx.maxROMBank()
		items = make([]zenui.Item, maxBank+1)
		for i := range items {
			items[i] = zenui.Item{Label: fmt.Sprintf("Bank %d", i)}
		}
	} else {
		roms, folders := scanCustomROMs(b.romDir)
		b.romRows = buildROMRows(roms, folders, b.romRowsFit())
		if len(b.romRows) == 0 {
			items = []zenui.Item{{Label: "No custom ROMs found", Disabled: true}}
		} else {
			items = make([]zenui.Item, len(b.romRows))
			for i, r := range b.romRows {
				items[i] = zenui.Item{Label: r.Label}
				if r.isMenu() {
					items[i].SubItems = make([]zenui.Item, len(r.Subs))
					for j, s := range r.Subs {
						items[i].SubItems[j] = zenui.Item{Label: s.Label}
					}
				}
			}
		}
		// After the ROMs, a way to get more: a row that opens the pages where they are published
		// (none, when rom-archives.json is an empty list). Its submenu opens level with the row and
		// goes down, so only as many pages as fit under it are shown.
		b.romLinks = nil
		if archives := b.romArchives(); len(archives) > 0 {
			room := b.romRowsFit() - (len(items) + 1)
			if room < minROMRows {
				room = minROMRows
			}
			if len(archives) > room {
				archives = archives[:room]
			}
			b.romLinks = archives
			sites := make([]zenui.Item, len(archives))
			for i, s := range archives {
				sites[i] = zenui.Item{Label: s.Label}
			}
			items = append(items, zenui.Item{Separator: true}, zenui.Item{Label: "Download more ROMs", SubItems: sites})
			b.romMoreAt = len(items) - 1
		}
	}
	b.widget.SetItems(barCustomROM, items)
}

// romRowsFit is how many rows of the Custom ROM dropdown the window has room for below the bar,
// which is what buildROMRows cuts the folders' submenus and "More..." to. A window that is not
// there (a test) says a dozen.
func (b *appMenuBar) romRowsFit() int {
	const noWindow = 12
	h := int(rl.GetScreenHeight())
	if h <= 0 || b.renderer.Text == nil {
		return noWindow
	}
	lh := b.renderer.LineHeight(b.scale)
	itemH := lh + 2*(lh*b.theme.ItemPadYPercent/100) // the height zenui gives a row
	if itemH <= 0 {
		return noWindow
	}
	return (h - b.stripH() - b.theme.DropdownBottomPadding) / itemH
}

// pickCustomROM carries out a choice in the Custom ROM dropdown: a link to a page of ROMs, a
// ROM (which, on a machine with several ROM banks, goes on to ask for the bank), or the bank.
func (b *appMenuBar) pickCustomROM(zx *ZenZX, sel zenui.MenuBarSelection) {
	if b.isRomSites(sel) {
		if sel.SubIndex >= 0 && sel.SubIndex < len(b.romLinks) {
			openURL(b.romLinks[sel.SubIndex].URL)
		}
		return
	}
	if b.bankMode {
		bank := sel.ItemIndex
		rom := b.pendingROM
		b.bankMode, b.pendingROM = false, ""
		applyCustomROMLive(zx, b.romDir, rom, bank)
		return
	}
	rom, ok := romAt(b.romRows, sel.ItemIndex, sel.SubIndex)
	if !ok {
		return // the disabled "No custom ROMs found" placeholder, or a folder or "More..." chosen as a whole
	}
	if zx.maxROMBank() == 0 {
		applyCustomROMLive(zx, b.romDir, rom, 0)
		return
	}
	b.pendingROM = rom
	b.bankMode = true
}

// Update advances the idle timer, the slide animation, and the widget
// itself, dispatching any accepted selection to the right action (or, for
// Custom ROM, advancing the two-step flow).
func (b *appMenuBar) Update(zx *ZenZX) {
	b.barK = b.barScale(int(rl.GetScreenWidth()), int(rl.GetScreenHeight())) // before the strip's height is asked for
	b.updateBarCursor()
	b.stepToolbar(rl.GetFrameTime())
	b.reconcileFixed(zx)
	b.enforceSingleSurface()
	if b.prefs != nil {
		if b.prefs.Update(zenuiraylib.Input(), b.renderer, int(rl.GetScreenWidth()), int(rl.GetScreenHeight())) == prefsClosed {
			b.prefs = nil
		}
		b.updatePrefsCursor()
		return
	}
	b.updatePrefsCursor() // the arrow again, if the window went while the pointer was on a tab
	if b.cards != nil {
		b.updateCards(zx)
		return
	}
	if b.replaceAsk != nil {
		b.updateReplaceAsk(zx)
		return
	}
	if b.cardsAsk != nil && b.diskDialog == nil { // the file browser, when it is open over it, is run below
		b.updateCardsAsk(zx)
		return
	}
	if b.snaps != nil {
		b.updateSnapshots(zx)
		return
	}
	if b.shots != nil {
		b.updateScreenshots(zx)
		return
	}
	if b.tapes != nil {
		b.updateTapes(zx)
		return
	}
	if b.bin != nil {
		b.updateBinary(zx)
		return
	}
	if b.disks != nil {
		b.updateDisks()
		return
	}
	if b.nextSetup != nil {
		switch b.nextSetup.update(zenuiraylib.Input()) {
		case nextSetupReady:
			dir := b.nextSetup.dir
			b.nextSetup = nil
			b.attachNext(zx, nextcard.In(dir))
		case nextSetupDeclined:
			b.nextSetup = nil
		}
		return
	}

	if b.activeModal != nil {
		open := b.activeModal.Update(zenuiraylib.Input())
		setHandCursor(&b.modalHand, open && b.activeModal.OverLink())
		if !open {
			b.activeModal = nil
		}
		return
	}

	if b.diskDialog != nil {
		switch b.diskDialog.Update(zenuiraylib.Input()) {
		case zenui.Accepted:
			b.acceptDialog(zx, b.diskDialog.Result())
		case zenui.Cancelled:
			b.endFileDialog(zx, b.diskDialog)
			b.diskDialog, b.dialogKind, b.newCardSize, b.saveFormat = nil, dialogDisk, 0, "" // the Export form, if it was open under it, stays
		}
		return
	}

	if b.exportAsk != nil {
		b.updateExport(zx, zenuiraylib.Input())
		return
	}

	if b.logoMenu != nil {
		switch b.logoMenu.Update(zenuiraylib.Input()) {
		case zenui.Accepted:
			result, sub := b.logoMenu.Result(), b.logoMenu.SubResult()
			b.logoMenu = nil
			b.handleLogoMenuResult(zx, result, sub)
		case zenui.Toggled:
			// A checkbox (Fixed menu bar): Item.Toggle has already flipped its bool
			// through the pointer, and the row's own action does the rest. The menu stays
			// open, as a checkbox's does.
			b.handleLogoMenuResult(zx, b.logoMenu.Result(), b.logoMenu.SubResult())
		case zenui.Cancelled:
			b.logoMenu = nil
		}
		return
	}

	mx, my := rl.GetMouseX(), rl.GetMouseY()

	if b.state == barHidden || b.state == barSlidingOut {
		if mx == b.lastMouseX && my == b.lastMouseY {
			if !b.idling {
				b.idling = true
				b.idleSince = time.Now()
			}
		} else {
			b.idling = false
		}

		atEdge := my <= menuBarEdgeDistance
		if atEdge && b.idling && time.Since(b.idleSince) >= menuBarIdleDelay {
			b.show()
		}
	}
	b.lastMouseX, b.lastMouseY = mx, my

	switch b.state {
	case barSlidingIn:
		t := float32(time.Since(b.stateChangedAt)) / float32(menuBarSlideTime)
		if t >= 1 {
			b.progress, b.state = 1, barShown
		} else {
			b.progress = easeInOutCubic(t)
		}
	case barSlidingOut:
		t := float32(time.Since(b.stateChangedAt)) / float32(menuBarSlideTime)
		if t >= 1 {
			b.progress, b.state = 0, barHidden
		} else {
			b.progress = 1 - easeInOutCubic(t)
		}
	}

	// Dismiss once shown, nothing open, and the mouse has moved well
	// clear of the bar -- a dropdown being open holds the bar regardless
	// of mouse position, since its own items can sit below where this
	// margin would otherwise trigger. A fixed bar bypasses this
	// entirely, staying shown regardless of mouse position, until
	// unfixed via the logo menu.
	if b.state == barShown && !b.fixed && !b.widget.Active() && my > int32(b.stripH())+menuBarHideMargin {
		b.hide()
	}

	if b.state == barShown {
		b.refreshCustomROMItems(zx)
		b.refreshViewItems(zx)
		b.refreshMachineItems(zx)
		b.refreshTapeItems(zx)        // which of Play and Stop can be chosen: tapemenu_gui.go
		b.refreshStorageMenu(zx)      // Floppy Disk, or SD Card on a Next: sdcardmenu_gui.go
		b.refreshSnapshotItemsFor(zx) // Load Timestamped follows the snapshots folder: snapshotmenu_gui.go
		b.toolbarCur = b.toolbarItems(zx)
	}

	// Widget interaction (hover-to-open, click-to-select) is gated on
	// the bar being fully, restingly shown -- not just visible. Without
	// this, hit-testing during barSlidingIn/barSlidingOut runs against
	// labelRects computed at whatever Y the bar's mid-animation position
	// happened to be on the last Draw call, so a dropdown could open
	// anchored to a position the bar hadn't actually settled into yet --
	// the "menu floating mid-air while the bar is still unrolling" bug.
	if b.state != barShown {
		return
	}

	input := zenuiraylib.Input()
	if b.barButtonsInput(zx, input, int(rl.GetScreenWidth())) {
		return
	}
	if b.toolbarOn || b.toolbarT > 0 {
		return // the labels are out of the strip: they have no input while the toolbar is, or is sliding
	}
	sel, ok := b.widget.Update(input)
	if ok && zx.stopped && b.needsMachine(sel) {
		zx.notice("No machine is running")
		return
	}
	if !ok {
		return
	}

	if sel.BarIndex == barCustomROM {
		b.pickCustomROM(zx, sel)
		return
	}

	// Machine's model rows (Reset, Play, Pause and Stop are index 1 to 4, handled
	// by the generic dispatch below same as always) start at index machineHeadRows.
	// A row is either a flat model (direct index lookup into
	// machineModelFlags) or a submenu (its own index has an entry in
	// machineSubmenuFlags, and sel.SubIndex picks which of that
	// submenu's own models was chosen) -- machines.json's "submenu"
	// node type is what produces the latter; every other node type
	// (separator/title/model) produces the former. Title/Separator
	// rows carry "" and are additionally never selectable in the first
	// place (Menu.Update's own itemEnabled already excludes them), so
	// these checks are a second, belt-and-braces guard, not the
	// primary one.
	if sel.BarIndex == barMachine && sel.ItemIndex >= machineHeadRows {
		b.pickMachineModel(zx, sel)
		return
	}

	// Theme's dropdown has one item with SubItems: "Font", holding
	// every font/zoom choice as a nested submenu (an earlier version
	// had Font as its own top-level bar label). Its own index within
	// Theme's items is themeFontAt, set where the items are made.
	if sel.BarIndex == barTheme && sel.ItemIndex == b.themeFontAt {
		if sel.SubIndex >= 0 && sel.SubIndex < len(b.fontActions) {
			b.fontActions[sel.SubIndex](zx)
		}
		return
	}

	if sel.BarIndex == barFloppyDisk {
		b.storageSelected(zx, sel.ItemIndex, sel.SubIndex)
		return
	}
	// The Tape menu has a submenu, Recent Tapes, which the flat dispatch below cannot reach.
	if sel.BarIndex == barTape {
		b.tapeMenu.pick(zx, sel.ItemIndex, sel.SubIndex)
		return
	}
	// The File menu is a menuSpec, which the flat dispatch below cannot reach.
	if sel.BarIndex == barFile {
		b.fileMenu.pick(zx, sel.ItemIndex, sel.SubIndex)
		return
	}
	// The View menu has a submenu, Screenshot, which the flat dispatch below cannot reach.
	if sel.BarIndex == barView {
		b.viewMenu.pick(zx, sel.ItemIndex, sel.SubIndex)
		return
	}
	// The Snapshot menu has a submenu, Save As, which the flat dispatch below cannot reach.
	if sel.BarIndex == barSnapshot {
		b.snapSpec.pick(zx, sel.ItemIndex, sel.SubIndex)
		return
	}

	if list, ok := b.actions[sel.BarIndex]; ok && sel.ItemIndex >= 0 && sel.ItemIndex < len(list) {
		list[sel.ItemIndex](zx)
	}
}

// acceptDialog carries out the file browser's answer: path is the file chosen, and what
// the browser was open for (dialogKind) says what is done with it.
func (b *appMenuBar) acceptDialog(zx *ZenZX, path string) {
	kind, size, format := b.dialogKind, b.newCardSize, b.saveFormat
	which := b.endFileDialog(zx, b.diskDialog)
	b.diskDialog, b.dialogKind, b.newCardSize, b.saveFormat = nil, dialogDisk, 0, ""
	switch kind {
	case dialogSaveSnapshot:
		if saved := zx.saveSnapshotAs(path, format); saved != "" {
			b.noteRecentSnapshot(zx, saved)
		}
		return
	case dialogLoadSnapshot:
		if b.loadSnapshotAt(zx, path) {
			b.noteRecentSnapshot(zx, path)
		}
		return
	case dialogSaveMulti:
		b.saveMultiSnapshotAt(zx, path)
		return
	case dialogLoadMulti:
		b.loadMultiSnapshotAt(zx, path)
		return
	case dialogChooseCard:
		if b.cardsAsk != nil {
			b.cardsAsk.choose(path)
		}
		return
	case dialogExportBrowse:
		b.exportBrowsed(path)
		return
	case dialogNewCard:
		b.newCard(zx, path, size)
		return
	case dialogInsertCard:
		b.insertCard(zx, path)
		return
	case dialogLoadTape:
		if b.openFiles != nil { // a tape is loaded as one dropped on the window is, and noted among the recent ones there
			b.openFiles([]string{path})
		}
		return
	case dialogOpenFile:
		b.keepFolder(zx, which, path) // the next File > Open starts where this file is
		b.noteRecentFile(zx, path)
		if b.openFiles != nil {
			b.openFiles([]string{path})
		}
		return
	}
	b.loadDisk(zx, path, which)
}

// loadDisk puts the disk image at path in the drive, as Load Disk... and Recent Disks do: a disk that
// is in another machine's drive stays there, and the folder it came from is kept for the next Load Disk.
func (b *appMenuBar) loadDisk(zx *ZenZX, path string, which fileDialogKind) {
	if n, taken := b.diskTakenElsewhere(zx, path); taken {
		zx.notice("%s is in the drive of machine %d: a disk file is in one machine at a time", filepath.Base(path), n)
	} else if err := zx.io.LoadDisk(path); err != nil {
		zx.notice("The disk %s could not be loaded: %v", filepath.Base(path), err)
	} else {
		if which != fileDlgNone {
			b.keepFolder(zx, which, path) // the next Load Disk starts where this disk is
		}
		b.noteRecentFile(zx, path)
		zx.notice("The disk %s is in the drive", filepath.Base(path))
	}
}

// currentModel is the model the Machine menu has ticked, "" when it is none
// of the menu's.
func (b *appMenuBar) currentModel() string {
	for k, checked := range b.modelChecked {
		if *checked {
			return k
		}
	}
	return ""
}

// setCurrentModel updates modelChecked so the Machine menu's submenus
// show a checkmark on whichever model is now running, clearing every
// other model's checkmark first.
func (b *appMenuBar) setCurrentModel(model string) {
	for k, checked := range b.modelChecked {
		*checked = k == model
	}
}

// Draw renders the bar strip (slid according to b.progress) and whichever
// dropdown is open. Call from within Render's preEndDrawHook -- see
// display.go's preEndDrawHook doc comment for why a plain post-Render call
// doesn't work. The gradient and rainbow decorations (theme.UseBarGradient/
// ShowBarRainbow) are drawn by MenuBar.Draw itself, entirely through the
// Renderer interface -- this file has no raylib calls of its own left for
// either.
//
// A window that is open covers the whole screen and is all that is drawn. One that has just closed
// fades out over the bar and what is under it (dialogfade_gui.go).
func (b *appMenuBar) Draw(screenW, screenH int) {
	if modal := b.modalDrawer(screenW, screenH); modal != nil {
		b.fade.watch(modal)
		modal(b.renderer)
		return
	}
	b.fade.capture(b.renderer, screenW, screenH)
	b.drawUnderWindows(screenW, screenH)
	b.fade.paint(b.renderer)
}

// drawUnderWindows draws the bar, and the logo menu if it is open: what is there when no window is.
func (b *appMenuBar) drawUnderWindows(screenW, screenH int) {
	if b.logoMenu != nil {
		// The bar itself still needs to be drawn underneath (its own
		// hot zone is what the menu is anchored to), unlike diskDialog
		// and activeModal, which cover the whole screen with their own
		// backdrop.
		barY := b.drawBar(screenW, screenH)
		b.paintBarButtons(screenW, barY)
		b.logoMenu.Draw(b.withPopupShadow(b.renderer, false, barY+b.stripH()), screenW, screenH, b.theme)
		return
	}
	if b.state == barHidden && !b.widget.Active() {
		return
	}
	barY := b.drawBar(screenW, screenH)
	b.paintBarButtons(screenW, barY)
}
