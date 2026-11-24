//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/raycanvas"
	"github.com/ha1tch/zenui/zenuiraylib"
	"github.com/ha1tch/zenzx/pkg/fonts"
	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// The mount drawer slides down from the top edge while a drag gesture is in
// progress over the window, showing the focused machine's storage slots as
// drop targets. Dropping a file onto an eligible slot writes it into that
// slot's filesystem (SD cards now, via pkg/nextcard.AddFiles). A read-only or
// over-capacity slot is inert -- not a drop target.
//
// Trigger: raylib reports the dropped file only on release, but it reports the
// pointer and button during a drag, so the drawer opens on the drag gesture
// (mouse down + movement) and the file arrives on drop. A drop not over any
// slot falls through to the machine-load path (handleDroppedFiles).

const (
	mountSlideTime   = 220 * time.Millisecond  // matches the dock/HUD slide
	mountDragPx      = 6                       // movement that counts as a drag
	mountRevealDwell = 1400 * time.Millisecond // how long the drawer stays open after a drop
)

// mountSlot is one storage slot shown in the drawer.
type mountSlotKind int

const (
	slotSDCard mountSlotKind = iota // an SD card (nextcard / FAT32)
	slotFloppy                      // a +3 floppy drive (.dsk / +3DOS, via plus3)
)

type mountSlot struct {
	label    string        // "SD slot 2", "+3 Drive A"
	kind     mountSlotKind // which storage it is, so mount routes correctly
	contents string        // current card/disc name, or "empty"
	free     string        // free-space hint, or ""
	path     string        // backing image file, "" when none
	target   bool          // true when it is a real drop target (writable, buildable)
	reason   string        // why it is inert, when target is false
}

// mountDrawer is the overlay.
type mountDrawer struct {
	zx    *ZenZX
	slots []mountSlot

	armed   bool    // a drag gesture is in progress, or a drop reveal is showing
	anim    float32 // eased 0..1 slide position
	opening bool
	start   time.Time

	pressX, pressY int32
	pressed        bool

	// autoCloseAt is when a drop-revealed drawer should begin closing on its
	// own; the zero time means no auto-close is pending (an in-app drag).
	autoCloseAt time.Time

	// result feedback: an OSD toast that rises and fades after a drop, so the
	// acknowledgement outlives the drawer (which closes on drop). text is set
	// and seq bumped on a result; the host drives the toast every frame.
	toast    *toast
	toastTxt *zenuiraylib.BDFText
	msg      string
	seq      int

	// remount is a pending NextZXOS Browser refresh, driven one key per step
	// across frames (feeding the firmware's own M-then-Y) so it never blocks the
	// render loop. step counts down the frames until the next key; phase is how
	// far through the m, then y sequence we are. Zeroed means nothing pending.
	// remountZX is the machine the pending refresh targets, captured when it is
	// armed so a focus change mid-sequence cannot redirect it to another machine.
	remountPhase int
	remountStep  int
	remountZX    *ZenZX

	// front resolves the machine the drawer currently acts on. With several
	// machines (the deck) the focused one moves, so the drawer must act on
	// whichever is front at the moment of the drop, not the one it was built
	// with. It is nil for a single machine, where zx never changes.
	front func() *ZenZX

	// round draws the drawer's rounded shapes, the same way the dock does
	// (dockround_gui.go): a floating panel and tiles with soft corners, drawn
	// into the display's own frame. Created on first use, freed on Unload.
	round      *raycanvas.Context
	roundCache *raycanvas.SharedCache
}

func newMountDrawer(zx *ZenZX) *mountDrawer {
	m := &mountDrawer{zx: zx}
	if face, err := fonts.Sinclair(); err == nil {
		m.toastTxt = zenuiraylib.NewBDFText(face)
		m.toast = newToast(m.toastTxt)
	}
	return m
}

// useFront points the drawer at the currently focused machine (the deck's front),
// so a drop or a slot rebuild acts on the machine the user is looking at, not the
// one the drawer was constructed with. A no-op for a single machine (front nil).
func (m *mountDrawer) useFront() {
	if m.front != nil {
		if zx := m.front(); zx != nil {
			m.zx = zx
		}
	}
}

// say posts a result as a rising toast, shown after the drawer closes.
func (m *mountDrawer) say(s string) {
	m.msg = s
	m.seq++
}

// Tick drives the result toast every frame (not gated on the drawer being
// visible, so the acknowledgement persists after the drawer slides away).
func (m *mountDrawer) Tick() {
	if m == nil {
		return
	}
	m.driveRemount()
	if m.toast == nil || m.toastTxt == nil {
		return
	}
	m.toast.Update(rl.GetFrameTime(), m.seq, m.msg, rl.GetScreenWidth())
}

// armRemount schedules a NextZXOS Browser refresh after a mount onto the live
// card, to be driven by driveRemount over the coming frames. It is a no-op when
// the machine is not a NextZXOS boot chain or carries no Browser idle loop to
// drive (an older firmware), so nothing is injected where it would not be read.
func (m *mountDrawer) armRemount() {
	zx := m.zx
	if zx == nil || zx.next == nil {
		return
	}
	if _, ok := zx.locateBrowserIdleLoop(); !ok {
		return
	}
	m.remountZX = zx   // capture the target; focus may move before it fires
	m.remountPhase = 1 // phase 1 feeds "m", phase 2 feeds "y"
	m.remountStep = 1  // fire on the next frame
}

// driveRemount advances a pending Browser refresh one step per frame: it feeds
// the firmware "m" then "y" through its own key sysvars, each followed by a dwell
// of frames so the firmware picks the key up and acts on it (the confirm reads
// the card, so it dwells longer). Zeroed state means nothing is pending.
func (m *mountDrawer) driveRemount() {
	if m.remountPhase == 0 {
		return
	}
	if m.remountStep > 0 {
		m.remountStep--
		return
	}
	zx := m.remountZX
	if zx == nil { // the machine went away under us; drop the pending refresh
		m.remountPhase, m.remountStep = 0, 0
		return
	}
	switch m.remountPhase {
	case 1:
		zx.memory.Write(nextLASTK, nextRemountMountKey)
		zx.memory.Write(nextFLAGS, zx.memory.Read(nextFLAGS)|nextKeyReadyBit)
		m.remountPhase, m.remountStep = 2, nextRemountMountFrames
	case 2:
		zx.memory.Write(nextLASTK, nextRemountConfirmKey)
		zx.memory.Write(nextFLAGS, zx.memory.Read(nextFLAGS)|nextKeyReadyBit)
		m.remountPhase, m.remountStep, m.remountZX = 0, 0, nil
	}
}

// DrawToast draws the result toast (rising from the bottom-right, like the
// ROM-load caption). Called every frame by the host.
func (m *mountDrawer) DrawToast(screenW, screenH int) {
	if m == nil || m.toast == nil || m.toastTxt == nil || !m.toast.Active() {
		return
	}
	m.toast.Draw(screenW, screenH)
}

// Visible reports whether the drawer should draw this frame.
func (m *mountDrawer) Visible() bool { return m != nil && (m.armed || m.anim > 0) }

// hasMountableSlots reports whether the machine has any storage slot worth a
// drawer at all (a card or a +3 drive). No slots -> the drawer never arms.
func (m *mountDrawer) hasMountableSlots() bool {
	if m.zx == nil {
		return false
	}
	return m.zx.card2 != nil || (m.zx.io != nil && m.zx.io.hasFDC)
}

// Update runs each frame. It arms the drawer on a drag gesture over the window
// and advances the slide animation. It returns true while it owns the pointer
// (a drag it is tracking), so the host can keep that drag off other widgets.
//
// pointerTaken is true when another widget already owns the pointer this frame
// (the HUD being resized, a menu or dock drag, etc.). When set, the drawer does
// not arm and abandons any press it was tracking, so a HUD-resize drag or a
// menu drag never pops the drawer open.
func (m *mountDrawer) Update(pointerTaken bool) bool {
	if m == nil {
		return false
	}
	m.useFront() // the drawer tracks the focused machine's slots
	m.advance()

	// A drop-revealed drawer runs on its own timer and ignores the pointer: it
	// closes when its dwell elapses, whatever else is happening on screen.
	if !m.autoCloseAt.IsZero() {
		if time.Now().After(m.autoCloseAt) {
			m.autoCloseAt = time.Time{}
			m.setArmed(false)
		}
		return m.armed
	}

	if !m.hasMountableSlots() || pointerTaken {
		m.pressed = false
		m.setArmed(false)
		return false
	}

	down := rl.IsMouseButtonDown(rl.MouseLeftButton)
	mx, my := rl.GetMouseX(), rl.GetMouseY()

	switch {
	case down && !m.pressed:
		m.pressed, m.pressX, m.pressY = true, mx, my
	case down && m.pressed:
		if !m.armed && (absI(mx-m.pressX) > mountDragPx || absI(my-m.pressY) > mountDragPx) {
			m.rebuildSlots()
			m.setArmed(true)
		}
	case !down && m.pressed:
		// Button released without a file drop (an in-app drag or a stray
		// release); close the drawer. A real file drop is handled separately,
		// by TryDrop, before this runs.
		m.pressed = false
		m.setArmed(false)
	}
	return m.armed
}

// machineLoadExts are the dropped-file types that load into the running
// machine (handled by loadDroppedFile) rather than being copied onto a card.
var machineLoadExts = map[string]bool{
	".scr": true, ".zxs": true, ".sna": true, ".z80": true, ".szx": true, ".rzx": true,
	".dsk": true, ".tap": true, ".tzx": true, ".nex": true,
}

// allMachineLoadable reports whether every dropped file is a machine-load type.
// A mixed drop (some loadable, some not) is treated as a card copy as a whole,
// since it cannot be both.
func allMachineLoadable(files []string) bool {
	if len(files) == 0 {
		return false
	}
	for _, f := range files {
		if !machineLoadExts[strings.ToLower(filepath.Ext(f))] {
			return false
		}
	}
	return true
}

// TryDrop is called when files are dropped. raylib (GLFW) gives no drag-enter
// or drag-position event, so the drawer cannot appear while an OS file drag is
// in progress -- the window only learns of the drag at the drop. So TryDrop
// works drop-then-reveal.
//
// Routing is by file type: a drop of only machine-loadable files (tapes,
// snapshots, discs, .scr) falls through to the machine-load path and returns
// false, so dropping a game still loads it. Any other file is copied onto a
// card: TryDrop opens the drawer (as confirmation), mounts to the slot under
// the cursor -- or the nearest writable slot when the cursor is not over one --
// and returns true to consume the drop. With no storage at all it returns
// false so the file still reaches loadDroppedFile.
func (m *mountDrawer) TryDrop(files []string, x int32) bool {
	if m == nil {
		return false
	}
	m.useFront() // act on the focused machine, not the one built with
	if !m.hasMountableSlots() || allMachineLoadable(files) {
		return false
	}
	m.rebuildSlots()

	slot := m.slotForDrop(x)
	if slot == nil {
		// Storage exists but no slot resolved (e.g. every slot inert): reveal
		// the drawer so the user sees why, and consume the drop.
		m.reveal()
		m.say("no writable slot for the dropped file(s)")
		return true
	}

	m.reveal() // slide the drawer down as the acknowledgement
	if !slot.target {
		m.say(mountRefusedMsg(slot))
		return true
	}
	m.mount(slot, files)
	return true
}

// copyToCard copies files onto the focused machine's SD card, whatever their
// type -- the path the Next drop-choice dialog takes for its "Copy to SD Card"
// answer, where the files (a tape, a .scr, a snapshot) are machine-loadable types
// TryDrop would otherwise send to the machine. It resolves the slot and mounts
// the same way TryDrop does, and reports whether it had a slot to copy to.
func (m *mountDrawer) copyToCard(files []string, x int32) bool {
	if m == nil {
		return false
	}
	m.useFront()
	if !m.hasMountableSlots() {
		return false
	}
	m.rebuildSlots()
	slot := m.slotForDrop(x)
	if slot == nil {
		m.reveal()
		m.say("no writable slot for the dropped file(s)")
		return true
	}
	m.reveal()
	if !slot.target {
		m.say(mountRefusedMsg(slot))
		return true
	}
	m.mount(slot, files)
	return true
}

// reveal opens the drawer (slide down) and schedules it to close on its own
// after a short dwell, so the drop-then-reveal appearance is a brief
// confirmation rather than a panel the user must dismiss.
func (m *mountDrawer) reveal() {
	m.setArmed(true)
	m.autoCloseAt = time.Now().Add(mountRevealDwell)
}

// slotForDrop picks the target slot for a drop at cursor X. If the cursor falls
// in a slot's column it returns that slot (so aiming works); otherwise it
// returns the first writable slot, or the first slot if none is writable.
func (m *mountDrawer) slotForDrop(x int32) *mountSlot {
	if len(m.slots) == 0 {
		return nil
	}
	screenW := int(rl.GetScreenWidth())
	for i := range m.slots {
		r := m.slotRect(i, screenW, 0)
		if float32(x) >= r.X && float32(x) <= r.X+r.Width {
			return &m.slots[i]
		}
	}
	for i := range m.slots {
		if m.slots[i].target {
			return &m.slots[i]
		}
	}
	return &m.slots[0]
}

// mountRefusedMsg phrases why a drop onto an inert slot went nowhere, as a
// sentence for the toast. The tile's own terse caption (slot.reason) stays as
// it is; this is the spoken form.
func mountRefusedMsg(slot *mountSlot) string {
	switch slot.reason {
	case "read-only":
		return slot.label + " is read-only, so nothing was added"
	case "no disc in the drive":
		return "There is no disc in " + slot.label + ", so nothing was added"
	default:
		if slot.reason == "" {
			return "Nothing was added to " + slot.label
		}
		return "Nothing was added to " + slot.label + " (" + slot.reason + ")"
	}
}

// mount writes the dropped files into the slot, routing by its kind: an SD card
// goes through nextcard (FAT32) and a +3 floppy through plus3 (+3DOS).
func (m *mountDrawer) mount(slot *mountSlot, files []string) {
	if slot.kind == slotFloppy {
		m.mountFloppy(slot, files)
		return
	}
	ff := make([]nextcard.File, 0, len(files))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			m.say(filepath.Base(f) + " could not be read, so nothing was added")
			return
		}
		ff = append(ff, nextcard.File{Name: filepath.Base(f), Data: data})
	}
	if err := nextcard.AddFiles(context.Background(), slot.path, "", ff, nil); err != nil {
		m.say("Could not write to " + slot.label + ": " + err.Error())
		return
	}
	// AddFiles writes a new image and renames it over the card file, which
	// orphans the open handle the emulator holds -- it would keep serving the
	// old contents, so NextZXOS never sees the new files even after a remount.
	// Re-insert the card so the emulator reads the new image, then drive
	// NextZXOS's own Browser remount (its M-then-Y) from the host so the Browser
	// re-reads the directory and shows the new file without a keypress.
	m.reinsertIfLive(slot)
	m.armRemount()
	m.say(mountSuccessMsg(ff, slot))
}

// mountFloppy writes the dropped files into the +3 disc in the focused machine's
// drive, through plus3's +3DOS filesystem, then reloads the drive from the image
// so the running machine reads the new disc. It does not drive an OS re-read: on
// the +3 the new file is seen on the next directory access (a CAT or a loader),
// which is all that is needed -- the file lands on the right disc, in the focused
// machine's drive. The acknowledgement is the toast.
func (m *mountDrawer) mountFloppy(slot *mountSlot, files []string) {
	zx := m.zx
	if zx == nil || zx.io == nil || zx.io.fdc == nil {
		m.say("no +3 drive on this machine")
		return
	}
	written, err := addFilesToDisk(slot.path, files)
	if err != nil {
		m.say("Could not write to " + slot.label + ": " + err.Error())
		return
	}
	// addFilesToDisk saved the image back to the same file; reload the drive from
	// it so the running machine reads the new disc (the FDC holds the image in
	// memory, so without this it keeps serving the pre-write disc).
	if err := zx.io.fdc.LoadDisk(slot.path); err != nil {
		m.say("added to the disc, but the drive could not re-read it: " + err.Error())
		return
	}
	m.say(floppySuccessMsg(written, slot))
}

// floppySuccessMsg phrases the acknowledgement for a disc write: a few files are
// named, a larger batch is counted, each placed on the disc by its drive.
func floppySuccessMsg(written []string, slot *mountSlot) string {
	disc := filepath.Base(slot.path)
	switch len(written) {
	case 0:
		return "Nothing was added to " + slot.label
	case 1, 2, 3:
		return capitalise(joinAnd(written)) + " added to " + disc + " in " + slot.label
	default:
		return capitalise(countInWords(len(written))) + " files added to " + disc + " in " + slot.label
	}
}

// reinsertIfLive re-opens the slot's card in the machine when that card is the
// one currently in the second slot, so a write that replaced the backing file
// (AddFiles renames a new image into place) is actually seen by the emulator.
func (m *mountDrawer) reinsertIfLive(slot *mountSlot) {
	zx := m.zx
	if zx == nil || zx.card2 == nil || slot.path == "" {
		return
	}
	if !same(zx.card2.path, slot.path) {
		return
	}
	protected := zx.card2.protected
	if _, err := zx.InsertSecondCard(slot.path, protected); err != nil {
		m.say("mounted, but the card could not be re-read: " + err.Error())
	}
}

// mountSuccessMsg phrases the acknowledgement. A handful of files are named
// outright; a larger batch is counted (in words) and placed by card and slot,
// so the message says what landed where rather than a bare tally.
func mountSuccessMsg(ff []nextcard.File, slot *mountSlot) string {
	card := filepath.Base(slot.path)
	where := slot.label
	if card != "" && card != "." {
		where = card + " in " + slot.label
	}

	// Up to three files: name them.
	if len(ff) <= 3 {
		names := make([]string, len(ff))
		for i, f := range ff {
			names[i] = f.Name
		}
		list := joinAnd(names)
		if len(ff) == 1 {
			return list + " has been added to " + where
		}
		return list + " have been added to " + where
	}

	// More than a few: count them, in words where it reads well.
	return capitalise(countInWords(len(ff))) + " files have been added to " + where
}

// joinAnd renders a list as "a", "a and b", or "a, b and c" (no Oxford comma
// before the final "and", matching British usage).
func joinAnd(xs []string) string {
	switch len(xs) {
	case 0:
		return ""
	case 1:
		return xs[0]
	case 2:
		return xs[0] + " and " + xs[1]
	default:
		head := xs[:len(xs)-1]
		out := ""
		for i, s := range head {
			if i > 0 {
				out += ", "
			}
			out += s
		}
		return out + " and " + xs[len(xs)-1]
	}
}

// countInWords spells small counts and falls back to digits for larger ones.
func countInWords(n int) string {
	words := []string{
		"zero", "one", "two", "three", "four", "five", "six",
		"seven", "eight", "nine", "ten", "eleven", "twelve",
	}
	if n >= 0 && n < len(words) {
		return words[n]
	}
	return fmt.Sprintf("%d", n)
}

// capitalise upper-cases the first rune of s for sentence-leading words.
func capitalise(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if r[0] >= 'a' && r[0] <= 'z' {
		r[0] -= 'a' - 'A'
	}
	return string(r)
}

// rebuildSlots reads the current slot state for the drawer. For this build the
// working SD target is slot 2 (secondCard, which carries its path and its
// read-only flag); the +3 drive is shown but deferred.
func (m *mountDrawer) rebuildSlots() {
	m.useFront() // show the focused machine's slots
	m.slots = m.slots[:0]
	zx := m.zx

	if zx.card2 != nil {
		s := mountSlot{
			label:    "SD slot 2",
			kind:     slotSDCard,
			contents: filepath.Base(zx.card2.path),
			path:     zx.card2.path,
			target:   !zx.card2.protected,
		}
		if zx.card2.protected {
			s.reason = "read-only"
		} else if info, err := os.Stat(zx.card2.path); err == nil {
			s.free = humanBytes(info.Size())
		}
		m.slots = append(m.slots, s)
	}

	if zx.io != nil && zx.io.hasFDC {
		s := mountSlot{
			label: "+3 Drive A",
			kind:  slotFloppy,
		}
		// The disc in the drive is the backing .dsk file; a file can be added to
		// it only when there is one and it is a +3DOS disc plus3 can write.
		if disc := zx.io.fdc.DiskFile(); disc != "" {
			s.contents = filepath.Base(disc)
			s.path = disc
			s.target = true
			if info, err := os.Stat(disc); err == nil {
				s.free = humanBytes(info.Size())
			}
		} else {
			s.contents = "empty"
			s.reason = "no disc in the drive"
		}
		m.slots = append(m.slots, s)
	}
}

// --- animation -------------------------------------------------------------

func (m *mountDrawer) setArmed(on bool) {
	if on == m.armed {
		return
	}
	m.armed, m.opening, m.start = on, on, time.Now()
}

func (m *mountDrawer) advance() {
	if m.start.IsZero() {
		return
	}
	t := float32(time.Since(m.start)) / float32(mountSlideTime)
	if t >= 1 {
		if m.opening {
			m.anim = 1
		} else {
			m.anim = 0
		}
		return
	}
	if m.opening {
		m.anim = easeInOutCubic(t)
	} else {
		m.anim = 1 - easeInOutCubic(t)
	}
}

func absI(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

func humanBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%d MB", n>>20)
	case n >= 1<<10:
		return fmt.Sprintf("%d KB", n>>10)
	default:
		return fmt.Sprintf("%d B", n)
	}
}
