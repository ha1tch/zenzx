# ZenZX — resolution record

Closed register items, newest first. Each entry is the item's detail
text verbatim as at closure, stamped with the closing version and date.
The changelog says what shipped; this record says what was wrong and
how it was resolved. Append-only.

## [0.50.0] T-81 — TestBatchThreeModeCompare passes without its corpus instead of skipping (v0.50.0, 2026-11-23)

Theme: testing · closed 0.50.0 · 2026-11-23
Trigger: found 2026-11-23. `TestBatchThreeModeCompare` passes when its corpus is not there, so a run without the corpus reports a pass for a comparison that did not happen.

Scope: make it skip, saying the corpus is missing, rather than pass. Done when: a run without the corpus shows SKIP and a run with it still compares.

Resolved 0.50.0 (2026-11-23). Fix: the test stats `corpusRoot` first and skips, saying where it looked. Checked: a run here, without the corpus, shows SKIP. The run with the corpus was not made (the corpus is not in the sandbox); that path is untouched by the change.

Cross-ref: CHANGELOG 0.50.0.

## [0.50.0] T-78 — +3: a write to port 0x7FFD with bit 5 set does not lock paging while special (all-RAM) paging is on (v0.50.0, 2026-11-23)

Theme: models · closed 0.50.0 · 2026-11-23
Trigger: the T-74 reset test (reset_test.go, TestResetPutsBackThePaging) set special paging with port 0x1FFD and then wrote 0x37 to port 0x7FFD; the lock did not take. SpectrumMemory.SetPaging (memory.go) returns after the screen select in special mode, before bit 5 is looked at. The +3's lock latch is the 0x7FFD port's own, independent of the 0x1FFD paging mode, so the lock should be set there too. Scope: set the lock before the early return, with a test (special paging on, write 0x20 to 0x7FFD, assert pagingLocked and that a later 0x1FFD write is refused). Done when: the test passes and the +3 special-paging tests still do.

Resolved 0.50.0 (2026-11-23). Cause: `SetPaging` returned after the screen select in special mode, before it looked at bit 5. Fix: the 0x7FFD latch (`port7FFD`) and the lock are now taken before the special-mode return, and `SetPlus3Paging` keeps the latch's RAM bank and ROM bit so that they come back when special paging ends. Tests: `paging_plus3_test.go` (`TestPlus3LockHoldsInSpecialPaging`, `TestPlus3LatchComesBackWhenSpecialPagingEnds`), and the +3 special-paging and reset tests still pass.

Cross-ref: CHANGELOG 0.50.0.

## [0.50.0] T-76 — Next booted from the card: -rom, -romN and -rom-bin are replaced by the firmware's ROM load, silently; hot patches over it (v0.50.0, 2026-11-23)

Theme: next-support · closed 0.50.0 · 2026-11-23
Trigger: a study of ROM handling, 2026-11-22. On a Next booted from the card, `-rom`, `-rom0` to `-rom3` and `-rom-bin` put their ROM in the four ROM banks at start, and the firmware replaces all four when it loads its ROMs, about 450 frames in. Measured on the real firmware. Nothing says so.

Scope: `docs/proposals/NEXT_ROM_OVERLAY.md`. In order: (1) a warning at start, on a Next with the boot chain, that these switches are replaced by the firmware; (2) a patch list on the machine, applied when config mode is left (NextREG 0x03 choosing the machine), written through `nextPhysWrite` so that banks 0-7 are reached; (3) `-rom-fill rom`, `-rom-save` and the cut warning made at that event, since the base is what the firmware loaded; (4) `-rom-bank` 0-7 and a physical address on a Next, over a 256 KiB base; (5) a view of which ROM is live (lock bits, NextREG 0x8C, `romBank`, slots 0 and 1).

Done when: with the real firmware, a patch over bank 0 runs at the hand-over and a patch of the same bytes still reaches the NextZXOS menu (G-04), a hard reset applies the list again, and the unit test of the config-mode sequence holds the rest.

Resolved 0.50.0 (2026-11-23). Cause: the switches wrote their ROM into the four ROM banks at start, and the firmware's own load, about 450 frames in, wrote over all four. Fix: the switches are a patch list on the machine (`rom_patches.go`), applied when the firmware leaves config mode (NextREG 0x03 choosing the machine, then the soft reset ~10 frames on) and after every hard reset; a Next's `-rom-bank` takes 0-7 and `-rom-at` is a physical address over the first 256 KiB of SRAM (`rom_next_region.go`); `-rom-fill rom`, `-rom-save` and the cut warning are made at the hand-over; a warning at start says the firmware replaces the banks; Show Status says which ROM is live; the boot cache's key has one entry per patch. Tests: `rom_patches_test.go` (the config-mode sequence driven by hand: nothing before the firmware, the patch after it, again after a hard reset, not after a soft one; banks, alternate ROMs and the cut; the live-ROM view; the cache key) and, against the real firmware, `TestNextROMPatchesOverTheFirmware` (G-16, 2026-11-23), with G-04, G-13 and G-15 re-run after the change.

Cross-ref: CHANGELOG 0.50.0.

## [0.50.0] T-74 — Reset restarted only the CPU: paging, the AY, the Timex ports and, on a Next, the firmware and registers kept the running program's state (v0.50.0, 2026-11-23)

Theme: next-support · closed 0.50.0 · 2026-11-23
Trigger: feedback from a colleague testing 0.50.0, 2026-11-22: the Next emulation resets in a weird way. Widened by the tech team the next day: Reset has not been stable on any machine since the machines system, and does not completely reset some of them.

Cause, measured: `ZenZX.Reset` restarted the CPU, the keyboard, the tape and the link, and nothing else. Every machine that was reset after running a program was compared field by field with one switched on, after loading real programs (testdata/snapshots, and the Next's examples gallop, fuji and orbit on the real firmware). The 48K was fine (the ROM clears RAM and writes the ULA's latch again). The 128K, +2, +2A and +3 started again in whatever ROM the program had left selected, so a 128K that had been in ROM 1 started in the 48K BASIC and never showed its menu; the paging lock, the +3's special paging and its 0x1FFD latch survived, and so did the AY's registers and counters. The TS2068 kept ports 0xF4 and 0xFF, and with them its hi-colour screen mode. The +3's disc controller kept its command and motor. On a Next, the Machine menu, F1 and the debugger's reset called only `Reset`, which never asked for the hard reset that restarts the firmware; the CPU started at 0 in the +3 ROM with config mode off, the boot ROM off, the machine type chosen, and every register, palette, sprite, the copper, the CTC and the DMA as the program had left them (Layer 2 and the tilemap over the boot screen). The hard reset that was there (serviceNextReset) put back the memory map, speed, line interrupt, DMA, interrupts, Multiface and DivMMC, but not the NextREG file, and it forced config mode on even with no boot ROM to run there.

Fix (0.50.0): `Reset` now calls `resetHardware` (reset.go): memory paging, lock and special paging back to switch-on (the shadow screen is put back in its bank), the AY, the TS2068's ports and screen mode, the +3's controller (the disc stays in the drive), and on a Next a hard reset. A Next's hard reset now also puts back every NextREG, the clip windows, the tilemap's state, the sprites, the palettes, the copper, the Timex port and the DivMMC's automap, forgets the machine type, and goes into config mode only when there is a boot ROM. Kept, as the hardware keeps them: RAM, the ULA's border/speaker/MIC latch, a disc, a tape, the host's devices. The reference was the machine as switched on (the comparison is the machine-state codec's own dump, so a field added to a model later is compared too); CSpect itself was not available here.

Test: `TestResetIsASwitchOn` (eight machine and program pairs, straight after Reset and 400 frames on), `TestResetPutsBackThePaging`, `TestResetKeepsTheShadowScreenInItsBank`, `TestResetSilencesTheAY`, `TestResetPutsBackTheTimexPorts`, `TestResetStartsTheDiscControllerAgain`, `TestResetKeepsMemoryAndTheULALatch` and `TestResetNextIsAHardReset` (reset_test.go) run with the ordinary suite; with the fix taken out, they fail on every model but the 48K, which was already right. `TestNextResetRestartsTheFirmware` (reset_next_hw_test.go, dormant guard G-15) resets a Next in the middle of each of three examples against the real firmware: config mode, the registers and the program's copper and interrupts are gone and NextZXOS comes back to its welcome screen and menu.

Cross-ref: CHANGELOG 0.50.0.

## [0.50.0] T-77 — A Next that boots once: the settled state cached on the first boot and restored like a ROM (v0.50.0, 2026-11-23)

Theme: next-support · closed 0.50.0 · 2026-11-23
Trigger: the idea, 2026-11-22, that the first boot of a Next, once NextZXOS is up and settled, caches the state of the machine and every later start restores it, so that the Next behaves like a ROM and boots once. The start to the menu is 760 frames, 15 s at the real speed.

Scope: `docs/proposals/NEXT_BOOT_CACHE.md`. A detector of "booted and stable" (idle prompt, display steady, nothing in flight); a new state format for the Next, since the snapshots are 48K and 128K only; a key (boot files, one deterministic SHA-256 of the firmware files on the card (sorted, length-prefixed, case-folded paths), board issue, patch list, ZenZX version) and one function that compares a cache with it, which `-next-firmware check`, `setup` and `cleanup` and the start-up all use; `-next-boot-cache auto|off|refresh`. First a cache at the hand-over, with the cold-versus-restored difference test; then the settled state.

Done when: a cold boot and a boot from the cache agree in memory, registers and screen at the same point and reach the menu in the same frames; a changed boot file, card, patch list or version, or a corrupt file, boots from the firmware; and `-next-firmware check` names what differs.

Cross-ref: CHANGELOG 0.50.0.

## [0.41.0] T-71 — Switch the focused machine by keyboard (focus-by-number) in a multi-machine session (v0.41.0, 2026-11-18)

Theme: input · closed 0.41.0 · 2026-11-18
With -multi several machines run at once but only one has keyboard focus, changed by clicking its window. A keyboard way to switch focus (e.g. a modifier+digit, 1..9 for the first nine machines) would let someone drive a grid without reaching for the mouse. The chord must avoid clashing with OS shortcuts: CMD+number is taken by macOS (Spaces/tabs) and some Linux DEs; prefer a combination free on macOS, Windows and common Linux desktops (candidates to weigh: Alt+digit, Ctrl+Alt+digit, or a two-key leader like Alt+M then digit). Decide the chord, document it alongside the existing Alt-key controls, and show the focused machine (the cyan frame already marks it).

Cross-ref: CHANGELOG 0.41.0.

## [0.41.0] T-70 — Autorun a prebuilt .tap (machine-code games), not only text BASIC (v0.41.0, 2026-11-18)

Theme: link · closed 0.41.0 · 2026-11-18
AutorunFile tokenises its input as text BASIC and types LOAD empty-string. A compiled game is distributed as a .tap with its own BASIC loader. Detect a TAP (by extension or signature) and load it straight into the deck, keeping the same LOAD keystrokes, so any assembled program can -autorun. Needed by examples/world.

Cross-ref: CHANGELOG 0.41.0.

## [0.34.1] T-06 — 15 Go files are not gofmt-clean (v0.34.1, 2026-11-17)

Theme: hygiene · closed 0.34.1 · 2026-11-17
- **Trigger:** session review 2026-08-17: `gofmt -l .` lists audio_oto.go, audio_sinc_filter.go, fdc.go, fdc_read_test.go, io.go, keyboard_script.go, memory.go, scheduler.go, screen_read.go, script.go, script_test.go, snapshot_formats.go, tape_types.go, version_main.go, zenzx.go. Pre-existing; not introduced this session.
- **Fix:** run `gofmt -w` on the tree in a dedicated commit with no other changes, then add a gofmt check as a release step so it cannot recur.

Cross-ref: CHANGELOG 0.34.1.

## [0.27.0] T-38 — Migrate onto the shared zenui module and zentools' bdf (v0.27.0, 2026-11-16)

Theme: shared · closed 0.27.0 · 2026-11-16
Trigger: zenzx carried the only copy of the UI toolkit it grew. zenui v0.1.0 extracts it, with `zxpalette` and the raylib adapter, and zentools' `pkg/bdf` is the canonical BDF reader.

Scope: switch the root package and `pkg/fonts` to the shared modules; delete `pkg/zenui`, `pkg/zenuiraylib`, `pkg/zxpalette` and `pkg/bdf`; move zentools to v0.8.6.

Verified: the headless build, vet and tests, the `./pkg/...` tests, and a cgo-free windows build and vet of every GUI file and test all give the same results as before the change. This was checked against a local simulation of the published module.

Outstanding: build against the real published zenui tag; run the GUI-tagged tests on a host with a display (they are only compile-checked in the sandbox); land it on main, which still carries the in-tree copies as of v0.10.0 (the change itself lives in the zenplay workspace's copy of zenzx), then cut a release.

Cross-ref: CHANGELOG 0.27.0.

## [0.22.0] T-52 — A snapshot started from NextZXOS ends at address 0 (v0.22.0, 2026-11-16)

Theme: next-support · closed 0.22.0 · 2026-11-16
- **Trigger:** 0.14.0, trying the card's palette cycling demo: `SPECTRUM "!RunMe.snx"` from NextBASIC in `/demos/PaletteCycle` leaves a black screen with PC=0000, SP=5C01.
- **Known:** NextZXOS gets as far as setting the memory mapping and the palette for the snapshot (NextREG 0x8E=02, MMU slots changed, the ULA palette written through NextREG 0x41), and NextREG 0x02 reads 0x08 afterwards. `.nexload` of `.nex` files works; only a snapshot was tried this way.
- **In jnext (0.20.0):** the same command does not run the demo there either: jnext is back at the NextZXOS welcome screen afterwards, where ZenZX is at a black screen. CSpect was not tried.
- **Not known:** what the loader does next. NextREG 0x02 bit 3 is the Multiface NMI request and the Multiface is not modelled, so that is the first thing to look at.
- **Done when:** the palette cycling demo runs from NextBASIC and from the Browser.
- **Resolved in 0.22.0:** NextZXOS starts a snapshot through the Multiface. It loads the snapshot, asks for a Multiface NMI with NextREG 0x02 bit 3 from a fixed address in its ROM (0x0AB8), and the Multiface ROM, seeing that address in NextREG 0xC2/0xC3 (the stackless NMI's return address), takes the registers from the snapshot's header and leaves with RETN to a RET in the 48K ROM, which takes the program's start from its own stack. Two things were in the way. There was no Multiface (`nextmultiface.go`, new). And the demo then stopped at its first interrupt: the CPU core clears bit 0 of the IM 2 vector, and the demo sets only the two bytes of its table that a vector of 0xFF reads (`im2vector.go`, T-60). With both, the demo runs from NextBASIC and from the Browser, and the card's other snapshots (Pogie, The Hollow Earth Hypothesis, OnlyYou6ch) start too. Neither reference runs it: jnext 1.3.0 is back at the welcome screen, CSpect 3.3.1.0 starts the snapshot (its red border) and stays at a black screen.

Cross-ref: CHANGELOG 0.22.0.

## [0.17.0] T-54 — The Next's line interrupt (NextREG 0x22, 0x23) is not raised (v0.17.0, 2026-11-16)

Theme: next-support · closed 0.17.0 · 2026-11-16
- **Trigger:** 0.15.0, writing the copper. NextREG 0x22 and 0x23 are stored only: bit 1 of 0x22 with a 9-bit line in 0x22 bit 0 and 0x23 should raise a maskable interrupt at the start of that line (the counter NextREG 0x1E/0x1F reads, NextREG 0x64 included), and bit 2 of 0x22 turns the ULA's frame interrupt off. Bit 7 does not read back the ULA's interrupt.
- **Effect:** a program that splits the screen with a line interrupt, in place of the copper, gets no interrupt at the line. A change it makes from the frame interrupt is drawn at the right place since 0.15.0; one that waits for the line interrupt never happens.
- **Fix:** raise the interrupt in the frame loop at the T-state of the line, for the same length as the frame interrupt; honour bit 2; read bit 7 back. The interrupt registers of NextREG 0xC0-0xCF (vectors, enables, status) are a separate matter and are not modelled.
- **Done when:** a test program that changes the border from a line interrupt shows the change at that line.

Cross-ref: CHANGELOG 0.17.0.

## [0.16.0] T-53 — NextREG 0x07 is stored but the CPU always runs at 3.5 MHz (v0.16.0, 2026-11-16)

Theme: next-support · closed 0.16.0 · 2026-11-16
- **Trigger:** 0.15.0, comparing the card's show512 demo with CSpect after the frame began to be drawn in pieces. The demo, like NextZXOS itself, asks for 28 MHz (NextREG 0x07 = 3); the register is stored and the CPU goes on at 3.5 MHz.
- **Effect:** everything a program does takes up to eight times as many frames as on the machine, and what is timed against the beam lands in the wrong place. show512 writes a 256 colour palette between the two halves of its picture: at 28 MHz that fits in the gap between them, at 3.5 MHz it is still going on twelve lines into the lower half, and those lines (frame lines 76 to 87) come out black where CSpect has colour. They are the only pixels of that screen that differ.
- **Fix:** run 2, 4 or 8 times the T-states in a frame when NextREG 0x07 asks, keeping the frame, the lines, the sound and the tape in 3.5 MHz T-states. NextREG 0x07 reads back the speed in bits 5-4 as well.
- **Done when:** show512 is the same as CSpect's picture on every pixel, and the NextZXOS boot and save guard (G-04) passes.

Cross-ref: CHANGELOG 0.16.0.

## [0.13.0] T-48 — GUI: Next layers under the ULA, with the border hidden, and at 1x (v0.13.0, 2026-11-16)

Theme: next-support · closed 0.13.0 · 2026-11-16
- **Trigger:** 0.11.0, reading display.go and videorender_gpu.go while making the GUI draw the tilemap, and one observation in `TestGLNextPicture`.
- **Gaps:** (1) With the border hidden the standard picture is drawn from the window's top left, but Layer 2 and the sprites are still placed at the border's offset, so they are displaced; seen as a missing Layer 2 pixel when the GL test ran without a border. (2) The GUI draws only the layers that NextREG 0x15 puts above the ULA and treats the ULA as opaque, so a layer under the ULA never shows through a transparent ULA colour; the headless decode composites all three. (3) The tilemap is drawn as part of the ULA picture, so sprites and Layer 2 are always over it. (4) At 1x a 512 pixel picture (Timex hi-res, 80 column tilemap) loses every other column; the 640x256 Layer 2 mode has a minimum scale of 2 for this reason.
- **Fix:** the simplest route is the one the software picture already takes: composite all the Next layers with `DecodeDisplay` whenever a Next has any of them on, and show that as one texture.
- **Done when:** `TestGLNextPicture` covers a layer under a transparent ULA colour and the hidden border, and passes.
- **Resolved (0.13.0):** by the route named above, and further. The trigger was the firmware's loading screen, which showed only its left half in both builds: it is Layer 2 at 320x256, whose clip window counts X in units of two pixels and which starts in the border. `ComposeNextFrame` (nextframe.go) draws the whole 320x256 frame, border included, and both front ends show it whenever a Next has Layer 2, the sprites or the tilemap on. Gap 1: with the border hidden the GUI shows the classic picture's part of the frame (`TestGLNextPicture`). Gap 2: every layer is composed, with the global transparency colour and the fallback colour (`TestNextFrameOrderTransparencyAndFallback`, the function both builds use). Gap 3 was not a gap: the tilemap shares the ULA's place in the order on the hardware too. Gap 4 is T-51. The classic picture moved from 28 to 32 pixels down in the GUI's 256 line canvas so that it is where the frame has it. Seen whole in both builds: the firmware's loading screen; in the headless build also the card's Daybreak, ScrollNutter, show512 and tilemap demos. The earlier drawing code is still in the tree (T-50).

Cross-ref: CHANGELOG 0.13.0.

## [0.11.0] T-49 — next_card.py leaves the card's free space record as for an empty volume (v0.11.0, 2026-11-16)

Theme: next-support · closed 0.11.0 · 2026-11-16
- **Trigger:** 0.11.0, checking saves to the card after a report that saving did not work in a local run: `CAT` showed "1022M free" on a card with 902M free.
- **Cause:** `scripts/next_card.py` formats the card with pyfatfs and copies the files in, and the library leaves the FSInfo sector as it was for the empty volume: 261,619 clusters free, next free cluster 2. NextZXOS takes both from there.
- **Fix:** the script counts the free clusters in the finished FAT and writes the count and the first free cluster to the FSInfo sector and its backup. `--repair IMAGE` does the same to an existing card in place.
- **Verified:** a rebuilt card reads 230,952 free, first free 30,670, equal to a count of the FAT; saves on it leave the record in step.
- **Not established:** whether this was what failed in the local run. In the sandbox every save tried works in both builds with or without the fix (see T-36's fifteenth finding).

Cross-ref: CHANGELOG 0.11.0.

## [0.11.0] T-42 — NextREG 0x12 and 0x13 are 16K bank numbers; zenzx reads them as 8K pages (v0.11.0, 2026-11-16)

Theme: next-support · closed 0.11.0 · 2026-11-16
- **Trigger:** 0.11.0, while getting NextZXOS to boot. The Next's register reference (`registers.txt` in the distribution, `/docs/extra-hw/io-port-system`) says NextREG 0x12 and 0x13 hold a *16K RAM bank* number, reset values 8 and 11. `layer2BaseBank` (layer2.go) and the port 0x123B mapping (layer2accessport.go) use the value as an 8K page number, and the register file's reset value for 0x12 is 0. NextZXOS writes 9 at start-up; a program that draws Layer 2 through the documented banks would be shown from the wrong memory.
- **Fix:** take the visible and shadow bases as bank numbers (page = 2 x bank), give 0x12 and 0x13 their reset values, and update the Layer 2 tests, which were written against the page reading. The GUI path (videorender_gpu.go) uses the same accessor.
- **Done when:** a Layer 2 picture written to banks 8-10 with NextREG 0x12 at its reset value is displayed, in the headless decode and in the GUI.

Shipped in 0.11.0 (2026-11-16): `layer2BankToPage` (layer2.go) takes both registers as bank numbers, their reset values are 8 and 11, and port 0x123B's mapping uses the same conversion; the Layer 2 tests give bank numbers now, and `TestLayer2UsesBankNumbersAndResetValues` checks the reset values and the mapping. The GUI: `videorender_gpu.go` reads the base through the same accessor, and `TestGLNextPicture` (gl_gui_test.go, dormant guard G-05) draws a frame through the GUI's renderer with a byte in page 16 and NextREG 0x12 at its reset value, and reads that pixel back from the window: pass on 2026-11-16 under Xvfb with software OpenGL. The same test found that NextREG 0x15 had the wrong reset value (T-47).

Cross-ref: CHANGELOG 0.11.0.

## [0.11.0] T-47 — NextREG 0x15 reset value is S U L; the hardware resets to S L U (v0.11.0, 2026-11-16)

Theme: next-support · closed 0.11.0 · 2026-11-16
- **Trigger:** 0.11.0, `TestGLNextPicture`: a Layer 2 picture did not appear in the GUI with NextREG 0x15 left alone.
- **Cause:** the register table gave NextREG 0x15 the value 0x08, the layer order S U L, so Layer 2 was under the ULA until a program wrote the register. The Next's registers.txt gives bits 4-2 the soft reset value 000, S L U. T-29's notes had flagged the stored value as doubtful.
- **Fix:** the reset value is 0x00. `TestNextRegDefaults` asserts it; one GUI test that relied on S U L sets it.

Cross-ref: CHANGELOG 0.11.0.

## [0.11.0] T-46 — GUI-tagged tests had never been run; four were wrong and one starts a browser (v0.11.0, 2026-11-16)

Theme: tooling · closed 0.11.0 · 2026-11-16
- **Trigger:** 0.11.0. The tests without the `headless` tag had only ever been compiled; with a display available they were run for the first time.
- **Found:** (1) `TestHandleLogoMenuResultZenZXWebsiteOpensURL` called `rl.OpenURL` for real: it started a browser, which kept `go test` from ending until its ten minute limit. (2) Three tests in layer2_viewport_test.go expected the 640x256 Layer 2 mode to count with Layer 2 off; since 0.11.0 it counts only while Layer 2 is displayed. (3) `TestInSpanishGroupListsExpectedModels` took the Timex Sinclair 2068, which follows the Spanish group after a separator, for a member of it.
- **Fix:** the opener is a variable (`openURL`, menubar_gui.go) that the test replaces and checks; the three tests turn Layer 2 on; a separator ends a group in the menu test. machines.json was right and is unchanged.
- **Verified:** the whole GUI-build suite, `go test .` without the `headless` tag, passes in under a second under Xvfb with software OpenGL, with `ZENZX_GL_TESTS` set and without.

Cross-ref: CHANGELOG 0.11.0.

## [0.11.0] T-45 — Script key input never reaches the machine in the GUI build (v0.11.0, 2026-11-16)

Theme: tooling · closed 0.11.0 · 2026-11-16
- **Trigger:** 0.11.0, driving NextZXOS in the GUI build with a script: SPACE at the welcome screen did nothing.
- **Cause:** the GUI loop ran the script, then `handleKeyboard` (input.go) set every row of the key matrix to "released" before copying the host keyboard in, then ran the frame. `key`, `type` and `press` therefore never reached the machine in the GUI build, on any model. The headless build has no host keyboard and was not affected.
- **Fix:** `Scheduler.OwnsKeyboard` reports typing in progress or a key held by `press`; while it is true the GUI leaves the matrix alone (`zx.scriptKeys`).
- **Verified:** `TestSchedulerOwnsKeyboardWhileKeysAreDown`; and in the GUI build under Xvfb a script now goes from the welcome screen through the menu into NextBASIC and types a line.

Cross-ref: CHANGELOG 0.11.0.

## [0.11.0] T-43 — SD card is read-only: no CMD24, CMD25 or CMD13 (v0.11.0, 2026-11-16)

Theme: next-support · closed 0.11.0 · 2026-11-16
- **Trigger:** 0.11.0. `SDCardImage` is an `io.ReaderAt`, and sdcard.go has no CMD24 or CMD25 and no CMD13. The firmware and NextZXOS both write to the card in normal use: the firmware saves the chosen video mode to `config.ini`, NextZXOS saves its settings and whatever the user saves. With the card read-only the writes are refused or ignored; nothing has been seen to fail at start-up, but saving cannot work.
- **Fix:** CMD24 (single block write) and CMD25 (multiple), with the data-response token and the busy period the firmware polls for, CMD13 (status), and an image opened read-write, with a flag for a write-protected card. Ochenta's notes record that its cards were once damaged because CMD13 was not answered while writes were accepted; the two belong together.
- **Done when:** a file saved from NextBASIC is on the card image afterwards and the image still mounts on the host.

Done (2026-11-16, 0.11.0): `sdcard_write.go` adds CMD24, CMD25, CMD13, CMD16 and ACMD23; both front ends open the image for writing unless `-sdcard-readonly` is given, and fall back to a write-protected card when the file cannot be written. Run in the headless build: NextBASIC chosen from the menu, `10 REM written from zenzx` entered and `SAVE "zztest.bas"` given; NextZXOS wrote seven blocks with CMD24, each followed by CMD13, and the image afterwards holds `/zztest.bas`, 152 bytes with a +3DOS header and the line in it, among 1,394 files read back by a host FAT library. Unit tests in `sdcard_write_test.go` cover single and multiple block writes, the write-protect switch, a read-only image, an out-of-range block and an abandoned write.

Cross-ref: CHANGELOG 0.11.0.

## [0.11.0] T-41 — Boot ROM stage of the Next chain has no repeatable check (blank FAT32 card, expected stop at TBBLUE.FW) (v0.11.0, 2026-11-16)

Theme: next-support · closed 0.11.0 · 2026-11-16
- **Trigger:** 0.11.0 consolidation. The first boot stage was confirmed by hand against a generated blank FAT32 card (T-36, eleventh finding), and nothing repeats that check. The T-36 history shows eleven findings, most of them written before the code could be built or run.
- **Fix:** a test that builds the blank FAT32 image itself, attaches the boot ROM and DivMMC ROM named by environment variables (third-party files, not in the tree; skip when absent), runs to the point the boot ROM gives up, and asserts the SD command sequence (init, then CMD17 reads of the MBR and volume) and that the boot ROM overlay is still mapped. Register it in the dormant-guards table when written.
- **Done when:** the test passes locally with the ROMs present and is listed as a dormant guard with its first execution record.

Done (2026-11-16, 0.11.0): closed by a wider check than the one asked for. `TestNextZXOSBoots` (next_boot_test.go) runs the whole chain against the real firmware and a card made by `scripts/next_card.py`, and asserts the hand-over, NextZXOS loading its welcome guide from the card, and the main menu after SPACE; the boot ROM stage is its first part. It is dormant guard G-04, first run 2026-11-16 in the sandbox: pass in 1.2 s. The blank-card variant was not written.

Cross-ref: CHANGELOG 0.11.0.

## [0.10.0] T-17 — AY-3-8912 ports (128K-style 0xFFFD/0xBFFD) respond regardless of -model, including 48K which has no AY chip (v0.10.0, 2026-11-12)

Theme: model · closed 0.10.0 · 2026-11-12
- **Trigger:** 2026-08-17, noticed while implementing per-model default configuration for -joystick (this session's work matching real built-in hardware per -model -- see joystick.go's defaultJoystickModeForModel). The AY port dispatch (io.go, 'AY-3-8912 sound chip register select (128K)' / 'data write (128K)' comments) is unconditional -- port 0xFFFD/0xBFFD access works identically regardless of io.memory.is128K, meaning a 48K-model program reading/writing these ports gets real AY chip responses even though real 48K hardware has no AY-3-8912 at all (beeper only).
- **Pre-existing, not a regression from this session's work** -- this gating gap predates the joystick/mouse/TS2068 work entirely; noted here because it's the same underlying architectural principle (-model should imply which hardware genuinely exists) applied to a different subsystem, and was directly observed while auditing port dispatch for the joystick fix.
- **Scope:** genuine 128K-family AY presence, confirmed real: 128K/+2/+2A/+3 all have AY-3-8912 (128K sound); 48K does not (beeper only); TS2068 has its own, per Technical Manual, already correctly gated to its own F5H/F6H ports (ts2068.go) and unaffected by this item.
- **Fix:** gate the existing 0xFFFD/0xBFFD dispatch in io.go behind io.memory.is128K (already tracked per-instance), matching how ts2068WritePort/ts2068ReadPort are already gated behind io.memory.isTS2068. Low-risk, mechanical change -- add the same guard already used for the TS2068 ports.

Cross-ref: CHANGELOG 0.10.0.
## [0.9.2] T-35 — writeNext never mirrored bank-5 screen writes into the cached screen buffer the renderer reads (videoRenderer.Decode), leaving Next-mode boot blank despite correct memory content (v0.9.2, 2026-09-18)

Theme: next-support · closed 0.9.2 · 2026-09-18
Root-caused via direct plus3-vs-next comparison: CPU execution and raw memory content were confirmed byte-for-byte identical between a working -model=plus3 boot and a blank -model=next boot (same ROM, same PC trace, same steady-state idle loop). The divergence was isolated to rendering: DecodeDisplay's raw pre-composite ULA image was already black on the Next run. Root cause: the classic (non-Next) write path mirrors every bank-5 write into zx.screen.bitmap/attributes, a separate cached buffer videoRenderer.Decode reads directly -- but writeNext (the path used whenever isNext=true) only ever wrote through nextPageWrite into the raw ram array, never touching that cache. Fixed by adding the equivalent bank-5 (page 10) mirror to writeNext, matching the classic path's own 0x4000-0x5AFF address range. This also resolves Wave 3 / ti0.r0's placeholder scope (audit Z80N wiring as a real machine mode, confirm no gap beyond opcode semantics) -- the gap found was exactly this, well beyond opcode semantics.

Cross-ref: CHANGELOG 0.9.2.

## [0.9.0] T-34 — NextREG 0x00 (machine ID) default is 0x00; real hardware/VHDL value is 0x0A (v0.9.0, 2026-09-17)

Theme: next-support · closed 0.9.0 · 2026-09-17
Trigger: jnext source comparison (2026-09-15). jnext's NR 0x00 read handler returns a fixed 0x0A (VHDL g_machine_id), matching a documented 2026-07-09 fix: an earlier 0x08 value made NextZXOS take a wrong "emulator" branch at its ROM1 machine-ID check and broke boot. zenzx's nextRegDefaults table currently defaults register 0x00 to 0x00, a value no real firmware or hardware expects.

Done (2026-09-15): nextreg.go's 0x00 entry in nextRegDefaults changed from 0x00 to 0x0A. pkg/... and headless . suites re-run green; go vet clean. Fix is in the working tree, uncommitted -- close once it ships in a tagged zenzx release (the register's closure procedure needs a --version to close against; there is no pending zenzx release yet).

Cross-ref: CHANGELOG 0.9.0.

## [0.9.0] T-33 — NextREG NEXTREG-opcode write must bypass port-0x243B select latch, not clobber it (v0.9.0, 2026-09-17)

Theme: next-support · closed 0.9.0 · 2026-09-17
Trigger: jnext source comparison (2026-09-15) against zenzx's ti0.r1 NextREG implementation, per real hardware and jnext's own bug history (GH #54, RevivalSurvival.nex). zen80's NEXTREG opcode (ED 91/92) was implemented as two sequential ioOut() calls -- select then data -- identical to a raw port OUT. Real Next hardware writes the opcode's target register directly into the register file and never touches the port-0x243B select latch; jnext shipped the port-pair version once, hung a real game's raster-wait loop on it, and fixed it. zenzx's nextreg_test.go used to assert the WRONG invariant (TestNextRegRawPortAndInstructionAgree treated port-pair/opcode equivalence as correct).

Done (2026-09-15): zen80's Z80.NextregWrite hook is implemented, tested, committed and tagged v0.5.7 in the zen80 working tree (commit c928706) -- but NOT yet pushed to github.com/ha1tch/zen80. SpectrumIO now has nextRegWriteDirect/nextRegOpcodeWrite (nextreg.go); EnableZ80N wires zx.cpu.NextregWrite = zx.io.nextRegOpcodeWrite (zenzx.go). nextreg_test.go's TestNextRegRawPortAndInstructionAgree was split into TestNextRegRawPortAndInstructionAgreeOnValue (keeps the still-true value-agreement assertion) and the new TestNextRegOpcodeDoesNotTouchSelectedReg (asserts the two paths correctly DIVERGE on `selected` state). Full pkg/... and headless . suites green; go vet clean.

Remaining: zenzx's go.mod names zen80 v0.5.7 in its require line, but that version is not yet reachable via `go get` from origin (tagged locally in the zen80 working tree only). Development in this sandbox session used a temporary local-path replace directive, since removed for release hygiene (a sandbox-local filesystem path cannot ship). Close this item only once zen80 v0.5.7 is pushed AND `go mod tidy` + the full test suite are re-run clean against the real published module (no replace directive needed at that point -- the require line already names the right version).

Cross-ref: CHANGELOG 0.9.0.

## [0.7.0] T-28 — Extended MMU: 8K-page paging beyond the 128K/+3 ceiling (v0.7.0, 2026-09-15)

Theme: next-support · closed 0.7.0 · 2026-09-15

Tier 0 prerequisite. memory.go is architecturally a fixed three-slot, 16K-granularity model (ramBankLow/ramBankHigh/ramBankTop mapped straight to 0x4000/0x8000/0xC000) -- the 128K/+3 shape, not the Next's eight independent 8K MMU slots. This is a replacement of the current paging subsystem, not an extension of it. Target: at least the 768K-1MB unexpanded-machine tier; NextREG MMU slot registers (0x50-0x57) plus the 0xdffd extension port.

**jnext-comparison addendum (2026-09-15):** when NR 0x50/0x51 (slots 0/1) are written 0xFF, real hardware/jnext re-engages legacy ROM auto-paging (sram_rom-derived) rather than treating 0xFF as an ordinary page number for those two slots specifically (slots 2-7 do treat 0xFF as "slot inactive" -- jnext had a real boot-loop bug from getting slots 0/1 wrong the same way). Build this distinction into the NR 0x50/0x51 write path from the start rather than generalizing todays flat store-and-read.

**Implementation split (2026-09-15):** decomposed into 4 dependency-ordered parts, matching Wave 5's own item numbering (#19 -> #26 -> #27 -> #28) exactly:

1. New 8-slot/8K-granularity paging model itself (wave item #19).
2. Wire NextREG 0x50-0x57 register writes to actually drive the new slot mapping (wave item #26) -- currently nextreg.go's `mmuSlots` only mirrors the register value with no effect on memory access.
3. The 0x50/0x51-into-legacy-ROM special case from the jnext-comparison addendum above, plus extension port 0xdffd (wave item #27).
4. Compatibility regression + a Next-mode multi-slot test program (wave item #28).

**Done (part 1 of 4, 2026-09-15):** memory.go gains `isNext`/`nextSlots [8]uint8`/`nextExtraRAM` as new, additive fields -- `ramBankLow`/`ramBankHigh`/`ramBankTop` and every existing 48K/128K/+3/TS2068 Read/Write branch are completely untouched (isNext defaults false; both `Read`/`Write` check it first, before every existing branch, and fall through unchanged when false). Page numbering follows the jnext-VHDL-verified convention already used by nextreg.go's `mmuBootDefaults`: classic 16K bank N is 8K pages 2N/2N+1. Pages 0-15 deliberately alias the existing `ram` array (not a copy) so Next-mode and classic-128K/+3 addressing of the same bank see the same bytes; pages 16-95 (640K) are backed by a new `nextExtraRAM` array, for a 768K total matching the "at least the 768K-1MB unexpanded-machine tier" target. An out-of-range page (96 and above, which includes treating 0xFF as the literal page number 255) reads 0xFF and drops writes -- safe, well-defined, but NOT yet the real 0x50/0x51-defers-to-legacy-ROM behaviour the addendum above describes; that upgrade is part 3 (item #27), not this change. `EnableNext()` added (mirrors `Enable128K`/`EnablePlus3`) but nothing calls it yet -- no register wiring exists until part 2. New `memory_test.go`: 7 tests covering isNext-defaults-false, 48K/128K unaffected, the bank-aliasing property, the extra-store, out-of-range handling, and all 8 slots independently. Full `pkg/...`+headless `.` suite green (pre-existing tests untouched, zero failures); `go vet`/`gofmt` clean.

**RAM-capacity correction (2026-09-15, same-day follow-up to part 1 above):** the "768K total matching the 'at least the 768K-1MB unexpanded-machine tier' target" figure just above was wrong -- that wording is T-28's own stated floor, not a ceiling, and was mistakenly implemented as if it were the target. jnext's own source (`src/memory/ram.h`, `doc/analysis/FPGA-REPO-ANALYSIS.md`) states the real hardware supports 768K (unexpanded board) OR 2048K (expanded board), and jnext's emulator always allocates for the 2048K ceiling. Corrected to match: `nextTotalPages` raised from 96 to 224, `nextExtraRAM` from 80 to 208 pages. The real ceiling is honestly 1792K (224 pages), not a flat 2048K -- confirmed directly against jnext's `Mmu::rebuild_ptr` (`src/memory/mmu.cpp`) that NR 0x50-0x57 register values 0xE0-0xFF (32 of 256) are a reserved sentinel range gated *before* any RAM lookup on real hardware (inactive slot / floating bus for slots 2-7, legacy-ROM-defer for slots 0/1), never ordinary RAM regardless of capacity; jnext's own further `+0x20` wrap on top of that is specific to how jnext lays out its ROM-in-SRAM images and does not apply to zenzx, which keeps ROM in a separate array. Added `nextReservedPageBase = 0xE0` and a new `nextPageIsReserved` helper so 0xE0-0xFF is explicitly gated in `nextPageRead`/`nextPageWrite`, rather than merely falling outside `nextTotalPages` by accident of array bounds -- correct regardless of any future change to `nextTotalPages`. `memory_test.go`'s `TestNextMMUOutOfRangePage` updated (renamed in spirit, not name) to exercise the reserved range explicitly and to assert the boundary: the last *ordinary* page (`nextTotalPages-1` = 0xDF) is real, working storage, not reserved. Full `pkg/...`+headless `.` suite re-verified green; `go vet`/`gofmt`/`repoman badcode check` clean.

**Done (part 2 of 4, item #26, 2026-09-15):** NextREG 0x50-0x57 writes now genuinely drive the paging model, not just the `mmuSlots` readback mirror. New `SpectrumMemory.SetNextSlot(slot, page)`; `nextreg.go`'s `nextRegWriteDirect` (shared by both the raw-port and NEXTREG-opcode write paths) and `SetNextRegister` (snapshot restore) call it whenever reg is 0x50-0x57. Also fixed a gap this surfaced: `newNextRegisterFile`'s `mmuBootDefaults` were seeding `io.nextRegs.mmuSlots` but never reaching `memory.nextSlots` at construction (no `*SpectrumMemory` reference exists inside that standalone constructor) -- `NewSpectrumIO` now pushes the computed defaults into memory right after construction, so the two start in agreement rather than only converging after the first register write. Deliberately does NOT yet implement the slots-0/1-defer-to-legacy-ROM special case for the reserved range (0xE0-0xFF, including 0xFF) the jnext-comparison addendum above describes -- that remains item #27, layered on top of this wiring. New tests in `nextreg_test.go`: `TestNextRegMMUBootDefaultsReachMemory`, `TestNextRegMMUWriteDrivesPaging` (raw-port path, proves a register write changes `Read`/`Write` behaviour, not just the readback), `TestNextRegMMUOpcodeWriteDrivesPaging` (same proof via the NEXTREG-opcode path). Full suite green; `go vet`/`gofmt`/`repoman badcode check`/`repoman provenance check` clean.

**Done (part 3 of 4, item #27, 2026-09-15):** both sub-items from the split above, implemented as an entirely additive path per explicit user direction ("no need to restructure, recreate, or retouch anything about the legacy memory system... build on the side next to it") -- zero existing classic-mode field or function (SetPaging, SetPlus3Paging, port7FFD, port1FFD, ramBankLow/High/Top, romBank, pagingLocked, screenBank, the `ram` array) is read, written, or called by any of this.

*0x50/0x51-into-legacy-ROM special case:* new `nextSlotDefersToLegacyROM(slot, page)` helper (memory.go) implements the jnext-comparison addendum's own distinction -- slots 0/1 holding a reserved-range page (nextPageIsReserved, 0xE0-0xFF) defer to the machine's current ROM selection instead of the plain floating-bus treatment slots 2-7 keep. `readNext`/`writeNext` updated to check it before falling through to `nextPageRead`/`nextPageWrite`; the deferred case reads `m.rom[m.romBank]` directly (slot 0 -> ROM offset 0x0000-0x1FFF, slot 1 -> 0x2000-0x3FFF) and drops writes, reusing the EXISTING classic ROM storage and `romBank` selection entirely -- no new ROM state, no jnext-style SRAM-shift duplication. Confirmed against jnext's `Mmu::rebuild_ptr` (`src/memory/mmu.cpp`).

*Port 0xDFFD extended paging:* new, wholly separate storage (`nextLegacyBank7FFD`/`nextLegacyPortDFFD` fields) and methods (`SetNextLegacyBank7FFD`, `SetNextLegacyPortDFFD`, `recomposeNextLegacySlots`, all in memory.go) recompose Next MMU slots 6/7 from the last 0x7FFD and 0xDFFD writes, mirroring jnext's own principle ("the legacy ports maintain no map of their own: they recompose the same eight slots") without touching the classic ports' own handlers at all. Bit mapping reimplements jnext's `compose_bank_` bit-for-bit (confirmed against `src/memory/mmu.cpp`, not re-derived from a written description): bank(2:0)=7FFD(2:0), bank(4:3)=DFFD(1:0), bank(5)=DFFD(2), bank(6)=DFFD(3); the composed 16K bank's two 8K halves land in slots 6/7 via the existing `SetNextSlot`. A composed bank at or beyond the 224-page ceiling needs no separate bounds check -- it lands at or past `nextReservedPageBase` and the existing `nextPageIsReserved` gate in `nextPageRead`/`nextPageWrite` already gives correct floating-bus/dropped-write behaviour. `io.go`'s WritePort gets `SetNextLegacyBank7FFD` called alongside the existing `SetPaging` in the 0x7FFD block (side by side, neither aware of the other), and a new dispatch block for 0xDFFD itself, decoded per jnext's own VHDL citation (`src/memory/mmu.h`: A15:12="1101" AND A1:0="01", i.e. `port&0xF003==0xD001`). Both setters are complete no-ops when `isNext` is false. Read-side 0xDFFD is deliberately NOT implemented -- scoped write-only, matching jnext's own DFFD-readback-tied-to-Multiface (a peripheral zenzx doesn't model) rather than inventing an unfounded readback value.

*Real port-decode collision found and fixed while wiring 0xDFFD:* `0xDFFD & 0xC002 == 0xC000`, identical to zenzx's pre-existing AY-3-8912 register-select decode mask (conventionally targeting 0xFFFD). This collision predates T-28 and was harmless before -- nothing cared about the address 0xDFFD specifically -- but would have made a genuine Next-mode 0xDFFD write also silently corrupt `io.ayRegister`. Fixed with a narrow, address-exact exclusion: `port&0xC002==0xC000 && !(io.memory.isNext && port==0xDFFD)`, provably a no-op on every classic 48K/128K/+3/TS2068 machine (isNext is always false there). Deliberately does NOT change behaviour on a non-Next machine -- 0xDFFD still falls through to the pre-existing AY register-select interpretation there, unchanged, since 0xDFFD has no defined role at all on those machines and this is purely a compatibility/no-op boundary, not a general-purpose decode fix. A full re-scan of every other `port&0x...` mask in `io.go`'s WritePort/ReadPort confirmed no further collision with 0xDFFD or with the 0x7FFD block's now-dual dispatch.

*Known, explicit limitation:* the classic 0x7FFD paging-lock bit (bit 5) is NOT modelled for this Next-mode legacy-port recomposition path -- `SetNextLegacyBank7FFD` recomposes unconditionally on every 0x7FFD write regardless of the classic `pagingLocked` state (which SetPaging honours independently, on its own separate call). Real Next hardware's actual lock semantics for this path were not investigated; recorded here as an open gap rather than silently ignored.

Tests: 9 new cases in `memory_test.go` (`TestNextSlotDefersToLegacyROMPure`, `TestNextReadWriteDefersToLegacyROMSlots0And1`, `TestNextReadWriteReservedSlots2To7StillFloat`, `TestNextReadDefersToLIVERomBankNotCached`, `TestNextLegacyPagingNoOpUnlessNext`, `TestNextLegacyPagingBankComposition`, `TestNextLegacyPagingActuallyPagesMemory`, `TestNextLegacyPagingOverflowIsReservedNoExtraBoundsCheck`, `TestNextLegacyPagingDoesNotTouchClassicFields`) and 6 in `nextreg_test.go` (`TestPort7FFDDrivesNextLegacySlots67`, `TestPortDFFDDrivesNextLegacySlots67`, `TestPortDFFDNoOpOnClassicMachine`, `TestPortDFFDOnClassicMachinePreservesPreexistingAYCollision`, `TestPortDFFDOnNextMachineDoesNotCorruptAYRegister` -- the AY-collision regression proof -- and `TestPortFFFDStillSelectsAYRegisterOnNextMachine`). `TestNextMMUOutOfRangePage` (part-1-era) updated to exercise slot 2 instead of slot 0, since slot 0's reserved-range behaviour is now genuinely different (legacy-ROM-defer, not floating-bus) and that split is covered by the new tests above. Full `pkg/...`+headless `.` suite green; `go vet`/`gofmt`/`repoman badcode check`/`repoman provenance check` clean.

**Done (part 4 of 4, item #28, 2026-09-15):** the two halves of the split's own description, both added to `nextreg_test.go`.

*Compatibility regression:* `TestT28CompatibilityRegressionClassicModels` (table-driven over 48K/128K/+3) is the first T-28 test to build the machine through the SAME production entry point real `-model` selection uses (`NewZenZX` + `LoadROMBytes`, which is what actually decides `is128K`/`isPlus3` from ROM size) rather than the bare `newTestMemoryAndScreen()` every earlier T-28 unit test constructs, and to drive it through the real Z80 CPU executing actual `OUT`/`LD` machine code rather than calling `SetPaging`/`Read`/`Write` directly. Confirms `isNext` stays false and `nextSlots` stays exactly as `NewSpectrumIO` seeded it at construction, for all three classic ROM sizes, after a genuine CPU-executed paging-and-round-trip program -- the regression proof `EnableZ80N`'s own doc comment depends on ("none of ZenZX's existing -model options are the Next").

*Next-mode multi-slot test program:* `TestT28NextModeMultiSlotProgram` is a single continuous Z80N program, executed entirely through `zx.cpu.Step()` (no direct Go-level `SetNextSlot`/`Read`/`Write` calls standing in for what a real program would do), that exercises items #19/#26/#27 TOGETHER against four different slots in one scenario -- every earlier T-28 test exercises one mechanism at a time. Sequence: `NEXTREG $50,$FF` (slot 0 -> ROM-defer sentinel), `NEXTREG $52,$10`/`$53,$11` (slots 2/3 -> extra-store pages 16/17), then real `OUT (C),A` writes to ports 0x7FFD and 0xDFFD composing bank 9 into slots 6/7 (page 18/19) -- followed by real `LD (nn),A`/`LD A,(nn)` round-trips through slots 0, 2, 3, and 6, each checked individually (slot 0 must read the seeded ROM byte 0xAB; slots 2/3/6 must round-trip their written values and land in the exact `nextExtraRAM` index the driving mechanism claims). Caught one real test-authoring bug in its own first draft (register numbers for NEXTREG-driven slots are `0x50+slot`, not the bare slot number -- an initial `NEXTREG $02,$10` silently hit the unrelated "reset" register instead of slot 2, and the resulting failure's own diagnostic output is what surfaced it) -- fixed before this entry was written, not left as a known issue.

Full `pkg/...`+headless `.` suite green; `go vet`/`gofmt`/`repoman badcode check`/`repoman provenance check` clean.

Cross-ref: CHANGELOG 0.7.0.

## [0.6.13] T-27 — NextREG port handler / register file (0x243B select, 0x253B data) (v0.6.13, 2026-09-15)

Theme: next-support · closed 0.6.13 · 2026-09-15
Tier 0 prerequisite for ZX Spectrum Next emulation. zen80's NEXTREG instruction (ED 91/92) already funnels through the CPU's own ioOut to these two ports, so the instruction-decode half is done; io.go has no branch for either port yet. Needs a NextREG register file, full 16-bit port match (Next ports are fully decoded, unlike the ULA's partial decode), and read-back semantics matching the SpecNext wiki's per-register spec.

Cross-ref: CHANGELOG 0.6.13.

## [0.6.10] T-26 — Trace harness (zzz_trace_harness_test.go) is hardwired to one boot+tape-load scenario; can't trace arbitrary code, memory reads, or port writes (v0.6.10, 2026-08-29)

Theme: tooling · closed 0.6.10 · 2026-08-29


Resolved (2026-08-29): Wave 1 (`docs/WAVE_TRACKING.md`) complete, 10/10
items, all verified with real data (hand-checked expected values, or
cross-checked against `zenas`/`BootDetector` directly). Items 4/9 were
briefly ◐ pending zen80 v0.5.5 landing as a real tag; confirmed pushed
and live 2026-08-29 (`git ls-remote`, and the real Go module proxy),
`go.mod`'s local `replace` removed, dependency re-pulled clean with a
real `go.sum` checksum, both re-verified against the published module.
See `docs/TRACE_GENERALIZATION_TRACKING.md` for full per-stage detail.
Wave 2 (disassembly, multi-window tracing, conditional-breakpoint
expressions, a live debugger, etc.) remains priced but unscheduled --
separately-scoped future work, not part of what this item required.

Trigger: session review 2026-08-28, following up on the T-16/T-22 trace instrumentation added in 0.6.1. TestTraceHarness only reaches a traceable state via boot-48K-then-type-LOAD""; hooks install after that setup, so nothing before a LOAD command is observable regardless of config. zen80 v0.5.1 exposes only three debug hooks (DebugPCHook/DebugMemWriteHook/DebugIOInHook) plus an existing but unused M1Hook (interrupt/prefix-fetch context, already sufficient for interrupt tracing -- no zen80 change needed there). Missing: memory-read and port-write hooks (need zen80-side additions, see docs/proposals/zen80-tracing-hooks.md), pluggable non-tape setup, model selection, pre-setup hook installation, opcode disassembly, conditional stops, coverage aggregation. See docs/TRACE_GENERALIZATION_DEVELOPMENT_PLAN.md / docs/TRACE_GENERALIZATION_TRACKING.md for the staged plan. Supports T-22 and T-24's continued investigation, not required by them.

Cross-ref: CHANGELOG 0.6.10.

## [0.6.8] T-05 — fdc_read_test.go depends on a sandbox-only absolute path (v0.6.8, 2026-08-28)

Theme: test · closed 0.6.8 · 2026-08-28


- **Trigger:** session review 2026-08-17. The test opens `/mnt/user-data/uploads/artist.dsk` and skips when absent, so it can never run on any other machine and is invisible to CI.
- **Fix:** take the image path from an environment variable (e.g. `ZENZX_TEST_DSK`) or ship a small redistributable DSK under `testdata/`; update G-02's invocation accordingly.

Cross-ref: CHANGELOG 0.6.8.

## [0.6.6] T-25 — GUI build's -tapemode flag never gained a turbo case; passing -tapemode=turbo to the GUI silently falls back to fast instead of erroring or working (v0.6.6, 2026-08-27)

Theme: tape · closed 0.6.6 · 2026-08-27


Found auditing README.md/MANUAL.md for staleness: zenzx_gui.go's own -tapemode flag.String default/help text still says only "fast or accurate", and the only comparison in its parsing is 'if strings.ToLower(*tapeMode) == "accurate"' -- any other value, including "turbo", falls through to fast with no warning. Turbo mode itself (loadingFrame + zen80 FastMem, T-19 fixed for 128K) is headless-only as a result, not by documented design -- there's no indication this was a deliberate scope decision, just that GUI wiring was never added when turbo was built. Low priority since turbo's actual value (a real but modest ~15-27% wall-clock speedup, not a correctness improvement) matters most for batch/headless use where it already works; but the silent fallback (no error, no warning) means a GUI user who tries -tapemode=turbo gets fast mode's very different behavior without being told.

Cross-ref: CHANGELOG 0.6.6.

## [0.6.0] T-16 — No memory contention modeling exists for any model (v0.6.0, 2026-08-25)

Theme: timing · closed 0.6.0 · 2026-08-25


- **Trigger:** 2026-08-17, surfaced while scoping TS2068 Stage 6 (docs/TS2068_TRACKING.md). Checked directly rather than from memory: no contention modeling exists anywhere in zenzx or zen80 (the CPU core), for any model -- not 48K, not 128K/+3, not TS2068. Every memory access currently takes the same fixed number of T-states regardless of address or where the ULA is in its own scanline at that moment.
- **Scope, deliberately general, not TS2068-specific:** TS2068 Stage 6 was originally framed as "TS2068's own contention pattern, not the standard ULA's" -- but there is no standard-ULA baseline to diverge from either. Building contention only for TS2068 would leave it more cycle-accurate than the models it shares a substrate with, a strange asymmetry to introduce as a side effect of one model's checklist. This item covers the real gap: general per-address, per-scanline contention timing, threaded into the CPU stepping loop, correctly synchronized with ULA scanline position -- new infrastructure affecting every model equally, not an adaptation of an existing table.
- **Impact, honestly scoped:** affects cycle-exact border effects, 128K "snow," and a handful of extremely timing-sensitive demos/loaders that depend on exact contention stalls. Does not affect ordinary program correctness -- BASIC, most games, and everything verified across TS2068 Stages 1-5 works fine without it. Not a functional blocker for any current feature.
- **Fix:** design a per-address/per-cycle contention timing table for each model's own ULA (48K, 128K/+2/+2A, +3, and eventually TS2068's own SCLD-driven pattern per Technical Manual 2.1.8.2/2.1.8.3), integrate into the CPU step loop's cycle accounting, verify against known contention-sensitive test cases (e.g. the community's standard contention test ROMs) before trusting it.

Cross-ref: CHANGELOG 0.6.0.

## [0.5.1] T-19 — Turbo tape mode (-tapemode=turbo) fails on 128K titles; 48K confirmed correct (v0.5.1, 2026-08-24)

Theme: tape · closed 0.5.1 · 2026-08-24


**Trigger:** Building Turbo mode (zen80 FastMem/FastPort fast path + zenzx per-block memory/paging sync, turbo.go) for accelerated tape-loading batch runs. 48K validated correct (Chase HQ: byte-identical outcome to accurate mode, confirmed via direct screenshot comparison and full memory diff, after fixing a real zen80 decode.go bug -- 16 call sites bypassing the fast path entirely due to a receiver-variable-name mismatch the original mechanical substitution missed).

128K fails: reproduced with Cybernoid 2 (Cybernoid II - The Revenge/Cybernoid 2 - Side 1.tzx, -model=128k). Symptom: CPU ends up stuck at PC=0x0049 (idling in the interrupt handler) instead of the game's own code, despite Tape.Position reaching the tape's natural end -- the same symptom class the decode.go fix resolved for 48K, but confirmed to be a DIFFERENT root cause:
  - A comprehensive grep across every zen80 file for any remaining direct <recv>.Memory./<recv>.IO. call (any receiver name, not just z./cpu.) found nothing else missed -- the CPU core itself is confirmed complete.
  - Not a bank-switching/paging bug: traced with a temporary ioOut hook: zero writes to port 0x7FFD occur during the entire observed loading window, so turboSyncBlock's paging-port replay was never even exercised.
  - Snapshot diff (zx.SaveSnapshot at the same checkpoint in both modes) confirms accurate mode reaches real game code (PC=0xF9D3) while turbo mode does not, at a substantial (~35,768 T-state) cycle-count divergence -- a genuine stall, not just interrupt-timing sampling noise.

**Root cause (found after the above was filed):** the original write-based reproduction was itself unreliable -- traced to a flawed test-harness boot wait (a fixed 150-frame guess rather than a proper BootDetector), which caused both modes to fail identically in a masking way and produced a false "zero writes to port 0x7FFD" reading. With correct boot detection, Cybernoid 2 genuinely, actively switches RAM and ROM banks via port 0x7FFD *during ongoing code execution* -- not just between tape blocks (47 paging writes observed in the first 4000 turbo frames alone, including rapid ROM-bank toggling and sequential RAM-bank cycling). The flat 64K FastMem snapshot, synced only at tape-block boundaries, structurally could not represent this: between syncs, any bank switch left FastMem showing stale data for whichever bank had just been paged out.

**Fix:** replaced the flat-snapshot model with a page-based one matching SpectrumMemory's own real layout exactly -- a fixed pool of 12 independently-addressable 16K pages (turbo_pages.go: `MemoryPageCount`, `MemoryPage16k`, `MemoryFlags`; pages 0-3 = ROM banks 0-3, pages 4-11 = RAM banks 0-7), with a `turboSlotPage[4]` tracking which page is currently mapped at each of the four logical 16K slots. zen80 gained a new hook, `FastPortWriteOut` (fires on every OUT while FastPort is active, mirroring FastPortReadIn's existing design), letting turboRemap fire immediately the moment a paging write actually happens, not at the next block boundary -- swapping FastMem's affected slot to the newly-selected page there and then, preserving whatever the loader had just written to the page being paged out. turboSyncBlock (renamed turboSyncToReal) now writes back all 12 pages, not just the four currently mapped, since a bank the loader paged out earlier still needs its data to land in real SpectrumMemory. Scope: standard 128K/+2/+3 paging via port 0x7FFD only; +3's "special" all-RAM paging mode (0x1FFD) and TS2068's separate Extension-ROM scheme are out of scope for now, both being rare for tape-based titles specifically.

Verified: Cybernoid 2 now reaches real game code (PC=0xF9D9, IM=2, I=0xFE -- identical to accurate mode) and produces a screenshot pixel-identical to the accurate-mode ground truth, via the real CLI end to end. Chase HQ (48K) re-verified still correct after the rewrite. Performance: ~14-17% turbo-vs-accurate speedup (down from ~23% pre-fix on 48K, due to the small added per-OUT check in FastPortWriteOut) -- a reasonable trade for 128K correctness, which was completely broken before this fix, not merely slow.

Cross-ref: CHANGELOG 0.5.1.

## [0.4.35] T-18 — FLASH attribute stopped blinking on every model (not just TS2068) -- updateFlash() documented as called from Render but no call site existed (v0.4.35, 2026-08-18)

Theme: video · closed 0.4.35 · 2026-08-18


screen.updateFlash() advances flashTickTock on a 320ms cadence and is the sole driver of the classic FLASH ink/paper swap. videorender.go's design comment documents it as "called once per frame from the GUI's live path (zx.Render(), via DisplayManager) unconditionally regardless of which renderer is active" -- but no such call site existed anywhere in the codebase. flashTickTock never advanced past its initial value, so FLASH-attributed cells never blinked, on every model including standard 48K/128K/+2/+2A/+3, not just TS2068. Caught because TS2068 hi-colour work correctly added TestHicolourFlashIgnored (confirming hi-colour deliberately ignores FLASH) but no equivalent test existed confirming standard mode actually honours it -- a coverage gap, not a design gap. Fixed: added the missing dm.screen.updateFlash() call as the first line of display.go's Render(). 4 new tests (flash_regression_test.go): standard-mode swap correctness, flashEnabled=false fully disabling (not just un-ticking), and updateFlash()'s own 320ms timing directly.

Cross-ref: CHANGELOG 0.4.35.

## [0.4.23] T-12 — -model ts2068 not implemented (own ROM, 8K-chunk banking, dedicated AY ports) (v0.4.23, 2026-08-17)

Theme: model · closed 0.4.23 · 2026-08-17


- **Trigger:** 2026-08-17, deferred from T-11 (hi-colour mode is testable and implementable against the existing models without a TS2068 boot ROM; this is a separate, larger feature).
- **ROMs in-tree, 2026-08-17:** `rom/ts2068-0.rom` (16384 bytes, Home) and `rom/ts2068-1.rom` (8192 bytes, Extension) -- verified sizes, sensible Z80 reset vectors, expected embedded copyright strings. Provenance and copyright status in `rom/TIMEX.txt`.
- **Staged plan, 2026-08-17:** `docs/TS2068_DEVELOPMENT_PLAN.md` (frozen design) and `docs/TS2068_TRACKING.md` (stage status) -- 6 stages: model skeleton/ROM loading/chunk-0 banking/boot; NTSC frame timing; `CHNG_VID` (the only deferred-list item in scope, per direction -- everything else deferrable stays unimplemented indefinitely: full 8-chunk Dock/cartridge banking, the TS2040 printer protocol, composite/RF video generation); AY ports + joystick; tape I/O; memory contention. Guiding principle recorded in the plan: because zenzx runs the *real* ROM images, most of what looks like "implement ROM service X" is actually "get the memory/port/timing substrate correct and let the real ROM code do it" -- this reshapes `CHNG_VID` specifically from "reimplement this routine" to "verify it executes correctly once chunk-0 banking is right."
- **Stages 1-2 done, 2026-08-17** (`ts2068.go`, `ts2068_test.go`): boots to the real copyright screen and, confirmed by injecting a keypress, to a genuinely responsive BASIC-ready prompt (`PRINT 9` [ENTER] produces `9` and `0 OK`) -- not just a static screenshot that looks the same whether idling correctly or stalled. Genuine NTSC frame timing (58688 cycles/frame, not the PAL default) verified per-instance, every other model confirmed unaffected. Detail in `docs/TS2068_TRACKING.md`.
- **Stage 3 done, 2026-08-17, rescoped from the original plan** (`ts2068.go`, `ts2068_test.go`): real TS2068 software engaged hi-colour mode via a direct port `FFH` write, not the documented Extension ROM `CHNG_VID` service -- and the hardware never auto-clears the screen, so real software (and this test) cleared the hi-colour plane by hand. Implemented dynamic renderer switching on the guest's own port write. Verified end to end with garbage pre-seeded in the framebuffer to prove the clear is real, not assumed. The full real-service-call route was attempted and hit a genuine, undiagnosed divergence -- filed separately as T-15, not blocking since real software didn't depend on that path either.
- **Stage 4 done, 2026-08-17** (`ts2068.go`, `ts2068_test.go`, `input.go`): AY sound chip at `F5H`/`F6H` reusing the existing AY emulation via a new dispatch path only; joystick via AY register 14, both ports (address bit 8/9), active low, always on for this model. Detail in `docs/TS2068_TRACKING.md`.
- **Stage 5 done, 2026-08-17** (`tape.go`, `tape_fastload_test.go`): fast mode reuses T-02's hardened `trapLoad` unchanged -- `R_TAPE`/`W_TAPE` confirmed by direct disassembly to be a byte-for-byte relocated copy of the standard ROM's tape routines, same register contract, only the trap addresses and a new chunk-0-aware context check differ. Accurate mode confirmed genuinely working with zero TS2068-specific code, both verified via real ROM execution (not assumption). Detail in `docs/TS2068_TRACKING.md`.
- **Stage 6 postponed, 2026-08-17, spun out as a general item.** Investigation found no memory contention modeling exists for *any* model, zenzx or zen80 alike -- meaning this was never really "TS2068's own pattern vs the standard one," since there is no standard-model baseline to diverge from. Filed as T-16, deliberately not TS2068-scoped, rather than built one-off here (which would have left TS2068 more cycle-accurate than the models it shares a substrate with for no good reason). Affects cycle-exact border effects and a handful of timing-sensitive demos only, not ordinary program correctness -- everything verified across Stages 1-5 already works without it.
- **Closed, 2026-08-17: 5 of 6 stages complete, the 6th a deliberate, recorded decision (T-16) rather than a stalled task.** `-model ts2068` boots to a genuinely responsive BASIC prompt, with real NTSC timing, dynamic hi-colour switching, AY sound and built-in joystick, and both tape modes -- all verified against the real ROM throughout, not assumed. `README.md`, `docs/KNOWN_ISSUES.md`, and `smoke_headless.sh` (the release-gate boot check) updated to reflect TS2068 as a supported model, not just code that happens to exist.

Cross-ref: CHANGELOG 0.4.23.

## [0.4.21] T-02 — ROM-trap tape LOAD path unverified end to end (v0.4.21, 2026-08-17)

Theme: tape · closed 0.4.21 · 2026-08-17


- **Trigger:** README Known issues (0.4.1). The instant-inject fast-tape path is verified byte-identical for `.tap`/`.tzx` CODE blocks. The separate ROM-trap path that intercepts a guest's own `LOAD ""` from BASIC in `-tapemode=fast` is present but has never been driven end to end.
- **Fix:** write a headless zenscript that types `LOAD ""` against a known tape and asserts on screen state; either verify the trap or remove the dead path.

Cross-ref: CHANGELOG 0.4.21.

## [0.4.17] T-14 — AMX mouse not implemented (raw quadrature encoder via Z80 PIO, not a position register) (v0.4.17, 2026-08-17)

Theme: input · closed 0.4.17 · 2026-08-17


- **Trigger:** 2026-08-17, deliberately deferred from the -mouse kempston implementation (0.4.14). -mouse amx is a recognised flag value that hard-errors with an explanation (mouse.go's ParseMouseMode), rather than being silently accepted and doing nothing.
- **Why this is a different, harder problem than Kempston Mouse:** per the Sinclair Wiki's AMX Mouse technical page, AMX uses a Z80 PIO chip and exposes movement as a raw quadrature-encoded signal, not an accumulated position register: port 0x1F bit 0 carries X-axis quadrature (note: same port as Kempston Joystick -- real hardware conflict, would need explicit mutual exclusion if both are ever selectable together), port 0x3F bit 0 carries Y-axis quadrature, port 0xDF carries the 3 buttons. Software decodes direction from the relative phase/timing of transitions on these bits, sampled repeatedly -- fundamentally a bit-level timing problem, not a value-write problem the way Kempston's three byte-ports are.
- **Existing-emulator research (2026-08-17, condensed):** before the primary source below was available, checked four active open-source emulators (Fuse, ZEsarUX, SpecEmu, SpecIde) directly in source -- none had a working AMX implementation; only a vestigial, unused `.szx` snapshot-format chunk in Fuse's `libspectrum`. Full trail in `docs/RESOLVED.md` once this closes.
- **Primary source obtained, 2026-08-17 -- the gap above is now closed.** A genuine AMX Art tape (`AMXART_1.TAP`, 32 blocks, multiple loadable tools) was supplied this session. Extracted every CODE block to its correct load address (from the header's param1) and disassembled with the `z80dis` package, then confirmed the ports found by disassembling `ART 1.1`'s own copy independently -- both agree exactly, so this is the real, consistent protocol, not a one-off routine.
- **The mechanism is materially simpler than assumed, and different in kind:** it is not software-side quadrature phase-decoding at all. AMX's Z80 PIO does the edge-detection and debounce in hardware and delivers **one interrupt per step of movement**, direction pre-decoded into a single bit. The interrupt handler's entire job is: read the port once, look at bit 0, increment or decrement a position counter by exactly 1.
  ```
  ; X-axis interrupt handler (installed via IM2 vector table; identical
  ; pattern at 0x82E7 in ART 1.1 and 0xE843 in the AMX driver block)
  PUSH AF
  IN A,(0x1F)        ; ONE read per interrupt, not a sampling loop
  ...
  AND 1              ; bit 0 = direction
  JR NZ,+
  INC (HL)           ; bit0=0: step one way
  JR ++
  DEC (HL)           ; bit0=1: step the other way
  ...
  EI
  RETI
  ```
  Y-axis: byte-for-byte the same pattern, different vector, port `0x3F`.
- **Setup, before the handlers are live**: `IM 2`; `LD A,0xE9 / LD I,A`; then PIO control-word writes -- `0x4F, 0xD0, 0x87` to port `0x5F` and `0x4F, 0xD2, 0x87` to port `0x7F` (a *separate* control-port pair from the `0x1F`/`0x3F` data ports). These program interrupt vectors `0xD0` (Y, table entry `0xE9D0`) and `0xD2` (X, table entry `0xE9D2`); the code then writes `JP <handler>` addresses into those table slots directly.
- **Buttons, confirmed precisely**: polled, not interrupt-driven, from port `0xDF` -- **bits 5, 6, 7** (not bits 0-2, which is Kempston Mouse's layout on a different port; genuinely different device). Each poll retries up to 10 times waiting for its bit (`LD B,10` / `IN A,(0xDF)` / `BIT n,A` / `JR NZ` / `DJNZ`), a hardware debounce pattern, returning a distinct code (`0x7F`/`0xBF`/`0xDF`) per button -- consistent with Wikipedia's "3 buttons."
- **What this means for implementation**: the hard part was never the three data ports -- it's that AMX needs a *second, independent interrupt source* from the standard 50/60Hz frame interrupt, IM2-vectored, firing once per host-mouse step in each axis at whatever vector table address the guest program installs (read from `I`/the written vector bytes, not hardcoded). zenzx's Z80 core already supports IM2 (needed for other software regardless of AMX); what's new is the *triggering* -- generating that interrupt from host mouse movement, correctly interleaved with the standard frame interrupt, not a new decode algorithm. This is real, scoped work (new interrupt-source plumbing), not a mystery anymore.
- **Fix:** implement following mouse.go's existing MouseMode/MouseState separation, adding: (1) PIO control-port writes at `0x5F`/`0x7F` (mode/vector setup, mostly bookkeeping), (2) an IM2-vectored interrupt fired once per accumulated step in each axis (reusing the host-delta-accumulation logic already in mouse.go's Kempston path, but emitting a CPU interrupt per whole step instead of updating a counter register), (3) ports `0x1F`/`0x3F` returning a single direction bit per read, (4) port `0xDF` bits 5-7 for buttons with the same active-high-when-pressed convention observed above (poll-based, no interrupt). Port `0x1F` conflicts with Kempston Joystick -- needs explicit mutual exclusion if both are ever selectable in the same session, same as noted before.

Cross-ref: CHANGELOG 0.4.17.

## [0.4.9] T-11 — Timex hi-colour mode (mode-timex-001-hicolour) accepted but not implemented (v0.4.9, 2026-08-17)

Theme: graphics · closed 0.4.9 · 2026-08-17


- **Trigger:** 2026-08-17, per session request. Documented in `docs/timex-modes.md`. `-ns-graphics mode-timex-001-hicolour` validates and stores the request but no renderer reads it -- display stays standard.
- **What it needs, per docs/timex-modes.md:** screen 0's bitmap area as pixel data (standard non-linear addressing), screen 1's bitmap area (same non-linear addressing, *not* the standard 32x24 attribute layout) as one FLASH/BRIGHT/PAPER/INK attribute byte per 8x1 pixel strip, neither screen's own attribute block used.
- **Open design question before any rendering code, corrected 2026-08-17 against the original T/S 2068 manual:** the manual's own memory-map diagrams (Appendix C) show screen 1 is *ordinary directly-addressable RAM* at `6000H-77FFH` -- earlier text here wrongly inferred it needed DOCK/EX-ROM-style bank switching (that bit pages expansion cartridges, unrelated to the built-in second screen). What the diagrams actually show: enabling the second screen relocates where the machine's "free RAM" (system variables, BASIC program/variables) starts -- from ~5B00H (right after screen 0) to ~7B00H (right after screen 1) -- the ROM does this housekeeping on mode switch, not just an unenforced convention. zenzx's memory model (`memory.go`, Sinclair +3-style 4x16K ROM / 8x16K RAM paging, bank-granular, no notion of a relocatable "free RAM start") has no equivalent mechanism. The concrete question is now: how does a fixed 6912-byte carve-out in the middle of a 16K bank window get modelled given zenzx's paging is bank-granular and the real machine's isn't -- decide this before writing a renderer, not while writing one.
- **Verification gap closed 2026-08-17:** the hi-colour mechanism in docs/timex-modes.md is now corroborated by four independent sources -- Wikipedia, the z88dk issue, the ZX-Uno project's manual, and (uploaded to this session) the original 1983 Timex Computer Corporation / Sinclair Research **T/S 2068 User Manual** itself. The manual states outright, in Appendix C: "the organization of attributes (which reside in memory starting at 6000H) is the same as the organization of pixel data" -- the manufacturer's own documentation confirming the exact mechanism this register describes. Port `0xFF`'s full 8-entry palette table also matches the manual's Appendix C table exactly, bit for bit. This is as settled as a written spec gets short of a real machine's ROM dump. The memory-layout question (above) is the real remaining work, and per the correction above, it turned out different from what was assumed, not just unresolved.
- **Update 2026-08-17:** the video pipeline is now pluggable (`videorender.go`; see `docs/video-architecture.md`). This renderer's `Decode(mem, screen)` will need to read the screen-1 buffer through `mem` once the memory-layout question above is settled (screen's own bitmap/attributes fields are standard-screen-only). `Dimensions()` = (256, 192) -- hi-colour does not change resolution, only attribute granularity. This mode must not implement FLASH even though its attribute byte format has a FLASH bit (see docs/timex-modes.md) -- no non-standard mode supports FLASH, by design.
- Needs a headless zenscript regression (boot, engage the mode, screenshot, assert) before being considered done.

Cross-ref: CHANGELOG 0.4.9.

## [0.4.3] T-01 — README carries stale claims (.dsk loaders broken, beeper unfiltered) (v0.4.3, 2026-08-17)

Theme: docs · closed 0.4.3 · 2026-08-17


- **Trigger:** session review 2026-08-17. README lines 43 and 86 still say `.dsk` images do not load and point at a Known-issues bullet that no longer exists; floppy read/write/format shipped in 0.3.x. The Known-issues audio bullet still describes the beeper as unfiltered/unipolar; the DC blocker and 14 kHz lowpass shipped in 0.3.3.
- **Fix:** rewrite the affected README passages against current behaviour; move intentional limits to `docs/KNOWN_ISSUES.md` and defects to this register so the README stops doubling as the register.

Cross-ref: CHANGELOG 0.4.3.

## Namespace

The T-nn namespace opened at 0.4.2 (2026-08-17) with T-01. IDs are never
reused or renumbered.
