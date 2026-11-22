# ZenZX — known limits and dormant guards

Version: 0.50.0
Last reviewed: 2026-11-21

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
- **Last exercised:** 2026-11-18 env:sandbox — at 0.40.0: ./cmd/zenzx links with cgo; run under Xvfb with software OpenGL: four chat clients started with -multi -listen -autorun -layout grid, tiled, the Python host relaying Previous: 2026-11-18 env:sandbox — at 0.39.0: ./cmd/zenzx links with cgo; run under Xvfb with software OpenGL: the Tape, Disk and Card Info windows opened from their menus

### G-04. NextZXOS boot and save against the real firmware (`TestNextZXOSBoots`, `TestNextZXOSSavesToCard`, `TestNextZXOSBootsFromAMinimalCard`)

- **Gate:** `ZENZX_NEXT_FILES` names a folder with three third-party files that cannot be in the tree: `nextboot.rom`, `enNxtmmc.rom` and `next-card.img` (the last two from the official emulator distribution through `scripts/next_card.py`). Skipped when unset. No hardware requirement.
- **Invocation:** `ZENZX_NEXT_FILES=next go test -tags headless -run TestNextZXOS -v .` (the card file is not changed: the save test keeps its writes in memory)
- **Last exercised:** 2026-11-18 env:sandbox — at 0.40.0: passes, in the release's runs Previous: 2026-11-18 env:sandbox — at 0.39.0: passes, in the release's runs

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
- **Last exercised:** 2026-11-18 env:sandbox — at 0.40.0: passes, in the release's runs Previous: 2026-11-18 env:sandbox — at 0.39.0: passes, in the release's runs

### G-05. The Next's picture as the GUI draws it (`TestGLNextPicture`, gl_gui_test.go)

- **Gate:** as G-03: `ZENZX_GL_TESTS` set, the GUI build, a display (Xvfb with software OpenGL is enough).
- **Invocation:** `Xvfb :99 -screen 0 1280x1024x24 &` then `DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 go test -run TestGLNextPicture -v .`. It draws frames through `DisplayManager.Render` and reads the window back: a Layer 2 pixel from bank 8 and its disappearance when NextREG 0x69 bit 7 is cleared, the standard screen through the cell drawing, Timex hi-res at one window pixel per hi-res pixel, a tile of the tilemap, and the return to the cell drawing.
- **Last exercised:** 2026-11-18 env:sandbox — at 0.40.0: passes, in the release's runs Previous: 2026-11-18 env:sandbox — at 0.39.0: passes, in the release's runs

### G-06. ZenZX's Next examples run from NextZXOS (`TestNextZXOSRunsTheExamples`, next_examples_test.go)

- **Gate:** as G-04: `ZENZX_NEXT_FILES` names the folder with the three boot files. Skipped when unset. Since 0.24.0 the examples are on a card of their own, made in a temporary folder as `-next-examples` makes it and put in the second slot; the files in the folder are not changed.
- **Invocation:** `ZENZX_NEXT_FILES=next go test -tags headless -run TestNextZXOSRunsTheExamples -v .` (G-04's own invocation, `-run TestNextZXOS`, runs it too)
- **Last exercised:** 2026-11-18 env:sandbox — at 0.40.0: passes, in the release's runs Previous: 2026-11-18 env:sandbox — at 0.39.0: passes, in the release's runs

### G-07. A card write is seen by NextZXOS after re-insert (`TestNextZXOSSeesCardWriteAfterReinsert`, next_refresh_hw_test.go)

- **Gate:** as G-04/G-06: `ZENZX_NEXT_FILES` names the folder with the three boot files. Skipped when unset.
- **Invariant:** after a file is written to the inserted second card (as the mount drawer does, via `nextcard.AddFiles` which renames a new image over the file) and the card is re-inserted, NextZXOS's own Browser remount (M, then Y) lists and runs the new file. Guards against the emulator serving the orphaned pre-rename handle.
- **Invocation:** `ZENZX_NEXT_FILES=next go test -tags headless -run TestNextZXOSSeesCardWriteAfterReinsert -v .` (`-run TestNextZXOS` runs it too)
- **Last exercised:** 2026-11-20 env:sandbox — passes

### G-08. Two concurrent Next machines each see their own card write (`TestNextZXOSTwoMachinesSeeCardWrites`, next_refresh_multi_hw_test.go)

- **Gate:** as G-04/G-06/G-07: `ZENZX_NEXT_FILES`. Skipped when unset.
- **Invariant:** the G-07 refresh holds for two Next machines running at once, each booting its own minimal (280 MB) card -- made bootable from the 26.11 card -- with its own second card. After each machine writes to its own second card and re-inserts it, each machine's Browser remount gains the new file's row, independently (the cards cannot cross: separate DivMMCs, separate images).
- **Invocation:** `ZENZX_NEXT_FILES=next go test -tags headless -run TestNextZXOSTwoMachinesSeeCardWrites -v .`
- **Last exercised:** 2026-11-20 env:sandbox — passes

### G-09. The Browser refreshes on a host-driven remount, no keypress (`TestNextZXOSRemountRefreshesBrowser`, next_remount_hw_test.go)

- **Gate:** as G-04/G-06/G-07/G-08: `ZENZX_NEXT_FILES`. Skipped when unset.
- **Invariant:** `forceNextZXOSRemount` drives NextZXOS's own M-then-Y Browser remount from the host, by feeding the firmware's key sysvars (LAST_K $5C08, FLAGS $5C3B bit 5) the two keycodes its idle key-wait loop reads -- located by the signature `76 21 3B 5C CB 6E` (HALT / LD HL,FLAGS / BIT 5,(HL)), not a fixed address, so it survives firmware versions that keep the idiom and declines rather than guesses where they do not. After a file is written to the second card and the handle re-inserted (the G-07 fix), the host-driven remount gains the new file's row, exactly as a physical M-then-Y would; a second remount with nothing new is a stable no-op.
- **Invocation:** `ZENZX_NEXT_FILES=next go test -tags headless -run TestNextZXOSRemountRefreshesBrowser -v .`
- **Last exercised:** 2026-11-20 env:sandbox — passes

### G-10. A host-driven remount refreshes the focused machine, not the first (`TestNextZXOSRemountTargetsFocusedMachine`, next_remount_focus_hw_test.go)

- **Gate:** as G-04/G-06/G-07/G-08/G-09: `ZENZX_NEXT_FILES`. Skipped when unset.
- **Invariant:** with two Next machines running, a file dropped while the second is focused refreshes the second, not the first. The mount drawer resolves the focused machine (the deck's front) at drop time and runs `forceNextZXOSRemount` against that machine's own memory; the guard drives that per-machine API with focus on B and asserts B gains the new file's row while A is unchanged. (The drawer type is GUI-only, so the guard drives the API the drawer calls, not the drawer.)
- **Invocation:** `ZENZX_NEXT_FILES=next go test -tags headless -run TestNextZXOSRemountTargetsFocusedMachine -v .`
- **Last exercised:** 2026-11-20 env:sandbox — passes
