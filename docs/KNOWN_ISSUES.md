# ZenZX — known limits and dormant guards

Version: 0.50.0
Last reviewed: 2026-11-25

Intentional limits, invariant boundaries, and recorded decisions. Defects
and gaps that are meant to be fixed belong in `TRACKING.md`, not here.

## Limits

- **The tape deck is a first cut (T-88).** It is shown for one machine at a time, not while the
  machines are tiled, and not in Fast mode, where the ROM's loader is short-circuited and there
  is no tape to watch. The bar and the counter are time at a nominal 3.5 MHz (a 128K runs at 3.5469),
  so a tape's length is a second or so out in the minute; there is no seeking by clicking the bar,
  no fast forward, and a tape with several programs keeps its cassette (the colour changes with a
  new tape, not with each header). The border's bands are worked out from the display's own
  margins, and for the Next's wider Layer 2 modes the deck is put where the classic border would
  be. A big cassette is always over the picture, and covers a good part of it. The slide-in menu
  bar covers the deck in the top band, and the machine dock the bottom one,
  while they are out.

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
  `scripts/retired` with the image it ran. FreeBSD has the headless program
  only: its GUI cannot be cross-built from Linux (purego's FreeBSD layer and
  oto need cgo there). There is no GUI for 32-bit Windows either (the `ffi`
  package under raylib-go's layer has no 386 files); the headless program
  builds for windows/386 (T-96).
- **Nothing is signed.** The Windows programs and the `.msi`, and the macOS
  programs and the `.dmg`, carry no publisher's signature, so Windows
  SmartScreen and macOS Gatekeeper warn the first time one is opened (the
  macOS program is signed ad hoc, which Apple Silicon requires, and is not
  notarised). Authenticode for Windows and an Apple Developer ID with
  notarisation for macOS each have a yearly cost and an identity attached,
  and are for the tech team to decide (T-93). The `.deb` needs neither.
- **Full screen is a borderless window over the monitor, at the zoom that fits.**
  `Alt+Enter` and View > Full Screen put the window, without its border, over the
  monitor it is on (raylib's `ToggleBorderlessWindowed`), at the monitor's own mode:
  the desktop's resolution is never changed. The picture is the largest whole zoom,
  up to X 5, that fits, in the middle of the frame, so a large monitor shows a frame
  round it and not a picture stretched to fill; the zoom you chose is kept for when
  it ends and the menu's zoom does nothing meanwhile. `F11` is full screen as well, as `Alt+Enter` (on a Mac, F11 may be the
  system's own Show Desktop, and `Alt+Enter` is then the key); the timestamped snapshot it
  used to save is `Shift+F11`. Seen only in a window on Linux without a window manager (G-22); not on
  Windows, macOS or under a window manager (T-97).
- **The headless build has no input.** Keyboard interaction in headless
  runs goes through zenscript keyboard injection (`docs/zenscript.md`),
  not host input.
- **A snapshot is loaded into a machine of its own kind, and the machine made for it has the Sinclair
  ROMs.** A `.zxs`, `.sna`, `.z80` or `.szx` is a 48K, a 128K or a +3 (`snapshotmodel.go`), and the
  window makes the machine it is of, or reloads the one it made for the same file, or (where adding
  machines is off) turns the front machine into the model it needs. A snapshot does not say which
  ROM its machine had, so the machine made is a `48k`, `128k`, `plus2a` or `plus3`, never a Spanish
  model or a +2 (`.szx` says a +2, and gets one); one opened in a machine of the right kind that
  is already running (a Spanish 128K put to Quick Load again) keeps its ROM. A 128K `.sna` or
  `.z80` is not loaded into a +2A or +3 any more, in the window it makes a 128K. The kind of a
  `.sna` is its size, and of a `.z80` what zentools says, which is a 48K or a 128K. The TS2068
  has no kind: a snapshot is not refused by one. A Next has a kind of its own: its snapshots are
  `.zxs`, and a Next takes only a Next's snapshot.
- **A Next snapshot has limits.** It does not hold what is on the SD card, which is the file the
  machine has open, but it says which cards the machine had mounted: the signature in each card's
  partition table, its size and its layout. A card that was put in the second slot and not read from by
  the firmware (one put in after NextZXOS started, until its Browser remounts) was not mounted, and is
  not recorded. Loading finds them by that, wherever they are kept and
  whatever they are now called, asks for the file of one that is not to be found, and checks the
  file chosen; a card that is not the one is refused by the loader itself, unless the person chose
  to go on with it. A card with no signature (the standard one has none) is given one, four bytes of
  its partition table, when a snapshot is first made over it; a card opened read-only cannot be, and
  is known by its size and layout, so any card like it is taken for it. A copy made in the SD Cards
  window has a signature of its own; one made outside ZenZX has the original's, and a snapshot cannot
  tell them apart; when two files have the signature (and nothing in their names tells them apart) the
  window asks which is meant. The card files are looked at again when Load is pressed, and one that is gone,
  or another machine has, or is given for two slots, is refused before anything is changed. A Next the
  window makes for a snapshot is given the boot chain and the cards
  first, and needs the boot files for it: without them it says so and loads nothing. Where there is
  no window to ask in (a script's `snapshot` verb, the command line) a snapshot is loaded only over
  its own cards. The state is read by the layout
  of the program's own structures, so a snapshot is loaded only by the build that saved it (another
  build says so and loads nothing). A Next that is loading a tape cannot be saved. Snapshots of a Next
  made by builds before 0.50.0 were saved as a +3 without its hardware and cannot be recovered: save
  them again.
- **A multisnapshot is a set of snapshots, and has theirs.** It is refused when Multi is off in
  Preferences and it holds more than one machine, and it holds a Next only as a Next's snapshot does (the cards of its Nexts are found, or asked for
  and named, before anything is closed).
  The links between machines (`-multi`) are not kept: the machines come back unlinked. Loading one
  replaces every machine running.
- **`.rzx` recordings are played, not made, and only some play to the end.** Playback needs the
  recording's own model started first (opening one makes no machine and changes none, as opening a snapshot does), and is
  not built for the Next. Encrypted recordings and ones whose snapshot is a separate file are
  refused; signatures are skipped, not checked. The format fixes what the recorder's CPU did
  (how many opcodes ran before each frame interrupt, what each `IN` read) but not what its
  interrupt line did, so the rules in `rzx.go` were found by playing real recordings: the
  interrupt is offered at one instruction boundary and lost if it cannot be taken there, and a
  frame the recorder ended between a prefix and its opcode is played with the instruction
  whole. A recording that makes the emulation do something the recorder's did not goes out of
  step, which is noticed at the end of the frame and ends the playback with a message; the
  machine then runs on by itself. The one real recording tried (RoboCop, 38881 frames) plays
  5005 frames; why it then goes out of step is T-84.
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
  - No memory contention modeling: the 48K and the 128K family have it
    (T-16, closed; `docs/hardware-accuracy.md`), the TS2068 has none.
  - Its two built-in joystick ports use TS2068's own mechanism (the
    AY-3-8912's I/O port, `ts2068.go`) and are **not** Kempston-compatible
    -- confirmed directly against timexsinclair.com, the Timex/Sinclair
    preservation project. The genuinely Kempston-equipped Timex machine
    is the TC2048, a different, related computer this project does not
    emulate.

- **The Microdigital language switch is bit 7 of port 0xFE, as one ROM reads it.** It is modelled
  from `TK90X-pt-BR.rom`, whose `IN A,(0xFE)`, `RLA`, `RET NC` takes a low bit for Portuguese; the
  polarity (diode fitted pulls the bit low) and the port are that ROM's, and have not been checked on
  a TK90X. The `IN A,(0xBF)` of `TK90X.rom` and `TK95-pt.rom` is not modelled: its result is not
  used, and the port still floats as on any unattached address. The switch is the program's, not a
  machine's (every 48K machine of `-multi` sees the same position), and it is read only by the 48K
  models, so the Amstrad models cannot be disturbed by it; at Portuguese it changes that bit for any
  program on a 48K that reads it, not only the Microdigital ROMs.

- **The Custom ROM menu holds what one level of submenu can.** `romRowsFit` takes the window height,
  the bar's strip and the row height zenui gives a dropdown row, and `buildROMRows` cuts the folders'
  submenus and **More...** to what is left below their row (a submenu opens level with its row and
  runs down). zenui has no submenu in a submenu, so each **More...** row costs a row of the
  dropdown and holds a page that shrinks with its place: in a window of a dozen rows or fewer,
  fifty ROMs and two folders no longer fit and the last of them run below the window (G-26 finds the
  limit; the cutting itself is tested for every size). Folders are the way to keep a large
  collection within reach.

## CI and cross-platform builds

`.github/workflows/ci.yml` and `.github/workflows/release.yml`, modelled
on `github.com/ha1tch/zenimate`'s CI. The split between platforms is a
consequence of the pinned dependency versions, not a design choice:

- **raylib-go v0.60.1** ships a `purego` backend (selected automatically
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

### G-14. The interface's drawing, in a window: toasts, the FPS counter, the Play button and the audio overlay (`TestGLToastOverTheDemos`, `TestGLFPSCounter`, `TestGLPlayButton`, `TestGLAudioDebugOverlay`, `TestGLPictureFitsAWindowOfAnotherSize`, `TestGLPictureFrame`, `TestGLBlankBackground`, `TestGLTiledBackground`, `TestGLExportForm`, `TestGLBarScale`, `TestGLPreferencesTabs`, `TestGLPreferencesTips`; prefs_gl_test.go, prefs_tips_gl_test.go, toast_gl_test.go, fps_gl_test.go, playbutton_gl_test.go, audiodebug_gl_test.go, display_fit_gl_test.go, pictureframe_gl_test.go, blankframe_gl_test.go, tiledbackground_gl_test.go, export_form_gl_test.go, barscale_gl_test.go)

- **Gate:** as G-03: `ZENZX_GL_TESTS` set, the GUI build, a display (Xvfb with software OpenGL is enough). Skipped when unset. The pictures the demos' tests draw over are `docs/img/next-*.png`.
- **Invariant:** a toast's face is white inside a black stroke on a darkened box with round corners that is darker towards its bottom edge, and it is never wider or taller than half the window, whatever the message and window (wrapped in lines at the usual size, one notch smaller only when those lines are too tall or a word is wider than a line); the FPS counter is hidden until asked for and is drawn with the interface's BDF font, at the bottom right; the Play button is the debugger's sprite enlarged by whole pixels (every sprite pixel a solid square), centred, on a face with round corners that is brighter hovered and darker pressed, the sprite a pixel down and right when pressed; the audio debug overlay's bar is the health's colour as far as the buffer's level and its figures are drawn in the face's glyphs, one notch smaller when a line would not fit the window; with the window held at a size other than the one the display asked for (as a system that will not resize it does), the picture is whole and in the middle, the same margin on both sides, at four sizes: the size asked for (nothing changes), a little larger (the zoom stays, centred), too small for the zoom (the zoom that fits, centred) and smaller than the picture at its least (scaled down, in proportion). The test is checked to fail with the fit taken out. The frame round that picture is a gradient from 80% grey (#333333) under the bar to 90% grey (#1A1A1A) at the bottom of the window, with the same grey in every channel, darkening all the way down, and the five beveled lines across it are a dark row with a light row under it, 28% of black and 18% of white, evenly spaced; the picture's shadow darkens it beside and above the picture and not away from it; the picture's own first pixel is not touched by it (`TestGLPictureFrame`; `ZENZX_FRAME_SHOTS=dir` keeps the picture it reads). With no machine in the window the same gradient and lines fill it, the same all the way across, with no shadow, the bar's band black and the waiting figure alone in the middle (`TestGLBlankBackground`). In a tiled window (several machines) what the tiles leave uncovered is that same background, the gradient and its five lines with no shadow, the bar's band black, and a tile is drawn over it whole (`TestGLTiledBackground`). The Export form is drawn in two themes, with and without the reason for a refused answer: its explanation in the text colour, and the reason in the accent colour (`TestGLExportForm`; `ZENZX_EXPORT_SHOTS=dir` keeps the pictures). The menu bar is drawn at ten window sizes in two themes: its text is X1 below 950 pixels wide, X2 from 950 and X3 from 1425 (in Spleen, the default face), but only where the window is high enough (256 for X2, 384 for X3), and the strip is 24, 32 and 48 pixels high with the text taking half of it or less (`TestGLBarScale`; `ZENZX_BAR_SHOTS=dir` keeps the pictures). The Preferences window is drawn on each of its three tabs in two themes and two window sizes: the chosen tab alone is filled in the selection colour and the controls of the tab that is open are on screen. The Preferences window's help box is drawn with the pointer at rest on a row and on a group's title: the window's text colour with the panel's text, on the screen, and absent with the pointer on nothing (`TestGLPreferencesTips`). It does not stand in for a Mac: the system's own window sizes and the retina factor are not in the sandbox (T-75).
- **Invocation:** `Xvfb :99 -screen 0 1280x1024x24 &` then `DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 ZENZX_TOAST_SHOTS=shots ZENZX_PLAY_SHOTS=shots go test -run 'TestGLToast|TestGLFPSCounter|TestGLPlayButton|TestGLAudioDebugOverlay|TestGLPictureFit|TestGLPictureFrame|TestGLBlankBackground|TestGLTiledBackground|TestGLExportForm|TestGLBarScale|TestGLPreferencesTabs|TestGLPreferencesTips' -v .` (about 10 seconds; `ZENZX_PREFS_SHOTS=shots` keeps the Preferences pictures). The two folder variables are optional: the pictures the tests draw over the demos go there, to be looked at.
- **Last exercised:** 2026-11-25 env:sandbox — at 0.50.0 (re-release): every `TestGL` test (25) passes under Xvfb with software OpenGL, among them the new `TestGLTiledBackground` (a tile in a bigger window over the grey gradient with the five grooves). Previous: 2026-11-25 env:sandbox: `TestGLToastOverTheDemos` passes under Xvfb with software OpenGL with the new rule (a 49-letter message wrapped to three lines at the usual size in a 960-wide window, and a word too wide for a line at the smaller size), and the other tests of this guard pass in the same run. Before that, 2026-11-23: all nine pass under Xvfb with software OpenGL (the Preferences window's help box is the window's text colour with the panel's text, in Dark and Spectrum at two sizes, and absent with the pointer on nothing; the frame: #333333 under the bar, #1A1A1A at the bottom, the shadow beside the picture); `TestGLPictureFitsAWindowOfAnotherSize` fails at three of its four sizes with the fit taken out

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

### G-17. The tape deck, in a window (`TestGLTapeDeck`, `TestGLTapeDeckChip`, `TestGLTapeDeckMorph`; tapedeck_gl_test.go, tapedeck_chip_gl_test.go, tapedeck_morph_gl_test.go)

- **Gate:** as G-14: `ZENZX_GL_TESTS` set, the GUI build, a display (Xvfb with software OpenGL is enough). Skipped when unset.
- **Invariant:** the deck is drawn, with a small cassette, in the top and the bottom band of a border at zoom 2 and 3, and floating over the picture at zoom 1 and 3, and with a big cassette (under the buttons, its label holding the title) and a hidden one (buttons, bar, counter and the name's row) floating: its panel is on the backdrop; the title is in ink on the cassette's label in the label's rectangle (or in the name row when the cassette is too small for it); the bar is filled to the fraction played and not beyond (nothing at 0, to its end at 1); each of the twelve colours of cassette has a body colour of its own and the same paper on its label; and a Play that is on does not look like one that is off; the corner buttons (minimise and close) have their dash and cross drawn wherever the layout gives the deck room for them; and the chip (the deck made small), at the right end of the top and the bottom band and floating, has its mini cassette, a play button that is a pause button while the tape plays, the counter, a close cross and a hairline of progress filled to the fraction played; and the deck at five moments of a move (full to the chip, small cassette to big) has its panel drawn at every one and nothing of it outside the panel.
- **Invocation:** `Xvfb :99 -screen 0 1280x1024x24 &` then `DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 ZENZX_DECK_SHOTS=shots go test -run TestGLTapeDeck -v .` (about 2 seconds; `ZENZX_DECK_SHOTS` keeps the pictures)
- **Last exercised:** 2026-11-23 env:sandbox — at 0.50.0 (re-release): all three pass under Xvfb (the third, the move, new, after the deck's drag and moves were added); before that, both pass under Xvfb with software OpenGL; the first fails (the bar at half, a full bar) with the bar's fill taken out

### G-18. The stop animation, in a window (`TestGLStomp`; stomp_gl_test.go)

- **Gate:** as G-14: `ZENZX_GL_TESTS` set, the GUI build, a display (Xvfb with software OpenGL is enough). Skipped when unset.
- **Invariant:** the animation that plays when a machine is stopped is drawn in a window: nothing is in view at the start; the ground comes up from below the window, with the bin and the small Spectrum on it, to its place, on the dimmed picture; the leg is at the top edge (or the boot it ends in) and reaches further down as time goes on; when the boot has landed the icon is a fraction of its height, with the boot on it; when the boot has gone back up the leg is gone and the remains are still there, on the ground in its place; the ground then goes back down with the bin and the remains; and when the animation is over nothing of the icon or the ground is left.
- **Invocation:** `Xvfb :99 -screen 0 1280x1024x24 &` then `DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 ZENZX_STOMP_SHOTS=shots go test -run TestGLStomp -v .` (under a second; `ZENZX_STOMP_SHOTS` keeps the contact sheet of twelve moments)
- **Last exercised:** 2026-11-24 env:sandbox — at 0.50.0 (re-release): passes under Xvfb with software OpenGL, with the Spectrum icon drawn with the rainbow

### G-19. The Windows programs on the page, started on real Windows (`.github/workflows/windows-smoke.yml`)

- **Gate:** run by hand from the Actions tab (Windows smoke); needs GitHub's `windows-latest` runner, and `windows-11-arm` for the arm64 program.
- **Invariant:** each Windows program the release published on the page matches the SHA-256 in `runners.json`, and starts with `-version` and exits 0, which means the graphics libraries inside it (raylib.dll, libffi-8.dll) were written to `%LocalAppData%` and loaded; the job lists what was written. The window is not seen: the runners have no screen or graphics card.
- **Invocation:** Actions > Windows smoke > Run workflow (the page is `https://ha1tch.github.io/zenzx` unless another is given).
- **Last exercised:** 2026-11-25, amd64 on `windows-latest`, against the 0.50.0 page: passed (the program started with `-version` and exited 0). arm64 on `windows-11-arm`: passed the same day. (T-92 is what it was written for.)

### G-20. The installers of a release (`.github/workflows/installers.yml`)

- **Gate:** run after a release is published (called by release.yml) or by hand from the Actions tab; needs the Ubuntu, Windows, Windows ARM and macOS runners.
- **Invariant:** the `.deb` (amd64, arm64), `.msi` (amd64, arm64) and `.dmg` of a release are built from its published programs, each is installed, started with `-version` and removed on a runner of its own system, and only then are all five attached to the release with a checksum file.
- **Invocation:** Actions > Installers > Run workflow, with the tag (such as `v0.50.0`).
- **Last exercised:** 2026-11-25, in the Release run for v0.50.0 (called by release.yml): the five installers and `zenzx-0.50.0-installers.sha256` are on the release (the release page was read here). The attach job uploads only when every job before it passed, so the installs and starts on the runners passed; the job logs were not read here, and nobody has installed one on a real machine.

### G-21. The Windows 7 program of a release (`.github/workflows/windows7.yml`)

- **Gate:** run after a release is published (called by release.yml) or by hand from the Actions tab; the program can only be started on a real Windows 7 machine, which the runners are not.
- **Invariant:** the Windows 7 program built from the release tag with the patched toolchain has no `ProcessPrng` or `bcryptprimitives`, passes the binary checks, is made into a `.msi` that installs, runs with `-version` and removes cleanly on a current Windows, and is attached to the release with a checksum file; and a person on Windows 7 starts it and sees the window.
- **Invocation:** Actions > Windows 7 > Run workflow, with the tag (such as `v0.50.0`); then give the `.zip` or `.msi` from the release to a Windows 7 user.
- **Last exercised:** 2026-11-25, by hand, on a real Windows 7 machine (the tester's): the test build made on the session machine (the workflow's source and toolchain, but not the workflow's own output) was reported to work. What the tester saw was recorded only as "success". The workflow ran in the Release run for v0.50.0 on the same day and attached `zenzx-0.50.0-windows7-amd64.zip`, the `.msi` and `zenzx-0.50.0-windows7.sha256` (the attach job uploads only when the build, the `ProcessPrng` check and the install on a current Windows had passed); no one has yet tried on Windows 7 what the workflow made.

### G-22. Full screen, in a window (`TestGLFullscreen`, `TestViewMenuFullScreenRow`; fullscreen_gui_test.go)

- **Gate:** as G-14: `ZENZX_GL_TESTS` set, the GUI build, a display of at least 1000x800 (Xvfb with software OpenGL is enough). Skipped when unset, and when the monitor is too small to tell a window from a full screen. `TestViewMenuFullScreenRow` and the state test need no window and run by default.
- **Invariant:** `Alt+Enter` and the View menu put the window over the whole of its monitor without asking it for a size; the picture is the largest whole zoom (up to X 5) that fits, in the middle, with the canvas in use; the zoom the person chose is what the settings keep and what comes back; a tiled layout is the window's size; leaving puts the window back at the size it had, without growing to the monitor first.
- **Invocation:** `DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 go test -run TestGLFullscreen -v .`
- **Last exercised:** 2026-11-25 env:sandbox -- at 0.50.0 (re-release): passed under Xvfb at 1920x1080, no window manager (zoom 2 chosen, 4 fitted to the 1080 tall monitor with the border; then a tiled window two machines wide, in and out). With the guard that stops the single-machine resize from snapping a full screen window back removed, it failed (the zoom stayed 2, the picture was not fitted), and it passed again with the guard restored.

### G-23. The manual PDF, the examples zip and the Windows launchers (`.github/workflows/documents.yml`; `examples/**/*.cmd`, `examples/tools/`)

- **Gate:** the workflow is run after a release is published (called by release.yml) or by hand from the Actions tab (Documents); it needs the runner's Chrome. The launchers can only be run on a real Windows computer, which the runners here are not: `run.cmd`, `chat.cmd`, `next/run.cmd`, and `tools/findpy.cmd`.
- **Invariant:** the release carries `zenzx-<version>-manual.pdf` (at least 10 pages, the manual's pictures in it, one outline entry per chapter), `zenzx-<version>-examples.zip` (the launchers with CRLF line ends, `launch.py`, the prebuilt tape and `.nex` files) and `zenzx-<version>-documents.sha256`; the unpacked launcher run with no ZenZX says where to get it. On Windows, double-clicking `examples\world\run.cmd` with no Python offers the download (made by ZenZX, `-fetch python`) or the page, a downloaded Python is checked against its SHA-256, and the example starts.
- **Invocation:** Actions > Documents > Run workflow, with the tag (such as `v0.50.0`); locally `make manual-pdf examples-zip` and `python3 scripts/test_launch.py`. For the Windows part: unpack the examples zip on a Windows computer with no Python, double-click `examples\link\chat.cmd`, choose the download, and say what happened.
- **Last exercised:** 2026-11-25 env:sandbox -- the tools and their tests (`test_build_manual.py` 9, `test_build_examples.py` 7, `test_launch.py` 9, `test_findpy_sh.py` 15, `test_fetch_cli.py` 15) pass on Linux, the PDF was built and its pages looked at, the zip was built, and `launch.py` was run against a fake ZenZX and a `file://` page. Under Wine 9.0 (the prefix is made in a clean environment, `env -i`, since the sandbox's own makes Wine abort): `world/run.cmd`, `link/chat.cmd` and `next/run.cmd` ran with python.org's embeddable Python 3.12.10 in `examples\python` and started a stand-in `zenzx.exe` with the right arguments; the no-Python prompt, Q, and the failed-download message (which waits for a key) were seen. Also under Wine: `test_findpy_cmd.py` (8) -- `findpy.cmd` with the real ZenZX built for Windows fetching from a server of its own, as Windows 7 (Python 3.8.10) too. NOT run: the workflow on GitHub, the real downloads from python.org and python-build-standalone, Windows 7's root certificates, brew, uv, conda, apt, dnf and yum themselves, anything on Windows itself, the `.command` files and the real python-build-standalone download on a Mac.
### G-24. The download page in a browser (`PageInABrowser`; scripts/test_build_site.py)

- **Gate:** Playwright for Python and its Chromium (`pip install playwright`, `playwright install chromium`). Skipped, and said to be, where either is missing, which is the case on CI's plain Python: the page's contents, files and figure are checked there (`PageContents`), its script is not.
- **Invariant:** the script marks the build that fits each kind of computer (Windows 10 or 11 on Intel or AMD, Windows 7, Mac, Linux on Intel or AMD, Linux on ARM) and offers it in the black box, says there is no build for 32-bit Windows and none for a phone, opens the tab of the computer's own system, lists every system when there is no script, fits a phone's width, and moves the figure unless the person asked for less motion.
- **Invocation:** `python3 -W error scripts/test_build_site.py` with Playwright installed.
- **Last exercised:** 2026-11-25 env:sandbox -- 25 tests pass, 6 of them in Chromium. The agents are emulated with the client hints turned off; a real Windows on ARM and an Apple silicon Mac (which say Intel in the agent string and ARM only in the hints) were not tried, nor Safari or Firefox.

### G-25. The Windows launcher under Wine (`scripts/test_findpy_cmd.py`)

- **Gate:** Wine (`/usr/lib/wine/wine64`, or `wine64` or `wine` on the PATH) and Go, which builds the Windows program and a stand-in `python.exe` for it. Skipped, and said to be, where either is missing, as on CI's plain runners. The one Wine prefix is some 650 MB, made in a clean environment (`env -i`, since the sandbox's own makes Wine abort), and is set to Windows 7 for one test.
- **Invariant:** `examples\tools\findpy.cmd` finds the ZenZX that is beside the examples, has it fetch Python (`-fetch python ... -fetch-how private`) from the address in `ZENZX_PYTHON_URL`, takes the download only when its SHA-256 is right (a wrong one leaves nothing, not even a `.part`), waits for the program built for the windowed subsystem and takes its failure back, picks Python 3.8.10 when Windows says it is a 7, accepts `python\Scripts\python.exe` from an environment tool, and with no ZenZX says where it goes and the python.org page.
- **Invocation:** `python3 -W error scripts/test_findpy_cmd.py` (a clean environment where Wine aborts: `env -i PATH=$PATH HOME=$HOME python3 scripts/test_findpy_cmd.py`).
- **Last exercised:** 2026-11-25 env:sandbox -- 8 tests pass under Wine 9.0 (about 45 seconds). Not Windows: the tests cannot say what a real Windows 7 does with the certificates of today's sites, nor how SmartScreen and Explorer treat a downloaded program.

### G-26. The Custom ROM dropdown in a window (`TestGLCustomROMMenuFitsTheWindow`; custom_roms_fit_gl_test.go)

- **Gate:** as G-14: `ZENZX_GL_TESTS` set, the GUI build, a display (Xvfb with software OpenGL is enough). Skipped when unset.
- **Invariant:** with about fifty ROMs and two folders (one with a folder of its own), at window sizes from 1024x768 to 320x240 and each text size, every row of the dropdown and of the submenu of each of its rows is inside the window, and every ROM is reachable once. Sizes whose dropdown has fewer than 13 rows are logged and not checked: the data does not fit them (see Limits).
- **Invocation:** `Xvfb :99 -screen 0 1280x1024x24 &` then `DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 go test -run TestGLCustomROMMenuFitsTheWindow -v .`
- **Last exercised:** 2026-11-25 env:sandbox -- passes under Xvfb with software OpenGL; three of the twelve size and text size pairs are below 13 rows and not checked. Not seen on a real display, and the rows were measured by zenui's own layout, not by eye.

### G-27. A window that closes fades and shrinks, in a window (`TestGLDialogFadesAndShrinksWhenClosed`, `TestGLDropChoiceFadesToo`; dialogfade_gl_test.go)

- **Gate:** as G-14: `ZENZX_GL_TESTS` set, the GUI build, a display (Xvfb with software OpenGL is enough). Skipped when unset. The curve (`dialogFadeAt`) is tested without a window and runs by default.
- **Invariant:** the frame after the Help window closes is the window as it was, backdrop and all (and the same for the dialog about files dropped on a Next, which the program draws and not the bar: it is held whole while it is open and fades when it closes); half way through the fade the backdrop is half as dark, the window is fainter and has shrunk (a pixel that was just inside its edge is now the backdrop's); at the end the picture is the one with no window at all, nothing of it left, and a window opened during a fade ends the fade. The test is checked to fail with the capture taken out of `appMenuBar.Draw`, and with the shrink set to 1.
- **Invocation:** `Xvfb :99 -screen 0 1280x1024x24 &` then `DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 ZENZX_FADE_SHOTS=shots go test -run TestGLDialogFades -v .` (under a second; `ZENZX_FADE_SHOTS` keeps the pictures of the window open, at the start of the fade and half way)
- **Last exercised:** 2026-11-25 env:sandbox -- passes under Xvfb with software OpenGL; fails with the capture taken out, and fails (the window has not shrunk) with the shrink set to 1; the drop-choice test fails with the hold on the capture taken out (a fade started while the dialog was open). The program's own call of `watchDropChoice` in its draw hook is not under test. Looked at: half way the window is translucent over the lighter backdrop with no dark fringe at its edges. Only the Help window was drawn; the others share `modalDrawer` and the backdrop rule, and were not drawn one by one.

### G-28. A popup menu has a drop shadow and the bar has none, in a window (`TestGLPopupMenuHasADropShadowAndTheBarHasNone`; popupshadow_gl_test.go)

- **Gate:** as G-14: `ZENZX_GL_TESTS` set, the GUI build, a display (Xvfb with software OpenGL is enough). Skipped when unset. Which rectangle is a popup's panel (and that the shadow goes under it, inside a clip that starts under the strip) is tested without a window and runs by default (`popupshadow_gui_test.go`).
- **Invariant:** with the File menu open, the bar's strip is the same pixel for pixel with the shadow as without it, the panel is the same, and the ground beside and below the panel is much darker with it; the far corner of the window is untouched. The shadow is drawn from the picture frame's texture (`drawBoxShadow`).
- **Invocation:** `Xvfb :99 -screen 0 1280x1024x24 &` then `DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 ZENZX_FADE_SHOTS=shots go test -run TestGLPopupMenu -v .` (a fraction of a second; `ZENZX_FADE_SHOTS` keeps the picture)
- **Last exercised:** 2026-11-25 env:sandbox -- passes under Xvfb with software OpenGL. Looked at: the File menu in the Dark theme, and the View menu in the Light and Spectrum themes, with the shadow to the right and below and none on the strip. The logo menu's call (`withPopupShadow` with the strip not in the renderer) is not under test, and nor is a submenu in a window.

### G-29. The hex dumps are in a monospaced face, in a window (`TestGLHexDumpFontFollowsTheChosenFont`; monofont_gl_test.go)

- **Gate:** as G-14: `ZENZX_GL_TESTS` set, the GUI build, a display (Xvfb with software OpenGL is enough). Skipped when unset. Which face is chosen (`fonts.NearestMonospaced`) is tested without a window and runs by default (`pkg/fonts/monospaced_test.go`), and so is everything else about the Binary window (`binary_gui_test.go`, `hotmem_test.go`), with a recording renderer.
- **Invariant:** for each of the seven faces as the program's font, the renderer of the hex dumps of Snapshot > Load Binary (`appMenuBar.monoRenderer`) measures, through the real glyph renderer, the cell of that face when it is monospaced and otherwise of the monospaced face closest in size (Tom Thumb, which is proportional by design, gets Spleen); and the Hot Memory tab drawn with it has ink in its dump.
- **Invocation:** `Xvfb :99 -screen 0 1280x1024x24 &` then `DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 go test -run TestGLHexDump -v .` (under a second)
- **Last exercised:** 2026-11-25 env:sandbox -- passes under Xvfb with software OpenGL. Looked at: the three tabs of the window with the 128K and the Next, the drop-down open, in the Dark theme and the default font; the dumps are in Spleen there, so the Tom Thumb case was measured and not looked at.

### G-30. New on the dock adds a machine, in a window (`TestGLDockNewAddsAMachine`; deckadd_gl_test.go)

- **Gate:** as G-14: `ZENZX_GL_TESTS` set, the GUI build, a display (Xvfb with software OpenGL is enough). Skipped when unset. Adding a machine makes the picture of it, a texture, so it cannot run without the context; the refusal with adding machines off is tested without a window and runs by default (`deckaddoff_gui_test.go`).
- **Invariant:** with adding machines on in Preferences, `machineDeck.choose` on the "+" tile adds one machine, starts it flying to the front, and shows no toast; with it off, it adds none and shows the one toast, each time.
- **Invocation:** `Xvfb :99 -screen 0 1280x1024x24 &` then `DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 go test -run TestGLDockNew -v .` (under a second; with no Xvfb running, the window is not made and the test dies in the texture call, which is not a failure of the dock)
- **Last exercised:** 2026-11-25 env:sandbox -- passes under Xvfb with software OpenGL. Not looked at: no picture is drawn.

### G-31. A snapshot that makes a machine slides the dock in for it, with a chevron, in a window (`TestGLSnapshotMachineRevealsTheDock`; dockreveal_gl_test.go)

- **Gate:** as G-14: `ZENZX_GL_TESTS` set, the GUI build, a display (Xvfb with software OpenGL is enough). Skipped when unset. The dock's pictures are textures and the chevron is drawn with the GL context, so it cannot run without it; the chevron's bounce (that it lands on the picture and never goes into it), its fading, how long the dock stays and when the reveal ends, and that no dock is shown with it off in Preferences, are tested without a window and run by default (`dockreveal_gui_test.go`).
- **Invariant:** a snapshot of another kind makes a machine and the dock starts sliding in for it (not held open as a keyboard's is) with its picture small, to grow into place; the dock then stays its three seconds; the same file loaded again makes no machine and no dock; the chevron is a white V with a black outline, a picture of 34 by 16 pixels, its outline two pixels thick, expanded to the width of the picture it points at with each pixel a square, over the dock and, with the dock gone, standing on the bottom edge of the window; and nothing is drawn when the reveal is over.
- **Invocation:** `Xvfb :99 -screen 0 1280x1024x24 &` then `DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 go test -run TestGLSnapshotMachineReveals -v .`
- **Last exercised:** 2026-11-26 env:sandbox -- passes under Xvfb with software OpenGL. Measured in a frame: every pixel of the sprite, at the middle of the square it is expanded to, is white, black or the ground as the sprite says (over the dock, and at the window's bottom edge with it gone, against a chequered ground), the outline spans the picture's width, and nothing is drawn when there is no reveal. Not watched moving: the slide, the bounce and the fade.

### G-32. A Next snapshot is loaded into a machine made without the boot chain (`TestANextSnapshotLoadsIntoAMachineMadeWithItsCard`, snapshot_cards_hw_test.go)

- **Gate:** as the other Next guards: `ZENZX_NEXT_FILES` names the folder with the three boot files. Skipped when unset. The boot card is opened read-only.
- **Invariant:** a second card is read by the firmware when it was in the slot at the start and not when it was put in after, and the snapshot records the cards that were read (`TestASecondCardCountsOnlyWhenTheFirmwareMountedIt`); a Next that is at the NextZXOS menu is saved; a Next as the deck makes it, with no boot chain, refuses the snapshot for want of the card; with the chain and the card attached, as the window does before it loads (`mountCards`), it takes it, the menu is on the screen again, and 200 frames on it is a menu that waits for a key. The card is not given a signature, and the snapshot records it by its size and layout. The rest, which needs no firmware (identity, the search, the dialogue, multisnapshots), runs with the ordinary suite.
- **Invocation:** `ZENZX_NEXT_FILES=next go test -tags headless -run TestANextSnapshotLoadsIntoAMachineMadeWithItsCard -v .` (about 5 seconds)
- **Last exercised:** 2026-11-25 env:sandbox -- at 0.50.0 (re-release): both tests pass on the 26.11 boot files

### G-33. A card or a folder that cannot be read is passed over in the search (`TestACardThatCannotBeReadIsPassedOver`, `TestAFolderThatCannotBeReadDoesNotStopTheSearch`; cardadversarial_unix_test.go)

- **Gate:** a Unix system and a user that file permissions bind: both tests are skipped as root, which reads anything, so the sandbox's own run skips them. Everything else of the card search's adversarial tests (pipes and devices, folder names, copies, links) runs for everyone.
- **Invariant:** a card image with no read permission is not found and is said not to be a card image when chosen; a folder that cannot be read does not stop the others being looked through.
- **Invocation:** as an ordinary user: `go test -c -o /tmp/zz.test . && cd /tmp && setpriv --reuid=65534 --regid=65534 --clear-groups env HOME=/tmp TMPDIR=/tmp /tmp/zz.test -test.run "TestACardThatCannotBeReadIsPassedOver|TestAFolderThatCannotBeReadDoesNotStopTheSearch" -test.v`
- **Last exercised:** 2026-11-25 env:sandbox (as uid 65534, the test binary run apart) -- at 0.50.0 (re-release): both pass
