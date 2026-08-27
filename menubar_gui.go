//go:build !headless

package main

import (
	_ "embed"
	"fmt"
	"strings"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/zenzx/pkg/fonts"
	"github.com/ha1tch/zenzx/pkg/machineconfig"
	"github.com/ha1tch/zenzx/pkg/settingsconfig"
	"github.com/ha1tch/zenzx/pkg/zenui"
	"github.com/ha1tch/zenzx/pkg/zenuiraylib"
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
	"ts2068",
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
// root for the current grouping, including "En Español"'s own
// per-model manufacturer attribution (Sinclair/Investrónica/Amstrad --
// it spans three, unlike every other group here, which has exactly
// one).

// zoomLevels are the magnification factors the Font menu's "Font zoom
// X_" items offer -- applied to every dropdown's text/layout, never to
// the bar strip itself, which always draws at scale 1 regardless (see
// zenui.MenuBar.Draw). X2 is the default, matching what the bar already
// rendered at before this setting existed, so picking nothing changes
// nothing.
var zoomLevels = []int{1, 2, 3}

// barState is the slide animation's lifecycle.
type barState int

const (
	barHidden barState = iota
	barSlidingIn
	barShown
	barSlidingOut
)

// bar index constants -- Custom ROM (barCustomROM) gets dedicated handling
// in Update since picking a ROM is a two-step flow (file, then maybe bank);
// every other menu dispatches through the uniform actions table.
const (
	barMachine = iota
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

	romDir     string
	romNames   []string // snapshot of custom-roms/ taken when its dropdown opened
	pendingROM string   // ROM chosen in step one, awaiting a bank pick in step two
	bankMode   bool     // true while the Custom ROM dropdown is showing bank choices, not ROM names

	// diskDialog is the Open DSK Image file browser, non-nil only while
	// open. Unlike Custom ROM's two-step flow (which reuses the bar's
	// own dropdown), this is a separate zenui.Dialog instance -- a file
	// browser, not a flat item list -- drawn and updated independently
	// of b.widget.
	diskDialog *zenui.Dialog

	// logoMenu is the small dropdown associated with the rainbow
	// (Spectrum theme) / ZSP logo (Dark/Light) decoration on the
	// right of the bar -- Fix/Hide bar, Help, About, View ZenZX
	// homepage. Hardcoded rather than built through the same
	// zenui.MenuBarItem/actions machinery every other menu uses: the
	// decoration isn't really a menu bar label, it doesn't need a
	// checkmark/submenu/toggle, and there are only ever these four
	// fixed choices. Non-nil only while open.
	logoMenu *zenui.Menu
	// activeModal is whichever of Help/About is currently open (nil
	// otherwise) -- a single field, not two, since only one can ever
	// be open at a time (both are only reachable through logoMenu,
	// which itself can't be open at the same time as either).
	activeModal *markdownModal
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
	// (themeFontItemIndex identifies which of Theme's items it is),
	// not a top-level bar label with its own b.actions[barX] entry the
	// way every other menu still is.
	fontActions []func(zx *ZenZX)

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
	initialFontName := fonts.NameSinclair
	initialFontZoom := 2
	initialFixed := false
	if initialSettings != nil {
		initialFontName = fonts.Name(initialSettings.Font)
		initialFontZoom = initialSettings.FontZoom
		initialFixed = initialSettings.FixedMenuBar
	}

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
		settingsPath:        settingsPath,
		romDir:              romDir,
		actions:             make(map[int][]func(zx *ZenZX)),
		themeChecked:        make(map[zenui.ThemeName]*bool),
		fontChecked:         make(map[fonts.Name]*bool),
		zoomChecked:         make(map[int]*bool),
		modelChecked:        make(map[string]*bool),
		displayScaleChecked: make(map[int]*bool),
	}

	machineActions := []func(zx *ZenZX){
		func(zx *ZenZX) { zx.Reset(); fmt.Println("Reset") },
		func(zx *ZenZX) { zx.TogglePause() },
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
	}

	groupItems, groupFlags, groupSubFlags := b.buildMachineNodes(machinesRes.Config.Items, initialModel)
	machineItems := append([]zenui.Item{{Label: "Reset"}, {Label: "Pause/Resume"}}, groupItems...)
	machineModelFlags := append([]string{"", ""}, groupFlags...)
	b.machineModelFlags = machineModelFlags
	// groupSubFlags is indexed relative to groupItems (0-based); shift
	// every key by 2 (Reset/Pause/Resume) to match machineItems' own
	// indices, which is what sel.ItemIndex actually reports.
	b.machineSubmenuFlags = make(map[int][]string, len(groupSubFlags))
	for i, flags := range groupSubFlags {
		b.machineSubmenuFlags[i+2] = flags
	}

	tapeItems := []zenui.Item{
		{Label: "Play / Stop"}, {Label: "Rewind"},
		{Separator: true},
		{Label: "Accurate/Fast Mode"}, {Label: "Show Info"},
	}
	b.actions[barTape] = []func(zx *ZenZX){
		func(zx *ZenZX) { zx.PlayStopTape() },
		func(zx *ZenZX) { zx.RewindTape() },
		func(zx *ZenZX) {}, // separator placeholder
		func(zx *ZenZX) { zx.ToggleTapeMode() },
		func(zx *ZenZX) { zx.ShowTapeInfo() },
	}

	diskItems := []zenui.Item{
		{Label: "Open DSK Image..."}, {Label: "Insert Blank Disk"},
		{Separator: true},
		{Label: "Save Disk"}, {Label: "Save Disk As..."},
		{Separator: true},
		{Label: "Eject Disk"}, {Label: "Disk Info"},
	}
	b.actions[barFloppyDisk] = []func(zx *ZenZX){
		func(zx *ZenZX) { b.openDiskDialog(zx) },
		func(zx *ZenZX) { zx.handleInsertBlankDisk() },
		func(zx *ZenZX) {}, // separator placeholder
		func(zx *ZenZX) { zx.handleSaveDisk() },
		func(zx *ZenZX) { zx.handleSaveDiskAs() },
		func(zx *ZenZX) {}, // separator placeholder
		func(zx *ZenZX) { zx.handleEjectDisk() },
		func(zx *ZenZX) { printDiskInfo(zx) },
	}

	snapItems := []zenui.Item{
		{Label: "Quick Save"}, {Label: "Quick Load"}, {Label: "Save Timestamped"},
		{Separator: true},
		{Label: "Snapshot Info"}, {Label: "Run Diagnostics"},
	}
	b.actions[barSnapshot] = []func(zx *ZenZX){
		func(zx *ZenZX) { zx.handleQuickSave() },
		func(zx *ZenZX) { zx.handleQuickLoad() },
		func(zx *ZenZX) { zx.handleAutoSave() },
		func(zx *ZenZX) {}, // separator placeholder
		func(zx *ZenZX) { fmt.Println("Snapshot loading: Drop a .zxs file onto the window") },
		func(zx *ZenZX) {
			fmt.Println("\nRunning snapshot diagnostics...")
			NewDebugSnapshot(zx).RunFullDiagnostics()
		},
	}

	// FPS/Border are checkboxes pointing directly at zx.display's own
	// fields, not a separate synced copy -- the same bool the Alt+F/
	// Alt+B keyboard shortcuts already flip via ToggleFPS/ToggleBorder,
	// so the checkmark is always correct regardless of which path
	// changed it, with no refresh step needed. This does mean
	// ToggleBorder's own console print doesn't fire when toggled from
	// here (Toggle flips *Checked directly, bypassing that method) --
	// an accepted, minor trade-off given the checkbox itself now makes
	// the state visible without needing the console at all.
	viewItems := []zenui.Item{
		{Label: "FPS Counter", Checked: &zx.display.showFPS, Toggle: true},
		{Label: "Border Display", Checked: &zx.display.showBorder, Toggle: true},
		{Label: "Show Status"},
		{Separator: true},
		{Label: "X 1"}, {Label: "X 2"}, {Label: "X 3"}, // real Checked/Disabled values are set by refreshViewItems each frame the bar is shown
	}
	b.actions[barView] = []func(zx *ZenZX){
		func(zx *ZenZX) {}, // FPS is a pure checkbox: Item.Toggle flips the bool directly and ToggleFPS has no other side effect, so this generic-dispatch entry is a genuine no-op
		func(zx *ZenZX) { zx.display.updateTargetSize() }, // Border's own ToggleBorder does more than flip a bool: it also triggers a window-resize animation via updateTargetSize (isAnimating=true, animated in UpdateWindowSize). Item.Toggle already flipped showBorder directly; this closure -- which the generic dispatch already calls for every Toggled event, not just Accepted -- supplies the second half so Alt+B and this checkbox behave identically, not just agree on the bool.
		func(zx *ZenZX) { zx.showStatus() },
		func(zx *ZenZX) {}, // the separator's own index -- never actually called, Menu.itemEnabled excludes separators from selection entirely, but the generic dispatch is a flat index into this slice, so a placeholder keeps X1/X2/X3 below aligned
		func(zx *ZenZX) { zx.display.SetScale(1); b.saveSettings(zx) },
		func(zx *ZenZX) { zx.display.SetScale(2); b.saveSettings(zx) },
		func(zx *ZenZX) { zx.display.SetScale(3); b.saveSettings(zx) },
	}

	themeItems := make([]zenui.Item, len(zenui.Themes))
	themeActions := make([]func(zx *ZenZX), len(zenui.Themes))
	for i, name := range zenui.Themes {
		checked := name == initialTheme
		b.themeChecked[name] = &checked
		themeItems[i] = zenui.Item{Label: string(name), Checked: b.themeChecked[name]}
		themeName := name // capture per-iteration
		themeActions[i] = func(zx *ZenZX) { b.ApplyTheme(themeName); b.saveSettings(zx) }
	}
	b.actions[barTheme] = themeActions

	fontItems := make([]zenui.Item, 0, len(fonts.All)+len(zoomLevels))
	fontActions := make([]func(zx *ZenZX), 0, len(fonts.All)+len(zoomLevels))
	for _, name := range fonts.All {
		checked := name == b.fontName // b.fontName already reflects the real initial value (initialFontName above)
		b.fontChecked[name] = &checked
		fontItems = append(fontItems, zenui.Item{Label: string(name), Checked: b.fontChecked[name]})
		fontName := name // capture per-iteration
		fontActions = append(fontActions, func(zx *ZenZX) {
			if err := b.ApplyFont(fontName); err != nil {
				fmt.Printf("Warning: could not switch to font %s: %v\n", fontName, err)
				return
			}
			b.saveSettings(zx)
		})
	}
	fontItems = append(fontItems, zenui.Item{Separator: true})
	fontActions = append(fontActions, func(zx *ZenZX) {}) // separator's own index -- never actually called, placeholder to keep the zoom entries below aligned
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
	// separate ones. themeFontItemIndex (len(zenui.Themes)+1: the
	// themes, then a separator, then Font) is computed the same way
	// here and in Update's own dispatch, rather than hardcoded twice.
	themeItems = append(themeItems, zenui.Item{Separator: true})
	themeItems = append(themeItems, zenui.Item{Label: "Font", SubItems: fontItems})

	// Custom ROM's items are populated fresh each time it's opened
	// (refreshCustomROMItems), since the directory's contents can't be
	// known up front the way every other menu's fixed item list can.
	b.widget = zenui.NewMenuBar(zenui.MenuBarConfig{
		Items: []zenui.MenuBarItem{
			{Label: "Machine", Items: machineItems},
			{Label: "Custom ROM", Items: nil},
			{Label: "Tape", Items: tapeItems},
			{Label: "Floppy Disk", Items: diskItems},
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
// draws at -- never the bar strip itself, which zenui.MenuBar.Draw
// always renders at scale 1 regardless of this setting. Takes effect
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
	return b.widget.Active() || b.diskDialog != nil || b.logoMenu != nil || b.activeModal != nil
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

// zenZXHomepageURL is the ZenZX project's own page, opened by the logo
// menu's "View ZenZX Homepage" item.
const zenZXHomepageURL = "https://ha1tch.github.io/zsp/projects/zenzx"

// logoHotZone returns the clickable area associated with the rainbow
// (Spectrum theme) / ZSP logo (Dark/Light) decoration -- everything
// to the right of the bar's own labels, up to the screen's own right
// edge. Covers the decoration's own drawing area regardless of theme
// without needing to duplicate pkg/zenui's own (unexported)
// rainbowGeometry/zspLogoGeometry sizing logic here; a generously wide
// hot zone costs nothing since nothing else occupies that space.
func (b *appMenuBar) logoHotZone(screenW int) zenui.Rect {
	x := b.widget.LabelsEndX()
	return zenui.Rect{X: x, Y: 0, W: screenW - x, H: menuStripHeight}
}

// openLogoMenu opens the logo decoration's own small dropdown --
// hardcoded rather than built through the same zenui.MenuBarItem/
// actions machinery every other menu uses, since the decoration isn't
// really a menu bar label and there are only ever these four fixed
// choices.
func (b *appMenuBar) openLogoMenu(screenW int) {
	// A zero-width anchor at the desired right edge forces Menu's own
	// layout to right-align regardless of the menu's own width --
	// the same trick openSubmenu already uses for its own positioning.
	// b.logoHotZone's own rightMargin (8px) is reused here so the
	// menu's right edge lines up with wherever the rainbow/logo
	// decoration's own right edge falls.
	const rightMargin = 8
	b.logoMenu = zenui.NewMenu(zenui.MenuConfig{
		Items: []zenui.Item{
			{Label: "Fixed menu bar", Checked: &b.fixed, Toggle: true},
			{Label: "ZenZX website"},
			{Label: "Help"},
			{Label: "About..."},
		},
		Anchor: zenui.Rect{X: screenW - rightMargin, Y: 0, W: 0, H: menuStripHeight},
		Scale:  b.scale,
	})
}

// applyFixedState syncs the window's reserved top height with the
// bar's own b.fixed -- called after Item.Toggle has already flipped
// b.fixed directly via its own pointer, the same "Toggle flips the
// bool, a separate step supplies the side effect" pattern the View
// menu's Border checkbox already established.
func (b *appMenuBar) applyFixedState(zx *ZenZX) {
	if b.fixed {
		b.show()
		zx.display.SetReservedTopHeight(menuStripHeight)
	} else {
		zx.display.SetReservedTopHeight(0)
	}
	b.saveSettings(zx)
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
	s := &settingsconfig.Settings{
		Version:      1,
		Theme:        string(b.themeName),
		Font:         string(b.fontName),
		FontZoom:     b.scale,
		DisplayScale: zx.display.screen.multiplier,
		FixedMenuBar: b.fixed,
	}
	if err := settingsconfig.Save(b.settingsPath, s); err != nil {
		fmt.Printf("Warning: could not save %s: %v\n", b.settingsPath, err)
	}
}

// handleLogoMenuResult dispatches the logo menu's three select-and-
// close choices (Fixed menu bar is a checkbox now, handled via
// Toggled in Update, not here) -- there being only three, and them
// never changing, a switch on the index is simpler than reusing the
// actions-table machinery every other menu goes through.
func (b *appMenuBar) handleLogoMenuResult(zx *ZenZX, result int) {
	switch result {
	case 1: // ZenZX website
		rl.OpenURL(zenZXHomepageURL)
	case 2: // Help
		b.activeModal = newMarkdownModal("HELP", helpText, false)
	case 3: // About...
		aboutText := strings.ReplaceAll(aboutTextTemplate, "__VERSION__", version)
		b.activeModal = newMarkdownModal("ABOUT", aboutText, true)
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

// openDiskDialog opens the "Open DSK Image..." file browser, gated on the
// current model actually having a floppy controller (zx.io.hasFDC tracks
// this live -- switchModelLive enables it only for plus3/spanishplus3, so
// checking it here is equivalent to checking the model directly, without
// this file needing its own copy of that model list).
func (b *appMenuBar) openDiskDialog(zx *ZenZX) {
	if !zx.io.hasFDC {
		fmt.Println("Open DSK Image: the current model has no floppy controller -- switch to +3 or Spanish +3 first (Machine menu)")
		return
	}
	b.diskDialog = zenui.NewDialog(zenui.DialogConfig{
		Mode:     zenui.ModeOpen,
		Title:    "Open DSK Image",
		StartDir: ".",
		Filters:  []string{"dsk"},
		FS:       zenui.OSFS{},
	})
}

// printDiskInfo reports the floppy controller's actual current state --
// replaces the old static "restart with -disk=..." message, which is
// outdated now that Open DSK Image loads a disk live.
func printDiskInfo(zx *ZenZX) {
	if !zx.io.hasFDC {
		fmt.Println("Disk: no floppy controller on this model (+3 and Spanish +3 only)")
		return
	}
	if zx.io.fdc == nil || zx.io.fdc.diskFilename == "" {
		fmt.Println("Disk: floppy controller present, no disk loaded")
		return
	}
	modified := ""
	if zx.io.fdc.diskModified {
		modified = " (modified, not yet saved)"
	}
	fmt.Printf("Disk: %s%s\n", zx.io.fdc.diskFilename, modified)
}

// refreshViewItems rebuilds the View menu's zoom items (X1/X2/X3) each
// frame the bar is shown -- Item.Disabled (which of these fit the
// current monitor, via maxMultiplierThatFits) has no pointer-based
// live-update mechanism the way Checked does, and the current scale
// itself can change outside the menu entirely (PgUp/PgDn), so
// rebuilding via SetItems is what keeps both correct. The FPS/Border/
// Show Status items ahead of these are untouched -- FPS/Border already
// stay correct via their own direct-pointer Checked wiring, needing no
// refresh at all.
//
// This is where a real MenuBar.SetItems bug was actually discovered:
// an earlier version recreated the open dropdown's Menu from scratch
// on every SetItems call, so calling this every frame made the whole
// View menu feel unresponsive (empty itemRects until the next Draw)
// the entire time it was open. Fixed in pkg/zenui/menubar.go itself.
func (b *appMenuBar) refreshViewItems(zx *ZenZX) {
	items := []zenui.Item{
		{Label: "FPS Counter", Checked: &zx.display.showFPS, Toggle: true},
		{Label: "Border Display", Checked: &zx.display.showBorder, Toggle: true},
		{Label: "Show Status"},
		{Separator: true},
	}
	limit := zx.display.maxMultiplierThatFits()
	for n := 1; n <= 3; n++ {
		checked := zx.display.screen.multiplier == n
		b.displayScaleChecked[n] = &checked
		items = append(items, zenui.Item{
			Label:    fmt.Sprintf("X %d", n),
			Checked:  b.displayScaleChecked[n],
			Disabled: n > limit,
		})
	}
	b.widget.SetItems(barView, items)
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
	if b.bankMode {
		maxBank := zx.maxROMBank()
		items = make([]zenui.Item, maxBank+1)
		for i := range items {
			items[i] = zenui.Item{Label: fmt.Sprintf("Bank %d", i)}
		}
	} else {
		b.romNames = listCustomROMs(b.romDir)
		if len(b.romNames) == 0 {
			items = []zenui.Item{{Label: "No custom ROMs found", Disabled: true}}
		} else {
			items = make([]zenui.Item, len(b.romNames))
			for i, n := range b.romNames {
				items[i] = zenui.Item{Label: n}
			}
		}
	}
	b.widget.SetItems(barCustomROM, items)
}

// Update advances the idle timer, the slide animation, and the widget
// itself, dispatching any accepted selection to the right action (or, for
// Custom ROM, advancing the two-step flow).
func (b *appMenuBar) Update(zx *ZenZX) {
	if b.activeModal != nil {
		if !b.activeModal.update(zenuiraylib.Input()) {
			b.activeModal = nil
		}
		return
	}

	if b.diskDialog != nil {
		switch b.diskDialog.Update(zenuiraylib.Input()) {
		case zenui.Accepted:
			path := b.diskDialog.Result()
			b.diskDialog = nil
			if err := zx.io.LoadDisk(path); err != nil {
				fmt.Printf("Open DSK Image: could not load %s: %v\n", path, err)
			} else {
				fmt.Printf("Loaded disk: %s\n", path)
			}
		case zenui.Cancelled:
			b.diskDialog = nil
		}
		return
	}

	if b.logoMenu != nil {
		switch b.logoMenu.Update(zenuiraylib.Input()) {
		case zenui.Accepted:
			result := b.logoMenu.Result()
			b.logoMenu = nil
			b.handleLogoMenuResult(zx, result)
		case zenui.Toggled:
			// "Fixed menu bar" is the only toggle item in this menu --
			// Item.Toggle already flipped b.fixed directly via its
			// own pointer; this supplies the window-resize side
			// effect, matching how the View menu's Border checkbox
			// works. The menu stays open (that's the point of a
			// checkbox), so no b.logoMenu = nil here.
			b.applyFixedState(zx)
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
	if b.state == barShown && !b.fixed && !b.widget.Active() && my > menuStripHeight+menuBarHideMargin {
		b.hide()
	}

	if b.state == barShown {
		b.refreshCustomROMItems(zx)
		b.refreshViewItems(zx)
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
	sel, ok := b.widget.Update(input)
	if !ok {
		if input.MousePressed {
			screenW := int(rl.GetScreenWidth())
			if b.logoHotZone(screenW).Contains(input.MouseX, input.MouseY) {
				b.openLogoMenu(screenW)
			}
		}
		return
	}

	if sel.BarIndex == barCustomROM {
		if b.bankMode {
			bank := sel.ItemIndex
			rom := b.pendingROM
			b.bankMode, b.pendingROM = false, ""
			applyCustomROMLive(zx, b.romDir, rom, bank)
			return
		}
		if sel.ItemIndex < 0 || sel.ItemIndex >= len(b.romNames) {
			return // the disabled "No custom ROMs found" placeholder
		}
		rom := b.romNames[sel.ItemIndex]
		if zx.maxROMBank() == 0 {
			applyCustomROMLive(zx, b.romDir, rom, 0)
			return
		}
		b.pendingROM = rom
		b.bankMode = true
		return
	}

	// Machine's model rows (Reset/Pause/Resume are index 0/1, handled
	// by the generic dispatch below same as always) start at index 2.
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
	if sel.BarIndex == barMachine && sel.ItemIndex >= 2 {
		var flag string
		if sub, ok := b.machineSubmenuFlags[sel.ItemIndex]; ok {
			if sel.SubIndex >= 0 && sel.SubIndex < len(sub) {
				flag = sub[sel.SubIndex]
			}
		} else if sel.ItemIndex < len(b.machineModelFlags) {
			flag = b.machineModelFlags[sel.ItemIndex]
		}
		if flag != "" {
			switchModelLive(zx, flag, b.romDir)
			b.setCurrentModel(flag)
		}
		return
	}

	// Theme's dropdown has one item with SubItems: "Font", holding
	// every font/zoom choice as a nested submenu (an earlier version
	// had Font as its own top-level bar label). Its own index within
	// Theme's items is len(zenui.Themes)+1 -- the themes, then a
	// separator, then Font -- computed the same way here as at
	// construction, rather than hardcoded twice.
	if sel.BarIndex == barTheme && sel.ItemIndex == len(zenui.Themes)+1 {
		if sel.SubIndex >= 0 && sel.SubIndex < len(b.fontActions) {
			b.fontActions[sel.SubIndex](zx)
		}
		return
	}

	if list, ok := b.actions[sel.BarIndex]; ok && sel.ItemIndex >= 0 && sel.ItemIndex < len(list) {
		list[sel.ItemIndex](zx)
	}
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
func (b *appMenuBar) Draw(screenW, screenH int) {
	if b.activeModal != nil {
		b.activeModal.draw(b.renderer, screenW, screenH, b.theme)
		return
	}
	if b.diskDialog != nil {
		b.diskDialog.Draw(b.renderer, screenW, screenH, b.theme)
		return
	}
	if b.logoMenu != nil {
		// The bar itself still needs to be drawn underneath (its own
		// hot zone is what the menu is anchored to), unlike diskDialog
		// and activeModal, which cover the whole screen with their own
		// backdrop.
		barY := int(float32(-menuStripHeight) * (1 - b.progress))
		b.widget.Draw(b.renderer, screenW, screenH, barY, menuStripHeight, b.theme)
		b.logoMenu.Draw(b.renderer, screenW, screenH, b.theme)
		return
	}
	if b.state == barHidden && !b.widget.Active() {
		return
	}
	barY := int(float32(-menuStripHeight) * (1 - b.progress))
	b.widget.Draw(b.renderer, screenW, screenH, barY, menuStripHeight, b.theme)
}
