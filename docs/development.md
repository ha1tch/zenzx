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
builds") for why the split falls where it does, and why BSD isn't in it.

Pushing a `v*` tag runs `.github/workflows/release.yml`, which builds and
publishes prebuilt archives (headless and GUI, all platforms above, plus
headless-only for freebsd/amd64) as GitHub release assets. This is
separate from `repoman relcore`'s local checkpoint: run the local release
first to gate and verify, then tag once it's green.

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
