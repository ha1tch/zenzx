//go:build !headless

package zenzx

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// ============================================================================
// Keyboard Mapping
// ============================================================================

// SpectrumKey represents a key position on the Spectrum keyboard matrix
type SpectrumKey struct {
	Row uint8
	Col uint8
}

// KeyMapping represents a mapping from PC key to Spectrum key(s)
type KeyMapping struct {
	Key  int32         // Raylib key code
	Keys []SpectrumKey // Spectrum keys to press (for combinations)
}

// ============================================================================
// Input Handler
// ============================================================================

// HandleInput processes all input for the emulator
func (zx *ZenZX) HandleInput() {
	// Escape is handled by the GUI's main loop (the -escape mode), not here, so
	// this path no longer closes the window on Escape.

	// Handle special Alt combinations first (system controls)
	if rl.IsKeyDown(rl.KeyLeftAlt) {
		zx.handleAltCombinations()
	}

	zx.handleJoystickInput()
	zx.handleTS2068JoystickInput()
	zx.handleMouseInput()

	// Process keyboard input
	zx.handleKeyboard()
	zx.cancelTypistOnKey()

	// Handle function keys
	zx.handleFunctionKeys()

	// Handle window scaling
	if rl.IsKeyPressed(rl.KeyPageUp) {
		zx.window().ScaleUp()
	}

	if rl.IsKeyPressed(rl.KeyPageDown) {
		zx.window().ScaleDown()
	}

	// Update border color in display manager
	zx.window().SetBorderColor(zx.io.borderColor)
}

// ============================================================================
// Joystick Input (joystick.go carries the mode-independent translation
// logic; this is purely the host-side raylib gamepad read)
// ============================================================================

// gamepadDirectionThreshold is the analog-stick deflection past which an
// axis counts as a digital direction press. 0.5 is a common convention
// (past the stick's centre dead zone, well short of full deflection) --
// not derived from any Spectrum-hardware source, since this threshold has
// no real-hardware equivalent.
const gamepadDirectionThreshold = 0.5

// handleJoystickInput polls gamepad 0 (the first connected controller, if
// any) and translates it into a JoystickState for whichever hardware
// mechanism -joystick configured. A no-op, with no raylib calls at all,
// when no mode is configured -- matches SetJoystickState's own no-op
// behaviour for JoystickNone (see joystick.go), kept as an explicit early
// return here too so an idle gamepad poll never runs when it can't matter.
func (zx *ZenZX) handleJoystickInput() {
	if zx.io.joystickMode == JoystickNone {
		return
	}
	if zx.io.joystickTwoPorts() {
		// Two real, simultaneous ports (the +2/+2A/+3's built-in Sinclair
		// pair, or a neo-Spectrum platform's configured dual Kempston):
		// gamepad 0 drives port 1, gamepad 1 (if present) drives port 2,
		// the same convention already established for TS2068's own two
		// built-in ports below.
		zx.io.SetJoystickStateBoth(readGamepadState(0), readGamepadState(1))
		return
	}
	zx.io.SetJoystickState(readGamepadState(0))
}

// readGamepadState polls one raylib gamepad slot (0-based) into a
// JoystickState. Shared by the canonical Spectrum's Kempston/Sinclair
// path (handleJoystickInput, always gamepad 0) and TS2068's own
// built-in dual joystick ports (handleTS2068JoystickInput, one call per
// port) -- same host-input semantics, different destinations.
func readGamepadState(gamepadIndex int32) JoystickState {
	var s JoystickState
	if !rl.IsGamepadAvailable(gamepadIndex) {
		// No gamepad connected: stays the zero value (nothing pressed),
		// same as if a real joystick were unplugged.
		return s
	}
	// D-pad face buttons and the left analog stick both count --
	// whichever the controller actually reports.
	s.Up = rl.IsGamepadButtonDown(gamepadIndex, rl.GamepadButtonLeftFaceUp) ||
		rl.GetGamepadAxisMovement(gamepadIndex, rl.GamepadAxisLeftY) < -gamepadDirectionThreshold
	s.Down = rl.IsGamepadButtonDown(gamepadIndex, rl.GamepadButtonLeftFaceDown) ||
		rl.GetGamepadAxisMovement(gamepadIndex, rl.GamepadAxisLeftY) > gamepadDirectionThreshold
	s.Left = rl.IsGamepadButtonDown(gamepadIndex, rl.GamepadButtonLeftFaceLeft) ||
		rl.GetGamepadAxisMovement(gamepadIndex, rl.GamepadAxisLeftX) < -gamepadDirectionThreshold
	s.Right = rl.IsGamepadButtonDown(gamepadIndex, rl.GamepadButtonLeftFaceRight) ||
		rl.GetGamepadAxisMovement(gamepadIndex, rl.GamepadAxisLeftX) > gamepadDirectionThreshold
	// Fire is the lower face button. The classic interfaces have only that one. A Next's
	// ports read more (joyports.go): the right face button is the second fire button (C on
	// an MD pad), the left one is A and the right middle button is START.
	s.Fire = rl.IsGamepadButtonDown(gamepadIndex, rl.GamepadButtonRightFaceDown)
	s.Fire2 = rl.IsGamepadButtonDown(gamepadIndex, rl.GamepadButtonRightFaceRight)
	s.A = rl.IsGamepadButtonDown(gamepadIndex, rl.GamepadButtonRightFaceLeft)
	s.Start = rl.IsGamepadButtonDown(gamepadIndex, rl.GamepadButtonMiddleRight)
	return s
}

// handleTS2068JoystickInput drives TS2068's own built-in joystick ports
// (Technical Manual 2.1.6.1/2.1.7, Table 2.4.4-1) -- always active for
// this model, independent of -joystick, since these are the machine's
// own hardware ports, not an optional add-on peripheral the way
// Kempston/Sinclair interfaces are for the canonical Spectrum. Gamepad
// 0 drives port 1, gamepad 1 (if present) drives port 2.
func (zx *ZenZX) handleTS2068JoystickInput() {
	if !zx.memory.isTS2068 {
		return
	}
	zx.io.SetTS2068JoystickState(1, readGamepadState(0))
	zx.io.SetTS2068JoystickState(2, readGamepadState(1))
}

// handleMouseInput polls raylib's mouse and translates it into a
// MouseState for whichever hardware mechanism -mouse configured. A no-op
// when no mode is configured, same pattern as handleJoystickInput.
//
// rl.GetMouseDelta() is already in host window-client pixels (raylib's
// own per-frame movement vector -- no OS-level or window-position
// translation needed, exactly matching what the person asked to have
// handled). Border margin does not enter into this at all: Kempston
// Mouse is a relative device (mouse.go), and a real physical mouse
// reports ball rotation unconditionally, with no notion of where a
// cursor is on screen -- there is nothing for border offset to correct
// for here. What does need correcting is magnification: the same
// physical mouse movement covers more host pixels at a higher window
// scale, so the raw delta is divided by the current multiplier before
// being handed to SetMouseState, which expects native-Spectrum-pixel
// units.
func (zx *ZenZX) handleMouseInput() {
	if zx.io.mouseMode == MouseNone {
		return
	}

	mult := float32(zx.screen.GetMultiplier())
	if mult < 1 {
		mult = 1
	}
	d := rl.GetMouseDelta()

	s := MouseState{
		DeltaX: d.X / mult,
		DeltaY: d.Y / mult,
		Left:   rl.IsMouseButtonDown(rl.MouseButtonLeft),
		Right:  rl.IsMouseButtonDown(rl.MouseButtonRight),
		Middle: rl.IsMouseButtonDown(rl.MouseButtonMiddle),
		Wheel:  rl.GetMouseWheelMove(),
	}
	zx.io.SetMouseState(s)
}

// handleAltCombinations processes Alt+key combinations for system controls
func (zx *ZenZX) handleAltCombinations() {
	if rl.IsKeyPressed(rl.KeyF) {
		zx.window().ToggleFPS()
	}

	if rl.IsKeyPressed(rl.KeyB) {
		zx.window().ToggleBorder()
	}

	// Alt+Enter is full screen: the chord a Windows program has for it, and the window managers of
	// macOS and Linux leave it alone. F11 is full screen too (HandleInput).
	if rl.IsKeyPressed(rl.KeyEnter) || rl.IsKeyPressed(rl.KeyKpEnter) {
		zx.window().ToggleFullscreen()
	}

	if rl.IsKeyPressed(rl.KeyT) {
		zx.ToggleTapeMode()
	}

	if rl.IsKeyPressed(rl.KeyP) {
		zx.PlayStopTape()
	}

	if rl.IsKeyPressed(rl.KeyR) {
		zx.RewindTape()
	}

	if rl.IsKeyPressed(rl.KeyI) {
		zx.ShowTapeInfo()
	}
}

// ToggleTapeMode switches between accurate and fast tape loading, a no-op
// if no tape is loaded. Extracted from handleAltCombinations (Alt+T) so
// the menu bar's Tape menu can call the identical logic rather than
// duplicate it.
func (zx *ZenZX) ToggleTapeMode() {
	if zx.tape == nil || zx.tape.st == nil || !zx.tape.st.Loaded {
		return
	}
	if zx.tape.st.Mode == TapeAccurate {
		zx.tape.SetMode(TapeFast)
		fmt.Println("Tape: Fast mode")
	} else {
		zx.tape.SetMode(TapeAccurate)
		fmt.Println("Tape: Accurate mode")
	}
}

// PlayStopTape toggles tape playback, a no-op if no tape is loaded.
// Extracted from handleAltCombinations (Alt+P); see ToggleTapeMode.
func (zx *ZenZX) PlayStopTape() {
	if zx.tape == nil || zx.tape.st == nil || !zx.tape.st.Loaded {
		return
	}
	if zx.tape.st.Playing {
		zx.tape.Stop()
		fmt.Println("Tape: Stopped")
	} else {
		zx.tape.Play()
		modeStr := "Fast"
		if zx.tape.st.Mode == TapeAccurate {
			modeStr = "Accurate"
		}
		fmt.Printf("Tape: Playing (%s mode)\n", modeStr)
	}
}

// RewindTape rewinds the loaded tape to its start, a no-op if none is
// loaded. Extracted from handleAltCombinations (Alt+R); see ToggleTapeMode.
func (zx *ZenZX) RewindTape() {
	if zx.tape == nil || zx.tape.st == nil || !zx.tape.st.Loaded {
		return
	}
	zx.tape.Rewind()
	fmt.Println("Tape: Rewound")
}

// ShowTapeInfo prints the tape's status and block list, a no-op if none is
// loaded. Extracted from handleAltCombinations (Alt+I); see
// ToggleTapeMode.
func (zx *ZenZX) ShowTapeInfo() {
	if zx.OnInfo != nil { // the window, as Tape > Tape Info: the deck says so when there is no tape
		zx.OnInfo("TAPE INFO", tapeInfoText(zx))
		return
	}
	if zx.tape == nil || zx.tape.st == nil || !zx.tape.st.Loaded {
		return
	}
	status := zx.tape.GetStatus()
	fmt.Printf("Tape: %s\n", status)

	blocks := zx.tape.GetBlockInfo()
	if len(blocks) > 0 {
		fmt.Println("Blocks:")
		for _, info := range blocks {
			fmt.Println(info)
		}
	}
}

// TogglePause flips the emulator's paused state. Extracted from
// handleFunctionKeys (F2) so the menu bar's Machine menu can call the
// identical logic rather than duplicate it.
func (zx *ZenZX) TogglePause() {
	zx.paused = !zx.paused
	if zx.paused {
		fmt.Println("Paused")
	} else {
		fmt.Println("Resumed")
	}
}

// handleKeyboard processes keyboard input and maps to Spectrum keys
func (zx *ZenZX) handleKeyboard() {
	// A script that is typing owns the matrix: clearing it here would take its keys away
	// before the machine saw them.
	if zx.scriptKeys {
		return
	}
	// Clear all keyboard state first
	for i := range zx.io.keyboard {
		zx.io.keyboard[i] = 0x1F // All keys released
	}

	// Define basic key mappings (direct 1:1 mappings)
	basicKeyMap := map[int32][2]uint8{
		// Letters - QWERTY layout
		rl.KeyA: {1, 0}, rl.KeyS: {1, 1}, rl.KeyD: {1, 2}, rl.KeyF: {1, 3}, rl.KeyG: {1, 4},
		rl.KeyQ: {2, 0}, rl.KeyW: {2, 1}, rl.KeyE: {2, 2}, rl.KeyR: {2, 3}, rl.KeyT: {2, 4},
		rl.KeyZ: {0, 1}, rl.KeyX: {0, 2}, rl.KeyC: {0, 3}, rl.KeyV: {0, 4},
		rl.KeyY: {5, 4}, rl.KeyU: {5, 3}, rl.KeyI: {5, 2}, rl.KeyO: {5, 1}, rl.KeyP: {5, 0},
		rl.KeyH: {6, 4}, rl.KeyJ: {6, 3}, rl.KeyK: {6, 2}, rl.KeyL: {6, 1},
		rl.KeyB: {7, 4}, rl.KeyN: {7, 3}, rl.KeyM: {7, 2},

		// Numbers
		rl.KeyOne: {3, 0}, rl.KeyTwo: {3, 1}, rl.KeyThree: {3, 2}, rl.KeyFour: {3, 3}, rl.KeyFive: {3, 4},
		rl.KeySix: {4, 4}, rl.KeySeven: {4, 3}, rl.KeyEight: {4, 2}, rl.KeyNine: {4, 1}, rl.KeyZero: {4, 0},

		// Special keys
		rl.KeySpace: {7, 0},
		rl.KeyEnter: {6, 0},
	}

	// Process combination keys (PC key -> multiple Spectrum keys)

	// Shift keys -> CAPS SHIFT
	if rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift) {
		zx.io.keyboard[0] &^= 0x01 // CAPS SHIFT (row 0, col 0)
	}

	// Control keys -> Symbol Shift
	if rl.IsKeyDown(rl.KeyLeftControl) || rl.IsKeyDown(rl.KeyRightControl) {
		zx.io.keyboard[7] &^= 0x02 // SYMBOL SHIFT (row 7, col 1)
	}

	// Tab = EXTEND MODE (CAPS SHIFT + SYMBOL SHIFT) - improved mapping
	if rl.IsKeyDown(rl.KeyTab) {
		zx.io.keyboard[0] &^= 0x01 // CAPS SHIFT
		zx.io.keyboard[7] &^= 0x02 // SYMBOL SHIFT
	}

	// Tilde/Backtick = EDIT (CAPS SHIFT + 1) - improved mapping
	if rl.IsKeyDown(rl.KeyGrave) {
		zx.io.keyboard[0] &^= 0x01 // CAPS SHIFT
		zx.io.keyboard[3] &^= 0x01 // 1
	}

	// Backspace = DELETE (CAPS SHIFT + 0)
	if rl.IsKeyDown(rl.KeyBackspace) {
		zx.io.keyboard[0] &^= 0x01 // CAPS SHIFT
		zx.io.keyboard[4] &^= 0x01 // 0
	}

	// Delete key also maps to DELETE (alternative)
	if rl.IsKeyDown(rl.KeyDelete) {
		zx.io.keyboard[0] &^= 0x01 // CAPS SHIFT
		zx.io.keyboard[4] &^= 0x01 // 0
	}

	// Caps Lock = CAPS LOCK (CAPS SHIFT + 2)
	if rl.IsKeyDown(rl.KeyCapsLock) {
		zx.io.keyboard[0] &^= 0x01 // CAPS SHIFT
		zx.io.keyboard[3] &^= 0x02 // 2
	}

	// Arrow keys with CAPS SHIFT
	if rl.IsKeyDown(rl.KeyLeft) {
		zx.io.keyboard[0] &^= 0x01 // CAPS SHIFT
		zx.io.keyboard[3] &^= 0x10 // 5
	}

	if rl.IsKeyDown(rl.KeyDown) {
		zx.io.keyboard[0] &^= 0x01 // CAPS SHIFT
		zx.io.keyboard[4] &^= 0x10 // 6
	}

	if rl.IsKeyDown(rl.KeyUp) {
		zx.io.keyboard[0] &^= 0x01 // CAPS SHIFT
		zx.io.keyboard[4] &^= 0x08 // 7
	}

	if rl.IsKeyDown(rl.KeyRight) {
		zx.io.keyboard[0] &^= 0x01 // CAPS SHIFT
		zx.io.keyboard[4] &^= 0x04 // 8
	}

	// Punctuation and symbols

	// Comma key
	if rl.IsKeyDown(rl.KeyComma) {
		zx.io.keyboard[7] &^= 0x02 // SYMBOL SHIFT
		zx.io.keyboard[7] &^= 0x08 // N
	}

	// Period key
	if rl.IsKeyDown(rl.KeyPeriod) {
		zx.io.keyboard[7] &^= 0x02 // SYMBOL SHIFT
		zx.io.keyboard[7] &^= 0x04 // M
	}

	// Semicolon -> SYMBOL SHIFT + O (;)
	if rl.IsKeyDown(rl.KeySemicolon) {
		zx.io.keyboard[7] &^= 0x02 // SYMBOL SHIFT
		zx.io.keyboard[5] &^= 0x02 // O
	}

	// Apostrophe -> SYMBOL SHIFT + 7 (')
	if rl.IsKeyDown(rl.KeyApostrophe) {
		zx.io.keyboard[7] &^= 0x02 // SYMBOL SHIFT
		zx.io.keyboard[4] &^= 0x08 // 7
	}

	// Slash -> SYMBOL SHIFT + V (/)
	if rl.IsKeyDown(rl.KeySlash) {
		zx.io.keyboard[7] &^= 0x02 // SYMBOL SHIFT
		zx.io.keyboard[0] &^= 0x10 // V
	}

	// Minus -> SYMBOL SHIFT + J (-)
	if rl.IsKeyDown(rl.KeyMinus) {
		zx.io.keyboard[7] &^= 0x02 // SYMBOL SHIFT
		zx.io.keyboard[6] &^= 0x08 // J
	}

	// Equal -> SYMBOL SHIFT + L (=)
	if rl.IsKeyDown(rl.KeyEqual) {
		zx.io.keyboard[7] &^= 0x02 // SYMBOL SHIFT
		zx.io.keyboard[6] &^= 0x02 // L
	}

	// Left bracket -> SYMBOL SHIFT + 8 ([)
	if rl.IsKeyDown(rl.KeyLeftBracket) {
		zx.io.keyboard[7] &^= 0x02 // SYMBOL SHIFT
		zx.io.keyboard[4] &^= 0x04 // 8
	}

	// Right bracket -> SYMBOL SHIFT + 9 (])
	if rl.IsKeyDown(rl.KeyRightBracket) {
		zx.io.keyboard[7] &^= 0x02 // SYMBOL SHIFT
		zx.io.keyboard[4] &^= 0x02 // 9
	}

	// Process basic keys (but don't override combinations)
	// Skip if Alt is held to prevent conflicts with Alt+key combinations
	if !rl.IsKeyDown(rl.KeyLeftAlt) {
		for key, pos := range basicKeyMap {
			if rl.IsKeyDown(key) {
				zx.io.keyboard[pos[0]] &^= (1 << pos[1])
			}
		}
	}
}

// handleFunctionKeys processes function key inputs
func (zx *ZenZX) handleFunctionKeys() {
	// F1 - Reset
	if rl.IsKeyPressed(rl.KeyF1) {
		zx.Reset()
		fmt.Println("Reset")
	}

	// F2 - Pause/Resume
	if rl.IsKeyPressed(rl.KeyF2) {
		zx.TogglePause()
	}

	// F3 - Diagnostics, in a window, as Snapshot > Run Diagnostics
	if rl.IsKeyPressed(rl.KeyF3) && zx.OnDiagnostics != nil {
		zx.OnDiagnostics()
	}

	// F4 - Save disk (if modified)
	if rl.IsKeyPressed(rl.KeyF4) {
		zx.handleSaveDisk()
	}

	// F5 - Insert blank disk
	if rl.IsKeyPressed(rl.KeyF5) {
		zx.handleInsertBlankDisk()
	}

	// F6 - Eject disk
	if rl.IsKeyPressed(rl.KeyF6) {
		zx.handleEjectDisk()
	}

	// F7 - Load disk info
	if rl.IsKeyPressed(rl.KeyF7) {
		zx.notice("To load a disk, drop a .dsk file on the window, or start with -disk=file.dsk")
	}

	// F8 - Save disk as new file
	if rl.IsKeyPressed(rl.KeyF8) {
		zx.handleSaveDiskAs()
	}

	// F9 - Quick save snapshot
	if rl.IsKeyPressed(rl.KeyF9) {
		zx.handleQuickSave()
	}

	// F10 - Quick load snapshot
	if rl.IsKeyPressed(rl.KeyF10) {
		zx.handleQuickLoad()
	}

	// F11 - Full screen on and off; Shift+F11 saves a timestamped snapshot, which F11 did until
	// 0.50.0 (the menu's Snapshot > Save Timestamped is the same action).
	if rl.IsKeyPressed(rl.KeyF11) {
		if rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift) {
			zx.handleAutoSave()
		} else {
			zx.window().ToggleFullscreen()
		}
	}

	// F12 - Load snapshot info / Run diagnostics with Shift
	if rl.IsKeyPressed(rl.KeyF12) {
		if rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift) {
			// Shift+F12 - the diagnostics, in a window as F3 and Snapshot > Run Diagnostics
			if zx.OnDiagnostics != nil {
				zx.OnDiagnostics()
			} else {
				fmt.Println("\nRunning snapshot diagnostics...")
				NewDebugSnapshot(zx).RunFullDiagnostics()
			}
		} else if zx.OnInfo != nil { // F12 - the snapshot info, as Snapshot > Snapshot Info
			zx.OnInfo("SNAPSHOT INFO", snapshotInfoText(zx))
		} else {
			fmt.Println("To load a snapshot, drop a .zxs, .sna, .z80 or .szx file on the window")
			fmt.Println("(Shift+F12 runs the diagnostics)")
		}
	}
}

// notice says what happened because of something the person did. The GUI has OnNotice, which
// shows it as a toast: many people use only the window and never see the console, so with a
// toast to show it the console is left alone. Without one (the headless build, a machine nobody
// has given a window) it goes to the console.
func (zx *ZenZX) notice(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	if zx.OnNotice != nil {
		zx.OnNotice(msg)
		return
	}
	fmt.Println(msg)
}

// Disk operation handlers
func (zx *ZenZX) handleSaveDisk() {
	if zx.io.hasFDC && zx.io.fdc != nil && zx.io.fdc.IsModified() {
		if zx.io.fdc.diskFilename != "" {
			if err := zx.io.SaveDisk(); err != nil {
				zx.notice("The disk could not be saved: %v", err)
			} else {
				zx.notice("The disk is saved")
			}
		} else {
			zx.notice("This is a blank disk: use Save Disk As to put it in a file")
		}
	} else {
		zx.notice("There is nothing to save: the disk has not changed")
	}
}

func (zx *ZenZX) handleInsertBlankDisk() {
	if zx.io.hasFDC && zx.io.fdc != nil {
		if zx.io.fdc.IsModified() && zx.io.fdc.diskFilename != "" {
			if err := zx.io.SaveDisk(); err != nil {
				zx.notice("The disk was not saved before the blank one went in: %v", err)
			}
		}

		zx.io.fdc.CreateBlankDisk()
		zx.notice("A blank formatted disk is in the drive: SAVE \"name\" puts a BASIC program on it, CAT lists it, F8 keeps it in a file")
	} else {
		zx.notice("This model has no disk drive")
	}
}

func (zx *ZenZX) handleEjectDisk() {
	if zx.io.hasFDC && zx.io.fdc != nil {
		if zx.io.fdc.HasDisk() {
			zx.io.fdc.EjectDisk()
			zx.notice("The disk is ejected")
		} else {
			zx.notice("There is no disk in the drive")
		}
	} else {
		zx.notice("This model has no disk drive")
	}
}

func (zx *ZenZX) handleSaveDiskAs() {
	if zx.io.hasFDC && zx.io.fdc != nil && zx.io.fdc.HasDisk() {
		timestamp := time.Now().Format("20060102_150405")
		filename := fmt.Sprintf("disk_%s.dsk", timestamp)

		if err := zx.io.SaveDiskAs(filename); err != nil {
			zx.notice("The disk could not be saved: %v", err)
		} else {
			zx.notice("The disk is saved as %s", filename)
		}
	} else {
		zx.notice("There is no disk in the drive")
	}
}

// snapshotStartsPlaying says whether a snapshot that has just been loaded runs at once
// (the Preferences setting) rather than waiting paused. The GUI sets it; it is nil, and a
// loaded snapshot waits, everywhere else.
var snapshotStartsPlaying func() bool

// tapeAutoloadWanted says whether a tape that has just been loaded starts by itself (the
// Preferences setting). The GUI sets it; it is nil, and a tape only waits in the deck, everywhere
// else.
var tapeAutoloadWanted func() bool

// snapshotStartWord is the settings file's word for the choice.
func snapshotStartWord(plays bool) string {
	if plays {
		return "play"
	}
	return "paused"
}

// afterSnapshotLoad applies the Preferences choice to a snapshot that loaded: it waits
// paused unless the person has asked for it to play. (A snapshot is always saved with the
// machine paused, so this is the one place the choice is made.)
func (zx *ZenZX) afterSnapshotLoad() {
	zx.paused = snapshotStartsPlaying == nil || !snapshotStartsPlaying()
}

// loadSnapshotHere loads a snapshot into this machine, which waits paused or plays as Preferences
// says a loaded snapshot does; with wait it waits whatever Preferences say, as it always has after
// Quick Load. It is what loads one when there is no window to make the machine it is of (the
// headless program, a script), and what the window does once it has made it.
func (zx *ZenZX) loadSnapshotHere(file, format string, wait bool) error {
	zx.paused = true
	if err := zx.LoadSnapshotFormat(file, format); err != nil {
		zx.paused = false
		return err
	}
	if !wait {
		zx.afterSnapshotLoad()
	}
	return nil
}

// openSnapshot opens a snapshot as the host does (OpenSnapshot), else in this machine.
func (zx *ZenZX) openSnapshot(file, format string, wait bool) error {
	if zx.OpenSnapshot != nil {
		return zx.OpenSnapshot(file, format, wait)
	}
	return zx.loadSnapshotHere(file, format, wait)
}

// loadDroppedFile loads one dropped file by its extension into the machine
// directly -- a snapshot replaces state, a tape/disk is inserted, a .scr paints
// the screen. This is the "load into the machine" path; the mount drawer's
// "put the file into a slot's filesystem" path is separate.
//
// It returns a human-readable sentence ("The tape foo.tap is loaded and
// rewound ...", "bar.dsk could not be loaded as a disk") so the caller can
// surface it as an on-screen toast; the terse stdout prints are kept for the
// headless/CLI path. An empty string means the extension was not recognised
// and nothing was attempted.
func (zx *ZenZX) loadDroppedFile(file string) string {
	if len(file) <= 4 {
		return ""
	}
	base := filepath.Base(file)
	if zx.stopped && !isSnapshotExt(file) && !isMultiSnapshotPath(file) {
		// no machine is running: a snapshot makes its own, and a multisnapshot its machines; nothing
		// else has a machine to go into
		return "No machine is running: " + base + " has nothing to be loaded into"
	}
	switch strings.ToLower(file[len(file)-4:]) {
	case ".scr":
		if err := zx.screen.LoadFromFile(file); err != nil {
			fmt.Printf("Error loading screen %s: %v\n", file, err)
			return base + " could not be loaded as a screen"
		}
		fmt.Printf("Loaded screen: %s\n", file)
		return "The screen " + base + " is now on display"

	case ".zxs":
		if err := zx.openSnapshot(file, "zxs", false); err != nil {
			if errors.Is(err, errCardsAsked) {
				return "" // the window asks for the cards, and says what came of it
			}
			fmt.Printf("Error loading snapshot %s: %v\n", file, err)
			return snapshotRefusal(base, err)
		}
		fmt.Printf("Loaded snapshot: %s\n", file)
		return "The snapshot " + base + " has been restored"

	case ".zms":
		if zx.OpenMultisnapshot == nil {
			return base + " is a multisnapshot, which only the window loads"
		}
		zx.OpenMultisnapshot(file)
		return "" // the window asks first and says what came of it

	case ".sna", ".z80", ".szx":
		format := strings.ToLower(strings.TrimPrefix(file[len(file)-4:], "."))
		if err := zx.openSnapshot(file, format, false); err != nil {
			fmt.Printf("Error loading snapshot %s: %v\n", file, err)
			return snapshotRefusal(base, err)
		}
		fmt.Printf("Loaded %s snapshot: %s\n", strings.ToUpper(format), file)
		return "The " + strings.ToUpper(format) + " snapshot " + base + " has been restored"

	case ".rzx":
		if err := zx.LoadRZX(file); err != nil {
			fmt.Printf("Error loading recording %s: %v\n", file, err)
			return base + " could not be played: " + err.Error()
		}
		fmt.Printf("Loaded RZX recording: %s\n", file)
		zx.afterSnapshotLoad()
		_, _, total := zx.RZXPlaying()
		return fmt.Sprintf("The recording %s is ready: %d frames, about %d seconds", base, total, total/50)

	case ".dsk":
		if zx.io.hasFDC && zx.io.fdc != nil {
			if err := zx.io.LoadDisk(file); err != nil {
				fmt.Printf("Error loading disk %s: %v\n", file, err)
				return base + " could not be loaded as a disk"
			}
			fmt.Printf("Loaded disk: %s\n", file)
			return "The disk " + base + " has been inserted into drive A"
		}
		fmt.Println("Disk loading requires +3 mode with FDC enabled")
		return "Disks need a +3 with its drive enabled"

	case ".tap", ".tzx":
		if zx.tape == nil {
			zx.tape = NewTape(zx)
			zx.tape.AttachFastLoader(&FastLoader{Enabled: true})
		}
		if err := zx.tape.LoadFile(file); err != nil {
			fmt.Printf("Error loading tape %s: %v\n", file, err)
			return base + " could not be loaded as a tape"
		}
		fmt.Printf("Loaded tape: %s (%d blocks)\n", file, len(zx.tape.st.Blocks))
		zx.tape.Rewind()
		if tapeInsertedHook != nil {
			tapeInsertedHook(file) // the Recent Tapes keep it (tapedir.go)
		}
		return zx.TapeInsertedNote(base, len(zx.tape.st.Blocks), tapeAutoloadWanted != nil && tapeAutoloadWanted())

	case ".nex":
		// A NEX is a Next executable, so on a classic model there is nothing to
		// load -- it is not applicable. On a Next that boots NextZXOS it is run by
		// NextZXOS's own .nexload, started from the menus for the person (nexrun.go).
		// When the Next is busy the person is asked before it is reset; a host that
		// cannot ask leaves the file alone.
		msg, needsReset := zx.RunNEX(file, false)
		if needsReset {
			if zx.NEXAskReset != nil {
				zx.NEXAskReset(file)
				return ""
			}
			fmt.Printf("NEX not run, the Next is busy: %s\n", file)
		}
		return msg
	}
	return ""
}

// ============================================================================
// Keyboard Helper Functions
// ============================================================================

// PressKey simulates a key press at the given matrix position
func (zx *ZenZX) PressKey(row, col uint8) {
	zx.io.PressKey(row, col)
}

// ReleaseKey simulates a key release at the given matrix position
func (zx *ZenZX) ReleaseKey(row, col uint8) {
	zx.io.ReleaseKey(row, col)
}

// TypeText simulates typing a string of text
// This is useful for auto-typing commands
func (zx *ZenZX) TypeText(text string) {
	// This would need to be implemented with proper timing
	// to simulate actual typing with key press/release delays
	// For now, this is a placeholder
	fmt.Printf("TypeText not yet implemented: %s\n", text)
}

// ============================================================================
// Additional Keyboard Mappings Reference
// ============================================================================

// Spectrum keyboard matrix reference:
// Row 0: CAPS SHIFT, Z, X, C, V
// Row 1: A, S, D, F, G
// Row 2: Q, W, E, R, T
// Row 3: 1, 2, 3, 4, 5
// Row 4: 0, 9, 8, 7, 6
// Row 5: P, O, I, U, Y
// Row 6: ENTER, L, K, J, H
// Row 7: SPACE, SYMBOL SHIFT, M, N, B

// Common key combinations:
// DELETE = CAPS SHIFT + 0
// EDIT = CAPS SHIFT + 1
// CAPS LOCK = CAPS SHIFT + 2
// TRUE VIDEO = CAPS SHIFT + 3
// INV VIDEO = CAPS SHIFT + 4
// Cursor Left = CAPS SHIFT + 5
// Cursor Down = CAPS SHIFT + 6
// Cursor Up = CAPS SHIFT + 7
// Cursor Right = CAPS SHIFT + 8
// GRAPHICS = CAPS SHIFT + 9
// EXTEND MODE = CAPS SHIFT + SYMBOL SHIFT
// BREAK = CAPS SHIFT + SPACE

// Snapshot operation handlers
func (zx *ZenZX) handleQuickSave() {
	wasPaused := zx.paused
	zx.paused = true

	// Determine format and extension
	format := zx.snapshotFormat
	if format == "auto" {
		format = "zxs" // Default for quick save
	}

	filename := snapshotPath(quickSaveName(format))
	if err := ensureSnapshotDir(); err != nil {
		zx.notice("Quick save failed: %v", err)
		zx.paused = wasPaused
		return
	}

	if err := zx.SaveSnapshotFormat(filename, format); err != nil {
		zx.notice("Quick save failed: %v", err)
	} else {
		zx.notice("Quick saved to %s (%s format)", filename, strings.ToUpper(format))
	}

	zx.paused = wasPaused
}

func (zx *ZenZX) handleQuickLoad() {
	// (openSnapshot pauses the machine it loads into, and the host makes the machine the
	// snapshot is of when there is not one: snapshotopen_gui.go.)

	// Try different formats
	formats := []string{"zxs", "sna", "z80"}
	if zx.snapshotFormat != "auto" && zx.snapshotFormat != "" {
		// Try preferred format first
		formats = append([]string{zx.snapshotFormat}, formats...)
	}

	loaded := false
	for _, format := range formats {
		filename := snapshotPath(quickSaveName(format))
		if _, err := os.Stat(filename); err == nil {
			err := zx.openSnapshot(filename, format, true)
			if err == nil {
				zx.notice("Quick loaded from %s (%s format)", filename, strings.ToUpper(format))
				loaded = true
				break
			}
			if m, ok := snapshotMismatchOf(err); ok {
				// the host makes the machine a snapshot is of; where it cannot, say why
				zx.notice("The quick save is a %s snapshot and this machine is a %s", m.Snapshot, m.Machine)
				return
			}
		}
	}

	if !loaded {
		zx.notice("No quick save found (tried .zxs, .sna, .z80)")
	}
	// The machine waits paused, as it always has after Quick Load: Play or F2 starts it.
}

func (zx *ZenZX) handleAutoSave() {
	wasPaused := zx.paused
	zx.paused = true

	// Determine format and extension
	format := zx.snapshotFormat
	if format == "auto" {
		format = "zxs" // Default for auto save
	}

	filename := snapshotPath(timestampedName(time.Now(), format))
	if err := ensureSnapshotDir(); err != nil {
		zx.notice("Save snapshot failed: %v", err)
		zx.paused = wasPaused
		return
	}

	if err := zx.SaveSnapshotFormat(filename, format); err != nil {
		zx.notice("Save snapshot failed: %v", err)
	} else {
		zx.notice("Saved snapshot: %s (%s format)", filename, strings.ToUpper(format))
	}

	zx.paused = wasPaused
}
