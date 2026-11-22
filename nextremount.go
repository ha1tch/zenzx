// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

// Force NextZXOS to refresh its SD-card view from the host, so a file written to
// a card shows up in the Browser without the user pressing the Browser's M then
// Y by hand.
//
// The problem. NextZXOS's Browser is a running IDE_BROWSER call that keeps its
// own cached directory. It only re-reads the card on an in-Browser remount (the
// M then Y keystrokes): that path both remounts the volumes (IDE_MOUNT) and
// rebuilds the Browser's listing. Calling IDE_MOUNT alone from an injected CPU
// stub was tried against real NextZXOS 26.11 and does remount the volumes, but
// does NOT rebuild the Browser's listing, so the added file stays invisible; and
// a cold-injected +3DOS call cannot cleanly resume, because the firmware's
// dispatcher assumes its own caller context. Re-entering the Browser from the
// menu does not re-read either (the directory is cached across invocations).
//
// The mechanism. Instead of injecting code, this drives the firmware's OWN M-then-Y
// handler by feeding the two keycodes into the key sysvars its idle loop already
// reads: LAST_K ($5C08) for the key and FLAGS ($5C3B) bit 5 for "a new key is
// ready". The firmware, on its own thread and in its own context, runs exactly
// the path the physical keys would -- remount AND listing rebuild -- so the added
// file appears. Verified on real NextZXOS 26.11 (ZENZX_NEXT_FILES): identical
// result to the physical M-then-Y, a no-op when nothing was added, and stable
// (idempotent) when run twice.
//
// Version-robustness. The keycodes go to fixed Spectrum sysvars, but the method
// is gated on finding the firmware's idle key-wait loop by its byte signature --
// HALT / LD HL,$5C3B / BIT 5,(HL) -- the classic Spectrum-family key-wait idiom,
// not a fixed address. If a firmware does not carry that idiom the refresh is
// declined rather than guessed, and the manual M-then-Y (after the host
// re-inserts the card to reopen its handle) always works.

const (
	// nextFLAGS and nextLASTK are the Spectrum sysvars the Browser's idle loop
	// reads: FLAGS bit 5 means "a new key is ready", LAST_K is that key's code.
	nextFLAGS = 0x5C3B
	nextLASTK = 0x5C08

	// nextKeyReadyBit is FLAGS bit 5.
	nextKeyReadyBit = 0x20

	// nextRemountMountKey / nextRemountConfirmKey are the Browser's remount keys:
	// "m" opens the change-media prompt, "y" confirms it. Lowercase ASCII, as the
	// firmware wants in LAST_K (uppercase does not trigger it).
	nextRemountMountKey   = 'm'
	nextRemountConfirmKey = 'y'

	// nextRemountMountFrames / nextRemountConfirmFrames are how many frames to let
	// the firmware run after each injected key. The confirm triggers the card
	// re-read (reading the FAT), so it gets the longer budget.
	nextRemountMountFrames   = 80
	nextRemountConfirmFrames = 220
)

// locateBrowserIdleLoop scans the paged-in ROM ($0000-$3FFF) for the Browser's
// idle key-wait loop and returns the address of its HALT. The signature is the
// classic Spectrum-family key-wait idiom:
//
//	76           HALT              ; wait for the frame interrupt
//	21 3B 5C     LD HL,$5C3B       ; HL -> FLAGS
//	CB 6E        BIT 5,(HL)        ; test "a new key is ready"
//
// Finding it confirms the firmware uses FLAGS/LAST_K the standard way, so feeding
// those sysvars drives its own key handler. ok is false when no such loop is
// present, in which case the caller declines the automatic refresh.
func (zx *ZenZX) locateBrowserIdleLoop() (addr uint16, ok bool) {
	m := zx.memory
	for a := 0; a <= 0x3FFF-6; a++ {
		if m.Read(uint16(a)) == 0x76 &&
			m.Read(uint16(a+1)) == 0x21 && m.Read(uint16(a+2)) == 0x3B && m.Read(uint16(a+3)) == 0x5C &&
			m.Read(uint16(a+4)) == 0xCB && m.Read(uint16(a+5)) == 0x6E {
			return uint16(a), true
		}
	}
	return 0, false
}

// forceNextZXOSRemount drives NextZXOS's own M-then-Y Browser remount from the
// host, so the Browser re-reads the SD cards and shows a file just written to
// one, without any physical keypress. run advances the machine (the caller's
// RunFrame, so the firmware actually executes between the injected keys).
//
// It returns true when it drove the remount: the machine is a NextZXOS boot
// chain and its Browser idle loop was found by signature. It returns false, and
// does nothing, otherwise -- then the manual M-then-Y remains the way to refresh.
// It is safe to call when nothing was added (the listing is unchanged) and when
// called more than once (the second call is a no-op).
func (zx *ZenZX) forceNextZXOSRemount(run func()) bool {
	if zx.next == nil {
		return false // not a NextZXOS boot chain
	}
	if _, ok := zx.locateBrowserIdleLoop(); !ok {
		return false // this firmware does not carry the key-wait idiom
	}
	zx.feedBrowserKey(nextRemountMountKey, nextRemountMountFrames, run)
	zx.feedBrowserKey(nextRemountConfirmKey, nextRemountConfirmFrames, run)
	return true
}

// feedBrowserKey hands one key to the firmware the way its idle loop expects it:
// LAST_K gets the code and FLAGS bit 5 is set to say a key is ready. It then runs
// the machine for frames so the firmware picks the key up and acts on it.
func (zx *ZenZX) feedBrowserKey(code uint8, frames int, run func()) {
	m := zx.memory
	m.Write(nextLASTK, code)
	m.Write(nextFLAGS, m.Read(nextFLAGS)|nextKeyReadyBit)
	for i := 0; i < frames; i++ {
		run()
	}
}
