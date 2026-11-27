# ZenZX — live register

Version: 0.50.0
Last reviewed: 2026-11-25

Open, actionable items only. Closed items move verbatim to
`RESOLVED.md` (see `repoman register close`); a closed item still
listed here is itself a defect. Intentional limits and the dormant-guard
table live in `KNOWN_ISSUES.md`.

ID note (0.11.0): the 0.9.x and 0.10.0 lines were developed apart and both used T-35 and T-36. The 0.9.x numbers stand (T-35 closed in 0.9.2, T-36 is the NextZXOS boot chain, cited throughout the source). The 0.10.0 items were renumbered when the lines were merged: T-35 became T-38, T-36 became T-39 and T-37 became T-40; zenplay's T-05 and T-06 point at T-39 and T-40.

Status legend: ✓ done · ◐ partial · ☐ not started · ✗ dropped.

## Status table

| ID | Summary | Theme | Priority | Status | Blocks |
|---|---|---|---|---|---|

| T-03 | CPU clock modelled as 3.5 MHz; real 48K is 3.5469 MHz (~1.3 % pitch/timing error) | timing | P3 | ☐ | — |
| T-04 | AY-3-8912 residual aliasing beyond the 0.3.4 area-sampling baseline | audio | P3 | ☐ | — |
| T-07 | Dockerfile and build_bsd.sh are broken (unresolved template variables) | ci | P3 | ☐ | — |
| T-08 | GUI cross-platform CI depends on raylib-go/oto staying cgo-free on darwin+windows | ci | P3 | ☐ | — |
| T-09 | Non-standard graphics modes accepted but not implemented (mode-zenzx-01, mode-zenzx-02) | graphics | P2 | ☐ | — |
| T-10 | Non-standard storage backend accepted but not implemented (storage-zenzx-posix) | storage | P2 | ☐ | — |
| T-13 | Joystick emulation: headless zenscript driving not implemented | input | P3 | ◐ | — |
| T-15 | Real Extension ROM CHNG_VID service diverges partway through OPDFIL's relocation, root cause undiagnosed | model | P3 | ☐ | — |
| T-20 | 128K contention model is address-range based, not bank-aware; frame geometry (228 t/line, start 14361) is an unverified extrapolation | timing | P3 | ☐ | — |
| T-21 | Floating bus: instruction-start position granularity; 128K port-decode subtleties unmodeled; TS2068 FE-port idle levels legacy/unverified | model | P3 | ☐ | — |
| T-22 | Batman-class load failures: root cause still open after five hardware-fidelity fixes; next instrument is deep RAM-PC sequence + memory-write-delta trace | model | P2 | ☐ | — |
| T-23 | Fast mode's ROM-trap only accelerates while PC is literally inside the standard LD-BYTES routine; any custom/protected loader (common in real commercial tapes) gets zero benefit the moment it takes over | tape | P2 | ☐ | — |
| T-24 | No mode-independent, trustworthy signal exists for 'did this tape actually load correctly' -- every mode's own self-reported completion (fast's trap return, turbo's Position-reached-end, accurate's Playing==false) has been observed true while the load was wrong | tape | P1 | ☐ | — |
| T-29 | Layer 2 graphics mode (256x192 8-bit pixel-mapped) | next-support | P3 | ◐ | — |
| T-30 | Hardware sprites (128x 16x16, unscaled minimum) | next-support | P3 | ◐ | — |
| T-31 | 9-bit/512-colour Next palette with NextREG palette control | next-support | P3 | ◐ | — |
| T-32 | NEX file loading (direct memory/Layer2/palette injection, no OS) | next-support | P3 | ☐ | — |
| T-36 | Real NextZXOS/DivMMC/SD-card boot chain (Tier 2) | next-support | P3 | ◐ | — |
| T-39 | Extract the machine core from `package main` into an embeddable `pkg/machine` | embedding | P1 | ◐ | — |
| T-40 | Provide a purego-safe audio backend so the embedded machine builds with `CGO_ENABLED=0` | embedding | P1 | ☐ | after T-39 |
| T-44 | Tilemap and the Next's Timex modes are not drawn | next-support | P2 | ◐ | — |
| T-50 | Remove the Next drawing code the frame compositor replaced | next-support | P2 | ☐ | — |
| T-51 | What the Next's frame compositor does not draw | next-support | P3 | ☐ | — |
| T-55 | The 128K machines run the 48K's frame of 69888 T-states | timing | P2 | ☐ | — |
| T-56 | A NextBASIC loop takes 5% fewer frames than in CSpect | next-support | P3 | ☐ | — |
| T-57 | Controllers: what has not been tried with real ones, and what a Next has that is not modelled | input | P3 | ☐ | — |
| T-58 | The Next's DMA (ports 0x6B and 0x0B) is not emulated | next-support | P1 | ◐ | — |
| T-59 | The Next's interrupt mode (NextREG 0xC0 to 0xCF) and the CTC are not emulated | next-support | P2 | ◐ | — |
| T-60 | IM 2: the CPU core clears bit 0 of the vector; ZenZX reads the handler's address again | timing | P3 | ☐ | — |
| T-61 | The Multiface has no NMI key, and only what NextZXOS uses was tried | next-support | P3 | ☐ | — |
| T-62 | The Next examples were compared with jnext only, not with CSpect | next-support | P3 | ☐ | — |
| T-63 | SD Card menu: the first slot and per-slot write protection are missing | next-support | P3 | ☐ | — |
| T-64 | DMA+ and 4 MB: board ID, interrupts holding the DMA, and a comparison with CSpect 4.5 are missing | next-support | P3 | ☐ | — |
| T-65 | The workbench: the running machine as the thing being edited | embedding | P2 | ☐ | after T-39 |
| T-66 | gorepoman: no way to retire a tracked file; relcore's archive manifest is ordered differently from manifest.sh | hygiene | P3 | ☐ | — |
| T-67 | The machine dock: what the first cut leaves out | embedding | P3 | ☐ | — |
| T-68 | Escape does not close an open dropdown of the menu bar | input | P3 | ☐ | — |
| T-69 | The link, -multi and -autorun: what the first cut leaves out | embedding | P3 | ☐ | — |
| T-72 | NextZXOS .dsk disk-image mounting on the Next (A:/B:), not only copy-to-card | next-support | P3 | ☐ | — |
| T-73 | Root clean-up: scripts to scripts/, leaf subsystems out of the root package into pkg/ and internal/ | hygiene | P2 | ☐ | v1.0.0 |
| T-75 | On macOS, resizing the window crops the emulated screen or leaves a black area, and does not re-centre it: fixed in the sandbox, not yet seen on a Mac | graphics | P3 | ◐ | the public release |
| T-79 | The GUI has never been launched with the Next boot cache on auto | next-support | P3 | ☐ | — |
| T-80 | The Next boot cache over a changed card: memory fields not analysed one by one | next-support | P3 | ☐ | — |
| T-83 | zen80 raises R by three for every repeat of LDIR and its kin; the real Z80 raises it by two | timing | P2 | ☐ | — |
| T-84 | The RoboCop .rzx goes out of step at frame 5006 of 38881: a loop in a long interrupts-off routine runs about 130 fetches fewer than in the recorder | formats | P3 | ☐ | — |
| T-85 | Formats zentools has that ZenZX does not use: Next and Spectrum screenshots, .pzx tapes, symbol files, saving .szx, ZX0 | formats | P4 | ☐ | — |
| T-86 | Menu and key-handler messages other than the Storage menu's go to the terminal only; GUI-only users never see them | ux | P3 | ☐ | — |
| T-87 | The bar's text scale is made by a wrapper around zenui's renderer; zenui should take a label scale | ux | P4 | ☐ | — |
| T-88 | The tape deck's first cut: one machine at a time, no Fast mode, no seeking, nominal 3.5 MHz time, a colour per tape and not per header | ux | P3 | ☐ | — |
| T-89 | An exported program has not been run on Windows, or seen opening its window on Linux | embedding | P2 | ◐ | — |
| T-90 | Machine > Stop and the Pause/Play row have not been seen in a window with the machine's picture | ux | P3 | ☐ | — |
| T-91 | Export for another platform: fetch and verify the other system's stub, a Runs-on row in the Export form, and the signed-manifest CI stages | export | P3 | ☐ | — |
| T-92 | The Windows program does not start for two testers | windows | P1 | ☐ | — |
| T-93 | Installers: .deb (amd64, arm64), .msi (amd64, arm64) and .dmg with a ZenZX.app, built in the release workflow | packaging | P3 | ◐ | — |
| T-94 | A file named on the command line is ignored: it must open, so that installers can register file types | ux | P3 | ☐ | — |
| T-95 | A Windows 7 program, in addition to the Windows 10 ones (needs a patched Go toolchain) | windows | P2 | ◐ | — |
| T-96 | Platforms not built: windows/386 (headless builds, one test overflows a 32-bit int; no GUI) and the FreeBSD GUI (native build only) | ci | P4 | ☐ | — |
| T-97 | Full screen has been seen only in a window on Linux with no window manager: not on Windows, macOS or under a window manager | windows | P3 | ◐ | — |
| T-98 | The Windows launchers of the examples and the Python helper have never been run on Windows; the documents job (manual PDF, examples zip) has not run on GitHub | packaging | P2 | ◐ | — |
| T-108 | ZenZX needs zenui v0.4.0 published: it builds only through a local go.work until then | release | P1 | ☐ | — |

## timing

### T-03. CPU clock modelled as 3.5 MHz; real 48K is 3.5469 MHz (~1.3 % pitch/timing error)

Theme: timing · Priority: P3 · Status: ☐

- **Trigger:** README Known issues. Audio sample timing derives from the 3.5 MHz constant with per-sample integer truncation, so pitch is ~1.3 % low and can drift; frame timing is marginally affected.
- **Fix:** model the per-machine clock (48K 3.5 MHz vs 128K-family 3.5469 MHz) and derive the audio sample step from it with a fractional accumulator, not truncation.


### T-20. 128K contention model is address-range based, not bank-aware; frame geometry (228 t/line, start 14361) is an unverified extrapolation

Theme: timing · Priority: P3 · Status: ☐

contention.go's 128K path checks 0x4000-0x7FFF only: a program paging a contended bank elsewhere (or an uncontended one into that window) gets wrong delays. The pattern shape and start cycle for 128K were extrapolated from the 48K FAQ table, not confirmed against an authoritative source. 48K is precise (measured bounds ~0.33% mem / ~0% IO with zen80 0.5.0's offset tracking).

### T-55. The 128K machines run the 48K's frame of 69888 T-states

Theme: timing · Priority: P2 · Status: ☐

- **Trigger:** 0.17.0, writing the line interrupt: a Next's frame came out as 306 lines. Every machine runs a frame of 69888 T-states, the 48K's 312 lines of 224. The 128K, +2, +2A and +3 have 311 lines of 228: 70908. Their lines are counted as 228 T-states (NextREG 0x1E/0x1F, contention128 in contention.go, which also takes the frame as 70908), so the frame and the contention pattern disagree, as they did on the 48K before the frame was made 69888.
- **Effect:** on the 128K machines the frame is 1.4% short, so music and anything else paced by the interrupt runs that much fast, and the contention pattern drifts down the frame by 1020 T-states each frame.
- **Done in 0.17.0, for the Next only:** a Next's frame is 70908 T-states (`nextFrameLength`, nextvideoline.go), because its line counter, copper and line interrupt count the 311 lines and a program that waits for lines 306 to 310, the last of the top border, was never answered.
- **Fix:** the same frame for the 128K, +2, +2A and +3, with the sound, the border recording and the tape checked against it, and a contention test as the 48K has.
- **Done when:** a 128K machine's frame is 70908 T-states and its contention pattern stays in place from frame to frame.

### T-60. IM 2: the CPU core clears bit 0 of the vector; ZenZX reads the handler's address again

Theme: timing · Priority: P3 · Status: ☐

- **Trigger:** 0.22.0, T-52: the card's palette cycling demo stopped at its first interrupt. It sets I to 0xFE and only the bytes at 0xFE01 and 0xFF00 of its table, so that a vector of 0xFF (or 0x00) gives a handler in page 0xBF; zen80 0.5.7 computes the table's address as `I<<8 | vector&0xFE`, read the handler's address from 0xFEFE and went to 0x0000.
- **Known:** the Z80 uses all eight bits of the byte on the bus; an even vector is the convention of Zilog's peripherals, not the CPU's rule. A Spectrum's ULA gives none and the CPU reads 0xFF, which is why tables are 257 bytes long. This is on every model, and only shows with a table that is not 257 equal bytes.
- **Here:** `im2vector.go` reads the handler's address again from the whole vector when it was odd (called from `stepCPU` after an interrupt accepted in mode 2), with a test (`TestIM2UsesTheWholeVector`). With a corrected core it reads the same address twice.
- **Done when:** zen80 uses the whole vector (the one line in its `handleInterrupts`), ZenZX's go.mod asks for that version, and `im2vector.go` and its call are removed with the test kept.

### T-83. zen80 raises R by three for every repeat of LDIR and its kin; the real Z80 raises it by two

Theme: timing · Priority: P2 · Status: ☐

Trigger: found 2026-11-23 while building RZX replay (rzx.go). Counting the fetches of a recorded frame by R went wrong by one at frame 89 of the RoboCop recording, on an LDIR.

Scope: zen80 v0.5.7 puts R up by three for every repeat of LDIR, LDDR, CPIR, CPDR, INIR, INDR, OTIR and OTDR; the real Z80 re-fetches the two opcode bytes (ED and the second) on each repeat, so it is two. The eight sites are in z80/prefix_ed.go, each under the comment "Increment R for the extra M1 cycle when repeating". rzx.go counts a repeat as two by hand (rzxKind, rzxBlock) so replay does not depend on it, but any program that reads R after a block instruction (LD A,R as a random number, as RoboCop does; copy-protection loaders such as Speedlock, which T-22 may involve: not checked) sees the wrong value. A copy of zen80 with the eight lines removed built and ran the RoboCop replay to the same point as the unpatched one (T-84), so this is not what stops that recording. Fix belongs upstream in github.com/ha1tch/zen80 with a test (R after LDIR of N repeats is R0+2N); when it lands, rzxBlock and its test lines can go. Done when: zen80 is fixed and ZenZX is on the release that has it.

## audio

### T-04. AY-3-8912 residual aliasing beyond the 0.3.4 area-sampling baseline

Theme: audio · Priority: P3 · Status: ☐

- **Trigger:** 0.3.4 cycle. Area-sampling (accumulate `mixOutput()` per AY clock, average per output sample) cut HF energy ~40x. A polyBLEP attempt was abandoned because the measurement harness could not be trusted (contradictory metrics; the big-edge acceptance metric was wrong for band-limited signals).
- **Fix:** build a trustworthy inharmonic-energy metric first, then evaluate polyBLEP or a proper decimating lowpass against it. Do not touch the synthesis path until the metric is validated on a known signal.


## hygiene

### T-66. gorepoman: no way to retire a tracked file; relcore's archive manifest is ordered differently from manifest.sh

Theme: hygiene · Priority: P3 · Status: ☐

Trigger: cutting 0.28.0 (2026-11-17). Two gaps in gorepoman 0.16.0 that this repository now works around by hand.

1. A file the edit journal tracks cannot be removed on purpose. `repoman provenance check` reports it missing, `provenance sanction` refuses a file that no longer exists, and there is nothing else. At 0.27.0 two such files were cleared from the journal by hand, with approval. At 0.28.0 the oto audio code was kept in two files, `audio_oto.go` and `audio_device_oto.go`, only because both names were tracked. Every root `.go` file is tracked since the package rename, so any later removal or rename meets this.

2. `relcore`'s archive step writes its own `MANIFEST.sha256` into the zip, in a different line order from `manifest.sh` (which sorts paths by byte): `cmd/zenzx-headless/main.go` and `cmd/zenzx/main.go` swap. Unpacked into a git checkout, that manifest fails `sh manifest.sh check` as stale although every hash is right. The 0.28.0 archive had the repository's manifest put back into it after relcore.

Scope: raise both upstream in gorepoman (a `provenance retire FILE --reason` or the like; the archive keeping the manifest it finds, or sorting by byte). Until then: after `relcore`, replace the zip's `MANIFEST.sha256` with the repository's and check the unpacked tree with `sh manifest.sh check`.

Done when: a tracked file can be retired with a recorded reason, and the archive's manifest passes `manifest.sh check` in a checkout without a manual step.

### T-73. Root clean-up: scripts to scripts/, leaf subsystems out of the root package into pkg/ and internal/

Theme: hygiene · Priority: P2 · Status: ☐ · Blocks/after: v1.0.0

Trigger: a study of the project root, 2026-11-22. To be done before v1.0.0, not before.

The root holds 218 tracked entries: 190 Go files (about 123 production, 37k lines, and the tests, 23k lines) and 28 others. The Go files are one package, `zenzx`, with 395 exported identifiers, and everything hangs off `ZenZX`, `SpectrumMemory` and `SpectrumIO`. A package cannot be split across directories, so moving a file means extracting a subsystem behind a new API: every unexported name that crosses the cut is exported, and the tests that read private fields go with it. Where a moved type is exported from the root today, a type alias in the root keeps the public API as it is.

Work, in order:

1. Scripts: `build*.sh`, `check_*.sh`, `manifest.sh`, `run_next_divmmc.sh` and `smoke_headless.sh` to `scripts/`, with the `Makefile`, the `.repoman.json` release steps, `ci.yml`, `nextchain.go` and the manual following them. They assume the repo root as their working directory. The embedded files (`help.txt`, `about.txt`, `machines.json`, `settings.json`, the ROMs, the examples) stay while the code that embeds them does, since `go:embed` reaches only below its own package.
2. A way to move journal-tracked files in gorepoman (a move or rehome verb), or a recorded journal reset. Without one a move shows as a provenance failure for each of the 63 tracked files.
3. Leaf extraction, each with its tests and both build tags: the AY chip and the sinc filter, then the SD card (3 files, 6 names needed from the rest), then the floppy controller and disk images (3 files, 7), then tape (2 files, 10, through a small machine interface). Standalone pieces go in `pkg/`, machine glue in `internal/`.
4. The snapshot codecs, after the machine state structs are separate (41 names needed from `snapshot.go` today).
5. Left alone: the Next hardware (26 files, woven into memory and I/O), video, and the GUI split (about 100 core members would have to be exported or reached through a host interface), unless a design for the core and the GUI comes first.

Each step is held to both builds, `go vet`, the full suite, G-04 and a fresh relcore. The cluster sizes were measured by name matching over the files, which over-counts a little where names collide (`Open`, `Read`).

## ci

### T-07. Dockerfile and build_bsd.sh are broken (unresolved template variables)

Theme: ci · Priority: P3 · Status: ☐

- **Trigger:** session review 2026-08-17 while wiring CI. `Dockerfile` has empty/unresolved variables (`${BINARY_NAME}`, `${VERSION}`, blank `CC=`/`CXX=++`) and never worked as committed. `build_bsd.sh` / `build_example_bsd.sh` are similarly a broken template (empty shell variables, a garbled heredoc, hardcoded absolute paths from another machine). Neither is exercised by CI or the release workflow — CI/release now cover linux/darwin/windows natively; BSD has no purego path (see KNOWN_ISSUES) and was never covered by these scripts working correctly either.
- **Fix:** rewrite both from the working `build_linux.sh` pattern (real variables, no leftover template placeholders), or remove them if BSD support is not worth maintaining.
- **Where they are (0.27.0):** `scripts/retired`, out of the repository's root, with `_bsd.sh`; nothing calls them.


### T-08. GUI cross-platform CI depends on raylib-go/oto staying cgo-free on darwin+windows

Theme: ci · Priority: P3 · Status: ☐

- **Trigger:** session review 2026-08-17. The darwin/windows GUI build in CI and the release workflow is cgo-free only because raylib-go v0.60.0's purego backend embeds a native library and oto v3.4.0 has no cgo import on those platforms (verified by reading both vendored sources). Neither guarantee is part of either library's stated compatibility contract.
- **Fix:** when bumping raylib-go or oto, re-run `go build` with `CGO_ENABLED=0` for darwin/amd64, darwin/arm64, windows/amd64, windows/arm64 locally before assuming the CI matrix still passes; grep the new vendored source for `import "C"` on those platforms as a quick check.

### T-96. Platforms not built: windows/386 (headless builds, one test overflows a 32-bit int; no GUI) and the FreeBSD GUI (native build only)

Theme: ci · Priority: P4 · Status: ☐

- **Trigger:** 2026-11-25, a question: can ZenZX be built for 32-bit Windows (windows/386) and for FreeBSD? Tried by building into a scratch directory; nothing was changed.
- **What builds:** windows/386 headless (`CGO_ENABLED=0 GOOS=windows GOARCH=386 go build -tags headless ./cmd/zenzx-headless`) and freebsd/amd64 headless, which the release already ships. The headless tests run as 32-bit code (`GOOS=linux GOARCH=386`, which a 64-bit Linux runs natively) passed in every package but one.
- **What does not:** (1) `pkg/romimage/romimage_test.go:97` uses `1 << 40` as an `int`, which overflows on a 32-bit `int`; the package's tests do not compile there. (2) The windows/386 GUI: `github.com/jupiterrider/ffi` v0.7.0, which raylib-go's cgo-free layer uses, has files for windows/amd64 and windows/arm64 only, so every file is excluded. (3) The FreeBSD GUI cannot be cross-built from Linux: purego's `fakecgo` for FreeBSD and oto's audio need cgo there. It would be a native build on FreeBSD (a virtual machine in CI, or by hand) with the raylib, X11 and audio libraries; untried, and there is no FreeBSD machine here to start it on.
- **Fix:** for 32-bit Windows headless: make that test's size an `int64`, add windows/386 to the headless build matrix of `release.yml` and `ci.yml`, and run the headless tests as 386 once in CI. The 32-bit GUI and the FreeBSD GUI are separate decisions, each needing a different binding or a native build.
- **Done when:** the headless program for windows/386 is built and attached by the release and its tests pass as 32-bit code, or the tech team decides not to ship it and this item is dropped; the same decision is recorded for the FreeBSD GUI.

## graphics

### T-09. Non-standard graphics modes accepted but not implemented (mode-zenzx-01, mode-zenzx-02)

Theme: graphics · Priority: P2 · Status: ☐

- **Trigger:** 2026-08-17, master-switch infrastructure landed in nonstandard.go. `-ns-graphics` validates and stores the requested mode on `zx.nonStandard.Graphics` but no rendering code reads that field yet -- display stays standard regardless of the value. (Timex hi-colour split out to T-11.)
- **Update 2026-08-17:** the video pipeline is now pluggable (`videorender.go`; see `docs/video-architecture.md`). Implementing either mode means writing a `VideoRenderer` (`Name`, `Decode`, `Dimensions`, `BorderMargins`) and calling `RegisterVideoRenderer` in an `init()` -- no changes needed to display.go, display_headless.go, or either main.
- **Fix, per mode:**
  - `mode-zenzx-01`: 256x192, 3 pixels/byte, linear framebuffer, no attribute clash -- a genuinely new pixel format distinct from the standard bitmap+attribute layout, needs its own memory region, write path, and renderer. `Dimensions()` = (256, 192); `BorderMargins()` needs a decision (proportional to standard, or none -- point 2 in video-architecture.md, border is optional per mode).
  - `mode-zenzx-02`: 512x384, double resolution -- needs a renderer producing 4x the standard pixel count and a decision on how source data maps to the higher resolution (upscale vs a native double-res source format). `Dimensions()` = (512, 384); `BorderMargins()` needs the same decision, doubled if proportional. `DisplayManager.maxMultiplierThatFits()` already clamps the window to the monitor, but verify it in practice once this exists -- untested against a real high-res mode so far.
  - Neither mode supports FLASH (no other mode does -- see video-architecture.md); do not read `screen.flashEnabled`/`flashTickTock` from either renderer.
  - Each mode needs a headless zenscript regression (boot, engage the mode, screenshot, assert) before being considered done.

### T-75. On macOS, resizing the window crops the emulated screen or leaves a black area, and does not re-centre it

Theme: graphics · Priority: P3 · Status: ◐ · Blocks/after: the public release

Trigger: the same feedback, on macOS, 2026-11-22: resizing the window is unpredictable. It either crops the emulated screen or leaves a black area outside it, and it does not re-centre the screen area.

Scope: reproduce on a Mac -- the cloud sandbox cannot -- and read how the picture is scaled and placed when the window changes size (the destination rectangle, the menu bar and dock heights, and the HiDPI factor, which differs between the window's size and the render size on a retina display; to check, not yet known to be the cause). Make the picture fit and sit centred at every window size, as it does on the other systems.

Done when: after a resize on macOS, retina or not, the picture is the whole screen area, centred, with the same border on every side, and a test of the placement arithmetic at HiDPI factors 1 and 2 holds it.

Progress, 0.50.0 (2026-11-23). Shipped: the display compares the window's real size (the framebuffer's, in drawing units: `GetRenderWidth/Height` over `GetWindowScaleDPI`; `GetScreenWidth` is not used because `SetWindowSize` sets it to the size asked for whether the system gave it or not) with the size it asked for; when the window has kept another size for three frames it fits the zoom to it, and where the window stays a size the zoom cannot fill it draws the picture into a canvas and puts that in the middle, scaled down in proportion when the window is smaller than it (`picturefit.go`, `display_fit_gui.go`). While the window is the size asked for, nothing changes. Tests: `picturefit_test.go` (arithmetic at HiDPI factors 1 and 2: equal margins, nothing cropped, proportions kept) and `TestGLPictureFitsAWindowOfAnotherSize` (G-14: the window held at four sizes, the picture's border found, checked to fail with the fit taken out). Added later the same day: found by running the program under a window manager (Xvfb and a small one, enlarging the window as a Mac's zoom does) and reproducing the report that a maximised window left the picture at the top left on black: the fixed bar's reservation followed the window's width through the bar's text scale and sent the window back, and the zoom the fit chose was asked of the window, which a maximised one may refuse. Both are fixed (`windowkept_gui_test.go`; the display keeps the window it is given and centres the picture in it) and `ZENZX_WINDOW_DEBUG=1` prints what the window reports. Outstanding: nothing here has run on a Mac, and the cause (the system's own clamping of the window to the usable screen, retina) is read from the platform's known behaviour, not seen. Closure needs a run on macOS, retina and not, with the window dragged and the zoom changed; what ran, where and when is recorded here.

## storage

### T-10. Non-standard storage backend accepted but not implemented (storage-zenzx-posix)

Theme: storage · Priority: P2 · Status: ☐

- **Trigger:** 2026-08-17, master-switch infrastructure landed in nonstandard.go. `-ns-storage storage-zenzx-posix` validates and stores the request on `zx.nonStandard.Storage` but no storage backend reads that field yet -- the +3 FDC/DSK path (fdc.go) is unaffected.
- **Fix:** design and implement a POSIX-filesystem-backed storage device (distinct from FDC/DSK emulation -- presumably direct host-filesystem access from guest code via a custom port or trap protocol). Needs a design note before implementation: what guest-side interface exposes it (new I/O ports? ROM trap? memory-mapped?), and what host paths it is allowed to touch.


## model

### T-15. Real Extension ROM CHNG_VID service diverges partway through OPDFIL's relocation, root cause undiagnosed

Theme: model · Priority: P3 · Status: ☐

- **Trigger:** 2026-08-17, during TS2068 Stage 3 verification. Attempted to call the real Extension ROM CHNG_VID service (via the documented IFRTN trampoline pattern) to engage hi-colour mode. Confirmed working correctly via direct register tracing: chunk-0 Home/Extension banking itself; CHG_V's early logic (VIDMOD check, memory-availability check); the call into REMGSZ via CALL_BANK (a nested Home-Bank round-trip, genuinely completes, 14-iteration bounded loop, correct RET); OPDFIL's relocation fix-up table walk (61 entries, correctly zero-terminated in the real ROM, confirmed progressing to completion via direct HL tracing); OPDFIL's 'clear the second display file' loop (genuinely progresses byte-by-byte from 0x6000, confirmed via direct HL tracing, NOT stuck as initially misread -- an early debugging mistake, caught by checking real register values instead of trusting the repeating-address pattern).
- **What actually goes wrong:** partway through the clear loop (observed around HL=0x6842 of the expected 0x7AFF/6912-byte target, not completing), execution ends up in a genuine repeating cycle through Home ROM addresses that look like character-output/print machinery (0x11F6 -> 0x1264 -> 0x0C0E -> 0x1202 -> 0x11D9 -> back to 0x11F6), with chunk-0 banking fully reverted to Home (chunk0=false, exrom=false) -- meaning execution left CHG_V's Extension ROM context entirely, not stuck inside it.
- **Hypothesis tested and ruled out:** that this was fallout from OS-resident code never being copied to RAM during normal boot (a real, separately-known gap -- T-12/Stage 1's note that chunk-0 banking wasn't observed to engage during ordinary boot). Checked directly: genuine, real-looking Z80 code exists at 0x6200 after boot, not zeros/garbage. Root cause remains unknown.
- **Why not pursued further:** real TS2068 software, per direct correction during this session, predominantly engaged hi-colour mode via a raw port-FFH write, not the documented CHNG_VID service call -- Stage 3 was rescoped around that (dynamic renderer switching on the guest's own port write, see docs/TS2068_TRACKING.md Stage 3), so this gap doesn't block any current functionality. Left open in case genuine TS2068 software is ever found that specifically requires the real service call (e.g. dual-screen mode, which does go through this same CHNG_VID path).
- **Fix:** would need either a CALL_BANK/REMGSZ semantics reference beyond what's been disassembled so far, or continued register-level tracing past the point this investigation stopped (immediately after the clear loop diverges, before reaching the Home ROM print-code cycle).

### T-21. Floating bus: instruction-start position granularity; 128K port-decode subtleties unmodeled; TS2068 FE-port idle levels legacy/unverified

Theme: model · Priority: P3 · Status: ☐

floatingbus.go uses cpu.Cycles (up to ~11 T-states early for the IN's IO cycle) -- fine for sync polling, not pixel-exact effects; the within-instruction offset zen80's contention hooks receive could be plumbed in if needed. The 128K shares the 48K model, ignoring 128K-specific unattached-port decode rules. Separately, io.go's TS2068 branch deliberately keeps legacy FE-port idle behaviour (bit 6 tape-driven, bit 7 high) -- FUSE uses a fixed 0x5F for Timex; unverified for TS2068 here.

### T-22. Batman-class load failures: root cause still open after five hardware-fidelity fixes; next instrument is deep RAM-PC sequence + memory-write-delta trace

Theme: model · Priority: P2 · Status: ☐

Eliminated by measurement this session: ULA contention magnitude and precision, frame-timebase incoherence, EAR idle feedback, floating bus, TZX pause levels. FUSE-vs-zenzx instruction diff from Speedlock entry (0x5D15): RAM-resident PC sequences and registers IDENTICAL through step 6000; all observed deltas (WORKSP/DE +-1 from typed-space, R at entry, interrupt landing phase) are history artifacts a real machine also varies -- ruled out by the any-kid-typing invariance argument. Fork lies beyond step 6000 of ~10841 frames. Next: full-depth RAM-only PC sequence diff plus memory-write-delta streams (addr/old/new, RAM-filtered, ~100MB budget per side; FUSE side builds on the existing ZENZXWRITE hook pattern).

## input

### T-13. Joystick emulation: headless zenscript driving not implemented

Theme: input · Priority: P3 · Status: ◐

- **Trigger:** 2026-08-17, deliberately out of scope for the initial -joystick kempston/sinclair implementation (0.4.13). Per direction, only Kempston and Sinclair Joystick 1 were requested.
- **Sinclair Joystick 2 done, 2026-08-17** (joystick.go, `JoystickSinclair2`): needed sooner than expected -- turned out to be required to correctly represent the +2/+2A/+3's real built-in hardware (two simultaneous Sinclair ports, not one), not merely a nice-to-have completion. `JoystickSinclairBoth` drives both from two independent host inputs at once, matching the real machine; `-model`'s own `auto` default now resolves to it for those models. See T-12/`docs/TS2068_TRACKING.md` and the 0.4.24 changelog entry for the full model-defaulting work this prompted.
- **Second Kempston port done, 2026-08-17** (`JoystickKempston2`/`JoystickKempstonBoth`, port `0x37`): a genuinely different case from Sinclair 2 -- real, but never a `-model` default, since no stock model ever had a second Kempston port built in. Confirmed against the ZX Spectrum Next's own I/O port register documentation (`specnext.com/tbblue-io-port-system`: "Kempston 2 (port 0x37)") and cross-checked against a separate, independent hobbyist interface's own port numbering ("KEMPSTON_MAX 2": "Kempston Port 55" = `0x37` hex). Represents modern neo-Spectrum platforms (ZX-Uno, ZX-Tres, the Omni, the Next) rather than any classic-era hardware -- explicit-choice only, same category as `-ns-graphics`/`-ns-storage`.
- **Cursor joystick done, 0.18.0** (`JoystickCursor`, `-joystick cursor`, joyports.go): 5 left, 6 down, 7 up, 8 right, 0 fire, as the Next's registers.txt lists it for NextREG 0x05 mode 010.
- **Headless zenscript joystick driving**: no .zen verb currently exists to simulate joystick input for automated testing (script.go's knownVerbs has no 'joystick' entry). joystick.go's JoystickState/SetJoystickState split was deliberately designed so this is cheap to add later -- a new verb parsing up/down/left/right/fire arguments and calling the same SetJoystickState a real gamepad poll would, no new translation logic needed. Still open.


### T-57. Controllers: what has not been tried with real ones, and what a Next has that is not modelled

Theme: input · Priority: P3 · Status: ☐

- **Trigger:** 0.18.0, going over the joystick and mouse code, which had not been looked at since 0.4.17.
- **Not tried with real controllers:** the only thing ever tried on a host is one gamepad as a Kempston joystick on a 48K. Everything else is checked by tests that read the ports and nothing more: both Sinclair ports, the Cursor joystick, the second Kempston port, a gamepad on a Next (whose ports are MD pads as NextZXOS sets them), a second gamepad, the Kempston mouse on any machine, the Next's mouse wheel, and the AMX mouse (0.4.17, checked against AMX Art's own driver code only).
- **Not modelled on a Next:** the user defined keys joystick (NextREG 0x05 mode 111, programmed through NextREG 0x28, 0x29 and 0x2B); the joystick ports' I/O mode (NextREG 0x0B); the X, Y, Z and MODE buttons of a six button MD pad (NextREG 0xB2); the mouse's sensitivity (NextREG 0x0A bits 1-0); port 0xDF as a second address of the left Kempston port.
- **Not agreed with CSpect:** how far the mouse counters move. CSpect at twice the size moved X by 13 for 40 pixels of the host's pointer; ZenZX divides the host's movement by the window's magnification and would move 20.
- **No way to choose in the GUI:** the joystick and the mouse are chosen by `-joystick` and `-mouse` only; the menu changes them only by changing the model.
- **Done when:** each line here is tried, modelled or has its own item.

### T-68. Escape does not close an open dropdown of the menu bar

Theme: input · Priority: P3 · Status: ☐

Trigger: comparing builds under Xvfb, 2026-11-17: with a dropdown of the menu bar open, Escape leaves it open. The same in the 0.27.0 build, whose changelog says Escape "closes the open menu or window". It closes the Help, About, first-start and SD Cards windows and the dock, and it does not end the program while a menu is open; only the dropdown stays. zenui's `MenuBar` and `Menu` take no Escape in their own Update when driven by the bar.

Scope: find whether the bar's dropdown is meant to close on Escape in zenui (its `Menu` reports Cancelled for a standalone menu) and make the bar's do so, in zenui; a test with a recorded Input.

Done when: Escape closes an open dropdown, with a test, and 0.27.0's sentence is true.

## tape

### T-23. Fast mode's ROM-trap only accelerates while PC is literally inside the standard LD-BYTES routine; any custom/protected loader (common in real commercial tapes) gets zero benefit the moment it takes over

Theme: tape · Priority: P2 · Status: ☐

Confirmed with the Chase H.Q. corpus (four working tape variants tested in all three modes): trapLoad's entry condition (tape.go TryIntercept, PC==ldBytesTrapPC) fired and completed near-instantly for two tapes (CHASHQ48.TAP, chase.tap) but never engaged at all for the other two (Side 1.tzx, (48K-128K).tap) -- those sat at the ordinary in-progress ROM/128K loading UI ("Program: CHASE HQ", "Tape Loader -- press BREAK twice") for the full duration, meaning fast mode provided no speed benefit whatsoever and would need the same 5-12 minutes of real tape-time as accurate mode to complete. Root cause structural, not a bug: once a tape's bulk data is delivered via a non-standard loader (flag bytes outside 0x00/0xFF, which all four working Chase H.Q. tapes use for their post-header blocks), PC never revisits ldBytesTrapPC, so TryIntercept has nothing to catch. This is likely the majority case for real 1980s-90s commercial releases (T-22's whole Speedlock investigation is one large example of the same pattern). Fast mode's actual real-world coverage is narrower than its historical verification (T-02, 'byte-identical for .tap/.tzx CODE blocks') suggested -- that verification used a tape whose structure happened to route fully through the standard ROM loader, not a representative sample.


### T-24. No mode-independent, trustworthy signal exists for 'did this tape actually load correctly' -- every mode's own self-reported completion (fast's trap return, turbo's Position-reached-end, accurate's Playing==false) has been observed true while the load was wrong

Theme: tape · Priority: P1 · Status: ☐

Pattern recurring across this project's own history, not a one-off: turbo mode's Position reached the tape's exact end while the screen was completely blank and no game code had ever been read (T-19's original bug, RESOLVED.md) -- the exact signal a race/fallback mechanism would use to declare success. This session's own Chase H.Q. investigation found the corpus-report 'verified' heuristic (screen hash differs from boot + has visible content) gave a false positive for eleven minutes of static title-screen before the true stuck state was found (three of seven Chase H.Q. tape variants). And CHASHQ48.TAP's fast-mode result reached a screen that differed from boot and had content, yet showed visibly corrupted icon graphics plus a crack-signature line -- 'verified' by the existing heuristic, not actually correct. Blocks any confidence-based automation (mode racing, fast-with-fallback, unattended corpus scoring) until a real verifier exists: one that checks actual memory/screen content against something independent of the mode's own bookkeeping, not merely 'differs from the pre-load state.' See docs/TAPE_LOADING_HANDOVER.md for the fuller writeup and a candidate design sketch.

## next-support

### T-29. Layer 2 graphics mode (256x192 8-bit pixel-mapped)

Theme: next-support · Priority: P3 · Status: ◐ · Wave: 6

Tier 0 feature. Base mode done 2026-09-15: 256x192 8-bit-colour linear framebuffer (layer2.go), base bank via NextREG 0x12 (visible) / 0x13 (shadow, stored but not yet consumed -- no mechanism selects it), clip window via the already-implemented NR 0x18 (T-27), composited against the ULA layer in DecodeDisplay per NR 0x15's real bit-4:2 priority table -- both orderings NEXT_SUPPORT_DEVELOPMENT_PLAN.md scopes (ULA-on-top, Layer2-on-top) are implemented, decoding all 8 possible NR 0x15 values rather than just those two, since sprites don't exist yet to make the other four orderings distinguishable. Verified via layer2_test.go (register/pixel/clip/priority-table unit tests plus a full Z80N NEXTREG/OUT end-to-end regression) and a no-op regression proving DecodeDisplay is byte-identical to its pre-T-29 form on every classic machine. Correction to this item's own original wording: "0x12/0x13/0x14 clip window registers" was imprecise -- confirmed against wiki.specnext.dev that 0x12/0x13 are the visible/shadow base-bank registers (not clip window; that's 0x18, already correct in nextreg.go since T-27) and 0x70 (not 0x14) is the resolution/palette-offset control register. Remaining, not yet done: 320x256/640x256 4-bit modes (explicitly deferred until the base mode rendered correctly, which it now has), the real 256-entry 9-bit Next palette (T-31 -- layer2Pixel currently maps down to the existing 16-colour zxPalette as a documented interim measure), and IO port 0x123B (CPU-address-space paging into Layer 2 memory, and the shadow/visible select bit it carries) -- none of these were in this wave's "done when" bar per NEXT_SUPPORT_DEVELOPMENT_PLAN.md, so this item stays open at ◐ rather than closing, pending a decision on whether they're filed as new items or folded into a later wave. GPU fast path added 2026-09-15: renderLayer2GPU (videorender_gpu.go) draws Layer 2 directly via raylib texture blits within the clip window when the GUI's live rendering is active, reusing the existing paperColourGPU cache rather than a new one (NR 0x70's palette offset can change at runtime, so no per-raw-byte bake would stay correct) -- matching this project's own established GPU technique of pre-baking a small texture set once and issuing draw calls per frame rather than re-uploading a CPU-composited image every frame. The portable DecodeLayer2/CompositeLayer2 path is unchanged and still used by headless screenshot capture and by the GUI as a fallback when the texture cache isn't ready. Seam-extraction refactor added 2026-09-15 per docs/proposals/next-gpu-compositing-seam.md's Phase 1, ahead of starting T-30: layer2Pixel's inline colour-resolution arithmetic moved into a standalone resolveNextPixel(raw, paletteOffset) function that T-30's sprite pixel path is meant to call too, instead of inventing its own copy; and a new nextPaletteVersion counter (SpectrumIO, io.go) now increments on every NR 0x70 write, on both the live-write path (nextRegWriteDirect) and the snapshot-restore path (SetNextRegister), verified by a dedicated test sweeping several other registers to confirm nothing else bumps it. No GPU cache reacts to this counter yet -- paperColourGPU is still genuinely runtime-invariant under the interim 16-colour mapping -- the counter exists and is proven correct ahead of T-30/T-31 needing to consume it, per the proposal's own stated intent. Bug found and fixed 2026-09-15 while building T-30's genuine three-layer priority decode: layer2ULAOnTop's switch statement misclassified NR 0x15's %010 ordering (S U L) as Layer2-on-top, even though its own doc comment's stated rule ("U before L means ULA-on-top") already implied ULA-on-top for that case -- U does appear before L in "S U L". Practical impact: NR 0x15's construction/power-on default is 0x08 (bits 4-2 = %010), so every Next-mode session that never explicitly wrote NR 0x15 was compositing Layer2-over-ULA when it should have been ULA-over-Layer2. Fixed the switch statement and documented the correction inline; also fixed a second, related bug this exposed in TestT29NextModeLayer2EndToEnd, whose own comment incorrectly claimed the NR 0x15 construction default was 0x00 (confirmed via a temporary scratch test, removed immediately after, that the real default is 0x08) -- both bugs had been masking each other, since the test happened to still pass under the wrong assumption paired with the wrong classification. The test now explicitly selects S L U via a NEXTREG write rather than relying on any default.

**Completion pass (2026-09-15).** Correction to this entry's own earlier text: the "real 256-entry 9-bit Next palette" gap listed above as remaining was ALREADY closed by T-31 (nextColourFor/resolveNextPixelIndex resolve through the real palette, not the interim 16-colour zxPalette map) -- that earlier wording had gone stale after T-31 landed and was never updated here. Two of the three genuinely remaining gaps are now closed: (1) the two wider/narrower Layer 2 modes -- NR 0x70 bits 5-4 %01 (320x256x8bpp) and %10 (640x256x4bpp) -- are implemented in layer2.go (layer2CurrentMode/layer2FramebufferDimensions/layer2Pixel's per-mode switch), with the wider modes' real column-major (X-then-Y) addressing confirmed against wiki.specnext.dev/Layer_2 and cross-checked numerically against both of that page's own illustrative byte-offset examples (see layer2.go's own header comment and layer2_test.go's TestLayer2Pixel320x256UsesColumnMajorAddressing doc comment for the full working -- an earlier draft of both the code comment and the test itself misread which coordinate produced which byte offset, caught only once the test was actually run and failed); the 640x256x4bpp mode's palette-offset interaction with a native 4-bit pixel is genuinely undocumented by any source consulted and is implemented as the closest sane interpretation, flagged inline as inferred rather than confirmed, matching T-31's own established standard for its own inferred bits. (2) IO port 0x123B (Layer 2 Access Port) is now implemented (layer2accessport.go, new file) -- confirmed against wiki.specnext.dev/Layer_2_Access_Port to be purely a CPU-memory-paging feature (maps Layer 2 memory into the Z80's lower 16K address window for direct read/write) with NO effect on which bank is displayed (display is always driven by NR 0x12 regardless of this port's own bit 3 target-register selection) -- this corrects this entry's own and nextreg.go's earlier looser framing of NR 0x13 as a "shadow screen" switch. Implementing this required one small architectural addition: SpectrumMemory (memory.go) gained two optional closure-hook fields (layer2AccessReadFn/WriteFn) wired by NewSpectrumIO, since readNext/writeNext are plain SpectrumMemory methods with no SpectrumIO reference threaded through their many call sites, and Layer 2 Access Port state genuinely belongs on SpectrumIO (it reads NR 0x12/0x13). Verified via 9 new tests in layer2_test.go (mode decoding, per-mode dimensions/page counts, column-major addressing for both wider modes with the 4bpp nibble-packing, the access port's standard-mode read/write mapping, its fall-through when disabled, its shadow-bank targeting, and its own read-back). Extended mode (port 0x123B bit 4, core 3.0.7+ bank-offset addressing) remains unimplemented -- no source consulted documents a real consumer of it and it was not required by this pass's own scope; stored-and-ignored, matching this project's established convention. Genuinely still not done: real X/Y scroll register support (NR 0x16/0x17/0x71 are not implemented anywhere in this emulator), so a wider-mode screen coordinate samples the framebuffer's own top-left corner directly with no scroll offset -- undocumented-beyond-that-point behaviour is not invented.

**GUI presentation gap found post-completion-pass (2026-09-15, flagged by the tech team, not caught before reporting this pass done) -- since CLOSED the same day by a second, corrective pass, also prompted by the tech team.** The wider modes' correctness above was data-plane only (layer2Pixel addresses the right byte for a given (x,y)); the GUI's presentation plane had never been updated to match. Fixed: DisplayManager's window sizing is now a fixed reference canvas (RefWidth x RefHeight, display_constants.go: 320x256, replacing the pre-existing 320x248 TotalWidth/TotalHeight for window-sizing purposes) that never shapeshifts on its own -- only an explicit user zoom changes the window. The classic 256x192 picture's own border grew by 4px top and 4px bottom (classicScreenExtraTopMargin/BottomMargin, display.go) to absorb RefHeight's 8px growth over the old 248 without an asymmetric split. Layer 2/sprites now draw at their own centred offset (nextLayersViewportOffset, display.go) rather than reusing the classic screen's scissor-bound offset, with one deliberate exception: Layer 2's own base 256x192 mode is forced to exactly classicScreenOffset()'s value rather than being independently centred, because Layer 2's base mode composites pixel-for-pixel with the ULA layer and an independent centring formula was found, before shipping, to disagree with classicScreenOffset's own y by 4px -- would have shown as a visible seam between the two layers in the most common Layer 2 mode.

Getting 640x256x4bpp mode right required a real correction mid-pass: an earlier draft of this fix assumed every mode's pixels draw at uniform full screen-pixel width and tried to solve 640-mode's apparent canvas overflow by widening the window specifically for that mode -- wrong, caught by fetching wiki.specnext.dev/Layer_2_Control_Register and finding an exact quote once the arithmetic refused to reconcile with the tech team's own stated design ("the modes of 640-pixel width starting at zoom level x2... should occupy the same space as a 320-pixel width mode, but with twice the horizontal resolution"): "The 640x256x4bpp mode is more like 320x256x8bpp mode, but every byte is displayed as two half-width paired pixels." 640-mode's 640 columns occupy the SAME physical picture width as 320-mode's 320 columns -- double horizontal density, not a wider picture -- so the fixed-canvas design was right all along and only the per-pixel draw width needed fixing (layer2PixelWidthScale/layer2DisplayedSize, layer2.go): every mode draws each pixel at a screen width of one multiplier unit except 640x256x4bpp, whose pixels draw at half that. minMultiplierForMode's pre-existing 2x zoom floor for 640-mode is kept, but its documented justification is corrected from the wrong "canvas overflow" framing to the real, narrower reason: a half-width pixel is a whole number of screen pixels only at even multipliers, so refusing 1x avoids the worst (0.5px) case; the gate does not extend to skipping odd multipliers above 1 (3x still renders 640-mode with 1.5px sub-pixel blur), a known, minor, documented cosmetic limitation rather than a claimed full fix.

Fixing the geometry also surfaced a genuine regression risk in the initial (wrong) fix: renderLayer2GPU's own per-pixel draw technique deliberately splats each pixel wider than its own column spacing (an existing, pre-T-29 technique using bakeLayer2ColourGPU's 8x1 texture, relying on left-to-right painter's-algorithm overwrite so only the rightmost column's splat overreaches) and had always depended on Render's classic-screen scissor rect to contain that overreach as a side effect. The first draft of this fix removed that scissor (specifically so wide-mode content wouldn't be wrongly cropped by the classic screen's narrower bounds) without reinstating containment at the right granularity, which would have let every row's rightmost pixel bleed several pixel-widths of its own colour into the border. Fixed by scissoring renderLayer2GPU to its own mode's real displayed footprint (layer2DisplayedSize) rather than either the old classic-screen rect or no scissor at all -- caught and fixed before shipping, not discovered from a visual check this sandbox cannot perform (no live GL context), but from re-deriving the pre-existing draw technique's own dependency on being contained.

Verified via ~10 new tests (layer2_viewport_test.go): fixed reference canvas size and its independence for non-standard renderers; the 640-mode zoom floor across ScaleDown/SetScale and the raw gate function, including the "no gate on a non-Next machine" case (layer2Enabled() is io.memory.isNext, not an NR 0x12 bit -- confirmed by reading that method directly after an earlier draft of this same test guessed a bit that doesn't exist); the classic screen's extra top margin, present only for the standard renderer; the centring formula across three multipliers (1x/2x/3x) for all three Layer 2 modes, confirming 640-mode centres at (0,0) at every multiplier tested, not just even ones; and layer2DisplayedSize's own width/height split in isolation. Not fixed by this pass, and explicitly out of scope: the headless/CPU decode path (DecodeLayer2/CompositeLayer2, layer2.go) is still hardcoded to the base 256x192 canvas (layer2Width/layer2Height) regardless of the active mode -- a separate, pre-existing gap only surfaced while checking this pass's own scope, affecting headless screenshot capture, not the live GUI this pass targeted; and NR 0x18's own 8-bit-per-coordinate clip window still cannot express a clip rectangle wider than 256 columns (unchanged from the first finding above -- genuinely unknown whether real hardware widens its semantics in wide modes). This item now stays open at ◐ for three things: the scroll-register gap (inside T-29's original scope), the headless/CPU decode path's fixed-256x192 assumption, and NR 0x18's clip-window width limit -- the GUI window/viewport/geometry gap itself is now closed.

### T-30. Hardware sprites (128x 16x16, unscaled minimum)

Theme: next-support · Priority: P3 · Status: ◐ · Wave: 7

Tier 0 feature. Base implementation done 2026-09-15 (sprite.go, new): 128 sprites, 16x16 pixels, 8-bit colour, unscaled, attribute table (position, palette offset, visibility, pattern index) and 64-slot pattern memory reachable via NextREG 0x34 (sprite select) + 0x35-0x38 (attribute writes, both the live-write and snapshot-restore NextREG paths -- nextreg.go), and pixel colour resolved through the same resolveNextPixel(raw, paletteOffset) seam Layer 2 uses (T-29's own extraction, per docs/proposals/next-gpu-compositing-seam.md's Phase 1) rather than a second independent colour mapping. Compositing replaced T-29's two-layer ULA/Layer2 boolean with a genuine three-layer decode of NR 0x15's bit-4:2 priority field: nextLayerOrder (layer2.go) returns all three layers' real bottom-to-top order for all 6 real orderings (the remaining 2 are core-3.1.1 blending modes, not implemented, falling back to the all-layers-on-top ordering), and layersAboveULA/layerIsAboveULA answer "is this layer above ULA" for any of the three -- DecodeDisplay/CompositeNextLayers (zenzx.go/layer2.go) and the GUI's renderNextLayersGPU (videorender_gpu.go, dispatching to renderLayer2GPU/renderSpritesGPU in the correct relative order) both consume it. GUI visibility uses a capped, LRU-evicted GPU texture cache (spriteGPUCache, videorender_gpu.go) keyed by (pattern, paletteOffset) rather than pattern alone -- an earlier pattern-only-keyed draft was caught and discarded before shipping, since attribute 2's palette offset is per-sprite, not per-pattern, and two sprites sharing a pattern with different offsets need distinct baked textures. Default capacity 64, configurable via DisplayManager.SetSpriteGPUCacheCapacity; a stale or missing entry is rebaked from spritePixel and re-cached, so a running program's actual working set of distinct sprite appearances becomes GPU-resident after its first few frames rather than needing every possible combination pre-baked. Verified via sprite_test.go (17 tests: attribute decode, pattern-memory writes and generation bump, transparency, overlap priority, the three-layer order table cross-checked against layer2ULAOnTop for all 6 real NR 0x15 orderings, and full Z80N NEXTREG end-to-end regressions with sprites alone and sprites-above-Layer2 together) and sprite_gui_test.go (renderSpritesGPU/renderNextLayersGPU early-return gates, plus spriteGPUCache's GL-context-free logic: construction, miss, a hit under capacity, distinct-key correctness, and capacity changes that don't trigger eviction). Not unit-tested: spriteGPUCache's eviction, key-replacement, and clear paths -- each calls rl.UnloadTexture, confirmed this session (a throwaway scratch test, removed after) to segfault without a live GL context, which this container's test suite does not set up; covered by reading, not by a running test, same category of gap as the rest of this project's GPU draw calls never having their actual pixel output checked. Bug found and fixed while building the three-layer decode: layer2ULAOnTop (T-29) misclassified NR 0x15's %010 ordering (S U L, where U legitimately precedes L) as Layer2-on-top -- see T-29's own entry below for the fix and its practical impact. Explicitly deferred, per NEXT_SUPPORT_DEVELOPMENT_PLAN.md's own "Deferred within Tier 0" list: scaling (x1/x2/x4/x8), rotation, mirroring, 4-bit colour patterns, composite (anchor+relative) sprites, the real hardware's port-0x303B/0x57/0x5B access path (this emulator's NextREG-mirror path satisfies the plan's "reachable via NextREG" bar without it), NR 0x19 (sprite clip window), NR 0x43 (palette selection), NR 0x4B (transparency index -- a fixed stand-in is used instead, documented in sprite.go), attribute 4 (extended attributes), and sprite Y-position MSB (0-255 range only). None of these were in this wave's "done when" bar, so this item stays open at ◐ rather than closing.

**Completion pass (2026-09-15).** Correction to this entry's own earlier text: NR 0x43 (palette selection) was already closed by T-31, not by this pass -- listed above as deferred, that wording had gone stale once T-31 landed and was never updated here (same class of staleness T-29's own completion paragraph corrected for its palette claim). This pass closes the remaining sprite gaps against wiki.specnext.dev/Sprites, confirmed rather than assumed: sprite.go's spriteAttributes struct was rewritten to store the five raw attribute bytes (attr0-attr4, corresponding to NR 0x35-0x39) rather than a pre-decoded subset, since the real hardware overloads several bits' meaning between anchor sprites and relative sprites and a pre-decode had no way to represent that; a new spriteResolved struct holds the per-frame decoded render state and is produced by effectiveAttrs(), which walks all 128 sprites in index order tracking the most recent anchor sprite (real hardware's own sequential dependency -- a relative sprite inherits from whichever anchor precedes it in the table) and its relative-type (composite/unified). Attribute 4 (NR 0x39) is now fully implemented per its dual format, confirmed via wiki fetch: for anchor sprites, bit 7 (extra attribute enable, already required for attribute 4 to apply at all -- attr3 bit 6), bit 1 (Y-MSB, extending Y to the full 0-511 range) and bits 0/2-3 (per-axis scale, x1/x2/x4/x8 via decodeSpriteScale); for relative sprites, bit 1 selects composite-vs-unified relative type, bit 6 (PR) and bit 7 (PO) flag whether the relative sprite's palette offset and pattern index are added to the anchor's rather than used standalone. NR 0x4B is now a real, configurable register (nextreg.go's nextRegDefaults gained an entry seeded from the same default sprite.go used as its fixed stand-in) rather than the fixed value T-30's base pass used -- spriteTransparentIndex() now reads NR 0x4B live. Fixed while wiring NR 0x4B: a genuine pre-existing bug in spritePixelIndexAt, where the transparency check compared the raw pattern byte against the transparent index before applying the palette offset, when the real hardware (and this emulator's own resolveNextPixelIndex convention elsewhere) compares the RAW byte directly -- confirmed this was already correct by re-reading the wiki's own worked description, but the offset-vs-raw ordering had drifted during the rewrite and was caught by a dedicated regression test before it shipped, not by inspection. Scaling and rotation/mirroring are implemented as GPU draw-call geometry (renderSpritesGPU, videorender_gpu.go), via rl.DrawTexturePro's source/dest rectangles, origin and rotation parameters, rather than as baked texture variants -- a deliberate deviation from what might otherwise have been the obvious approach, made once DrawTexturePro's real capabilities were confirmed against vendored raylib-go source: negative source-rectangle width/height flips that axis (giving mirroring for free), and dest-rectangle scaling plus a rotation angle around a centred origin gives scale and 90-degree rotation without touching the sprite's cached texture at all, so spriteGPUCache's key remains (pattern, paletteOffset) rather than needing a third transform dimension as originally anticipated when this pass was scoped. The corresponding CPU-side inverse (spriteLocalCoord, used by the portable/headless pixel path) required numerically verifying the inverse of the forward scale-then-rotate-then-mirror pipeline; a first draft's rotation-inverse term used the wrong pre-rotation dimension (scaledW instead of scaledH) and was caught only by a brute-force Python verification script, not by derivation -- fixed, and a full 16x16 rotation round-trip regression test was added specifically to guard against this recurring (see sprite.go's own inline comment, which insists future readers re-verify numerically rather than by eye). Composite and unified relative sprites are both implemented: composite relative sprites resolve their own visibility/mirror/rotate/scale independently (ANDed with the anchor's visibility per the wiki's documented behaviour) while inheriting only position-offset and optionally palette/pattern (PR/PO bits); unified relative sprites additionally inherit the anchor's mirror/rotate/scale and have their own position offset transformed through the anchor's own rotate/mirror/scale pipeline before being added to the anchor's position, per the wiki's "big sprite" description -- rotatePoint90CW's forward formula was itself numerically verified (a labelled-grid brute-force check) before being reused here. sprite Y-position MSB is implemented via attribute 4 bit 1 as described above. Verified via sprite_test.go, extended with roughly 16 new tests: raw-attribute decode for both anchor and relative formats, attribute-4 scale/Y-MSB decode, the transparency-order regression, scale/rotation footprint calculations, spriteLocalCoord round-trips (including the full 16x16 rotation case, not just corners), rotatePoint90CW's physical-intuition check, and composite/unified relative sprite resolution including palette/pattern inheritance and visibility-ANDing. Explicitly still deferred, matching this project's established store-and-ignore convention for genuinely out-of-scope hardware: 4-bit colour sprite patterns (no consumer of this format exists anywhere in the current sprite pipeline, and NEXT_SUPPORT_DEVELOPMENT_PLAN.md does not require it for Tier 0) and the real hardware's alternate port-0x303B/0x57/0x5B access path (the existing NextREG-mirror path already satisfies the plan's "reachable via NextREG" bar). NR 0x19 (sprite clip window) also remains unimplemented -- not addressed by this pass, since it was outside the two items ("Layer 2 and sprites... one of the waves or both") this pass was explicitly scoped to complete, and clipping is a compositing-order concern rather than a per-sprite attribute gap. This item now stays open at ◐ only for those three items (4-bit patterns, the alternate hardware access path, and NR 0x19), none of which were in this wave's original "done when" bar and none of which block calling the sprite attribute/rendering model itself complete.

### T-31. 9-bit/512-colour Next palette with NextREG palette control

Theme: next-support · Priority: P3 · Status: ◐ · Wave: 8

Tier 0 feature. Implemented 2026-09-15 (nextpalette.go, new; pkg/zxpalette/nextpalette.go, new): all 8 real hardware palettes (ULA/Layer2/Sprites/Tilemap x 1st/2nd, 256 entries each -- ULA is stored at the same 256-entry width as the others for storage-uniformity simplicity, even though real hardware's ULA palette is only 32 entries wide) plus the full NextREG protocol -- NR 0x40 (index select, also resets NR 0x44's two-write sequence), NR 0x41 (8-bit RRRGGGBB write with the missing third blue bit synthesised as bit1 OR bit0, auto-incrementing unless NR 0x43 bit 7 disables it; reads return the CURRENT entry re-encoded rather than the last byte written, matching the wiki's documented read behaviour, and do not themselves advance the index), NR 0x43 (bits 6-4 select which palette 0x40/0x41/0x44 read/write, bits 3-1 independently select which of 1st/2nd is the currently DISPLAYED palette per layer, bit 0 stores ULANext-enable unconsumed), and NR 0x44 (two-write 9-bit path: first byte RRRGGGBB, second byte's bit 0 the true third blue bit and bit 7 a Layer-2-only priority flag accepted but not stored). Colour-math (RGB9 decode/encode/scaling) lives in the portable pkg/zxpalette package per its own header's anticipation of this exact split; the live, per-machine, NextREG-driven palette state lives in zenzx's own nextpalette.go, matching how spriteSystem/nextRegClipWin are already owned on SpectrumIO. layer2.go's resolveNextPixel was split into resolveNextPixelIndex (pure arithmetic, unchanged from T-29/T-30, confirmed by re-deriving both Layer 2's 8bpp formula and sprites' PPPP0000+IIIIIIII formula from the wiki and finding they reduce to the identical raw+paletteOffset<<4) and nextColourFor (the actual palette lookup, now resolving through the real per-layer displayed palette instead of collapsing to the old 16-colour zxPalette approximation). sprite.go's spriteTransparentIndex moved from the collapsed 0xE to NR 0x4B's real documented default 0xE3, now directly expressible with the full 8-bit index. GPU fast path: renderLayer2GPU gained its own layer2ColourGPU (256-entry texture cache, display.go/videorender_gpu.go) rather than continuing to reuse paperColourGPU -- safe pre-T-31 only because the underlying palette was immutable, no longer true once NR 0x40/0x41/0x43/0x44 make it genuinely writable at runtime; the cache rebakes in full (not per-entry) whenever nextPaletteVersion diverges from the version it was last baked against, mirroring the existing spriteGPUCache's staleness-comparison technique at a coarser grain appropriate to a small, fully-enumerable 256-entry space. nextPaletteVersion (added ahead of need in an earlier T-29-adjacent session specifically for this) now has its trigger set widened from NR 0x70 alone to also include NR 0x41/0x43/completing NR 0x44 writes, on both the live-write and snapshot-restore paths. Verified via nextpalette_test.go (12 tests: index/control register decode, 8-bit write+auto-increment+disable, 8-bit read returning the current entry rather than the last write, the full two-write 9-bit sequence including the true-vs-synthesised third blue bit, NR 0x40/0x43 both resetting a pending 9-bit sequence, the display-select bits' independence from the read/write-target bits, per-target storage non-aliasing, and a full Z80N NEXTREG end-to-end regression writing distinct known colours through both Layer 2 and Sprites) plus layer2_gui_test.go's new CleanupTextures gate test for the new cache. layer2_test.go and sprite_test.go's own existing register/pixel and end-to-end tests were updated in step to assert against real resolved colours (via known palette writes) rather than the retired zxPalette/collapsed-index assumptions they previously depended on. Honest gaps, documented rather than silently assumed: the palette's power-on/construction default is all-black (RGB9{0,0,0}) for every entry, not an approximation of the classic 16 ZX Spectrum colours real hardware almost certainly boots with -- no source consulted for this wave documents the actual default table, so this stays an open gap rather than an invented colour-accuracy claim; NR 0x43 also resetting NR 0x44's two-write sequence is an inferred, not confirmed, defensive assumption (flagged inline in nextpalette.go); and Colour()'s 3-bit-to-8-bit channel scaling is plain linear (v*255/7), not a claim of measured hardware DAC accuracy. Remaining, not yet done: the ULA and Tilemap palettes are stored and correctly readable back but not wired into any renderer (matching this project's established store-and-ignore convention for genuinely out-of-scope hardware -- ULANext and the Tilemap layer are both separate, larger Tier 1-2 features), and NR 0x4B (sprite transparency index) remains the fixed stand-in T-30 introduced rather than becoming configurable. Also open: whether the GPU-baked layer2ColourGPU texture's actual uploaded pixel data agrees with the same 9-bit table it's baked from has no test evidence either way -- see docs/KNOWN_ISSUES.md's new G-03 dormant guard, never yet exercised (needs a live GL context this sandbox cannot provide). None of these were in this wave's "done when" bar, so this item stays open at ◐ rather than closing.

### T-32. NEX file loading (direct memory/Layer2/palette injection, no OS)

Theme: next-support · Priority: P3 · Status: ☐ · Wave: 9

Tier 0 feature. No .nex handling exists anywhere in the current snapshot/tape loading code. This is the Next-equivalent shortcut zenzx already takes for .sna/.z80 on classic models -- lets real Next programs run without a DivMMC/SD/NextZXOS boot chain. Minimum target: NEX v1.0-1.2 header parsing, direct page loading, Layer 2 screen/palette-from-header.

### T-36. Real NextZXOS/DivMMC/SD-card boot chain (Tier 2)

Theme: next-support · Priority: P3 · Status: ◐ · Wave: 10

Tier 2 feature (NEXT_SUPPORT_DEVELOPMENT_PLAN.md), substantially larger than any single Tier 0 wave -- comparable to several combined. Nothing here exists in zenzx yet: DivMMC hardware (automap logic, 8KB SRAM, esxDOS filesystem layer) and SD-card image reading were both outstanding. Firmware licensing/sourcing is tracked outside this codebase, not here. Recommended path (NEXT_BOOT_REFERENCE.md's own boot-sequence stages, verified in isolation rather than chained): (1) FAT32 image reading, (2) minimal DivMMC/SD stub that reads one fixed sector and jumps to 0x6000, (3) FAT32 support sufficient for TBBLUE.FW to find/read its files, verified by reaching NR 0x03=0xB3 + the soft reset, (4) NextZXOS's own init sequence (112-bank RAM test, peripheral init) reaching the welcome screen, screenshot-verified.

Progress this session: `pkg/nextdisk` added, wrapping `github.com/ha1tch/blipper/fatfs` (real dependency, `go.mod` updated) -- opens a FAT16/FAT32 image read-only and reads named files out of it (item 1). `sdcard.go` added: an SD/MMC-over-SPI command stub (ports 0xE7 CS, 0xEB data) covering CMD0/8/12/17/41/55/58, enough for the boot sequence's own documented init (NEXT_BOOT_REFERENCE.md stage 1) plus single-block reads, backed by a raw `io.ReaderAt` image -- explicitly not CRC-checked, not multi-block, and faster-than-real on ACMD41/NCR timing (see its own doc comment for the full list of simplifications). `divmmc.go` added: the automap/CONMEM memory overlay behind port 0xE3, confirmed against two independent real-hardware sources cross-checked against each other (ZX-Uno's own Next-compatible DivMMC core, and a ZX-Uno developer's independent account of implementing the same automap addresses) -- wired into `memory.go` (`divMMCReadFn`/`WriteFn`, checked before even the Next MMU gate) and `io.go` (ports 0xE3/0xE7/0xEB, `EnableDivMMC`), with `zenzx.go`'s own `EnableDivMMC` additionally wiring `zx.cpu.M1Hook` to drive automap paging. Explicitly out of scope, not conflated with this: the FPGA's own separate config-mode boot ROM (NR 0x03 gated, item 6) and a real-hardware "late automap enable" edge case reported against the actual Next FPGA core (divmmc.go's own header comment). **None of this has been built or tested this session**: this sandbox has no Go toolchain installed and no network egress, so `go mod tidy`/`go build`/`go test` could not be run against the new dependency or the new files at all -- every signature was hand-verified against `pkg.go.dev`'s rendered documentation for `blipper/fatfs` rather than compiled, which is not a substitute for actually building it. See this session's summary for the exact commands still needed. Item 2 (the DivMMC hardware bridge) has a first pass now; items 3-4 (TBBLUE.FW/FAT32 milestone, NextZXOS welcome-screen milestone) are next. Stays open at ◐ -- this item tracks the whole Tier 2 chain across many future sessions, not a single wave's worth of work.

Update, same session, against real firmware: the user ran this against the official CSpect-support ROMs (`enNxtmmc.rom`, `cspect-next-1gb.img`, from `sn-emulator-24.11.zip`) with `-divmmc-debug` and hit a genuine bug this surfaced: the ROM writes alternating `0x80`/`0xC1` to port 0xE3 in a tight loop and never proceeds. Root cause, confirmed against several independent DivIDE/DivMMC programming-model references (velesoft.speccy.cz, k1.spdns.de): **MAPRAM (bit 6) is a one-way hardware latch** -- once set, only a power-on clears it again, not a later write and not a soft reset. `WriteControl` was doing a plain `d.ctrl = value`, so it let the ROM clear MAPRAM, which real hardware never allows -- the ROM's loop is exactly a self-check for this that our emulation was failing. Fixed by forcing bit 6 to stay set once it's ever been written 1, for one `DivMMC` instance's lifetime (no power-on-vs-soft-reset distinction exists yet to do this more precisely -- noted on `WriteControl` itself). This is the first real-firmware-driven correction to this file since it was written from cross-referenced documentation alone; expect more of these as further stages get exercised against real ROMs.

Second correction, same session: past the MAPRAM fix, automap now exits cleanly and the CPU lands in +3DOS (ROM bank 2) -- confirmed via the emulator's own F3 status readout -- meaning ESXDOS's boot code, unable to reach anything useful over SD (zero SPI traffic ever seen, even with full per-byte logging), falls through to the host's real floppy-boot path as its own designed fallback. Two independent, pre-existing gaps surfaced by this, neither specific to DivMMC: `-model=next` never called `EnableFDC()` at all (only `-model=plus3` did), and even once enabled, no disk was ever loaded, so `FDC765`'s own Sense Drive Status command correctly reports "not ready" (`fdc.go`, `diskPresent=false` clears ST3's Ready bit) -- +3DOS hangs waiting for that to change. Fixed in both entry points: `-model=next` now calls `EnableFDC()` and either loads `-disk` (newly wired into this case, previously `-model=plus3`-only) or creates a blank formatted disk by default. Screenshot evidence (an actual `©1982, 1986, 1987 Amstrad Plc.` / "Drive" render, not a blank screen) confirms real progress before this fix; outcome after it not yet confirmed.

Third correction, same session, via direct jnext source comparison (user supplied jorgegv/jnext 0.99.144 source directly): two real, confirmed findings, not guesses.

1. **`enNxtmmc.rom` was never sufficient on its own.** jnext's `doc/analysis/SDCARD-IMAGE-BOOTING.md` and its project notes both confirm the real first boot stage is a *separate* 8K "FPGA config-mode boot ROM" (`nextboot.rom` on real hardware; bundled at `roms/nextboot.rom` in the jnext package, "silicon-baked... mirrors the on-FPGA flash IPL") that does SD init (CMD0/CMD8/ACMD41/CMD58) + MBR + FAT32 + `TBBLUE.FW` + `config.ini`/`menu.def` loading, only disabling itself via a NextREG 0x03 write once done. `enNxtmmc.rom` ("DivMMC disk OS ROM") is a later-stage, much smaller component ESXDOS uses for its own ongoing dot-command disk access -- it was never going to perform the initial SD/FAT32 boot sequence by itself, which is exactly why zero SPI traffic was ever observed no matter how much logging was added. This is now implemented for real: `memory.go` gained a boot ROM overlay (`bootROM`/`bootROMEnabled`/`SetBootROM`/`DisableBootROM`), confirmed against jnext's `src/memory/mmu.h`/`.cpp` and `src/core/emulator.cpp` (which cite exact VHDL line numbers) rather than inferred: hardwired to 8KB (zero-pad/truncate), mirrored across the *full* 16K low range via `addr&0x1FFF` (not just the low 8K DivMMC's own overlay covers), highest priority of the whole overlay cascade (checked before DivMMC in `Read`/`Write`), enabled immediately once attached, and disabled by *any* write to NextREG 0x03 regardless of value (wired into both `nextRegWriteDirect` and `SetNextRegister` in `nextreg.go`). New `-bootrom` flag (both entry points), independent of `-divmmc-rom`. jnext's fuller NR 0x03 semantics (config_mode transitions, machine_timing, user_dt_lock) and its reset-time re-enable rule are explicitly NOT replicated -- scoped to the one trigger this overlay needs.
2. **The official `cspect-next-1gb.img` has a real, independently-documented defect**, unrelated to zenzx: jnext's own `tools/fix-sdcard-image.sh` states outright that its 32K clusters yield ~32,758 data clusters, below the FAT32 spec minimum of 65,525 -- a spec-correct FatFs (what a real boot ROM uses) rejects it; CSpect's own lenient SD driver just tolerates the out-of-spec image. `MACHINES/NEXT/CONFIG.INI` is also confirmed missing from the shipped image. jnext's script (requires `mtools`) fixes both by re-clustering and injecting a default config.ini; not reimplemented in zenzx, since jnext's own tool already does this correctly and is right there in the same package.

Neither of these has been tested end-to-end this session (same standing sandbox limitation: no Go toolchain, no network). `run_next_divmmc.sh` updated to use `-bootrom`+a `-fixed` image name and documents both required pre-steps.

Fourth correction, same session: with the boot ROM overlay wired in, real `[sdcard]` traffic appeared for the first time -- confirming `nextboot.rom` genuinely runs the documented `CMD12 -> CMD0 -> CMD8` sequence. But it looped on CMD0 forever. Root cause: `sdcard.go`'s `WriteData` treated draining a queued response byte and accumulating a new command byte as mutually exclusive (`if/else`), but real SPI shifts a bit each way on every clock, simultaneously -- the byte that clocked out CMD12's last response byte (R1) was discarded instead of also being captured, silently eating CMD0's leading `0x40` and shifting every subsequent frame by one byte permanently (observable directly in the log: repeated 6-byte windows that were CMD0's own canonical `40 00 00 00 00 95` rotating by one position each retry). Fixed: the read (pending-drain) and write (cmdBuf-accumulate) sides are now independent, and cmdBuf accumulation resyncs on the SD command start-bit pattern (`value&0xC0==0x40`) rather than blindly counting every byte -- which also pre-empts a second latent version of the same class of bug once CMD17's 512-byte data block is exercised (0xFF filler sent while draining a large response would otherwise get miscounted as command bytes every 6 bytes). Not yet re-tested against real firmware.

Fifth correction, same session, and the largest: rerunning against real firmware got CMD0 correctly framed (`40 00 00 00 00 95`, the canonical bytes) every time, R1=0x01 every time -- and the ROM still retried it 12 times before firmware itself printed a legible "Error initializing SD card!" (screenshot-confirmed). Direct comparison against jorgegv/jnext's own `src/peripheral/sd_card.cpp` (user supplied the full jnext 0.99.144 source) explained why and rewrote `sdcard.go` comprehensively rather than patching further guesses:

- **Persistent post-response byte, not a fallback to 0xFF.** SD Physical Layer Simplified Spec § 7.3.2 says the bus goes idle (0xFF) after R1, but real TBBlue-family firmware's MMC_Init polls for the first non-0xFF byte and checks `(R1&0xFE)==0` -- a spec-faithful 0xFF tail reads as 0xFE once masked, which firmware treats as a CRC error and never proceeds. jnext (citing ZEsarUX's own real-hardware-derived `mmc_read()`) sustains a fixed byte per command until a new command or CS-deassert: CMD0 -> 0x01, CMD12 -> 0xFF (not R1's own 0x00, confirmed against the exact NextZXOS supervisor polling code at bank-2 $196D-$1978, which treats 0x00 as "still busy"). This was almost certainly the actual root cause of the 12 retries.
- **CMD8's R7 register has a fixed command-version byte (0x10), not an echo of the host's argument** -- the previous version's verbatim-echo approach produced an illegal command-version field.
- **CMD9 (SEND_CSD) was entirely missing** -- confirmed required specifically by enNxtmmc.rom's own SD-init flow (jnext cites firmware addresses $1925/$1F3D); without a real CSD register the firmware reads 0xFF for all 16 bytes and aborts. Implemented with a generic-but-valid CSD v2.0/SDHC layout ported from jnext, sized from the actual attached image.
- **CMD12 needs 8 leading "stuff bytes"** before its NCR+R1 (firmware reads them at TBBLUE.FW address 0x7AB0) -- the previous version returned a single byte total.
- **A real CRC-16 (XMODEM variant: poly 0x1021, init 0x0000)**, not dummy bytes, now backs CMD9's CSD block and CMD17's data block -- independently re-verified in this sandbox via a standalone Python port against the standard check value `CRC16("123456789")=0x31C3` before being ported to Go (this environment still has no Go toolchain to check the Go code itself).
- **CMD58's OCR voltage-window byte must never be 0xFF** -- tbblue.fw's OCR reader treats any 0xFF response byte as idle filler and skips it, misaligning every subsequent OCR byte by one; changed to 0x7F (still a valid, plausible voltage window).
- CCS (SDHC) in CMD58's OCR now correctly reflects the HCS bit ACMD41 was actually sent with, rather than being unconditionally set.
- CMD17's out-of-range case now sets R1's PARAMETER_ERROR bit (0x40) plus a data-error token, not a misleadingly-successful R1=0x00.
- Every non-obvious byte/timing value above is cited to the specific jnext comment it came from (many of which themselves cite exact TBBLUE.FW/NextZXOS memory addresses from jnext's own real-firmware debugging history), not re-derived independently.

Not yet re-tested against real firmware -- this is a large rewrite and, per the standing sandbox limitation, unbuilt.

Sixth finding, same session: rebuilding with the jnext-verified sdcard.go rewrite got CMD0 correctly formed and its persistent-byte response correct (`resp FF FF 01`, sustained 0x01) -- and the ROM *still* looped CMD0 the same number of times, still landing on "Error initializing SD card!" (screenshot-confirmed again). Critically, the log shows the firmware reading exactly ONE byte after each command (always the leading 0xFF NCR filler, never advancing to read R1 itself) before immediately restarting the whole 6-byte frame -- true both before and after the persistent-byte fix, meaning the retry count was likely never response-driven at all; a fixed-iteration "wake the card" ritual is a plausible read, but unconfirmed. Rather than continue guessing from the SD/SPI side alone, the trace harness (zzz_trace_harness_test.go, T-26's ZTRACE_* instrumentation -- full M1/memory-read/write/IO-in/IO-out hooks already wired to zen80) was extended to actually watch what nextboot.rom itself executes around this point: `loadModelROM` gained a `next` case (mirrors zenzx_headless.go's own -model=next setup, FDC+blank-disk included), and three new env vars -- `ZTRACE_BOOTROM`, `ZTRACE_DIVMMC_ROM`, `ZTRACE_SDCARD` -- attach the same boot chain the CLI flags do. One real wrinkle surfaced doing this: `M1Hook` is a single func field, so DivMMC's own OnM1 and the tracer's own M1 hook are composed manually at this one call site (chaining, not general hook-chaining infrastructure) since both are genuinely needed together here for the first time. Also noted: `github.com/ha1tch/zendis`, a separate Z80/Z80N disassembler cross-checked against zen80's own execution, as a complementary tool for turning a PC trace into readable assembly if the port-level trace alone isn't enough. None of this has been run -- same standing sandbox limitation.

Seventh finding, same session, and the first one confirmed via direct instruction trace rather than inference: the extended trace harness (above) caught the actual stuck loop verbatim -- `IN A,(0xEB)` / `CP 0xFF` / `RET NZ`, repeating for the entire rest of a million-step capture, decrementing a retry counter in BC, with **zero `OUT` instructions anywhere in the loop** (confirmed by their total absence from the O-line trace, which was enabled). Root cause: `sdcard.go`'s `ReadData` had always been a pure getter -- only `WriteData` ever advanced the response queue -- so a bare-read polling loop with no paired write could only ever see the same stale byte forever, even though the real queued response (e.g. CMD0's `01` a couple of positions past its leading NCR bytes) was sitting right there waiting to be drained. Fixed by making `ReadData` clock independently whenever it isn't immediately following a `WriteData` call (new `freshByte` field: true means "this read is the paired read for the write that just happened, don't clock again"; false means "bare read, clock it like a `WriteData(0xFF)` would") -- preserves the earlier, working OUT-then-paired-IN behaviour (CMD0/CMD8/CMD9 sending) while fixing the bare-polling case the trace caught directly. Not yet re-tested.

Eighth finding, same session (Anti-Brick): with the freshByte fix and a correctly re-clustered image (jnext's fix-sdcard-image.sh, after patching its GNU-only `du -sb` for macOS's BSD `du`), the boot got all the way into TBBLUE.FW itself -- screenshots show its "ZX Spectrum Next Configuration" UI, first failing on a missing CONFIG.INI (fixed by the image rebuild), then reaching an "Anti-Brick / No TBBLUE.TBU update file found!" screen. The message itself is benign (Anti-Brick is TBBLUE.FW's "reset" module, module 7 of its container, a firmware-recovery path that looks for a TBBLUE.TBU update file); the real question is why it is being entered at all. jnext's docs do not spell out the trigger, so what follows is a hypothesis, not a confirmed cause: zenzx left NR 0x01 (core version) at 0x00 and did not implement NR 0x10 (core id) or NR 0x0A at all, so all read 0x00, while jnext's VHDL-cited power-on values are NR 0x01=0x32 (v3.02), NR 0x0E=0x03, NR 0x0F=0x00, NR 0x0A=0x01, NR 0x10=0x04 (coreid=1). Firmware seeing "core version 0.0 / core id 0" plausibly concludes it is on a bricked or golden core. Those five registers now carry jnext's values (`nextreg.go`); no existing test asserts the old defaults (checked). Not yet run. If Anti-Brick persists, the next candidates are (1) the SPI flash chip-select: port 0xE7 is shared by SD, flash and RPi ("SPI CS (sd card, flash, rpi)"), we only model bit 0 (SD), and firmware probing an unmodelled flash may read a blank or absent one, and (2) NR 0x05/0x06/0x11 (also read by the supervisor; jnext seeds 0x41/0xA0/0x03, zenzx leaves 0x00). Separately, and already known from jnext's history: `sdcard.go` does not implement CMD18 (READ_MULTIPLE_BLOCK), which jnext documents TBBLUE.FW's FatFs needing for larger reads ("Error reading TBBLUE.FW data!" without it) -- expect it to be the next blocker once past this screen; not built yet.

Ninth finding, same session: past Anti-Brick, TBBLUE.FW itself now runs and fails with "Error reading TBBLUE.FW data!" -- exactly the CMD18 gap predicted at the end of the Eighth finding, confirmed directly rather than guessed. Implemented CMD18 (READ_MULTIPLE_BLOCK) in sdcard.go, ported from jnext's own cmd18_read_multiple_block/SENDING_DATA-continuation logic rather than re-derived: shaped like CMD17 for the first block, then a new nextResponseByte/loadNextMultiBlock pair transparently continues the stream (one 0xFF filler + 0xFE token + 512 bytes + real CRC-16 per subsequent sector -- the filler byte is deliberate, confirmed against jnext's own comment on why real firmware's token-poll needs it) until CMD12 or the image runs out (single 0x08 data-error token, also ported). Two related, easy-to-miss pieces came with it, both confirmed against jnext's own commit history for this exact firmware: an open CMD18 stream now survives CS deassert instead of being aborted by WriteCS -- jnext's own commit (dated 2026-07-09) documents this as a real, previously-hit NextZXOS-boot bug, since esxDOS's driver deselects between blocks and polls port 0xEB again on reselect without sending a new command; and CMD12, or any other new command arriving mid-stream, still correctly terminates it (jnext's own defensive-handling comment, guarding against exactly the bug of leaving multiBlock set and silently resuming after an unrelated command). Not yet run.

Tenth finding, same session, and the deepest yet: past CMD18, the boot got dramatically further -- confirmed via live GUI description, not just logs: the real core/video-mode selector UI ("Press SPACEBAR for menu / Press C for extra cores / A=All, D=Digital, V=VGA, R=RGB"), a correct ~30s timeout that reloads the same four files (keymap.bin, enNxtmmc.rom, enNextMF.rom, enNextZX.rom) matching real default-core-selection behaviour, and only then a permanent stall -- confirmed via a 50,000,000-step instruction trace (zzz_trace_harness_test.go, ZTRACE_MAX_STEPS bumped, port-level logging dropped since -divmmc-debug had already shown SD activity ends cleanly) to be `JR $` at PC=0x6D31, spinning forever with zero further port activity of any kind. Ruled out as benign idle-for-keypress specifically because real hardware would proceed with the default core after its own timeout, not stay parked indefinitely.

Direct comparison against jnext (src/peripheral/divmmc.h/.cpp) found the cause: DivMMC's automap has never been gated on NextREG 0x0A bit 4 (automap enable) in this codebase -- it fired on every trigger address unconditionally since T-36 began. Real hardware's own power-on default for that bit is OFF (confirmed against jnext's VHDL-cited default); only CONMEM (explicit port 0xE3 writes) works until firmware explicitly enables automap. jnext's own commit history describes having hit exactly this shape of bug before adding the gate: "cold-boot automap fire[d] at PC=0x0000 ... and overlay[ed] enNxtmmc.rom on slot 0 throughout the boot ROM phase." In zenzx's case the boot-ROM-overlay priority ordering happened to shadow the PC=0x0000 instance of this, but automap firing unconditionally on the IM1 vector (0x0038) during TBBLUE.FW's own core-selection phase -- hijacking its interrupt handler with enNxtmmc.rom's automap-hosted code before real hardware would have enabled automap at all -- is a well-motivated match for a stall appearing exactly where TBBLUE.FW's own interrupt-driven UI (keyboard scan, timeout counter, screen animation) would need to keep running.

Fixed: `divmmc.go` gained `automapEnabled` (false by default, matching real hardware), gating the trigger check in `OnM1` (automapExit stays ungated, matching jnext); CONMEM remains entirely independent of this gate, per jnext's own "CONMEM sails through automap_en=0" note. A disabled-while-paged transition immediately clears the overlay too (jnext's DA-08). Wired from NextREG 0x0A bit 4 in both `nextRegWriteDirect` and `SetNextRegister` (mirroring the established NR 0x03 pattern). No existing test touched any of this. Not yet run.

Eleventh finding (2026-11-16, 0.11.0, the first run of this chain where the code was built): with `nextboot.rom`, `enNxtmmc.rom` and a generated blank FAT32 card (80 MB, 512-byte clusters, MBR partition, empty root), the boot ROM initialises the card (CMD12, CMD0, CMD8, ACMD41, CMD58), reads the MBR and the volume with CMD17 and stops at "Error opening TBBLUE.FW file", which is the right answer for an empty card. With no card it prints "Error initializing SD card!". So the first stage (boot ROM overlay, SD over SPI, FAT32 mount) is confirmed working headless; everything from TBBLUE.FW onwards, the automap gate included, still waits for a run against the fixed card image.

Twelfth finding (2026-11-16, 0.11.0): the whole chain run against the official 26.11 emulator distribution, with the card prepared by `scripts/next_card.py`. How the other emulators (jnext, Ochenta, zx_go) get a card to boot was read from their documentation, and what the firmware expects from the hardware from its own source, which the distribution carries in `/src/firmware`; no code was taken from any of them. What was missing, in the order the boot found it:

1. **The card.** The distribution image has 32,758 clusters of 32K, under the FAT32 minimum; rebuilt with 4K clusters (261,620). Its `config.ini` says `timing=8`, which sends the firmware into its video-mode test, a wait for a key with a counter of 524,287 passes; set to `timing=0`.
2. **Config mode.** The firmware loads each ROM by reading the file to address 0 with NextREG 0x04 naming the SRAM bank. zenzx had no config window, so the files went nowhere and the machine restarted in the built-in +3 ROM. Added (`nextconfig.go`), with NextREG 0x03's machine-type and config-mode rules and NextREG 0x02's soft and hard reset.
3. **NextREG 0x1E/0x1F** (video line) always read 0; the firmware waits for bit 8 to change before it draws. Added (`nextvideoline.go`).
4. **NextREG 0x8E** only stored its value. NextZXOS's inter-ROM call (RST 20h, code at 0x3E00) writes it and expects the next fetch from the other ROM; without it the call returned into the memory test and the ROM signalled an error on the border. Added (`nextreg8e.go`).
5. **NextREG 0x57 read back a stale value** after port 0x7FFD had changed slot 7; ROM 2 saves and restores the slot by reading it, restored page 223, and the system stack at 0xFF4F was gone, so a `RET` went to 0x0000 and the machine restarted in a loop. NextREG 0x50-0x57 now read the live mapping.

Where it stands: firmware screens "Loading keymap ... enNextZX.rom...OK!", NextREG 0x03 <- 0xB3, soft reset, NextZXOS runs from the loaded ROM, passes its memory test (112 banks), sets up its stacks, and DivMMC automap enters at 0x0038 on every interrupt and leaves at 0x1FF8 (control 0x40, MAPRAM set). NextZXOS asks for a second soft reset during start-up, which is carried out. Then it sends CMD12 and CMD17 for block 0x47D5F9AF, which the card answers with a parameter error, and no further card traffic follows. Next candidates, not yet examined: where that block number comes from (the OS does not initialise the card again after the firmware, so it may depend on state the firmware left, or on NextREG 0x09 bit 3 clearing MAPRAM, which is not modelled); the SD card is read-only (`SDCardImage` is an `io.ReaderAt`; CMD24 is not implemented); the tilemap, which NextZXOS's screens use, is not drawn, and headless screenshots show only the ULA layer, so its bank 5 data appears as noise.

Thirteenth finding (2026-11-16, 0.11.0): **NextZXOS boots.** Welcome screen, main menu (NextZXOS v2.10, 1792K) and the Browser listing the card's root, from the headless build; `TestNextZXOSBoots` (dormant guard G-04) repeats it in 1.2 s. The twelfth finding's guesses about the bad block number were wrong: the number was garbage because the system had already gone astray. What was actually missing, found by following what Ochenta documents about this stage and checking each point against `registers.txt` and `ports.txt` on the card:

1. **The DivMMC's entry points are programmable on the Next (NextREG 0xB8-0xBB), and most count only with ROM 3 at 0x0000.** zenzx mapped the interface in at 0x0038 in every ROM, so each interrupt in NextZXOS's own ROMs was taken by the DivMMC. Added (`divmmc_entries.go`), with NextREG 0x09 bit 3 (clears MAPRAM) and port 0xE3 reading back.
2. **Delayed and instant mapping.** zen80 calls its M1 hook after it has read the opcode, so mapping there is always one fetch late for an instant entry point (0x3Dxx, NextZXOS's call gate into the DivMMC) and the code ran the 48K ROM's character set as a NOP slide. Instant entry points are now recognised in the memory read, when the address read is the program counter; delayed ones take effect on the next memory access.
3. **zen80 also reports the interrupt acknowledge as an M1 at 0x0038**, which mapped the DivMMC before the opcode at 0x0038 was fetched. The ROM's `PUSH AF` there and the DivMMC ROM's continuation at 0x0039 are written to be run in that order; the other way round the handler returned into the DivMMC still mapped and popped a wrong IX, which is where the bad block number came from. Acknowledge cycles are now ignored.
4. **The alternate ROM and ROM lock (NextREG 0x8C).** NextZXOS writes an alternate ROM through the ROM's addresses and runs from it; without it the system restarted after reading the card, about once a second. Added (`nextaltrom.go`).
5. **The screen cache.** On a change of displayed bank `updateScreenBank` copied the renderer's cache back into RAM, which is right for the classic models (the cache is the displayed bank) and wrong on the Next (RAM is written first), so bank 7 was overwritten; and every write to page 10 reached the cache whichever bank was displayed. RAM is now authoritative on the Next.
6. **Layer 2 was always composited** on a Next, over the ULA picture, from T-29's reading that no enable bit was documented. `ports.txt` documents it: port 0x123B bit 1, alias NextREG 0x69 bit 7. Gated on it; five tests that drew Layer 2 without turning it on now turn it on.

Fourteenth finding (2026-11-16, 0.11.0): **the GUI build boots NextZXOS too.** Linked in the sandbox once Mesa and the X11 development packages were there, and run under Xvfb with software OpenGL: welcome screen, main menu, NextBASIC's editor in 64 columns, the tilemap demo. Getting there needed the GUI to draw the tilemap and the Timex modes (T-44) and to let a script's keys through (T-45). Running the GUI-tagged tests for the first time found four of them wrong (T-46) and the reset value of NextREG 0x15 wrong (T-47).

Fifteenth finding (2026-11-16, 0.11.0): **saving to the card, checked again after a report that it failed in a local run.** On a card made by `scripts/next_card.py`, in the headless build, each of these ended in "0 OK" and was then found on the card image with a FAT library: `SAVE` of a BASIC program, `SAVE ... CODE 16384,40000` (40,128 bytes), `MKDIR`, `SAVE` into the new folder, `SAVE` over an existing file (which leaves a `.bak`), `COPY ... TO`, `.mkdir`, `.cp` (the copy equal to its source byte for byte), `ERASE` and `MOVE ... TO`. No block was refused and both FAT copies stayed equal. A program saved from the GUI build was loaded and listed after a second boot. The failure was not reproduced; what was found on the way is T-49 (the free space record) and that `run_next_divmmc.sh` kept using a `./zenzx` built before card writes existed. `TestNextZXOSSavesToCard` now repeats the save and the read-back.

Outstanding before this closes: T-44 (what is left of the tilemap and Timex modes) and T-48 (the GUI's handling of the Next layers).

### T-44. Tilemap and the Next's Timex modes are not drawn

Theme: next-support · Priority: P2 · Status: ◐

- **Trigger:** 0.11.0. NextZXOS reaches its welcome screen, main menu and Browser using the ULA screen only, so they display. The tilemap (NextREG 0x6B, 0x6C, 0x6E, 0x6F, 0x4C and its palette) is not drawn at all, and the editor's 64 and 85 column modes, many programs and the tilemap `.nex` loading screens use it. Timex modes through NextREG 0x69 bits 5-0 or port 0xFF are not connected on the Next either.
- **Fix:** a tilemap decoder (40x32 and 80x32, 4-bit and 1-bit tiles, attribute byte or global attribute, 512-tile mode, clip window, scroll) composited with the other layers under NextREG 0x15 and the tilemap-over-ULA bit, and the Timex modes wired to the existing hi-colour renderer and a hi-res one.
- **Done when:** NextBASIC's editor is readable in its 64 column mode, checked against a screenshot from another emulator or real hardware.

Shipped in 0.11.0 (2026-11-16), in the display decode that the headless build, scripts and tests use:

- **The tilemap** (`nexttilemap.go`): 40 and 80 columns, 4-bit tiles with palette offset, mirroring and rotation, text mode, one-byte entries with NextREG 0x6C, 512 tiles, either palette, map and definitions in bank 5 or 7, the clip window (NextREG 0x1B, 0x1C), scrolling, and its combination with the ULA: above or below per tile, forced on top, the ULA off, stencil mode, the fallback colour. A ULA pixel in the global transparency colour lets the tilemap through, which needed the ULA's pixels taken through the Next's ULA palette; the palettes now start with the Spectrum's colours and the tilemap's with the identity. In 80 column mode the picture is 512 pixels wide.
- **The Timex modes** (`nexttimex.go`): port 0xFF and NextREG 0x69 bits 5-0 as one latch; screen 1, hi-colour and hi-res, the last 512 pixels wide.
- The trigger had one thing wrong: the editor's 64 and 85 column modes are the Timex hi-res mode, not the tilemap.

Run in the headless build against the 26.11 card: the card's `/demos/tilemap/tm.nex`, started with `.nexload`, shows its coloured character map in 80 and in 40 columns with the ULA's text over it; and NextBASIC's editor is readable in 64 and in 85 columns. Unit tests: `nexttilemap_test.go`, `nexttimex_test.go`.

The GUI (2026-11-16): its standard picture is drawn on the GPU cell by cell and did not go through this decode. Both front ends now take the picture from `nextSoftwareULA` (nextsoftula.go) whenever a Next is in a Timex mode or has the tilemap on; the GUI shows it as a texture in place of the cell drawing (display_nextula.go). Seen in the GUI build under Xvfb with software OpenGL: NextBASIC's editor in 64 columns and the tilemap demo in 80; `TestGLNextPicture` (guard G-05) reads hi-res and a tile back from the window.

0.13.0: the tilemap is now drawn by the frame compositor (nextframe.go, T-48) over the whole 320x256 frame, border included, in both builds; the card's tilemap demo fills the frame in 80 columns.

Outstanding: (1) In the GUI at 1x a picture 512 or 640 pixels across is shown at half that, so every other column is lost (T-51). (2) No comparison with a reference screenshot has been made; the direction of tile rotation in particular is unchecked. (3) Closed in 0.14.0: the classic picture goes through the Next's ULA palette whenever that palette is not at its starting values, which under NextZXOS is always, and in ULANext mode. (4) NextREG 0x69 bit 6 (shadow screen) and port 0xFF bit 6 (interrupt disable) are stored and do nothing.

### T-50. Remove the Next drawing code the frame compositor replaced

Theme: next-support · Priority: P2 · Status: ☐

- **Trigger:** 0.13.0. The frame compositor (nextframe.go) replaced the Next's earlier drawing in both front ends; the earlier code is still in the tree and nothing but its own tests calls it.
- **What is superseded:** in the GUI, `renderNextLayersGPU`, `renderLayer2GPU`, `renderSpritesGPU`, the sprite texture cache, the Layer 2 palette textures and `nextLayersViewportOffset` (videorender_gpu.go, display.go); shared, `DecodeLayer2`, `DecodeSprites`, `CompositeLayer2`, `CompositeNextLayers`, `layersAboveULA`, `widenToMatch`, `CombineULAAndTilemap` and the tilemap branch of `nextSoftwareULA`. They place Layer 2 and the sprites wrongly (no 32 pixel inset for the sprites, the clip window not doubled in the wide modes, nothing in the border), which is why they were replaced.
- **Why not removed at once:** the compositor had been run only under software OpenGL; the earlier path stays one release so that it can be put back if the new one misbehaves on a real GPU.
- **Done when:** the code above and its tests are deleted, guard G-03 (the Layer 2 palette textures) is retired with them, and both suites pass.

### T-51. What the Next's frame compositor does not draw

Theme: next-support · Priority: P3 · Status: ☐

- **Trigger:** 0.13.0, writing the frame compositor (nextframe.go).
- **Not drawn:** (1) Since 0.15.0 the frame is drawn in pieces as the beam goes down it (nextraster.go), so what the copper or the CPU changes part of the way down shows from where it changed, and a border that changes colour is striped. Left of that: the pieces are whole lines, as CSpect draws them, where the machine changes at the pixel; a write by the CPU is placed at the start of its instruction, up to 20 T-states early, which matters near the end of a line; the pieces hold the memory as it was when each was composed, so a picture redrawn under the beam does not tear; (2) LoRes is drawn since 0.20.0 (nextlores.go), standard and Radastan, with its scroll; its place when the ULA is turned off by NextREG 0x68 has not been checked. (3) The two blending layer orders (NextREG 0x15 bits 4-2 = 110 and 111), taken as S L U without the blend. (4) The ULA's own scrolling (NextREG 0x26, 0x27) and half pixel scroll. (5) The Layer 2 palette's priority bit. (6) Sprite priority reversed (NextREG 0x15 bit 6), and sprite collision and line overflow (port 0x303B reads 0). 4-bit sprite patterns are drawn since 0.20.0. (7) At 1x in the GUI a frame 640 pixels across is shown in 320, so every other column is lost.
- **Compared with CSpect (0.14.1, 2026-11-16):** CSpect 3.3.1.0 under Mono, same ROMs and distribution. The NextZXOS welcome screen has the same seven colours in both. The warm grey white of 0.14.0 was a fault here, not the hardware's: NextREG 0x44 could not be read back.
- **Compared with CSpect (0.16.1, 2026-11-16):** CSpect is the reference for the picture. Two copper lists typed in NextBASIC, each giving the white paper a new colour on each of the 192 lines (start control 11). With WAIT line,0 the two pictures are the same on every pixel. With the WAIT's horizontal position going from 0 to 47 in bands of four lines, they are the same for positions 0 to 33 (lines 0 to 135); from 34 on CSpect shows a line's own colour on five lines of every eight and the line before's on the other three, whatever the position, and ZenZX shows the line's own colour for 34 and 35 and the line before's from 36: 33 lines differ. CSpect draws a line whole, from the state near the end of its right border; ZenZX does the same since 0.16.1 (at 0.15.0 it changed at the pixel, which put a step at the left of every bar). show512 is the same on every pixel since 0.16.0.
- **Done when:** each of these is drawn or has its own item.

### T-56. A NextBASIC loop takes 5% fewer frames than in CSpect

Theme: next-support · Priority: P3 · Status: ☐

- **Trigger:** 0.17.0, checking the line interrupt against CSpect 3.3.1.0 from NextBASIC: `POKE 23672,0: FOR i=1 TO 500: NEXT i: PRINT PEEK 23672` counts the interrupts a loop takes. CSpect prints 94, 191 with the line interrupt on and 93 with the frame interrupt off as well; ZenZX prints 89, 181 and 89. The ratios agree, which is what the check was for. The loop itself takes 5% fewer frames here.
- **Not known:** where the 5% is. Candidates: memory contention, which the Next has at 3.5 MHz and which CSpect may charge differently; instruction timings; the frame's length in CSpect.
- **Done when:** the cause is known, and either the count agrees or the difference is written down as CSpect's.

### T-58. The Next's DMA (ports 0x6B and 0x0B) is not emulated

Theme: next-support · Priority: P1 · Status: ◐

- **Trigger:** 0.18.1, taking screenshots of the card's games for the README. Ports 0x6B (zxnDMA) and 0x0B (Z80 DMA mode) are not there at all: a program that sends the DMA a transfer gets nothing done.
- **Effect:** the games that use it to clear or fill the screen, copy pictures or play samples draw wrongly. Of the eight programs tried, the four that draw wrongly (Warhawk, Nextoid, AngryBloaters, ScrollNutter) all contain code that sends a program to port 0x6B, and the two that draw right throughout (Night-Knight, Spectron 2084) contain none; The Lords of Midnight has one such place and draws its title right and its next screens wrongly. That is what points at the DMA; no run has yet shown the DMA to be the cause in any one of them. `docs/next-programs.md` has the list.
- **Fix:** the zxnDMA as the Next's ports.txt and the page it names describe it: the subset of the Z80 DMA's registers the Next has, memory to memory, memory to port and port to memory transfers, continuous and burst modes with the prescaler, and the transfer's T-states taken from the CPU.
- **Shipped in 0.19.0 (2026-11-16):** `nextdma.go`: the registers, memory and port transfers in both directions, counting up, down or staying, the read sequence and status, port 0x0B's extra byte, and timed transfers by the prescaler. Warhawk, Nextoid, AngryBloaters and The Lords of Midnight now draw their first screens cleanly (`docs/next-programs.md`).
- **Compared with jnext (0.19.1, 2026-11-16):** jnext 1.3.0, built here without its GUI and run headless on the same card, shows the same title and high score screens for Warhawk, the same menu for Nextoid, the same title and table for AngryBloaters and the same title for The Lords of Midnight, by eye, at moments that are not the same frame in the two. It is a second reference, used as a running program only.
- **Outstanding:** nothing has been compared with CSpect. CSpect 3.3.1.0, the version that runs here, shows a black screen for Warhawk; 4.5.1.0 does not start under the Mono installed (it wants the netstandard library). No timed transfer has been heard: the sound of a sample player was not checked. Auto restart without a prescaler and the prescaler in continuous mode are as the file's header says, not as the machine.
- **Done when:** Warhawk's title and first level are the same as CSpect's, and each program in `docs/next-programs.md` whose fault this was is marked as drawing right.

### T-59. The Next's interrupt mode (NextREG 0xC0 to 0xCF) and the CTC are not emulated

Theme: next-support · Priority: P2 · Status: ◐

- **Trigger:** 0.20.0, comparing the card's games with jnext. TX-1696 writes NextREG 0xC0 = 0xA9 (bit 0: the Next's own interrupt mode, with the vector's high bits), 0xC5 = 0x01 (the first CTC channel's interrupt) and 0xCC, 0xCD (which interrupts may break into a DMA transfer). None of it is acted on: every interrupt is the frame's or the line's, with 0xFF on the bus, and there is no CTC (ports 0x183B to 0x1F3B) at all.
- **Effect:** not seen yet. TX-1696 reaches its game and its first screens are the same as jnext's. Whatever it times with the CTC, its music most likely, does not run, and a program that takes its frame interrupt through the vector this mode gives it would not get it.
- **Fix:** the interrupt mode of NextREG 0xC0 with the vector it makes for each source, the enables and status of 0xC4 to 0xCA, and the four CTC channels as the Next's ports.txt describes them.
- **Shipped in 0.21.0 (2026-11-16):** `nextint.go`: the hardware IM 2 mode of NextREG 0xC0 with a vector for each source, the enables of 0xC4 and 0xC5 (0xC4's are NextREG 0x22's), the status of 0xC8 and 0xC9, and four CTC channels as timers and as counters of the channel before. A test program takes the frame, the line and a CTC interrupt each through its own vector and counts them. TX-1696 keeps the same screens as jnext and now reaches each of them at the same step.
- **Outstanding:** TX-1696's sound has not been listened to. Not there: the UARTs; a CTC interrupt in the ordinary mode; the chain that holds a lower source back while a higher one's handler runs; NextREG 0xCC to 0xCE; a CPU that is not in IM 2 while the mode is on.
- **Done when:** a test program takes a CTC interrupt and a line interrupt through their own vectors, and TX-1696's sound has been listened to beside jnext's.

### T-61. The Multiface has no NMI key, and only what NextZXOS uses was tried

Theme: next-support · Priority: P3 · Status: ☐

- **Trigger:** 0.22.0, T-52: the Multiface was added for what NextZXOS needs to start a snapshot.
- **Not there:** a key for the NMI button in the front ends (`MultifaceButton` is there and nothing calls it; the Multiface ROM's menu drew when the NMI was taken outside a snapshot start); the DivMMC NMI of NextREG 0x02 bit 2 and its button; the I/O traps that make a Multiface NMI (NextREG 0x02 bit 4); NextREG 0x02 bits 4-2 reading back as the cause of the NMI other than as last written.
- **Not tried:** the Multiface kinds other than the +3 (NextREG 0x0A bits 7-6), which differ in their ports and in what the enable port reads.
- **Chosen without a source:** a write to either of the Multiface's ports hides it until the next NMI (on the machine it is one of the two); DivMMC's entry points do not fire while the Multiface is paged in; RETN does not also page DivMMC out. The Next's ROM gives the same result either way; another program might not.
- **Done when:** the front ends have an NMI key that brings up the Multiface's menu and leaves it again, and each of the others is done or has its own item.

### T-62. The Next examples were compared with jnext only, not with CSpect

Theme: next-support · Priority: P3 · Status: ☐

- **Trigger:** 0.23.0, the examples in `examples/next` (gallop, fuji, orbit).
- **Known:** each shows the same picture in ZenZX and in jnext 1.3.0, started with `.nexload`. Sprite positions going round at 512 was taken from jnext for these and is in ZenZX since.
- **Not done:** none was run in CSpect 3.3.1.0, which is the standard for the picture, or on a Next; `fuji` and `orbit` were not started from the Browser; nothing in them makes a sound.
- **Done when:** each has been compared with CSpect from the same keys, or the reason it cannot be is written down.

### T-63. SD Card menu: the first slot and per-slot write protection are missing

Theme: next-support · Priority: P3 · Status: ☐

- **Trigger:** 0.24.0, the SD Card menu and the second card slot.
- **Not there:** changing the card in the first slot from the menu (it means starting the Next again from another card, with its own DivMMC ROM); write protection for each slot separately (`-sdcard-readonly` is for both); ZenScript actions for the second slot, so a headless script can change it.
- **Not tried:** the menu's items clicked one by one in a window (their actions are tested, and the menu and its card browser were looked at under Xvfb); a second card with more than one partition; a card the Next writes to from both slots at once.
- **Done when:** each of these is done or has its own item.

### T-64. DMA+ and 4 MB: board ID, interrupts holding the DMA, and a comparison with CSpect 4.5 are missing

Theme: next-support · Priority: P3 · Status: ☐

- **Trigger:** 0.26.0, DMA+ and the issue 5 board's 4 MB.
- **Not there:** the board ID in NextREG 0x0F is 0 (issue 2) whatever `-next-issue` says: the card's registers.txt lists issues 2 to 4 and gives no number for an issue 5; an interrupt holding the DMA off while a transfer runs (NextREG 0xCC to 0xCE); an issue 2 board with 1 MB (`-next-issue=2` is the one with the 2 MB expansion); banks 5 and 7's other SRAM is kept, but nothing models what the machine's ULA shows of a physical write there beyond "nothing".
- **Not tried:** the same nine examples in CSpect 4.5.1.0 (`-issue=5`), which does not start under the Mono installed here, or on a machine; a DMA+ transfer with a prescaler; a 2D transfer to or from a port; burst mode in 2D.
- **Done when:** the board ID follows the issue with a documented value, and `zxnddma_examples` and `zxnddma_compare` have been seen side by side with CSpect 4.5 or a KS3.

### T-72. NextZXOS .dsk disk-image mounting on the Next (A:/B:), not only copy-to-card

Theme: next-support · Priority: P3 · Status: ☐ · Blocks/after: —

ZenZX does not mount `.dsk` disk images on the Next. The `next` model is
built without a floppy controller (`machine_model.go` does not call
`EnableFDC()` for it, unlike `plus3`), and there is no NextZXOS/+3e
disk-image mount path in the code. So a `.dsk` dropped on a Next is copied
to the SD card as a file (the only reachable outcome), while on a +3 it is
mounted in Drive A and booted.

- **Trigger:** 2026-11-21, while documenting the drag-and-drop matrix. Real
  NextZXOS does support disk images through its +3e/+3DOS layer: C: is the
  default system drive (the SD card; paths are `C:/NEXTZXOS/...`), and drive
  letters A:-P: can be mapped, with A:/B:/M: present by default. Whether the
  Browser mounts a selected `.dsk`, and whether a Next boots from a disk
  image, was not confirmed from the +3e command reference or the dot-command
  reference (their Browser/`.RUN` extension lists show tap/snapshot/nex, not
  `.dsk`) -- it needs checking against a real Next or NextZXOS source before
  anything is implemented.
- **Fix:** decide the scope. Minimum: let a `.dsk` dropped on a Next be
  mounted as a NextZXOS drive (A:/B:) rather than only copied, matching how a
  real Next treats it. This likely means a DivMMC/NextZXOS-side disk layer,
  not the +3 FDC path, since the Next reaches storage through the SD card and
  its own OS, not a physical drive. Confirm the real NextZXOS behaviour first
  (which drive letters a mounted image takes; whether the Browser does the
  mounting; whether it is bootable).

### T-79. The GUI has never been launched with the Next boot cache on auto

Theme: next-support · Priority: P3 · Status: ☐

Trigger: found 2026-11-23 while finishing T-76. The GUI build has only been run with the boot cache off or never asked for; `-next-boot-cache auto` has been exercised by the headless guards (G-13) and not by the program that opens a window.

Scope: start the GUI under Xvfb on a Next with the boot files and the cache on `auto`: first run writes the cache, second run is put back from it, and the picture and the menu are as from a cold boot. Done when: that run is recorded here.

### T-80. The Next boot cache over a changed card: memory fields not analysed one by one

Theme: next-support · Priority: P3 · Status: ☐

Trigger: found 2026-11-23, T-76 review. G-13 compares a machine put back from the cache with a cold boot field by field, but the memory-related fields of a cache made over a changed card have not been gone through one by one for what the card's change could reach.

Scope: list the memory and DivMMC fields the cache holds, and for each say whether a changed card can make it stale; add to G-13's changed-card tests whatever is not covered. Done when: the list is in docs/proposals/NEXT_BOOT_CACHE.md and the tests cover each field it names.

## embedding

### T-39. Extract the machine core from `package main` into an embeddable `pkg/machine`

Theme: embedding · Priority: P1 · Status: ◐ · Filed-by: zenplay:T-05 · Blocks/after: —

Trigger: about 13.7k raylib-free lines (memory, ULA and contention, FDC and disc, AY, tape, snapshots, Next features) live in the root `package main`, with the wiring in a roughly 380-line `main()` in `zenzx_headless.go`. A `NewZenZX(audioBackend)` constructor and a `DisplayManager` with GUI and headless implementations already exist.

Scope: move the core into a library package; replace the `main()` wiring with a constructor plus options (model, ROM set, audio backend, display sink); leave the GUI and headless commands as thin shells over it; land it in its own release, apart from the Next-support closures (T-33, T-34).

Done when: the headless suite passes on top of the library, and an external module can construct and step a machine without importing anything graphical.

Added 2026-11-16: the library is also what several machines in one program, diffing them, the workbench (`docs/proposals/workbench.md`) and zenplay's Run page stand on. So: no mutable state shared between machines, the display and the audio as sinks the host supplies, and a test that runs two machines side by side and shows that neither disturbs the other, as part of "done".

Progress, 0.28.0 (2026-11-16): three of the four stages of `docs/MACHINE_MAP.md` shipped. The route taken is that document's option B, not a move to `pkg/machine`: the root package itself is the library, `package zenzx`, with the programs in `cmd/zenzx` and `cmd/zenzx-headless`. The core no longer prints (two sinks a host sets, `logging.go`); the audio is one engine with a device interface and the display an interface, so no name is declared per build tag any more; `twomachines_test.go` runs a 48K and a 128K machine alone, in turns and at the same time, and each ends as it does alone; `scripts/check_import.sh` imports the package from another module with `-tags headless,purego`, runs a machine and reads its picture back.

Progress, 0.36.0 (2026-11-18): a piece of stage 4. The set-up of a machine as a model (its ROM set, a +3's floppy controller, a Next's processor and memory) is one function, `loadModelROMs` in `machine_model.go`, used by the GUI's live model switch and by `newMachine`, which makes every machine after the first in the headless program. Neither is exported yet, and the first machine of each program is still wired by its start-up code. An embedder also gained `NewLink`/`AttachLink` and `AutorunBASIC`/`AutorunFile` (`docs/link.md`).

Outstanding: stage 4, the exported `Machine` over the internals (a constructor with options for the model and ROM set, the Next's wiring, snapshots from bytes, input), which is the "constructor plus options" of the scope above. Until then an embedder reaches only `NewZenZX`, the ROM loaders that take bytes, `Reset`, `RunFrame`, `DecodeDisplay`, `SetDisplay` and the audio's `SetDevice`. Verification outstanding: the two-machine test under the race detector on a machine with several cores (it passed here on 2), and sound on a real sound card after the audio change, including switching sound off and on again.


### T-40. Provide a purego-safe audio backend so the embedded machine builds with `CGO_ENABLED=0`

Theme: embedding · Priority: P1 · Status: ☐ · Filed-by: zenplay:T-06 · Blocks/after: after T-39

Trigger: the GUI frontends build raylib cgo-free with `-tags purego`, while this tree's audio uses `oto`, which needs cgo on Linux. An `AudioBackend` seam already exists.

Scope: implement a backend that works under purego (raylib's own audio is the candidate, unverified under purego); keep `oto` available behind a build tag.

Done when: a `CGO_ENABLED=0` build of a host that embeds the machine produces sound, or the blocker is recorded here with evidence.

### T-65. The workbench: the running machine as the thing being edited

Theme: embedding · Priority: P2 · Status: ☐ · Blocks/after: after T-39

A development workbench inside ZenZX: a debugger of CSpect's kind, assembling to hot memory, saving snapshots, tapes and NEX files from the hot state, a visual memory map whose blocks open as source and can be dragged to relocate them, block types found by the emulator and forced by hand, several machines at once and diffing them, with the further ideas recorded beside those. docs/ZENBENCH.md names the base it shares with zenplay. docs/proposals/workbench.md has all twenty (editing sprites and images in memory with editors from zenplay was added the same day), what is hard in each, and what exists to build on. Nothing is designed in detail or implemented.

Trigger: the discussion of 2026-11-16, after zenas and zendis became libraries and ZenZX moved onto zenui.

Done when: the proposal has become a staged plan with its own tracker, and this item points at it.

### T-67. The machine dock: what the first cut leaves out

Theme: embedding · Priority: P3 · Status: ☐

Trigger: the dock shipped in 0.29.0 as a first cut (2026-11-17). What it does not do yet, so that none of it is lost.

- New makes the front machine's model; there is no choice of model on the tile, and no way to start a machine from a snapshot, tape or disc directly.
- The deck (its machines, their names and order, the layout) is not saved between sessions.
- A name from tape is the tape's first Program header, not the header of what was actually loaded: a tape whose first program is skipped names the machine after it all the same.
- Only the front machine is heard. Mixing, or a choice per machine, is open (the workbench proposal's table of what machines share).
- A disc dropped on the window or given with `-disk` is not checked against the other machines' drives; the Floppy Disks window and Open DSK Image are.
- Tiled layouts: no border stripes in a tile; the pointer over a tile is not that machine's mouse; the layout is not saved; a tile's scale is whole, so a large monitor leaves room unused rather than fill it.
- New beside a Next makes a 48K; it could make a Next and open the SD Cards window at once (0.30.0 gave a second Next its own cards through the Machine menu).
- The dock is in ZenZX. As a widget it belongs in zenui (a strip of live pictures, magnification, the flight), with the host supplying the pictures; zenplay would be its second user. Since 0.35.0 its rounded shapes are drawn through raycanvas, called directly with raylib types (`dockround_gui.go`); a zenui widget would need rounded fills, strokes and images in zenui's `Renderer`, with raycanvas behind the raylib adapter.
- Not seen on macOS; sound passing between machines not heard.

Done when: each line above is done, dropped with a reason, or moved to its own item.

### T-69. The link, -multi and -autorun: what the first cut leaves out

Theme: embedding · Priority: P3 · Status: ☐

Trigger: 0.36.0 added `-multi`, `-listen` and `-autorun` (docs/link.md). What that first cut does not do:

- `-autorun` is not available on a Next or a TS2068: it presses the keys that load from tape, and those are known only for the 48K and the 128K family. On a Next the way in would be NextZXOS's own loader, or a file put on the card.
- A Next of `-multi` other than the primary one gets no card in its second slot, and `-sdcard-readonly` is the same for all of them.
- `-tape`, `-snapshot`, `-script`, `-disk` and the like go to one machine only (the first, or the Next). There is no way to give them to another.
- A machine added from the dock has no link, and a link cannot be given or taken away from the GUI.
- The link's two ports are fixed at 16384 and 16385 from the command line (`Link.SetPorts` exists for a program that embeds ZenZX).
- The host cannot tell from the socket when the machine has taken a byte, and nothing limits how many bytes it may queue.
- Not seen on macOS or Windows.

Scope: each of these is a small piece of its own; do them as they are wanted.

### T-89. An exported program has not been run on Windows, or seen opening its window on Linux

Theme: embedding · Priority: P2 · Status: ◐

Trigger: 0.50.0, File > Export (standalone.go, export_gui.go, export_dialog_gui.go).

Scope: the program is the ZenZX it was exported from with the machine appended after it. On Linux the headless program was run (a machine put back from a file is the machine that was exported, field by field, over nine models: standalone_test.go) and the GUI program was started under Xvfb, where its log shows the machine loaded and running at once; the window's picture is not drawn under Xvfb's software OpenGL here, so the program has not been seen on a screen. Run on macOS, 2026-11-24, by the tech team: a program exported there works perfectly (the Mac's model and system version were not recorded; the worry that data appended to a signed executable would invalidate its ad-hoc signature did not arise). Not run: on Windows (the program's .exe; the Windows subsystem and a console-less start). The Next and the Timex machines are not exported, and nor is the tape's position.

Done when: a program exported on Windows, and one on Linux seen on a real screen, has been started by double-click on that system, its window showed the machine as it was exported and running, and what ran, where and when is recorded here (macOS is recorded above).

---

## formats

### T-84. The RoboCop .rzx goes out of step at frame 5006 of 38881: a loop in a long interrupts-off routine runs about 130 fetches fewer than in the recorder

Theme: formats · Priority: P3 · Status: ☐

Trigger: found 2026-11-23, playing the RoboCop .rzx the user attached (a 128K recording of 38881 frames, about 13 minutes, recorded in another emulator).

Scope: playback stays in step for 5005 frames (about 100 seconds of play; the screen at frame 5000 shows the first level with score 03700 and time 2:37), then stops at frame 5006 with "0 IN reads where 0 were recorded". Frames 5005 and 5006 are one long routine run with the interrupts off (the screen blitter that sets SP from IX and pushes, with the AY register writer called twice and the 128K paging port written from banked code); the recording ends frame 5006 after 8489 fetches, before the IN at 84EF, where ZenZX reaches that IN at fetch 8359: so some loop in that routine runs about 130 fetches fewer here than in the recorder. Two things were found and handled on the way (rzx.go): the interrupt is offered to one instruction boundary only, not held for 32 cycles (frame 4502), and a frame the recorder ended between DD and the opcode of INC IXH is played with the instruction whole and the odd fetch carried into the next frame (frame 4537). Ruled out: R (zen80 patched as in T-83, same frame), the interrupt (the interrupts are off throughout), counting DD/FD half-register opcodes as one fetch (breaks at frame 696). Not ruled out: a flag or instruction detail in zen80 that the blitter's data depends on, port decoding of OUTD for the AY (BC=BFFD after the decrement), 128K paging from banked code, the snapshot's own state. Next step: a second emulator's trace of the same recording around frame 5005, to compare register state at the frame boundaries; ZenZX has none to hand. Done when: the recording plays to its end, or the cause is found and recorded in docs/KNOWN_ISSUES.md as the recorder's own.

### T-85. Formats zentools has that ZenZX does not use: Next and Spectrum screenshots, .pzx tapes, symbol files, saving .szx, ZX0

Theme: formats · Priority: P4 · Status: ☐

Trigger: the user asked, with RZX done, whether ZenZX could take more formats from the shared zentools library, Next screenshots among them. Checked 2026-11-23 against zentools v0.16.1 (the pinned tag): ZenZX imports basic, bdf, build, rzx, snapshot, szx, tap and tzx, and does its own .scr and .nex.

Scope: candidates, none started. (1) Screenshots: saving the screen as a .scr (pkg/scr, 48K-style and Timex modes) and as a .bmp (pkg/scr), and for the Next the Layer 2 bitmap (.nxi), palette (.nxp) and sprite sheet (.spr) formats of pkg/zxn, so a Next screen can be saved in the form its tools read. (2) .pzx tapes (pkg/pzx: read, and convert from .tap). (3) Symbol and map files (pkg/symfile: the CSpect map and other assembler formats), for naming addresses in trace and debug output. (4) Saving .szx (pkg/szx decodes; the encoder is not used here). (5) ZX0-compressed data (pkg/zx0), for loading a compressed .scr or block. Gaps zentools has nothing for, listed so they are not mistaken for omissions: tape recording, .csw and .wav tapes, .pok cheat files, TR-DOS (.trd, .scl) and Microdrive images. Each one that is taken up gets its own item. Done when: each candidate is either filed on its own or dropped here with a reason.

## ux

### T-86. Menu and key-handler messages other than the Storage menu's go to the terminal only; GUI-only users never see them

Theme: ux · Priority: P3 · Status: ☐

Theme: ux · Priority: P3 · Status: ☐

- **Trigger:** 0.50.0, the Storage menu rework, which sends the disc and card messages to a toast through `zx.notice` and `OnNotice`.
- **Not there:** the same for the rest of the GUI. About 50 `fmt.Println`/`fmt.Printf` calls in `input.go` (tape, snapshot, machine, display and preference handlers) and about 15 in `menubar_gui.go` still write to the terminal only, so a user who never sees the terminal gets no confirmation or refusal from them (the Snapshot menu's Quick Save, Quick Load, Save Timestamped and Save As are done, 0.50.0): tape Play/Stop/Rewind and mode changes, machine switches, paused/resumed, FPS and border toggles, preference changes, and the refusals ("no snapshot to load", model-specific messages).
- **Not tried:** which messages are worth a toast and which are noise (a toggle already shows its own checkmark); how a burst of messages should queue, since a new toast replaces the one in progress.
- **Done when:** each handler that refuses or confirms something the user cannot otherwise see goes through `zx.notice`, and the ones left terminal-only are listed here with the reason. The Storage menu screenshot in the manual (`docs/img/next-sd-card-menu.png`) shows the old layout and is taken again at the same time.


### T-87. The bar's text scale is made by a wrapper around zenui's renderer; zenui should take a label scale

Theme: ux · Priority: P4 · Status: ☐

Trigger: 0.50.0, the menu bar's text scale (menubarscale.go, menubarscale_gui.go). zenui v0.2.1's MenuBar draws its labels at scale 1 with no option, and its dropdowns at its own scale.

Scope: the bar's labels are made larger by drawing the strip through `barScaleRenderer`, which turns the labels' scale 1 into the bar's scale until the strip's one-pixel bottom line, and leaves the dropdowns alone; and the logo's and the rainbow's sizes are set through the package variables `zenui.RainbowScanlines` and `zenui.RainbowBandWidth` before every draw. Both depend on how MenuBar orders its drawing (the bottom line ends the strip) and on package state. Ask zenui for a label scale on MenuBar (a field or method beside SetScale) and for the rainbow's size as part of the bar's own height, then delete `barScaleRenderer`, its test, and the package-variable writes in `drawBar`. Done when: the bar's scale is passed to zenui, the wrapper is gone, and `TestGLBarScale` and `menubarscale_gui_test.go` pass unchanged.

### T-88. The tape deck's first cut: one machine at a time, no Fast mode, no seeking, nominal 3.5 MHz time, a colour per tape and not per header

Theme: ux · Priority: P3 · Status: ☐

Trigger: 0.50.0, the tape deck (tapedeck.go, tapedeck_gui.go, cassette_vector.go).

Scope: the deck is shown for the machine in front only, not while the machines are tiled (each tile would need its own deck and its own tape), and not in Fast mode (the ROM's loader is trapped, so there are no pulses to count). Its bar and counter are time at a nominal 3.5 MHz (tapeTicksPerSecond), where a 128K runs at 3.5469 MHz. There is no seeking by clicking the bar, no fast forward, and the cassette's colour changes with a new tape and not with each Program header of a tape that has several. The border's bands are worked out from the display's own margins (deckPictureBands); for the Next's wider Layer 2 modes the deck goes where the classic border would be, and that has not been looked at on a real display. The buttons have no tooltips and no keys of their own beyond Alt+P and Alt+R. The slide-in menu bar covers the deck's top band and the machine dock its bottom band while they are out. Done when: each of these is done, or recorded in KNOWN_ISSUES as a limit that stays.

### T-90. Machine > Stop and the Pause/Play row have not been seen in a window with the machine's picture

Theme: ux · Priority: P3 · Status: ☐

Trigger: 0.50.0, Machine > Stop (machinedeck_stop_gui.go, stomp_gui.go, machinemenu_gui.go).

Scope: the menu rows (label and icon), the stop of one of several machines, the stop of the last one (a window with only its background), the starting of a machine again from the dock's New and from the Machine menu, and the animation are covered by tests (stop_gui_test.go, stompicon_gui_test.go, TestGLStomp, G-18). The window has not been looked at as a person would with the machine's picture on it: the picture is not drawn under Xvfb's software OpenGL in this environment, so the dim over it, the leg coming down over a real picture and the dock after the last machine has gone are checked by the pieces and not together.

Done when: Stop has been used on a real display with one machine and with several, in a tiled layout and a single one, and the dock, the menu and the empty window were as described in docs/gui.md; what ran, where and when is recorded here.

### T-94. A file named on the command line is ignored: it must open, so that installers can register file types

Theme: ux · Priority: P3 · Status: ☐

Trigger: 2026-11-25, while scoping installers (T-93): a file association, or double-clicking a tape or snapshot in the file manager, starts `zenzx <file>`; ZenZX parses its flags and ignores any other word on the command line (`fs.Parse(args)` in `zenzx_gui.go`, no use of `fs.Args()`).

Scope: take one file name on the command line and open it as File > Open does (a tape, snapshot, disk, .nex or .scr; the drop handler already decides what each kind does, `dropchoice_gui.go`), on the first machine; say so with a toast if the file cannot be read or is not a kind ZenZX opens. Then the installers can register the file types: `MimeType=` in the .desktop file with a MIME package, `CFBundleDocumentTypes` in the macOS Info.plist (and the app must also take the file from the open-document event, which macOS delivers instead of an argument), and a ProgId with Open-with verbs in the MSI.

Done when: a file named on the command line opens, on all three systems, and the installers register the types, each shown to work by double-clicking a file on that system.

## export

### T-91. Export for another platform: fetch and verify the other system's stub, a Runs-on row in the Export form, and the signed-manifest CI stages

Theme: export · Priority: P3 · Status: ☐

Trigger: 0.50.0, after the page of programs was published (runners.json).

Scope: docs/proposals/export-for-another-platform.md: the plan in stages S0 (spikes on a Mac, Windows and Linux arm64), S1 (core, offline), S2 (signed manifest), S3 (client and form), S4 (CI: manifest, draft release, verify-release, publish), S5 (documents). Decisions D1 to D9 are taken there; the key custody and the macOS question are open.

Done when: S0 to S5 are done and an export for each system has been made from another and run on that system, with what ran, where and when recorded here.


## windows

### T-92. The Windows program does not start for two testers

Theme: windows · Priority: P1 · Status: ☐

Trigger: 0.50.0 release, reports from two testers (2026-11-25): the program does not start on Windows; no message, as the GUI program has no console and the failure is a panic in a dependency's init.

Scope: found by reading, not run: github.com/jupiterrider/ffi v0.7.0 embeds libffi only for windows/amd64 (embed_windows_amd64.go; there is no arm64 file), so on windows/arm64 its init panics unless libffi-8.dll is installed; the amd64 program's two embedded libraries (raylib.dll, libffi-8.dll) import only system DLLs and are extracted to %LocalAppData%\github.com\... and loaded; their init runs before main, so console_windows.go's AttachConsole is too late to show the panic. Run under Wine 9.0 (a fresh profile, so the libraries are extracted on the first run) on 2026-11-25: the windows/amd64 program built from this source and the one published on the page (SHA-256 a15dfb4c, as runners.json gives it) both start, show the menu bar and the Spectrum's boot screen. So the cause is not in the program's own start-up on amd64; it is something on the testers' machines (Windows version, antivirus or SmartScreen, a profile folder that cannot be written, or the processor being ARM) until their log says otherwise. A console build of both Windows programs and instructions went to the testers to get the message, and `.github/workflows/windows-smoke.yml` (run by hand, guard G-19) starts the published programs on GitHub's Windows runners. (Wine needed a clean environment: with the session's own variables it aborted at its start.) Run on GitHub's `windows-latest` runner on 2026-11-25 (guard G-19, the 0.50.0 page): the amd64 program started with `-version` and exited 0, so on a clean Windows machine the libraries are written out and loaded and the program reaches its options. What that does not show is a window: the runner has no screen or graphics card. The cause for the testers is therefore on their side of the start (the window and graphics driver, a profile folder that cannot be written, antivirus or SmartScreen, or an ARM processor) until their log says which. The windows/arm64 program also started with `-version` and exited 0 on `windows-11-arm` the same day, so the reading above, that its init panics for lack of an embedded libffi, was wrong or incomplete for the version in use: the arm64 program starts on a clean Windows ARM machine. It is not known why (not examined); the claim that arm64 cannot start is withdrawn.

Done when: the cause is found from a tester's log, fixed, and the program has been started by the testers or on a Windows machine with a screen; what ran, where and when is recorded here. (Started with `-version` on the GitHub runners, amd64 and arm64: 2026-11-25, G-19.)


### T-95. A Windows 7 program, in addition to the Windows 10 ones (needs a patched Go toolchain)

Theme: windows · Priority: P2 · Status: ◐

Trigger: 2026-11-25, a tester (Rodolfo) is on Windows 7. Go 1.21 and later need Windows 10 or Server 2016 or newer (go.dev/wiki/MinimumRequirements), and ZenZX is built with Go 1.26, so the Windows programs cannot start there; this very likely explains one of the two reports in T-92, and the testers' `ver` output will confirm it.

Scope: a separate Windows 7 program (windows/amd64 only: Windows 7 has no ARM64 and ZenZX has no 32-bit build), in addition to the Windows 10 programs. It needs a patched Go toolchain, because the standard runtime imports a function that Windows 7 lacks: `ProcessPrng` from bcryptprimitives.dll (checked on the 0.50.0 program, which has both names; the patched build has neither). `GetSystemTimePreciseAsFileTime` (from golang.org/x/sys) and the `Nt...WaitCompletionPacket` names (the runtime) are looked up at run time with a fallback and are in the patched build as well, so they are not what stops it; an earlier version of this item listed them as blockers. The libraries inside the program are not the problem: raylib.dll imports only KERNEL32, USER32, GDI32, WINMM and SHELL32, and libffi-8.dll only KERNEL32, none of them a function that Windows 7 lacks. Candidates: github.com/XTLS/go-win7 (patched source for 1.26, VERSION go1.26.11, release `current`, go-win7-win6-1.26.zip, SHA-256 0ad6b6a8c0da1e9854b26b1e83318fd0e95ef5362b7243efcd5065d4e8dbdbaa on 2026-11-25) and github.com/MetaCubeX/go (patches for 1.21 to 1.26). Both are third-party patches to the compiler and runtime, so using one means building a release program with a compiler other than Go's own; that trust is a decision for the tech team. Decision, 2026-11-25: the tech team accepts the patched toolchain (XTLS/go-win7, go-win7-win6-1.26.zip, SHA-256 above). Done, 2026-11-25: the toolchain built from that source on the Linux machine of the session (Go 1.24.7 as bootstrap, go1.26.11 out, 4 minutes); the 0.50.0 GUI program built with it (windows/amd64, the Release flags, 33 s) has no `ProcessPrng` and no `bcryptprimitives`, passes `tools/check_binaries.py --os windows --arch amd64 --kind gui --forbid ProcessPrng --forbid bcryptprimitives` (the standard build fails it), and started under Wine 9.0 set to Windows 7 (version 6.1) and showed the boot screen with the menu bar. `.github/workflows/windows7.yml` (called by release.yml after the release, or run by hand for a tag) builds the program the same way, checks it, makes `zenzx-<version>-windows7-amd64.zip` and a `.msi` of it, installs the .msi on a current Windows, and attaches both (guard G-21). The workflow ran in the Release run for v0.50.0 on 2026-11-25 and attached `zenzx-0.50.0-windows7-amd64.zip`, the `.msi` and the checksum file. The page lists the Windows 7 files from the next tag (`tools/build_site.py --extras`), and the manual's "Installing a release" covers them. Not done: the program the workflow builds has not been started on Windows 7. The tester reported success on a real Windows 7 machine on 2026-11-25 with the test build made on the session machine from the same source and toolchain (recorded only as "success"; Wine does not prove it, as it implements the functions that Windows 7 lacks).

Done when: a Windows 7 program is built in CI beside the others (named so that the page and the release say it is for Windows 7), checked for `ProcessPrng`, and has been started by a Windows 7 user, with what ran, where and when recorded here.
### T-97. Full screen has been seen only in a window on Linux with no window manager: not on Windows, macOS or under a window manager

Theme: windows · Priority: P3 · Status: ◐

- **Trigger:** 2026-11-25, the request: Windows has no full screen for a program that does not make one, and the window managers of macOS and Linux have theirs.
- **What was done:** `Alt+Enter` and View > Full Screen put the window, without its border, over the monitor it is on (raylib's `ToggleBorderlessWindowed`: `glfwSetWindowMonitor` at the monitor's own mode, so the desktop does not change resolution). The picture is the largest whole zoom (up to X 5) that fits, in the middle of the frame (T-75's path for a window of a size the system gave); a tiled layout is drawn to fill the window; the zoom the person chose is kept for the settings and comes back, and so does the window's size. Nothing asks a full screen window for a size: the single machine's snap back, the tiled drag snap, the animation, the zoom and the monitor clamp all stand aside. `TestGLFullscreen` (G-22) drives it in a window at 1920x1080 under Xvfb, single and tiled, in and out; `TestViewMenuFullScreenRow` and `TestFullscreenAsksForNoSizeAndKeepsTheChosenZoom` need no window. Builds for windows/amd64 and darwin/arm64 without cgo and passes `scripts/check_purego_ffi.py`.
- **What has not been seen:** a real Windows machine (the taskbar under the window, `Alt+Tab`, a second monitor, a display scaled to 125% or 150%, the window's size and place after leaving); a Mac (the borderless kind, over the menu bar and not macOS's own full screen); a Linux window manager, X11 and Wayland; tiled layouts of real machines in full screen; the exit when a window was maximised before. Also not decided: a picture that fills the screen at a zoom that is not whole (a 4K monitor shows X 5 with a wide frame) and a `-fullscreen` flag for starting in it.
- **Done when:** the Windows testers have used `Alt+Enter` and the menu on a program of the release, with a second monitor and a scaled display among what they try, and what they saw is reported (or filed as items of its own); and the choices above are made or dropped.

## packaging

### T-98. The Windows launchers of the examples and the Python helper have never been run on Windows; the documents job (manual PDF, examples zip) has not run on GitHub

Theme: packaging · Priority: P2 · Status: ◐

- **Trigger:** 2026-11-25, the request: the releases carry the manual as a PDF and the examples, with `.cmd` launchers for Windows and a way to get Python, or a warning with a link to it.
- **What was done:** `examples/launch.py` (finds ZenZX, offers to download it and checks it against the page's SHA-256, starts an example), `examples/{world/run.cmd,link/chat.cmd,next/run.cmd}`, `examples/tools/findpy.cmd` (the probe order is the private copy, `py -3`, `python`, `python3`, each started to see that it runs, which sets the Microsoft Store stub aside) and the general downloader that they call, `zenzx -fetch python` (`pkg/fetch`; it replaced `get_python.ps1`): python.org's embeddable Python 3.12.10, or 3.8.10 on Windows 7 and Vista, into `examples\python`, checked against a SHA-256, and on macOS and Linux a menu of a private python-build-standalone copy, Homebrew, apt, dnf or yum, or a uv or conda environment; shell launchers that name the package and the python.org page. `tools/build_manual.py`, `tools/build_examples.py` and `documents.yml`, the page's section, CI steps, Makefile targets. The tools and `launch.py` are tested on Linux (G-23).
- **What has not been seen:** Help > ZenZX Manual opening the PDF in a real reader, on Windows, macOS or a Linux desktop (raylib's opener is handed the path of the file); the real downloads from python.org and from python-build-standalone (the tests use a server of their own), and the downloader on a real Windows 7, whose root certificates may not include the authority that today's sites use, in which case Go's own TLS refuses them (only Wine's Windows 7 mode was seen to pick Python 3.8.10); `brew`, `uv`, `conda`, `mamba`, `apt-get`, `dnf` and `yum` run for real (fakes stood in, so the arguments are known but not what the real ones do); Windows itself (double-clicking from Explorer, SmartScreen, a path with spaces or non-ASCII characters, a console that closes); the macOS `.command` files and `findpy.sh`'s download of the real python-build-standalone archive on a Mac; the documents job on GitHub, whose files first appear with the next tag; the PDF opened in other readers. Seen under Wine (2026-11-25, clean environment: the sandbox's own makes Wine crash): `run.cmd`, `chat.cmd` and `next/run.cmd` with the real Windows Python 3.12.10 in `examples\python` start `launch.py`, which starts a stand-in `zenzx.exe` with the right arguments and downloads one from a page by `--yes`; with no Python the prompt is shown, Q quits, and a failed download is reported and waits for a key. Also under Wine: `findpy.cmd` with the real ZenZX built for Windows (the console build, and the windowed one that a batch file does not wait for by itself) fetched a stand-in Python from a local server, checked, 3.8.10 in Wine's Windows 7 mode, refused a wrong SHA-256 and reported it, and found an environment made in `python\Scripts`.
- **Done when:** a person has unpacked the examples zip on a Windows 10 or 11 computer with no Python and started the three examples by double-click (choosing the download once, and the python.org page once), on Windows 7 for one, and the documents job has run for a tag and its three files are on the release and the page. What was seen is filed, or fixed.

### T-93. Installers: .deb (amd64, arm64), .msi (amd64, arm64) and .dmg with a ZenZX.app, built in the release workflow

Theme: packaging · Priority: P3 · Status: ◐

Trigger: 2026-11-25, after the Windows start-up reports (T-92): people who are not comfortable with a bare executable need a normal way to install the emulator.

Scope: an installer for each system, built in the release workflow from the programs it already makes, listed on the page and attached to the release beside the raw programs (which stay: they are what `runners.json`, the smoke workflow and the export stubs use).

- Linux, `.deb` for amd64 and arm64: `dpkg-deb` on the Linux runners; the program in `/usr/bin`, a `.desktop` file, an icon, MIME types for the machine files (.z80, .sna, .tap, .tzx, .nex and the like), `Depends:` on the libraries the program loads (ALSA, X11) and glibc 2.38 or later, which `check_binaries.py` already measures. Check: install in a clean Ubuntu and a clean Debian container, run `-version`, remove, confirm nothing is left.
- Windows, `.msi` for amd64 and arm64: WiX 4 (a dotnet tool, runs on `windows-latest` and `windows-11-arm`); Start Menu shortcut, file associations, an uninstaller, a per-user install by default so that it needs no administrator. Check: `msiexec /i /qn`, start `-version` from the install folder, `msiexec /x`, list what is left. The program stays unsigned until a certificate exists, so SmartScreen warns about the MSI as it does about the program.
- macOS, `.dmg` holding a `ZenZX.app` (Info.plist, an `.icns` icon, document types), universal when the two builds are joined with `lipo`; `hdiutil` on the macOS runner. A drag-to-Applications image, not a package. Check: mount, copy, start `-version` from inside the bundle. Without notarisation Gatekeeper blocks the app on first start (right-click Open), and this needs a Mac to see (T-89).

Departures from the scope above, 2026-11-25 (what was built): the `.deb` is written by `tools/package.py` with the standard library (an ar archive of two tars), not with `dpkg-deb`, so that it can be made and tested on any machine; the `.msi` is built with WiX 3.14 (MS-RL, a pinned download checked by SHA-256), not WiX 4, because 4 and later carry the open-source maintenance fee terms and because 3.14 has an ARM64 target (`wixl`, the free MSI builder, has none); there are no file associations or MIME types in any of the three, because ZenZX ignores a file named on its command line (T-94) and an association would start the emulator with nothing in it. The installers are made by `.github/workflows/installers.yml` from the release's own archives, after the release is published (called from release.yml, or run by hand for an existing tag), and are attached only when all of them have passed; a failure leaves the release untouched. Written and tested here: `tools/package.py`, `scripts/test_package.py` (a CI step), the `.deb` installed, run and removed in a Linux container. Run since, 2026-11-25: the workflow ran in the Release run for v0.50.0 and attached all five installers (the `.msi` built with WiX on the Windows runners, the `.dmg` on macOS); the attach job uploads only when each was installed or mounted and started on a runner of its own system. The job logs were not read here and nobody has installed one on a real machine. Done since, 2026-11-25: the page lists the installers and the Windows 7 files with links to them on the release (`tools/build_site.py --extras`, the `pages` job of `release.yml`; it shows from the next tag), and the manual says how to install each ("Installing a release"). Not done: no signing (SmartScreen and Gatekeeper warn).

The icon is the Spectrum in `assets/icon/` (`speccy01.zani`), already made as `.ico`, `.icns` and square PNGs from 64 to 1024 pixels, enlarged by whole numbers with no smoothing; the installers use those files and nothing is to be redrawn. Not decided by the tech team and needed before installers are worth shipping to people who avoid warnings: Authenticode signing for Windows, and an Apple Developer ID with notarisation for macOS. Both have a yearly cost and an identity attached. The `.deb` needs neither.

Order: `.deb` first (no signing, cheapest to check), then `.msi`, then `.dmg`. None of it changes the programs, so it does not wait for T-92. It does not fix T-92 either: the programs start on clean Windows machines (G-19), and an installer does not change where the libraries are written or what SmartScreen thinks of an unsigned file.

Done when: each installer is built in the release workflow, installed and removed in a check job on its own system, attached to the release and linked from the page, and the manual says how to install each; what ran where is recorded here.


## release

### T-108. ZenZX needs zenui v0.4.0 published: it builds only through a local go.work until then

Theme: release · Priority: P1 · Status: ☐

- **Trigger:** 2026-11-27, adopting zenui 0.4.0 for T-104, T-106 and T-107.
- **What it does now:** `go.mod` requires `github.com/ha1tch/zenui v0.4.0`, which is not tagged or published yet (zenui's release is waiting on its provenance gate, zenui T-09 to T-11 are closed in 0.4.0). The tree builds and passes its suites only through a local `go.work` (`use .`, `replace github.com/ha1tch/zenui v0.4.0 => ../zenui`), which the archive leaves out. `go.sum` has no entry for v0.4.0.
- **What is missing:** zenui v0.4.0 released and pushed; `go mod tidy` for the `go.sum` lines; the `go.work` removed; the headless and GUI suites, `check_gofmt.sh` and `repoman gomod check` run again from the published module.
- **Done when:** with no `go.work`, `go build`, `go vet` and both suites pass against the published v0.4.0, and `go.sum` is committed with it.


---
