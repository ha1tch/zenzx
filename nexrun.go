// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// Running a .nex on a Next that boots NextZXOS.
//
// A NEX is a Next program, and the only thing in the machine that loads one is NextZXOS's
// own .nexload. So running one is what a person does at the keyboard, done for them, taking
// the least invasive way that works:
//
//  1. The Next is at NextZXOS's welcome screen, menu or Browser. Nothing is reset: the file
//     is put on a card the machine can see (stageNEX), the Browser is made to look at the
//     cards again (its M then Y, the same one the drop drawer drives), and the keys that
//     lead from the menu to the command line, to `cd` to the card's drive and to `.nexload`
//     the file are pressed one at a time, frame by frame, with the screen looked at between
//     them to know which screen the keys are meeting.
//  2. The Next is running a program, or is at its command line, from which the menu cannot
//     be reached by a key press that is known to work. It may be in the middle of a game, so
//     the person is asked; when they agree the machine is reset with the card already in
//     place, and the keys of 1 are pressed from the welcome screen. The card is seen as
//     NextZXOS starts, so the Browser is not needed.
//  3. Neither worked, or the keys met a screen they do not know. The Next is started from a
//     scratch card made on the spot: the system's boot files, the NEX file, and an
//     autoexec.bas that does the one thing NextZXOS does for it at start-up, `.nexload`.
//     It takes the place of the card the Next was started from, until it is changed back
//     (File > ... the Next's card menu, or a new machine), which is said.
//
// Cards. The file goes on the second card, which NextZXOS calls D:. Where the slot is empty a
// scratch card is made for it in a temporary folder; where it holds a card that can be written
// the file is added to that card; where the card is read-only the scratch card takes the slot,
// which is said.

// Limits, in frames of the machine (50 a second).
const (
	nexBootGiveUp  = 3000 // from a reset to an idle prompt
	nexRunGiveUp   = 6000 // the whole run
	nexSettle      = 25   // an idle prompt this long is the screen the keys will meet
	nexKeyWait     = 60   // after a key that changes the screen
	nexLineWait    = 60   // after a line typed at the command line
	nexWatchBusy   = 60   // a program that has run this long without the prompt has started
	nexWatchReturn = 60   // a command that has left the prompt untouched this long has failed
	nexWatchGiveUp = 1500 // a program that has not started and has not failed
	nexScratchName = "run.nex"
	nexScratchWait = 1500 // from a scratch card's reset: a program that has not left the screen for the menu
)

// nexScreen is which NextZXOS screen the display shows, as far as the keys that lead
// onward are concerned.
type nexScreen int

const (
	nexScreenOther nexScreen = iota // not one of the others: a command line, a program's picture
	nexScreenMenu1                  // the menu's first page, which has Browser and Command Line
	nexScreenMenu2                  // the menu's second page
	nexScreenWide                   // the welcome screen or the Browser: text across the screen
)

// classifyNextScreen says which screen img shows. The screens differ in where the first ink
// is and how wide it is: the menus are a 128-pixel box some way down, the welcome screen and
// the Browser fill the width from the top, and the menu's second page is the first with more
// rows. Anything else is nexScreenOther, and nothing is done to a screen that is not
// recognised.
func classifyNextScreen(img *image.RGBA) nexScreen {
	if img == nil {
		return nexScreenOther
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 256 || h < 192 {
		return nexScreenOther
	}
	at := func(x, y int) [3]uint8 {
		i := (y-b.Min.Y)*img.Stride + (x-b.Min.X)*4
		return [3]uint8{img.Pix[i], img.Pix[i+1], img.Pix[i+2]}
	}
	counts := map[[3]uint8]int{}
	for y := b.Min.Y; y < b.Max.Y; y += 2 {
		for x := b.Min.X; x < b.Max.X; x += 2 {
			counts[at(x, y)]++
		}
	}
	var bg [3]uint8
	most := -1
	for c, n := range counts {
		if n > most {
			bg, most = c, n
		}
	}
	// rows[y] is the extent of the ink on row y: its left and right ends and how many pixels.
	type extent struct{ lo, hi, n int }
	rows := make([]extent, h)
	for y := 0; y < h; y++ {
		e := extent{lo: w, hi: -1}
		for x := 0; x < w; x++ {
			if at(b.Min.X+x, b.Min.Y+y) != bg {
				if x < e.lo {
					e.lo = x
				}
				e.hi = x
				e.n++
			}
		}
		rows[y] = e
	}
	top := -1
	for y, e := range rows {
		if e.n >= 4 {
			top = y
			break
		}
	}
	if top < 0 || top > h/2 {
		return nexScreenOther
	}
	widest := 0
	for y := top; y < top+48 && y < h; y++ {
		if rows[y].n > 0 && rows[y].hi-rows[y].lo+1 > widest {
			widest = rows[y].hi - rows[y].lo + 1
		}
	}
	switch {
	case widest >= 200:
		return nexScreenWide
	case widest >= 100 && widest <= 170:
		more := 0
		for y := top + 88; y < top+104 && y < h; y++ {
			more += rows[y].n
		}
		if more > 64 {
			return nexScreenMenu2
		}
		return nexScreenMenu1
	}
	return nexScreenOther
}

// nexScreenNow is the screen the machine shows.
func (zx *ZenZX) nexScreenNow() nexScreen { return classifyNextScreen(zx.DecodeDisplay()) }

// nexRunStage is how far a run has got.
type nexRunStage int

const (
	nexBoot    nexRunStage = iota // after a reset: waiting for the welcome screen or the menu
	nexToMenu                     // pressing Space until the menu's first page is up
	nexBrowser                    // Enter on Browser, and the Browser is up
	nexRemount                    // the Browser's M then Y
	nexLeave                      // Space, back to the menu
	nexCmd                        // Down, Enter: the command line
	nexTypeCD                     // cd "d:"
	nexTypeRun                    // .nexload name
	nexWatch                      // the program starting, or the command failing
	nexScratch                    // started from a scratch card: the program starting, or the menu coming up
)

// nexRun is a run in progress.
type nexRun struct {
	stage   nexRunStage
	keys    *KeyQueue
	name    string // what is typed after .nexload, and what the file is called on the card
	label   string // what the person knows the file as
	remount bool   // the Browser has to look at the cards again
	frames  int    // since the run began
	inStage int    // since this stage began
	idle    int    // consecutive frames at an idle prompt
	busy    int    // consecutive frames away from it, while watching
	tries   int    // presses of Space, or retries, in this stage
	sub     int    // a stage's own step
	subWait int    // frames left to wait in the step
	data    []byte // the file, for a scratch card
	scratch bool   // it has come to a scratch card
}

// NEXRunning reports whether a run is in progress (the machine's keys belong to it).
func (zx *ZenZX) NEXRunning() bool { return zx.nexRun != nil }

// nexNote says something to the person: the host's hook where it set one, the log otherwise.
func (zx *ZenZX) nexNote(msg string) {
	if zx.OnNEXRun != nil {
		zx.OnNEXRun(msg)
		return
	}
	sayf("%s\n", msg)
}

// nexRunSafeName matches the names that can be typed as they are: no spaces or characters
// that need a shift the keyboard script does not have.
var nexRunSafeName = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,40}$`)

// nexV13 is the version string of a NEX file .nexload does not load.
func nexVersionOK(data []byte) error {
	if len(data) < 512 || string(data[:4]) != "Next" {
		return errors.New("it is not a NEX file")
	}
	if string(data[:8]) == "NextV1.3" {
		return errors.New("it is a version 1.3 NEX, which NextZXOS's .nexload does not load")
	}
	return nil
}

// RunNEX runs the NEX file at path on a Next that boots NextZXOS, the least invasive way
// that works (see above). msg says what is being done, or why nothing is. needsReset is true
// when the machine is running something else and allowReset was not given: nothing has been
// changed, and the caller asks the person before calling again with allowReset.
func (zx *ZenZX) RunNEX(path string, allowReset bool) (msg string, needsReset bool) {
	base := filepath.Base(path)
	if zx.next == nil || zx.memory == nil || !zx.memory.isNext {
		return base + " is a Next program, which this model can't run", false
	}
	if zx.nexRun != nil {
		return "Already starting a program; " + base + " was not run", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return base + " could not be read", false
	}
	if err := nexVersionOK(data); err != nil {
		return base + ": " + err.Error(), false
	}
	screen := zx.nexScreenNow()
	atMenus := zx.nextAtIdlePrompt() && screen != nexScreenOther
	if !atMenus && !allowReset {
		return base + " can't be run while the Next is busy; it has to be reset first", true
	}
	name, stored := base, base
	if !nexRunSafeName.MatchString(base) {
		name, stored = nexScratchName, nexScratchName
	}
	name = strings.ToLower(name)
	where, err := zx.stageNEX(stored, data)
	if err != nil {
		return base + " could not be put on a card: " + err.Error(), false
	}
	r := &nexRun{name: name, label: base, keys: NewKeyQueue(6, 8), remount: atMenus, data: data}
	zx.nexRun = r
	if atMenus {
		r.stage = nexToMenu
		return fmt.Sprintf("Running %s (%s)", base, where), false
	}
	r.stage = nexBoot
	zx.io.nextResetRequest = nextResetHard
	return fmt.Sprintf("Resetting the Next to run %s (%s)", base, where), false
}

// stageNEX puts the file on the second card and says where it went. The slot is empty: a
// scratch card is made for it. It holds a card that can be written: the file is added to it.
// It holds one that can't: the scratch card takes the slot.
func (zx *ZenZX) stageNEX(name string, data []byte) (where string, err error) {
	if cur := zx.SecondCard(); cur != "" && zx.card2 != nil && !zx.card2.protected && cur != zx.nexScratch {
		if err := nextcard.AddFiles(context.Background(), cur, "/", []nextcard.File{{Name: name, Data: data}}, nil); err != nil {
			return "", err
		}
		// AddFiles replaces the image; take the card out and in so the machine reads the new one.
		if _, err := zx.InsertSecondCard(cur, false); err != nil {
			return "", err
		}
		return "on the card in slot 2, " + filepath.Base(cur), nil
	}
	replaced := zx.SecondCard() != "" && zx.SecondCard() != zx.nexScratch
	zx.dropNEXScratch()
	dir, err := os.MkdirTemp("", "zenzx-nex-")
	if err != nil {
		return "", err
	}
	card := filepath.Join(dir, "scratch.img")
	if err := addToOwnCard(card, []nextcard.File{{Name: name, Data: data}}); err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	if _, err := zx.InsertSecondCard(card, true); err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	zx.nexScratch = card
	if replaced {
		return "on a scratch card in slot 2, which takes the place of a card that is read-only", nil
	}
	return "on a scratch card in slot 2", nil
}

// dropNEXScratch deletes the scratch card, when there is one, and empties the slot if it is
// what holds it.
func (zx *ZenZX) dropNEXScratch() {
	if zx.nexScratch == "" {
		return
	}
	if zx.SecondCard() == zx.nexScratch {
		zx.RemoveSecondCard()
	}
	os.RemoveAll(filepath.Dir(zx.nexScratch))
	zx.nexScratch = ""
}

// nexRunStep moves a run on by one frame. RunFrame calls it first, after the host's keyboard
// has been read, so the keys it presses are the ones the machine sees.
func (zx *ZenZX) nexRunStep() {
	r := zx.nexRun
	r.frames++
	r.inStage++
	if r.frames > nexRunGiveUp {
		zx.nexFail("it did not finish in time")
		return
	}
	idle := zx.nextAtIdlePrompt()
	if idle {
		r.idle++
	} else {
		r.idle = 0
	}
	// Keys being pressed: let them finish, then give the screen time to change.
	if r.keys.Active() {
		r.keys.Step(zx)
		r.subWait = r.sub
		return
	}
	if r.subWait > 0 {
		r.subWait--
		return
	}
	next := func(s nexRunStage) { r.stage, r.inStage, r.tries, r.sub, r.subWait, r.busy = s, 0, 0, 0, 0, 0 }
	press := func(wait int, names ...string) {
		for _, n := range names {
			r.keys.EnqueueChord(namedKeys[n])
		}
		r.sub, r.subWait = wait, wait
	}
	typed := func(wait int, text string) {
		r.keys.EnqueueText(text)
		r.sub, r.subWait = wait, wait
	}

	switch r.stage {
	case nexBoot:
		if r.inStage > nexBootGiveUp {
			zx.nexFail("the Next did not come up to its menu after the reset")
			return
		}
		if r.idle >= nexSettle && zx.nexScreenNow() != nexScreenOther {
			next(nexToMenu)
		}

	case nexToMenu:
		if r.idle < nexSettle {
			if r.inStage > 400 {
				zx.nexFail("the Next is not at its menu")
			}
			return
		}
		switch zx.nexScreenNow() {
		case nexScreenMenu1:
			if r.remount {
				next(nexBrowser)
			} else {
				next(nexCmd)
			}
		case nexScreenMenu2, nexScreenWide:
			// Space leaves the welcome screen and the Browser for the menu, and turns the
			// menu's pages.
			r.tries++
			if r.tries > 4 {
				zx.nexFail("could not reach the menu's first page")
				return
			}
			press(nexKeyWait, "space")
		default:
			zx.nexFail("the Next is not at its menu")
		}

	case nexBrowser:
		switch {
		case r.sub == 0 && r.tries == 0:
			r.tries = 1
			press(nexKeyWait+30, "enter")
		case r.idle < nexSettle:
		case zx.nexScreenNow() == nexScreenWide:
			next(nexRemount)
		default:
			zx.nexFail("the Browser did not open")
		}

	case nexRemount:
		// The Browser's own remount, as the drop drawer drives it: M, then Y, handed to the
		// firmware's key loop. M asks, Y confirms and rereads the cards.
		switch r.tries {
		case 0:
			zx.memory.Write(nextLASTK, nextRemountMountKey)
			zx.memory.Write(nextFLAGS, zx.memory.Read(nextFLAGS)|nextKeyReadyBit)
			r.tries, r.subWait = 1, nextRemountMountFrames
		case 1:
			zx.memory.Write(nextLASTK, nextRemountConfirmKey)
			zx.memory.Write(nextFLAGS, zx.memory.Read(nextFLAGS)|nextKeyReadyBit)
			r.tries, r.subWait = 2, nextRemountConfirmFrames
		default:
			next(nexLeave)
		}

	case nexLeave:
		switch {
		case r.tries == 0:
			if r.idle < nexSettle {
				return
			}
			r.tries = 1
			press(nexKeyWait, "space")
		case zx.nexScreenNow() == nexScreenMenu1:
			next(nexCmd)
		case r.tries < 4:
			r.tries++
			press(nexKeyWait, "space")
		default:
			zx.nexFail("could not get back to the menu")
		}

	case nexCmd:
		if r.tries == 0 {
			if r.idle < nexSettle {
				return
			}
			r.tries = 1
			press(20, "down")
			r.keys.steps = append(r.keys.steps, keyStep{frames: 20}) // the menu has moved before Enter
			r.keys.EnqueueChord(namedKeys["enter"])
			r.sub, r.subWait = nexKeyWait+30, nexKeyWait+30
			return
		}
		next(nexTypeCD)

	case nexTypeCD:
		if r.tries == 0 {
			if r.idle < nexSettle {
				return
			}
			r.tries = 1
			typed(nexLineWait, "cd \"d:\"\n")
			return
		}
		next(nexTypeRun)

	case nexTypeRun:
		if r.tries == 0 {
			if r.idle < nexSettle {
				return
			}
			r.tries = 1
			typed(10, ".nexload "+r.name+"\n")
			return
		}
		next(nexWatch)

	case nexScratch:
		// Nothing is typed. The card starts the program as the system comes up, so the
		// machine does not reach a prompt; one that has not by now is running it.
		if r.idle >= nexSettle {
			zx.nexFail("the scratch card started the system, but not the program")
		} else if r.inStage >= nexScratchWait {
			zx.nexDone()
		}

	case nexWatch:
		if idle {
			r.busy = 0
			if r.idle >= nexWatchReturn {
				zx.nexFail("NextZXOS did not start it")
			}
		} else {
			r.busy++
			if r.busy >= nexWatchBusy {
				zx.nexDone()
			}
		}
		if r.inStage > nexWatchGiveUp {
			zx.nexFail("NextZXOS did not start it in time")
		}
	}
}

// nexDone ends a run that worked.
func (zx *ZenZX) nexDone() {
	r := zx.nexRun
	zx.nexRun = nil
	zx.io.ResetKeyboard()
	zx.nexNote(r.label + " is running")
}

// nexFail ends a run that did not work, and says why -- after the last thing to try, the
// scratch card, when it has not been tried.
func (zx *ZenZX) nexFail(why string) {
	r := zx.nexRun
	if !r.scratch {
		err := zx.bootNEXScratch(r)
		if err == nil {
			return
		}
		why += " (and a scratch card could not be made: " + err.Error() + ")"
	}
	zx.nexRun = nil
	zx.io.ResetKeyboard()
	zx.nexNote(r.label + " could not be run: " + why)
}

// nexAutoexec is the NextBASIC program a scratch card starts the system with: one line, the
// dot command that loads and runs the file, saved as +3DOS keeps a BASIC program (a 128-byte
// header, then the line). A dot command's text is kept as it is typed, not as tokens, and its
// file name goes without quotes: a program passes the quotes to .nexload, which then does not
// find the file. The name is nexScratchName, which needs none.
func nexAutoexec(name string) []byte {
	text := []byte(".nexload " + name + "\r")
	line := []byte{0, 10, byte(len(text)), byte(len(text) >> 8)}
	line = append(line, text...)
	h := make([]byte, 128)
	copy(h, "PLUS3DOS")
	h[8], h[9], h[10] = 0x1A, 1, 0
	total := len(h) + len(line)
	h[11], h[12], h[13], h[14] = byte(total), byte(total>>8), byte(total>>16), byte(total>>24)
	h[15] = 0 // a BASIC program
	h[16], h[17] = byte(len(line)), byte(len(line)>>8)
	h[18], h[19] = 10, 0 // starts at line 10
	h[20], h[21] = byte(len(line)), byte(len(line)>>8)
	var sum byte
	for _, b := range h[:127] {
		sum += b
	}
	h[127] = sum
	return append(h, line...)
}

// bootNEXScratch makes the scratch card for r's file, puts it in place of the Next's card
// and starts the machine from it. The system's files come from the card the Next was first
// started from, and so does .nexload.
func (zx *ZenZX) bootNEXScratch(r *nexRun) error {
	if zx.next == nil {
		return errors.New("this is not a Next that boots NextZXOS")
	}
	home := zx.next.files
	if zx.next.home != nil {
		home = *zx.next.home
	}
	src, err := os.Open(home.Card)
	if err != nil {
		return err
	}
	defer src.Close()
	vol, err := nextcard.Open(src)
	if err != nil {
		return err
	}
	dot := vol.Root.Find("dot/NEXLOAD")
	if dot == nil || dot.IsDir() {
		return errors.New("the system card has no .nexload")
	}
	nexload, err := vol.ReadFile(dot)
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "zenzx-boot-")
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		if !ok {
			os.RemoveAll(dir)
		}
	}()
	card := filepath.Join(dir, "scratch-boot.img")
	if _, err := newEmptyCard(card, smallestCard); err != nil {
		return err
	}
	// The files a Next needs to reach its menu, and the ones it starts better with, except
	// the welcome program (autoexec.1st), which would run in place of ours.
	files := []nextcard.File{
		{Name: "dot/NEXLOAD", Data: nexload},
		{Name: "nextzxos/autoexec.bas", Data: nexAutoexec(nexScratchName)},
		{Name: nexScratchName, Data: r.data},
	}
	for i, path := range append(append([]string{}, nextcard.BootFiles...), nextcard.BootExtras...) {
		n := vol.Root.Find(path)
		if n == nil || n.IsDir() || strings.EqualFold(filepath.Base(path), "autoexec.1st") {
			if i < len(nextcard.BootFiles) {
				return fmt.Errorf("the system card has no %s", path)
			}
			continue
		}
		data, err := vol.ReadFile(n)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		files = append(files, nextcard.File{Name: path, Data: data})
	}
	if err := nextcard.AddTree(context.Background(), card, files, nil); err != nil {
		return err
	}
	f := home
	f.Card = card
	if _, err := zx.attachNextBootChain(f, false, false); err != nil {
		return err
	}
	zx.next.scratchDir, zx.next.home = dir, &home
	ok = true
	zx.dropNEXScratch()
	zx.Reset()
	zx.io.nextResetRequest = nextResetHard
	r.scratch, r.stage, r.inStage, r.idle, r.tries, r.sub, r.subWait = true, nexScratch, 0, 0, 0, 0, 0
	r.keys = NewKeyQueue(6, 8)
	zx.nexNote(r.label + " was not started from the menus; starting the Next again from a scratch card that runs it")
	return nil
}
