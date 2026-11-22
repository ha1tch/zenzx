# Machine map

Updated: 2026-11-17

What separates ZenZX's machine from its two programs, measured with
`tools/depmap`. The map and the tool were made in zenplay for the tree at
0.9.0, when the root package was 57 files; they are kept here now, since
they are about this package. It is the measured ground for T-39. The
figures were taken at 0.27.0 and again at 0.28.0, after the first three
stages below; where they changed, both are given.

## Reproducing

```
go build -o depmap ./tools/depmap
./depmap -dir . -tags headless -clusters docs/machine_clusters.json -alt-goos windows -alt-tags purego -frontier fe-
./depmap -dir . -goos windows -tags purego -clusters docs/machine_clusters.json -frontier fe-
```

`docs/machine_clusters.json` groups the root package's 93 files. Run the
tool again after each step of the extraction: the boundary figures
should only shrink.

## Sizes

The library clusters, which an embedder needs:

| Cluster | Files | Lines | Contents |
|---|---|---|---|
| machine-core | 13 | 5,334 | memory, I/O ports, ULA contention, floating bus, screen, scheduler, the `ZenZX` type, joysticks and their ports, mouse, the two message sinks |
| video | 3 | 348 | the video renderer interface, the classic screen's geometry, the `Display` interface and the display that draws nothing |
| audio | 6 | 1,612 | the AY chip and its gate, the audio engine (sample mixing and filtering) and its device interface, the backend selector |
| tape | 4 | 1,322 | the tape model, TAP and TZX playback, turbo loading |
| snapshot | 6 | 2,380 | snapshot formats, save and load, raw binary loading |
| disc | 2 | 1,245 | the +3's floppy controller and disc images |
| next | 26 | 5,662 | NextREG, Layer 2, sprites, the tilemap, LoRes and the Timex modes, the palette, the frame compositor and the raster, the copper, the DMA, interrupts, config mode, the Multiface, physical memory |
| next-storage | 8 | 1,706 | DivMMC, the SD cards, the boot chain, ZenZX's own card |
| model-other | 2 | 322 | the TS2068 and the non-standard modes |
| scripting | 5 | 820 | keyboard scripts, boot detection, symbol tables for scripts |
| roms | 3 | 183 | embedded ROMs, the custom-ROM menu's data, the version string |
| **library total** | **78** | **20,934** | |

At 0.27.0 the library was 75 files and 20,213 lines: the audio engine
(about 600 lines) was then counted with the frontends, because it existed
only in the GUI build.

The frontends, which an embedder replaces (counted across both build
configurations):

| Cluster | Files | Lines | Contents |
|---|---|---|---|
| fe-display | 3 | 1,600 | the window's display manager, the GPU renderer |
| fe-audio | 2 | 121 | the oto audio device and the context its devices share |
| fe-input | 1 | 845 | keyboard, mouse, tape and disc hot-keys, drag and drop |
| fe-gui | 8 | 3,180 | the menu bar, the SD Card menu, the first-start dialog, overlays and `RunGUI` |
| fe-main | 1 | 645 | `RunHeadless` and its flags |
| **frontend total** | **15** | **6,391** | |

There are 68 test files in the root package (45 untagged, 11 tagged
`!headless`, 12 tagged `headless`).

Since 0.9.0 the library has grown from 43 files and 13,750 lines to 78
and 20,934, nearly all of it the Next.

## The seams

A seam is a name declared once in each build configuration, so that the
core means a different thing by it in each. There are none since 0.28.0.

Until 0.27.0 there were 23, the same as at 0.9.0: `AudioManagerOto` (the
type, its constructor and 12 methods, a real one in the GUI build and a
silent stub in the headless one), `DisplayManager` (the type, its
constructor and 6 methods, likewise twice) and `main`. Now:

- The core has one `AudioEngine` (`audio_engine.go`), which makes samples
  and plays them through an `AudioDevice` if it is given one. The GUI
  program gives it the oto device; with none it is silent and does no
  work.
- The core draws through a `Display` interface (`display_core.go`). A
  machine starts with the display that draws nothing; the GUI program
  gives it the window's `DisplayManager` with `SetDisplay`.
- The two programs are `RunGUI` and `RunHeadless`, two names, called by
  `cmd/zenzx` and `cmd/zenzx-headless`.

## How tightly the programs hold the core

| Configuration | References from the frontends into everything else | Distinct names used | Of which unexported |
|---|---|---|---|
| GUI (windows, `purego`) | 947 | 270 | 126 |
| Headless (linux, `headless`) | 183 | 104 | 40 |

At 0.27.0 these were 1,013, 278 and 125 for the GUI and 196, 111 and 40
headless; at 0.9.0 the GUI's were 838, 203 and 69. The largest flows in
the GUI configuration are the menus and windows into the core (302
references) and input into the core (292).

## State and noise

- The headless build has 32 package-level variables: lookup tables, key
  maps, the embedded ROM and example file systems, the video-renderer
  registry, one function variable (`nextFilesUsable`), the `version`
  string, the audio debug switch read from the environment at start-up,
  and the two message sinks. Only the sinks are written after start-up,
  by `SetLogOutput` and `SetTraceOutput`, which a host calls once; they
  are the package's on purpose, since several machines share one place
  to speak. `twomachines_test.go` runs a 48K and a 128K machine alone,
  in turns and at the same time, and each ends exactly as it does alone.
- `os.Exit` and `flag` appear only in the two programs.
- The core no longer prints. What it has to say (a disc inserted, a
  snapshot saved, the debug dump of `snapshot_debug.go`) goes to one
  sink and its opt-in traces and script warnings to another
  (`logging.go`); both discard until a host sets them. The two programs
  set standard output and standard error, so they print what they did.
- ROMs, snapshots and discs are loaded by path in several places
  (`zenzx.go` reads files 8 times); byte-slice variants exist for the
  ROMs.

## Options

| Option | What happens | Cost | Verdict |
|---|---|---|---|
| A. Core into `pkg/machine` | The 75 library files move; the frontends stay in `package main` and call the library | Export the 125 unexported names the frontends use and rewrite their references; split the 67 test files between the two packages, since most probe unexported state | Possible later, once the library's surface has settled |
| B. Root package becomes the library | Rename the package in its files and the tests; the two `main()` functions become exported `RunGUI` and `RunHeadless` with thin `cmd/` wrappers; embedders build with `-tags headless,purego` | No names change and no test moves; the `version` linker variable moves; build scripts follow the new paths | Done in 0.28.0, and a step towards A |
| C. Subprocess | ZenZX runs as a separate program | None | Rejected: the decision is to embed |

B leaves the GUI files in the embedded library, compiled out under the
`headless` tag. The growth in the frontends' hold on the core since
0.9.0, from 69 unexported names to 125, is the cost of A growing while
it waits.

## Stages

As zenplay's register set them out (its T-21, T-22, T-19, T-20), carried
here under T-39:

| Stage | What | Done when | State |
|---|---|---|---|
| 1 | The seams: `AudioManagerOto` becomes a tag-free audio engine with a device interface, `DisplayManager` an interface | No core file declares anything under a build tag, and the seams report lists nothing | Done, 0.28.0 |
| 2 | The library is silent: the core's prints go through a logger that discards by default | An embedding program sees nothing printed by the library | Done, 0.28.0 |
| 3 | The root package is `package zenzx`; `RunGUI` and `RunHeadless`; `cmd/zenzx` and `cmd/zenzx-headless` | Both programs build and behave as before, and an external module imports the package with `-tags headless,purego` | Done, 0.28.0 |
| 4 | An exported `Machine` over the internals, for embedders | An external module builds a machine of any model, loads a snapshot from bytes, runs frames and reads the picture back | Not started |

Stages 2 and 3 are checked by `scripts/check_import.sh` (`make
check-import`, and in CI): a module made on the spot imports the package
with the embedder's tags, runs a 48K machine from a ROM it reads itself,
reads the picture back, and fails if the library printed anything. Two
machines run side by side, each undisturbed by the other, is
`twomachines_test.go`.

What an embedder can do today is what the exported names allow:
`NewZenZX`, the ROM loaders that take bytes, `Reset`, `RunFrame`,
`DecodeDisplay`, `SetDisplay`, and the audio's `SetDevice`. Choosing a
model, the Next's wiring, snapshots from bytes and input are still done
by the two programs with unexported names; stage 4 is the facade over
those.
