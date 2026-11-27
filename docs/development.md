# Development

Versioning, releases, continuous integration and where things are in the source.

## Versioning and releases

`VERSION` is canonical; `pkg/version/version.go` is a synced stamp. Tooling
is the [gorepoman](https://github.com/ha1tch/gorepoman) binary (migrated from the
vendored [python-repoman](https://github.com/ha1tch/repoman)), driven by
`.repoman.json`):

```
repoman syncver show
repoman register list         # open work
repoman guards stale          # dormant guards not run since the last release
repoman relcore <version>     # sync, gates, build, test, smoke, checkpoint zip
```

`repoman relcore` journals each step in `.release-state.json` and resumes with
`--resume`; full output goes to `release-<version>.log`. Before a release, every
stale dormant guard is run, handed off (`repoman guards handoff`), or its skip is
recorded in the changelog entry.

## Continuous integration and cross-platform builds

`.github/workflows/ci.yml` builds and tests every push: the cgo-free
headless core, the GUI cross-compiled for darwin/windows (amd64 and
arm64, cgo-free via raylib-go's `purego` backend), and the GUI natively
for linux/amd64 and linux/arm64 (cgo, via apt-installed ALSA/X11/Wayland/
GL dev libraries). See `docs/KNOWN_ISSUES.md` ("CI and cross-platform
builds") for why the split falls where it does. FreeBSD is not in it: the GUI for
FreeBSD (amd64 and arm64, cgo-free) is built and checked by its own workflow,
`.github/workflows/freebsd.yml`, run by hand and on every push to the main
branch, so that a failure there stops neither CI nor a release, and nothing is
published from it. Nothing starts the program on FreeBSD (GitHub has no runner
for it).

Pushing a `v*` tag runs `.github/workflows/release.yml`, which builds and
publishes prebuilt archives (headless and GUI, all platforms above, plus
headless-only for freebsd/amd64) as GitHub release assets. This is
separate from `repoman relcore`'s local checkpoint: run the local release
first to gate and verify, then tag once it's green.

Once the release is published, the same run makes two more things and attaches
them to it. Each is a job that calls a workflow of its own, so either can also be
run by hand for a tag that already has a release (Actions > Installers, or
Windows 7 > Run workflow, with the tag, such as `v0.50.0`). A failure in either
leaves the release as it was.

- `installers.yml` makes the installers from the release's own archives, with
  `tools/package.py`: a `.deb` for linux/amd64 and linux/arm64, a `.msi` (WiX 3.14)
  for windows/amd64 and windows/arm64, and a `.dmg` holding a `ZenZX.app` (both
  macOS processors, signed ad hoc). Each is installed or mounted, started with
  `-version` and removed on a runner of its own system, and the five are attached,
  with a checksum file, only if all of them passed. They register no file types
  (the programs ignore a file named on the command line), and nothing is signed.
  The design is `docs/proposals/installers.md`; the icon is `assets/icon/`, made by
  `scripts/make_icons.py` from `speccy01.zani`, which CI checks.
- `windows7.yml` makes a Windows 7 program: Go 1.21 and later do not run on Windows
  7, so it is built with XTLS/go-win7, Go's own source with patches that restore
  it, a fixed download checked against its SHA-256 (a third-party compiler; the
  decision to use it is recorded in T-95). It is windows/amd64 only, is checked
  for `ProcessPrng`, the one function the standard build imports that Windows 7
  lacks, and is attached as a `.zip` and a `.msi`. The runners cannot run Windows 7,
  so whether it starts there is for a person with the machine (guard G-21).
- `documents.yml` attaches the manual as a PDF and the examples as a zip, with a
  checksum file: `tools/build_manual.py` turns `docs/MANUAL.md` into HTML with a
  contents list and a print style (python-markdown) and the runner's Chrome prints
  it, with the pictures in it and links to the other documents of `docs/` made into
  links to the tag on GitHub; `tools/build_examples.py` packs `examples/` (the
  launchers, `launch.py`, the prebuilt tape and `.nex` files, without what building
  leaves behind) with the pictures their READMEs show. Both stop, and write
  nothing, when something is missing. Its own tests run first
  (`scripts/test_build_manual.py`, `test_build_examples.py`, `test_launch.py`,
  `test_findpy_sh.py` and `test_fetch_cli.py`, which build the headless program to fetch Python
  with; CI runs them too). `make manual-pdf` and `make examples-zip` make them into `dist/`.
  The Windows launchers (`examples/*/*.cmd`, `examples/tools/`) cannot be run by
  the runners here (guard G-23); `scripts/test_findpy_cmd.py` runs `findpy.cmd` under Wine where
  there is Wine (guard G-25).

`windows-smoke.yml` is run by hand: it takes the Windows programs from the page,
checks them against `runners.json` and starts each on a real Windows runner (guard
G-19).

The same tag also publishes the page at https://ha1tch.github.io/zenzx/
(the `pages` and `deploy` jobs of `release.yml`): the GUI program for each
platform as a raw executable, `bin/checksums.txt`, and `runners.json`, which
lists each program with its size and SHA-256 for a ZenZX client that needs
the program for another platform. `tools/build_site.py` makes it from the
archives the release's own GUI jobs built, so the page's programs are the
release's, byte for byte; it stops, and nothing is deployed, if a platform
is missing or a file is not an executable for its platform
(`scripts/test_build_site.py`, run by CI). The page also lists the installers, the
Windows 7 files, the manual and the examples, with a link to each on the release, its
size and its SHA-256; they are not in `runners.json`. It is laid out by `tools/site_page.py`
the way the program is (the Spectrum theme's bar and panels, Spleen, the icon, and the figure
that waits in an empty window, played from `sprites/wait/sequence.txt`), and it chooses first:
a script reads the browser's agent string (and its client hints, where it has them), opens the
tab of the visitor's system and marks the build that fits, and with no script the three systems
are listed one after another, each with a note on how to tell which build is the right one.
What the release lacks is not mentioned. The face is made from `pkg/fonts/spleen-5x8.bdf` by
`tools/make_site_font.py` (needs fontTools; `check` says whether the two files of `assets/site/`
are current: the face as the program has it, in cells of five pixels, for code and checksums, and the
same pixels each as wide as its ink, for the text, since Spleen is monospaced and an "i" in a cell as
wide as an "m" leaves gaps); `scripts/test_build_site.py` also drives the script in Chromium where Playwright
is installed (G-24). The `pages` job waits for the
three jobs above, but a failure in any only leaves its files off the page: it is built and deployed all
the same. The page holds the newest tag
only, since a deployment replaces the last; every version stays as an
archive on its GitHub release. The repository's Pages source must be set to
"GitHub Actions" (Settings > Pages), once, or the `deploy` job has nowhere to go.

## Project layout

Emulation core (raylib-free, shared by both builds): `zenzx.go`, `memory.go`,
`io.go`, `fdc.go`, `tape*.go`, `snapshot*.go`, `ay8912.go`, `audio_*.go`,
`loadbin.go`, `ts2068.go`, `joystick.go`, `mouse.go`. GUI-only (`!headless`):
`display.go`, `input.go`, `audio_oto.go`, `audio_device_oto.go`,
`zenzx_gui.go` (`RunGUI`). Headless-only: `zenzx_headless.go`
(`RunHeadless`). The root package is a library, `package zenzx`; the two
programs are `cmd/zenzx` and `cmd/zenzx-headless`, one call each. Another
program can import the machine with `-tags headless,purego` (no C compiler,
no window). What the library can reach so far is in `docs/MACHINE_MAP.md`.

`contrib/tape-corpus-harness/` is a standalone tool for measuring
real-world tape-loading coverage, in its own Go module with no import
relationship to zenzx in either direction -- `go list ./...` from the
repo root doesn't see it, and neither does the release build or test
gate. See its own README.
