# ZenZX — known limits and dormant guards

Version: 0.6.11
Last reviewed: 2026-08-28

Intentional limits, invariant boundaries, and recorded decisions. Defects
and gaps that are meant to be fixed belong in `TRACKING.md`, not here.

## Limits

- **The sandbox has no GUI system libraries.** `check_gui.sh` (the
  release-time gate) tolerates only cgo/system-library link failures on
  a host without ALSA/X11/Wayland/GL, since this sandbox is one. CI
  (`.github/workflows/ci.yml`) builds the real, linked GUI binary on
  every push: cgo-free for darwin/windows via raylib-go's purego backend
  (see "CI and cross-platform builds" below), natively with apt-installed
  dev libraries for linux/amd64 and linux/arm64.
- **BSD has no purego path.** raylib-go's embedded-library backend
  supports linux/darwin/windows only; FreeBSD/OpenBSD/NetBSD need a real
  cgo build with system dev libraries and are not covered by CI or the
  release workflow (GitHub Actions has no BSD runners). `build_bsd.sh`
  remains the only route, run locally via Docker (see T-07: as of this
  version that script does not work).
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
record is. `repoman/guards.py stale` lists guards not exercised since the
previous release; each is run, handed off (`guards.py handoff`), or its
skip is recorded in the release's changelog entry.

### G-01. GUI link build (`build.sh`, `build_linux.sh`)

- **Gate:** host with raylib/oto system libraries; not linkable in the sandbox
- **Invocation:** `./build.sh` (or `./build_linux.sh`) then launch the binary and load a snapshot
- **Last exercised:** 2026-08-27 env:sandbox — exercised via check_gui.sh during the -tapemode=turbo GUI wiring change (T-25) -- GUI build linked, system libs present Previous: 2026-08-25 env:sandbox — exercised by the v0.6.0 release's gui-check step (relcore, 6s, green)

**G-02 retired, 2026-08-28 (T-05 closed at v0.6.8):** `TestFDCReadMatchesDisk`
(`fdc_read_test.go`) now runs unconditionally against a checked-in fixture
(`testdata/synthetic.dsk`), built through the FDC765 controller's own
SEEK/FORMAT TRACK/WRITE DATA command interface by `TestGenerateSyntheticDSK`
in the same file. It is part of the default `go test -tags headless .`
invocation and is no longer a dormant guard. `ZENZX_TEST_DSK` still allows
pointing the test at a real captured image locally.
