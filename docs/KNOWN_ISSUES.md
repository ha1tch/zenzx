# ZenZX — known limits and dormant guards

Version: 0.50.0
Last reviewed: 2026-11-23

Intentional limits, invariant boundaries, and recorded decisions. Defects
and gaps that are meant to be fixed belong in `TRACKING.md`, not here.

## Limits

- **A sandbox may have no GUI system libraries.** With Mesa (GLX), the
  X11, Wayland and ALSA development packages and Xvfb installed, the GUI
  links and runs there on a virtual display with software OpenGL
  (`LIBGL_ALWAYS_SOFTWARE=1`), which is how G-01, G-03 and G-05 were
  exercised on 2026-11-16. Without them, `check_gui.sh` (the
  release-time gate) tolerates only cgo/system-library link failures on
  a host without ALSA/X11/Wayland/GL, since this sandbox is one. CI
  (`.github/workflows/ci.yml`) builds the real, linked GUI binary on
  every push: cgo-free for darwin/windows via raylib-go's purego backend
  (see "CI and cross-platform builds" below), natively with apt-installed
  dev libraries for linux/amd64 and linux/arm64.
- **BSD has no purego path.** raylib-go's embedded-library backend
  supports linux/darwin/windows only; FreeBSD/OpenBSD/NetBSD need a real
  cgo build with system dev libraries and are not covered by CI or the
  release workflow (GitHub Actions has no BSD runners). The Docker-based
  `build_bsd.sh` was the only route and does not work (T-07); it is in
  `scripts/retired` with the image it ran.
- **The headless build has no input.** Keyboard interaction in headless
  runs goes through zenscript keyboard injection (`docs/zenscript.md`),
  not host input.
- **The +2A shares the +3 ROM set** (including the unused +3DOS) but has
  no floppy controller; `-model plus2a` deliberately does not enable the
  FDC.
- **TS2068's supported feature set is deliberately scoped**, per
  `docs/TS2068_DEVELOPMENT_PLAN.md`/`docs/TS2068_TRACKING.md`:
  - No full 8-chunk Dock/cartridge (LROS/AROS) banking, no TS2040
    printer protocol, no composite/RF video generation detail -- named
    explicitly out of scope in the original plan, not overlooked.
  - Hi-colour/video-mode switching is driven by the guest's own port
    `FFH` writes (dynamic, matching how real software actually engaged
    these modes), not the documented Extension ROM `CHNG_VID` service
    call -- the real service-call route hit a genuine, undiagnosed
    divergence during relocation and was set aside (T-15) once real
    software was confirmed not to depend on it.
  - Save-side tape (`W_TAPE`) is not implemented, matching the standard
    models' `SA-BYTES` scope (T-02): zenzx has no tape-save capture to
    hook into for any model, TS2068 included.
  - No memory contention modeling, matching every other model (T-16) --
    not a TS2068-specific gap.
  - Its two built-in joystick ports use TS2068's own mechanism (the
    AY-3-8912's I/O port, `ts2068.go`) and are **not** Kempston-compatible
    -- confirmed directly against timexsinclair.com, the Timex/Sinclair
    preservation project. The genuinely Kempston-equipped Timex machine
    is the TC2048, a different, related computer this project does not
    emulate.

## CI and cross-platform builds

`.github/workflows/ci.yml` and `.github/workflows/release.yml`, modelled
on `github.com/ha1tch/zenimate`'s CI. The split between platforms is a
consequence of the pinned dependency versions, not a design choice:

- **raylib-go v0.60.0** ships a `purego` backend (selected automatically
  whenever `CGO_ENABLED=0`) that embeds a prebuilt native `raylib`
  shared library per `GOOS`/`GOARCH` (linux, darwin, windows; amd64 and
  arm64) and extracts it to the user cache directory at first run. No
  system OpenGL/X11/Wayland headers are needed to build, and the result
  is a complete, self-contained binary, not a stub.
- **oto v3.4.0** has no cgo dependency on darwin (CoreAudio via
  `ebitengine/purego`) or windows (WASAPI/WinMM via `golang.org/x/sys`).
  On Linux it always requires cgo and `libasound` (`#cgo pkg-config:
  alsa` in `driver_unix.go`) — there is no purego alternative in this
  version.

Net effect: darwin (amd64/arm64) and windows (amd64/arm64) GUI binaries
are built cgo-free, cross-compiled from a single Linux runner, in both
CI and the release workflow. Linux (amd64 and arm64) always needs a
native cgo build with `libasound2-dev`, `libgl1-mesa-dev`, and the
X11/Wayland dev headers installed on the runner; arm64 uses GitHub's
`ubuntu-24.04-arm` hosted runner (GA, free on public repositories — see
GitHub's runner documentation for private-repository terms). See T-08:
a future raylib-go or oto version bump should re-verify this split
before assuming it still holds.

## Dormant guards


Verification that does not run in the default `go test -tags headless`
invocation. A guard's existence is not evidence; only its execution
record is. `repoman guards stale` lists guards not exercised since the
previous release; each is run, handed off (`repoman guards handoff`), or its
skip is recorded in the release's changelog entry.

### G-01. GUI link build (`build.sh`, `build_linux.sh`)

- **Gate:** host with raylib/oto system libraries (Mesa with GLX, the X11, Wayland and ALSA development packages); a display, which can be Xvfb with software OpenGL
- **Invocation:** `./build.sh` (or `./build_linux.sh`) then launch the binary and load a snapshot
- **Last exercised:** 2026-11-22 env:sandbox — at 0.50.0 (re-release): ./cmd/zenzx links with cgo; launched under Xvfb with software OpenGL, the menu bar and Help menu driven with xdotool Previous: 2026-11-18 env:sandbox — at 0.40.0: ./cmd/zenzx links with cgo; run under Xvfb with software OpenGL: four chat clients started with -multi -listen -autorun -layout grid, tiled, the Python host relaying

### G-04. NextZXOS boot and save against the real firmware (`TestNextZXOSBoots`, `TestNextZXOSSavesToCard`, `TestNextZXOSBootsFromAMinimalCard`)

- **Gate:** `ZENZX_NEXT_FILES` names a folder with three third-party files that cannot be in the tree: `nextboot.rom`, `enNxtmmc.rom` and `next-card.img` (the last two from the official emulator distribution through `scripts/next_card.py`). Skipped when unset. No hardware requirement.
- **Invocation:** `ZENZX_NEXT_FILES=next go test -tags headless -run TestNextZXOS -v .` (the card file is not changed: the save test keeps its writes in memory)
- **What it checks besides finishing:** no golden images, since what is drawn depends on the card and the clock; instead milestone frame counts within bounds, the screens' structure (colours, background, runs, rows in use), a steady menu, the saved file as a +3DOS BASIC program (header checksum, tokenised line 10), the card after the save (FATs equal, writes only where a save writes), and the minimal card's menu against the full card's. The checks are in `next_quality_test.go` and are themselves tested, without the firmware, in `next_quality_unit_test.go`.
- **Last exercised:** 2026-11-23 env:sandbox — at 0.50.0 (re-release): all nine TestNextZXOS tests pass on the 26.11 boot files, with the result checks of next_quality_test.go, after the ROM patch list and the hand-over changes Previous: 2026-11-22 env:sandbox — at 0.50.0: all nine pass

**G-02 retired, 2026-08-28 (T-05 closed at v0.6.8):** `TestFDCReadMatchesDisk`
(`fdc_read_test.go`) now runs unconditionally against a checked-in fixture
(`testdata/synthetic.dsk`), built through the FDC765 controller's own
SEEK/FORMAT TRACK/WRITE DATA command interface by `TestGenerateSyntheticDSK`
in the same file. It is part of the default `go test -tags headless .`
invocation and is no longer a dormant guard. `ZENZX_TEST_DSK` still allows
pointing the test at a real captured image locally.

### G-03. Layer 2 GPU-baked palette texture matches the 9-bit colour table

- **Gate:** `ZENZX_GL_TESTS` set, the GUI build (no `headless` tag), and a display for `rl.InitWindow`; Xvfb with software OpenGL is enough. Any raylib GPU call segfaults without a window.
- **Invocation:** `Xvfb :99 -screen 0 1280x1024x24 &` then `DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 go test -run TestGLNextPicture -v .` (gl_gui_test.go). The test writes a 9-bit palette entry through NextREG 0x40, 0x43 and 0x44, bakes the textures and reads all 256 back with `rl.LoadImageFromTexture` against `io.nextColourFor`.
- **Last exercised:** 2026-11-23 env:sandbox — at 0.50.0 (re-release): passes Previous: 2026-11-22 env:sandbox — at 0.50.0 (re-release): passes

### G-05. The Next's picture as the GUI draws it (`TestGLNextPicture`, gl_gui_test.go)

- **Gate:** as G-03: `ZENZX_GL_TESTS` set, the GUI build, a display (Xvfb with software OpenGL is enough).
- **Invocation:** `Xvfb :99 -screen 0 1280x1024x24 &` then `DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 go test -run TestGLNextPicture -v .`. It draws frames through `DisplayManager.Render` and reads the window back: a Layer 2 pixel from bank 8 and its disappearance when NextREG 0x69 bit 7 is cleared, the standard screen through the cell drawing, Timex hi-res at one window pixel per hi-res pixel, a tile of the tilemap, and the return to the cell drawing.
- **Last exercised:** 2026-11-23 env:sandbox — at 0.50.0 (re-release): passes Previous: 2026-11-22 env:sandbox — at 0.50.0 (re-release): passes

### G-06. ZenZX's Next examples run from NextZXOS (`TestNextZXOSRunsTheExamples`, next_examples_test.go)

- **Gate:** as G-04: `ZENZX_NEXT_FILES` names the folder with the three boot files. Skipped when unset. Since 0.24.0 the examples are on a card of their own, made in a temporary folder as `-next-examples` makes it and put in the second slot; the files in the folder are not changed.
- **Invocation:** `ZENZX_NEXT_FILES=next go test -tags headless -run TestNextZXOSRunsTheExamples -v .` (G-04's own invocation, `-run TestNextZXOS`, runs it too)
- **Last exercised:** 2026-11-23 env:sandbox — at 0.50.0 (re-release): passes Previous: 2026-11-22 env:sandbox — at 0.50.0 (re-release): passes

### G-07. A card write is seen by NextZXOS after re-insert (`TestNextZXOSSeesCardWriteAfterReinsert`, next_refresh_hw_test.go)

- **Gate:** as G-04/G-06: `ZENZX_NEXT_FILES` names the folder with the three boot files. Skipped when unset.
- **Invariant:** after a file is written to the inserted second card (as the mount drawer does, via `nextcard.AddFiles` which renames a new image over the file) and the card is re-inserted, NextZXOS's own Browser remount (M, then Y) lists and runs the new file. Guards against the emulator serving the orphaned pre-rename handle.
- **Invocation:** `ZENZX_NEXT_FILES=next go test -tags headless -run TestNextZXOSSeesCardWriteAfterReinsert -v .` (`-run TestNextZXOS` runs it too)
- **Last exercised:** 2026-11-22 env:sandbox — at 0.50.0 (re-release): passes Previous: 2026-11-20 env:sandbox — passes

### G-08. Two concurrent Next machines each see their own card write (`TestNextZXOSTwoMachinesSeeCardWrites`, next_refresh_multi_hw_test.go)

- **Gate:** as G-04/G-06/G-07: `ZENZX_NEXT_FILES`. Skipped when unset.
- **Invariant:** the G-07 refresh holds for two Next machines running at once, each booting its own minimal (280 MB) card -- made bootable from the 26.11 card -- with its own second card. After each machine writes to its own second card and re-inserts it, each machine's Browser remount gains the new file's row, independently (the cards cannot cross: separate DivMMCs, separate images).
- **Invocation:** `ZENZX_NEXT_FILES=next go test -tags headless -run TestNextZXOSTwoMachinesSeeCardWrites -v .`
- **Last exercised:** 2026-11-22 env:sandbox — at 0.50.0 (re-release): passes Previous: 2026-11-20 env:sandbox — passes

### G-09. The Browser refreshes on a host-driven remount, no keypress (`TestNextZXOSRemountRefreshesBrowser`, next_remount_hw_test.go)

- **Gate:** as G-04/G-06/G-07/G-08: `ZENZX_NEXT_FILES`. Skipped when unset.
- **Invariant:** `forceNextZXOSRemount` drives NextZXOS's own M-then-Y Browser remount from the host, by feeding the firmware's key sysvars (LAST_K $5C08, FLAGS $5C3B bit 5) the two keycodes its idle key-wait loop reads -- located by the signature `76 21 3B 5C CB 6E` (HALT / LD HL,FLAGS / BIT 5,(HL)), not a fixed address, so it survives firmware versions that keep the idiom and declines rather than guesses where they do not. After a file is written to the second card and the handle re-inserted (the G-07 fix), the host-driven remount gains the new file's row, exactly as a physical M-then-Y would; a second remount with nothing new is a stable no-op.
- **Invocation:** `ZENZX_NEXT_FILES=next go test -tags headless -run TestNextZXOSRemountRefreshesBrowser -v .`
- **Last exercised:** 2026-11-22 env:sandbox — at 0.50.0 (re-release): passes Previous: 2026-11-20 env:sandbox — passes

### G-10. A host-driven remount refreshes the focused machine, not the first (`TestNextZXOSRemountTargetsFocusedMachine`, next_remount_focus_hw_test.go)

- **Gate:** as G-04/G-06/G-07/G-08/G-09: `ZENZX_NEXT_FILES`. Skipped when unset.
- **Invariant:** with two Next machines running, a file dropped while the second is focused refreshes the second, not the first. The mount drawer resolves the focused machine (the deck's front) at drop time and runs `forceNextZXOSRemount` against that machine's own memory; the guard drives that per-machine API with focus on B and asserts B gains the new file's row while A is unchanged. (The drawer type is GUI-only, so the guard drives the API the drawer calls, not the drawer.)
- **Invocation:** `ZENZX_NEXT_FILES=next go test -tags headless -run TestNextZXOSRemountTargetsFocusedMachine -v .`
- **Last exercised:** 2026-11-22 env:sandbox — at 0.50.0 (re-release): passes Previous: 2026-11-20 env:sandbox — passes

### G-11. nextAtIdlePrompt detects a Next at its menu/Browser (`TestNextAtIdlePromptDetectsMenu`, next_idleprompt_hw_test.go)

- **Gate:** as the other Next guards: `ZENZX_NEXT_FILES`. Skipped when unset.
- **Invariant:** `nextAtIdlePrompt` reports true when a Next sits at its NextZXOS menu or Browser (both spin on the key-wait idle loop located by signature) and false for a non-Next machine. It gates the drop-choice dialog: a tape, .scr or snapshot dropped while the Next is at an idle prompt offers copy-to-SD vs load-and-run.
- **Invocation:** `ZENZX_NEXT_FILES=next go test -tags headless -run TestNextAtIdlePromptDetectsMenu -v .`
- **Last exercised:** 2026-11-22 env:sandbox — at 0.50.0 (re-release): passes Previous: 2026-11-21 env:sandbox — passes

### G-12. A .nex is run from the menus, after a reset, and from a scratch card (`TestNextRunsANEXFromItsMenus`, nexrun_hw_test.go)

- **Gate:** as the other Next guards: `ZENZX_NEXT_FILES`. Skipped when unset.
- **Invariant:** `RunNEX` runs a NEX file on a Next that boots NextZXOS by the least invasive way that works: from the welcome screen with slot 2 empty (a scratch card is made for it), from the Browser with a card that can be written in slot 2 (the file is added to it, the Browser is made to look at the cards, the keys to the command line are pressed, `.nexload` is typed), after a reset when the Next is busy (it asks first and changes nothing until told it may), and from a scratch boot card whose autoexec.bas runs `.nexload` when the keys cannot be used. Each example is recognised by the registers it sets (gallop, fuji, orbit, as G-06 does), and the scratch card's folder is removed when it is detached. The screens the keys meet are recognised from the picture (`classifyNextScreen`), so this is the guard for a NextZXOS whose menus look different.
- **Invocation:** `ZENZX_NEXT_FILES=next go test -tags headless -run TestNextRunsANEXFromItsMenus -v .` (about 30 seconds; the boot card is opened read-only)
- **Last exercised:** 2026-11-22 env:sandbox — passes

### G-13. The Next's boot cache against the real firmware (`TestNextBootCacheOnTheRealFirmware`, `TestNextSettledCacheOverAChangedCard`, `TestNextSettledCacheSavesSafelyOverAChangedCard`; nextcache_hw_test.go, nextcache_save_hw_test.go)

- **Gate:** as the other Next guards: `ZENZX_NEXT_FILES`. Skipped when unset. The cards they use are made in a temporary folder from the boot files; the folder's own card is only read.
- **Invariant:** a machine put back from the hand-over cache (frame 428) matches, field for field, a cold boot at the same frame, and the settled cache (frame 555) the same; the settled cache made from the hand-over one is the one a cold boot makes; a different DivMMC ROM refuses the cache and the log names it. Over a card that has had files and folders added since the cache was made, a machine started from the settled cache shows the same pictures as a cold one at rest, at the menu, in the Browser listing and inside a folder, and a NextBASIC `SAVE` from it leaves every added file as it was and the saved program readable, as one from a cold boot does; the test refuses to run when the two cards have the same free-space hint, because then it could not tell.
- **Invocation:** `ZENZX_NEXT_FILES=next go test -tags headless -run 'TestNextBootCacheOnTheRealFirmware|TestNextSettledCache' -v .` (about 7, 8 and 15 seconds)
- **Last exercised:** 2026-11-23 env:sandbox — at 0.50.0 (re-release): all three pass on the 26.11 boot files, and again after the ROM patch list added its entries to the cache's key

### G-14. The interface's drawing, in a window: toasts, the FPS counter, the Play button and the audio overlay (`TestGLToastOverTheDemos`, `TestGLFPSCounter`, `TestGLPlayButton`, `TestGLAudioDebugOverlay`, `TestGLPictureFitsAWindowOfAnotherSize`, `TestGLPreferencesTabs`; prefs_gl_test.go, toast_gl_test.go, fps_gl_test.go, playbutton_gl_test.go, audiodebug_gl_test.go, display_fit_gl_test.go)

- **Gate:** as G-03: `ZENZX_GL_TESTS` set, the GUI build, a display (Xvfb with software OpenGL is enough). Skipped when unset. The pictures the demos' tests draw over are `docs/img/next-*.png`.
- **Invariant:** a toast's face is white inside a black stroke on a darkened box with round corners that is darker towards its bottom edge, and it is never wider than half the window, whatever the message and window (one notch smaller, then in lines); the FPS counter is hidden until asked for and is drawn with the interface's BDF font, at the bottom right; the Play button is the debugger's sprite enlarged by whole pixels (every sprite pixel a solid square), centred, on a face with round corners that is brighter hovered and darker pressed, the sprite a pixel down and right when pressed; the audio debug overlay's bar is the health's colour as far as the buffer's level and its figures are drawn in the face's glyphs, one notch smaller when a line would not fit the window; with the window held at a size other than the one the display asked for (as a system that will not resize it does), the picture is whole and in the middle, the same margin on both sides, at four sizes: the size asked for (nothing changes), a little larger (the zoom stays, centred), too small for the zoom (the zoom that fits, centred) and smaller than the picture at its least (scaled down, in proportion). The test is checked to fail with the fit taken out. The Preferences window is drawn on each of its three tabs in two themes and two window sizes: the chosen tab alone is filled in the selection colour and the controls of the tab that is open are on screen. It does not stand in for a Mac: the system's own window sizes and the retina factor are not in the sandbox (T-75).
- **Invocation:** `Xvfb :99 -screen 0 1280x1024x24 &` then `DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 ZENZX_TOAST_SHOTS=shots ZENZX_PLAY_SHOTS=shots go test -run 'TestGLToast|TestGLFPSCounter|TestGLPlayButton|TestGLAudioDebugOverlay|TestGLPictureFits|TestGLPreferencesTabs' -v .` (about 8 seconds; `ZENZX_PREFS_SHOTS=shots` keeps the Preferences pictures). The two folder variables are optional: the pictures the tests draw over the demos go there, to be looked at.
- **Last exercised:** 2026-11-23 env:sandbox — at 0.50.0 (re-release): all six pass under Xvfb with software OpenGL; `TestGLPictureFitsAWindowOfAnotherSize` fails at three of its four sizes with the fit taken out

### G-15. A Next that is reset restarts its firmware and loses the program's state (`TestNextResetRestartsTheFirmware`, reset_next_hw_test.go)

- **Gate:** as the other Next guards: `ZENZX_NEXT_FILES` names the folder with the three boot files. Skipped when unset. The boot card is opened read-only.
- **Invariant:** a Next that is reset (`Reset`: the Machine menu, F1, the debugger) in the middle of gallop, fuji or orbit is in config mode with its boot ROM on and no machine chosen one frame later; NextREG 0x15, 0x69, 0x6B, 0x70, 0x12, 0x13, 0x14, 0x4A, 0x07, 0x62, 0x0A, 0x6E and 0x6F hold their reset values; the copper, the hardware interrupts and the CTC, the DMA, the DivMMC's automap and the AY are as at power on; and NextZXOS comes back, the firmware handing over after about 400 frames and the menu being up (after the welcome screen's key) by about 700. The part that needs no firmware, a Next with no boot chain compared field by field with one just switched on, is `TestResetNextIsAHardReset`, which runs with the ordinary suite.
- **Invocation:** `ZENZX_NEXT_FILES=next go test -tags headless -run TestNextResetRestartsTheFirmware -v .` (about 21 seconds)
- **Last exercised:** 2026-11-23 env:sandbox — at 0.50.0 (re-release): passes on the 26.11 boot files; fails, on the registers and on config mode, with the hard reset request taken out of `Reset`

### G-16. ROM switches over the ROMs the real firmware loads (`TestNextROMPatchesOverTheFirmware`, rom_patches_hw_test.go)

- **Gate:** as the other Next guards: `ZENZX_NEXT_FILES` names the folder with the three boot files. Skipped when unset. The boot card is opened read-only.
- **Invariant:** `-rom-bin` on a Next that boots from its card does not act before the firmware has loaded its ROMs, and acts when the firmware leaves config mode (NextREG 0x03 choosing the machine, then the soft reset): after the hand-over the patch's bytes are in bank 0 and what the firmware loaded is round them. The unit tests of the config-mode sequence, a hard reset applying the list again and a soft reset leaving ROM alone need no firmware and run with the ordinary suite (`rom_patches_test.go`); this is the guard for a firmware that leaves config mode some other way.
- **Invocation:** `ZENZX_NEXT_FILES=next go test -tags headless -run TestNextROMPatchesOverTheFirmware -v .` (about 7 seconds)
- **Last exercised:** 2026-11-23 env:sandbox — at 0.50.0 (re-release): passes on the 26.11 boot files
