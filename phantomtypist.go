// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"bytes"
	"fmt"

	"github.com/ha1tch/zentools/pkg/tap"
)

// The phantom typist starts a tape the way a person would: the machine is reset, and when its ROM
// is asking for keys the keys that load from tape are pressed for it, on the emulated keyboard.
// It is the design of the phantom typist in FUSE, which types after a reset where other emulators
// load a prepared snapshot or watch for ROM addresses:
//
//   - It is ready when the ROM has read all eight half-rows of the keyboard within one frame. That
//     is a ROM at a prompt or a menu, whatever the ROM is, so no address and no measured boot time
//     is needed.
//   - Its keys are held in a layer of their own over the keyboard (SpectrumIO.typistKeys), and a key
//     is held until the ROM has read the ULA port with it down, so a key is never missed.
//   - What it types depends on the machine: a 48K or Timex goes through keyword entry (J, then
//     SYMBOL SHIFT+P twice, ENTER: LOAD ""); the 128K and +2 menu has Tape Loader highlighted, so
//     ENTER is all it takes; the +2A and +3 want a longer wait first (the +3 looks for a disk).
//   - A tape whose first block is a CODE header is loaded with LOAD ""CODE, which on the 128K family
//     goes through 128 BASIC, and on the +3 first chooses the tape device with LOAD "t:".
//   - While it types, the machine runs ahead of the clock (zx.typistRunAhead) and is silent, and
//     the tape starts playing when its last ENTER has been pressed.
//
// A Next boots NextZXOS and a machine with a custom ROM may not have the keys these use, so
// neither autoloads, as in FUSE.

// typistMode is what is typed: the strings of keys the typist has.
type typistMode int

const (
	typistModeJPP                typistMode = iota // J, SYMBOL SHIFT+P, SYMBOL SHIFT+P, ENTER: LOAD ""
	typistModeJPPI                                 // ... and EXTEND MODE, I, ENTER: LOAD ""CODE
	typistModeEnter                                // ENTER only
	typistModeDownLoadPPCode                       // Down, ENTER, L O A D " " C O D E: LOAD ""CODE in 128 BASIC
	typistModeWaitDownLoadPPCode                   // the same after a wait (the +2A)
	typistModePlus3CodeBlock                       // LOAD "t:" and then LOAD ""CODE in BASIC
	typistModeLoadPP                               // L O A D " ", ENTER: LOAD "" by single letters
	typistModeLoadPPCode                           // L O A D " " C O D E, ENTER
)

// typistState is where the typist is: which keys it holds, which it will hold next.
type typistState int

const (
	tsInactive       typistState = iota
	tsWaiting                    // for the ROM to read the whole keyboard
	tsLoad                       // J
	tsQuote1                     // SYMBOL SHIFT+P, the first "
	tsQuote2                     // SYMBOL SHIFT+P, the second "
	tsEnter                      // ENTER, then the next command or the end
	tsEnterOnly                  // ENTER by itself (a menu)
	tsWaitAfterEnter             // the pause in which the +3 looks for a disk
	tsLoadL                      // L
	tsLoadO                      // O
	tsLoadA                      // A
	tsLoadD                      // D
	tsExtendedMode               // CAPS SHIFT+SYMBOL SHIFT
	tsCode                       // I, which is CODE in extend mode
	tsCodeC                      // C
	tsCodeO                      // O
	tsCodeD                      // D
	tsCodeE                      // E
	tsLongWaitDown               // a long wait before the 128 menu's Down
	tsWaitDown                   // a wait before it
	tsDown                       // CAPS SHIFT+6: Down
	tsSelectBasic                // ENTER on 128 BASIC
	tsWait                       // the pause while 128 BASIC starts
	tsTColonT                    // T
	tsTColonColon                // SYMBOL SHIFT+Z: the colon
	tsSlowLoadL                  // L, with the time the +3 needs after its first command
	tsSlowLoadO                  // O, likewise
	tsSettle                     // a pause for a ROM that is not at its prompt yet when it first reads the keyboard
	tsStates
)

// typistStateInfo is what a state holds, and where it goes when the ROM has seen the keys.
type typistStateInfo struct {
	keys  []matrixPos
	next  typistState
	delay int // frames to wait, in the state, before its keys are pressed
}

// The keys the table uses.
var (
	tkJ     = matrixPos{6, 3}
	tkP     = matrixPos{5, 0}
	tkEnter = matrixPos{6, 0}
	tkL     = matrixPos{6, 1}
	tkO     = matrixPos{5, 1}
	tkA     = matrixPos{1, 0}
	tkD     = matrixPos{1, 2}
	tkI     = matrixPos{5, 2}
	tkC     = matrixPos{0, 3}
	tkE     = matrixPos{2, 2}
	tk6     = matrixPos{4, 4}
	tkT     = matrixPos{2, 4}
	tkZ     = matrixPos{0, 1}
)

// typistTable is the typist's states. A state with a next of tsInactive and a function in
// typistChoice (QUOTE1, QUOTE2 and ENTER) chooses it by the mode instead.
var typistTable = [tsStates]typistStateInfo{
	tsLoad:           {[]matrixPos{tkJ}, tsQuote1, 8},
	tsQuote1:         {[]matrixPos{symShift, tkP}, tsInactive, 0},
	tsQuote2:         {[]matrixPos{symShift, tkP}, tsInactive, 5},
	tsEnter:          {[]matrixPos{tkEnter}, tsInactive, 0},
	tsEnterOnly:      {[]matrixPos{tkEnter}, tsWaitAfterEnter, 3},
	tsWaitAfterEnter: {nil, tsInactive, 600},
	tsLoadL:          {[]matrixPos{tkL}, tsLoadO, 2},
	tsLoadO:          {[]matrixPos{tkO}, tsLoadA, 0},
	tsLoadA:          {[]matrixPos{tkA}, tsLoadD, 5},
	tsLoadD:          {[]matrixPos{tkD}, tsQuote1, 4},
	tsExtendedMode:   {[]matrixPos{capsShift, symShift}, tsCode, 0},
	tsCode:           {[]matrixPos{tkI}, tsEnter, 4},
	tsCodeC:          {[]matrixPos{tkC}, tsCodeO, 0},
	tsCodeO:          {[]matrixPos{tkO}, tsCodeD, 4},
	tsCodeD:          {[]matrixPos{tkD}, tsCodeE, 0},
	tsCodeE:          {[]matrixPos{tkE}, tsEnter, 4},
	tsLongWaitDown:   {nil, tsWaitDown, 50},
	tsWaitDown:       {nil, tsDown, 24},
	tsDown:           {[]matrixPos{capsShift, tk6}, tsSelectBasic, 4},
	tsSelectBasic:    {[]matrixPos{tkEnter}, tsWait, 10},
	tsWait:           {nil, tsLoadL, 28},
	tsTColonT:        {[]matrixPos{tkT}, tsTColonColon, 4},
	tsTColonColon:    {[]matrixPos{symShift, tkZ}, tsQuote2, 4},
	tsSlowLoadL:      {[]matrixPos{tkL}, tsSlowLoadO, 5},
	tsSlowLoadO:      {[]matrixPos{tkO}, tsLoadA, 30},
	tsSettle:         {nil, tsLoadL, 30},
}

// phantomTypist is one machine's typist.
type phantomTypist struct {
	zx       *ZenZX
	mode     typistMode
	state    typistState
	next     typistState
	delay    int
	rowsRead uint8 // the half-rows the ROM has read this frame, a bit for each
	commands int   // ENTERs pressed so far, for the +3's two commands
}

func newPhantomTypist(zx *ZenZX) *phantomTypist { return &phantomTypist{zx: zx} }

// Active says whether the typist has yet to finish.
func (p *phantomTypist) active() bool { return p != nil && p.state != tsInactive }

// start has the typist wait for the ROM to ask for keys, and then type what the mode says.
func (p *phantomTypist) start(mode typistMode) {
	p.mode = mode
	p.commands = 0
	p.delay = 0
	p.rowsRead = 0
	p.state, p.next = tsWaiting, tsWaiting
	p.releaseKeys()
	p.zx.typistBegan()
}

// stop gives the typing up at once: a reset, or a key of the person's.
func (p *phantomTypist) stop() {
	if p == nil || p.state == tsInactive {
		return
	}
	p.state, p.next = tsInactive, tsInactive
	p.releaseKeys()
	p.zx.typistEnded()
}

func (p *phantomTypist) releaseKeys() {
	if p.zx.io != nil {
		p.zx.io.typistKeys = [8]uint8{}
	}
}

// updateKeys holds the keys of the state, once its delay is over.
func (p *phantomTypist) updateKeys() {
	p.releaseKeys()
	if p.delay != 0 || p.state == tsInactive || p.state == tsWaiting || p.zx.io == nil {
		return
	}
	for _, k := range typistTable[p.state].keys {
		p.zx.io.typistKeys[k.row] |= 1 << k.col
	}
}

// ulaRead is told of every read of the ULA's port, which is how the typist knows that the ROM is
// looking at the keyboard: the whole of it, to be ready; and then with the typist's keys down.
func (p *phantomTypist) ulaRead(port uint16) {
	if p.delay != 0 {
		return
	}
	switch p.state {
	case tsInactive:
	case tsWaiting:
		switch high := uint8(port >> 8); high {
		case 0xFE, 0xFD, 0xFB, 0xF7, 0xEF, 0xDF, 0xBF, 0x7F:
			p.rowsRead |= ^high
		}
		if p.rowsRead == 0xFF {
			p.next = p.firstState()
		}
	default:
		p.next = p.after()
	}
}

// firstState is where each mode begins, once the ROM is ready.
func (p *phantomTypist) firstState() typistState {
	switch p.mode {
	case typistModeJPP, typistModeJPPI:
		return tsLoad
	case typistModeEnter:
		return tsEnterOnly
	case typistModeDownLoadPPCode:
		return tsDown
	case typistModeWaitDownLoadPPCode:
		return tsWaitDown
	case typistModePlus3CodeBlock:
		return tsLongWaitDown
	case typistModeLoadPP, typistModeLoadPPCode:
		return tsSettle
	}
	return tsInactive
}

// after is the state that follows this one.
func (p *phantomTypist) after() typistState {
	switch p.state {
	case tsQuote1:
		// the first command of the +3's two chooses the tape device: LOAD "t:"
		if p.mode == typistModePlus3CodeBlock && p.commands == 0 {
			return tsTColonT
		}
		return tsQuote2
	case tsQuote2:
		switch p.mode {
		case typistModeJPP, typistModeLoadPP:
			return tsEnter
		case typistModePlus3CodeBlock:
			if p.commands == 0 {
				return tsEnter
			}
			return tsCodeC
		case typistModeJPPI:
			return tsExtendedMode
		case typistModeLoadPPCode, typistModeDownLoadPPCode, typistModeWaitDownLoadPPCode:
			return tsCodeC
		}
		return tsInactive
	case tsEnter:
		if p.mode == typistModePlus3CodeBlock && p.commands == 0 {
			return tsSlowLoadL
		}
		return tsInactive
	}
	return typistTable[p.state].next
}

// frame is the end of a frame: what the ROM saw in it is acted on, and the keys of the next state
// are put down.
func (p *phantomTypist) frame() {
	p.rowsRead = 0
	if p.next != p.state {
		leaving := p.state
		if p.mode == typistModePlus3CodeBlock && leaving == tsEnter {
			p.commands++
		}
		p.state = p.next
		p.delay = typistTable[p.state].delay
		// The last ENTER has been pressed: the tape can play. The +3's first ENTER only chose the tape.
		if (leaving == tsEnter || leaving == tsEnterOnly) && (p.mode != typistModePlus3CodeBlock || p.commands == 2) {
			p.zx.typistEntered()
		}
		if p.state == tsInactive {
			p.zx.typistEnded()
		}
	}
	if p.delay > 0 {
		p.delay--
	}
	p.updateKeys()
}

// The machine's side of the typist.

// typistFrame is called at the end of each frame the machine runs.
func (zx *ZenZX) typistFrame() {
	if zx.typist.active() {
		zx.typist.frame()
	}
}

// TypistActive says whether a tape is being started by the typist: the machine runs ahead and is
// silent, and a person's keys are not wanted.
func (zx *ZenZX) TypistActive() bool { return zx.typist.active() }

// cancelTypistOnKey gives the typing up when the person presses a key of their own: they have
// taken the keyboard, and the typist's keys would mix with theirs.
func (zx *ZenZX) cancelTypistOnKey() {
	if !zx.TypistActive() || zx.io == nil {
		return
	}
	for _, row := range zx.io.keyboard {
		if row&0x1F != 0x1F {
			zx.typist.stop()
			return
		}
	}
}

// CancelTypist gives up a tape's autoload that is under way.
func (zx *ZenZX) CancelTypist() { zx.typist.stop() }

// defaultTypistRunAhead is the run-ahead a machine that has none of its own is given when it starts
// a tape: 0 (the clock exact) unless a front end that paces the emulation sets it.
var defaultTypistRunAhead int

// typistRunAhead is how many frames the host runs for each one of its own while the typist works
// (the host paces the emulation at 50 a second; a front end that wants the clock exact leaves it 0).
func (zx *ZenZX) typistRunAheadFrames() int {
	if !zx.typist.active() {
		return 0
	}
	return zx.typistRunAhead
}

// typistBegan silences the machine while the typist types, as running ahead of the clock would
// turn its sound into noise; typistEnded gives the sound back if it was this that took it.
func (zx *ZenZX) typistBegan() {
	if zx.typistRunAhead > 0 && zx.audio != nil && zx.audio.IsEnabled() && !zx.typistMuted {
		zx.audio.SetEnabled(false)
		zx.typistMuted = true
	}
}

func (zx *ZenZX) typistEnded() {
	if zx.typistMuted && zx.audio != nil {
		zx.audio.SetEnabled(true)
	}
	zx.typistMuted = false
}

// typistEntered is the typist's last ENTER: the tape, which a reset stopped, is rewound and played.
func (zx *ZenZX) typistEntered() {
	if zx.tape == nil || zx.tape.st == nil || !zx.tape.st.Loaded {
		return
	}
	zx.tape.Rewind()
	zx.tape.Play()
}

// tapeStartsWithCode says whether the tape's first block is the header of a CODE block, which
// LOAD "" does not take: a tape that starts with code is loaded with LOAD ""CODE.
func (zx *ZenZX) tapeStartsWithCode() bool {
	if zx.tape == nil || zx.tape.st == nil {
		return false
	}
	for _, b := range zx.tape.st.Blocks {
		if len(b.Data) == 0 {
			continue
		}
		return b.IsHeader && b.HeaderType == tap.TypeCode
	}
	return false
}

// typistKind is how the machine is told to load: what it types. A Next is left to NextZXOS, and
// a machine with a ROM of its own is left alone, as its keys may be anything.
func (zx *ZenZX) typistMachineMode(needsCode bool) (typistMode, bool) {
	if zx.io == nil || zx.memory == nil || zx.memory.isNext || zx.customROM {
		return 0, false
	}
	switch {
	case zx.memory.isTS2068:
		return keywordMode(needsCode), true
	case zx.memory.isPlus3 && zx.io.hasFDC: // the +3
		if needsCode {
			return typistModePlus3CodeBlock, true
		}
		return typistModeEnter, true
	case zx.memory.isPlus3: // the +2A and +2B, which have the +3's ROM and no disk drive
		if needsCode {
			return typistModeWaitDownLoadPPCode, true
		}
		return typistModeEnter, true
	case zx.memory.is128K && zx.isInvestronica128(): // no menu: it starts in BASIC, which takes its words letter by letter
		if needsCode {
			return typistModeLoadPPCode, true
		}
		return typistModeLoadPP, true
	case zx.memory.is128K: // the 128K and the +2: the menu
		if needsCode {
			return typistModeDownLoadPPCode, true
		}
		return typistModeEnter, true
	}
	return keywordMode(needsCode), true
}

// isInvestronica128 says whether the machine has the Spanish 128K's ROM of 1985, which has no menu
// and no keyword entry: it starts with a BASIC that is typed a letter at a time.
func (zx *ZenZX) isInvestronica128() bool {
	spanish, err := embeddedROMs.ReadFile("rom/128-spanish-0.rom")
	return err == nil && bytes.Equal(spanish, zx.memory.rom[0][:])
}

func keywordMode(needsCode bool) typistMode {
	if needsCode {
		return typistModeJPPI
	}
	return typistModeJPP
}

// tapeLoadHint is what a person does by hand to load the tape in the deck: it is said when the
// machine does not start the tape by itself. A Next has NextZXOS and nothing is said.
func (zx *ZenZX) tapeLoadHint() string {
	switch {
	case zx.memory == nil || zx.memory.isNext:
		return ""
	case zx.customROM:
		return "load it with that ROM's own command"
	case zx.memory.is128K && !zx.memory.isTS2068 && !zx.isInvestronica128():
		return "choose Tape Loader, then press Play"
	}
	return "type LOAD \"\" and press Play"
}

// TapeInsertedNote is the toast for a tape that has just been put in the deck (name is its file's
// name, n its blocks). With autoload set it starts the tape by itself where the machine can, and
// says so; otherwise it says what to do by hand.
func (zx *ZenZX) TapeInsertedNote(name string, n int, autoload bool) string {
	blocks := "blocks"
	if n == 1 {
		blocks = "block"
	}
	if autoload && zx.AutoloadTape() {
		zx.paused = false // a tape that loads by itself needs a machine that runs
		return fmt.Sprintf("The tape %s (%d %s) is loading by itself", name, n, blocks)
	}
	note := fmt.Sprintf("The tape %s is loaded and rewound (%d %s)", name, n, blocks)
	if hint := zx.tapeLoadHint(); hint != "" {
		note += ": " + hint
	}
	return note
}

// TapeAutoloads says whether this machine can start the tape in its deck by itself.
func (zx *ZenZX) TapeAutoloads() bool {
	if zx.tape == nil || zx.tape.st == nil || !zx.tape.st.Loaded || zx.stopped {
		return false
	}
	_, ok := zx.typistMachineMode(zx.tapeStartsWithCode())
	return ok
}

// AutoloadTape resets the machine and has the typist load the tape in its deck: the tape is
// rewound, and plays when the last key has been pressed. It says whether it could; when it could
// not (a Next, a custom ROM, no tape) nothing was changed.
func (zx *ZenZX) AutoloadTape() bool {
	if zx.tape == nil || zx.tape.st == nil || !zx.tape.st.Loaded || zx.stopped {
		return false
	}
	mode, ok := zx.typistMachineMode(zx.tapeStartsWithCode())
	if !ok {
		return false
	}
	zx.Reset() // which gives up a typist that was typing
	zx.tape.Rewind()
	if zx.typist == nil {
		zx.typist = newPhantomTypist(zx)
	}
	zx.io.typist = zx.typist
	if zx.typistRunAhead == 0 {
		zx.typistRunAhead = defaultTypistRunAhead
	}
	zx.typist.start(mode)
	return true
}
