# Changelog

All notable changes to ZenZX are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [0.2.0] - 2026-06-27

Adds a CGO-free headless build and raw-binary loading, intended for automated
testing and for debugging guest software (such as zxgui) without a display.

### Added

- **Headless build** (`-tags headless`): a CGO-free variant with no window and
  no audio device. It boots a Spectrum model, runs a fixed number of frames,
  and writes PNG screenshots of the 256x192 display decoded directly from
  display memory at 0x4000. Flags: `-frames`, `-shot-interval`, `-shot-dir`,
  `-shot-prefix`, `-quiet`, plus the existing model/ROM/snapshot/tape options.
- **Raw binary (`.bin`) direct memory loading**: load a binary blob straight
  into the address space, bypassing tape and disk. New `LoadBIN` and a
  bank-aware `SpectrumMemory.Load(address uint16, data []byte)` primitive
  (matching zen80's RAM/MappedMemory `Load` convention) write through the
  normal memory path, so RAM banking and the screen mirror are honoured and
  ROM regions are protected. Both binaries accept `-bin`, `-binaddr` (hex
  `0x..`/`$..` or decimal), and `-binstart` (PC after load; empty = load
  address, `-1` = leave PC unchanged).
- **Screen dump (`.scr`) loading**: `LoadSCR` and a `-scr` flag on both binaries
  paint a raw 6912-byte screen dump (6144 bitmap + 768 attributes) onto the
  display via the normal memory path, without running anything. Pairs with
  `-frames=1` to render a `.scr` to PNG. (`.scr` was already loadable via GUI
  drag-and-drop; this adds command-line and headless access.)
- `build_headless.sh` for the CGO-free build, and a `-version` flag on both
  binaries (version injected via `-ldflags "-X main.version=..."`).
- `README.md` documenting both build variants, the headless debugging
  workflow, and a **Known issues** section (broken SNA/Z80/DSK loaders and fast
  tape loading; harsh/aliased beeper audio with approximate pitch); `LICENSE`
  (Apache 2.0), `NOTICE`, and `.gitignore`.
- Unit tests for `.bin` loading, `memory.Load`, and the address parsers.

### Changed

- GUI rendering, keyboard input, and the raylib audio backend are now behind a
  `!headless` build constraint; raylib-free equivalents back the headless
  build. Shared display constants moved to `display_constants.go`.
- The on-screen GUI controls help no longer advertises unimplemented shortcuts
  (Alt+A/V/M for audio, Alt+H/S for overlay/stripes were printed but never
  wired); it now lists only the shortcuts that are actually handled. The README
  documents the full runtime and Spectrum key mappings.
- The build scripts (`build.sh`, `build_linux.sh`, `build_windows.sh`) now use
  the Go package build instead of hand-maintained per-file lists, which had
  drifted out of sync with the sources. The Linux Docker image is bumped to
  `golang:1.25` to match the `go.mod` directive.

### Removed

- Deleted the orphaned `audio.go` (the legacy raylib audio backend). It was
  excluded from every build script and duplicated the `AYChip` type that is
  canonically defined in `ay8912.go`; its removal resolves a duplicate-symbol
  error that prevented a clean module build.

## [0.1.0] - 2026-06-26

First versioned release. Establishes the release-hygiene baseline for the
existing ZenZX codebase.

### Added

- `go.mod` declaring `module github.com/ha1tch/zenzx` (`go 1.25`), depending on
  `github.com/ha1tch/zen80`.
- `pkg/version` package and the `main.version` build-time string.
- Release scaffolding: `VERSION`, `syncver.sh`, `release.sh`, and this
  changelog.

[0.2.0]: https://github.com/ha1tch/zenzx/releases/tag/v0.2.0
[0.1.0]: https://github.com/ha1tch/zenzx/releases/tag/v0.1.0
