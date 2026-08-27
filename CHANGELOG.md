# Changelog

All notable changes to ZenZX are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [0.6.7] - 2026-08-27

### Fixed

- go.mod required v0.1.0/v0.2.0 while replace pointed local checkouts
  at v0.5.1/v0.6.0 -- fixed to match. Replaces still needed until both
  are pushed.

## [0.6.6] - 2026-08-27

### Fixed

- **T-25 -- GUI build's `-tapemode` never accepted `turbo`** (see
  RESOLVED.md): `zenzx_gui.go`'s flag parsing only ever recognised
  `accurate`, silently falling back to `fast` for anything else,
  including `turbo`. Wired up to match `zenzx_headless.go`'s already-
  correct pattern exactly: a proper three-way `switch` on
  `TapeAccurate`/`TapeTurbo`/`TapeFast`, and the main loop now calls
  `zx.RunTurboAwareFrame(blockedOnRealMemory)` instead of a bare
  `RunFrame()` -- the same function every one of this session's own
  turbo-mode correctness work (T-19's 128K fix, the Chase H.Q.
  investigation) already exercised and validated via the headless
  build, now reachable from the GUI too. One real, user-visible
  consequence of turbo's existing (headless-proven) design, not a new
  limitation: the GUI will visibly freeze during a turbo-mode load
  rather than show incremental loading stripes, then jump to the
  final state once the tape finishes -- turbo's screen buffer is
  frozen throughout the fast path by design, documented already in
  `docs/TAPE_LOADING_HANDOVER.md`.

## [0.6.5] - 2026-08-27

### Changed

- Filed T-25 (GUI build's `-tapemode` never gained a `turbo` case;
  silently falls back to `fast` rather than erroring) -- the defect
  0.6.4's documentation fix surfaced but didn't itself resolve.

## [0.6.4] - 2026-08-27

### Fixed

- **README.md and docs/MANUAL.md were stale relative to shipped
  behaviour, in ways that stated things plainly wrong, not just
  outdated:**
  - Both claimed memory contention "affects every model" (i.e.
    doesn't exist anywhere) -- true when T-16 was open, false since
    0.6.0. Corrected to the real current state: precise for 48K
    (~0.33% memory / ~0% I/O measured position-error bounds), an
    address-range approximation for 128K (T-20, open).
  - `-tapemode`'s documentation listed only `fast`/`accurate` in both
    files, omitting `turbo` entirely despite it being a shipped,
    tested feature since before this changelog's visible history.
  - Investigating the fix surfaced a real, previously undocumented
    asymmetry: **`turbo` mode only works in the headless build.**
    `zenzx_gui.go`'s own flag parsing never gained a `turbo` case --
    passing `-tapemode=turbo` to the GUI silently falls back to
    `fast`. Documented accurately in both files rather than papered
    over; not fixed here, since wiring it up is a code change beyond
    a documentation pass.
  - Neither file mentioned `contrib/tape-corpus-harness/` or
    `docs/TAPE_LOADING_HANDOVER.md`, both new this cycle. Added to
    Project Layout and Known Issues in both README.md and MANUAL.md,
    including T-22/T-23/T-24 as headline items -- T-24 in particular
    is the single highest-priority open item in the whole register.

## [0.6.3] - 2026-08-27

### Changed

- **`docs/TAPE_LOADING_HANDOVER.md` updated for the state actually
  shipped in 0.6.2**: the corpus harness section now reflects that
  `contrib/tape-corpus-harness/` is built, run, and verified end to
  end (two real bugs found and fixed doing so -- `-shot-interval`
  being a no-op once a script drives the run, and missing required
  frame offsets on script action lines), not merely designed. The
  harness's variant-picking limitation (picks a tape alphabetically,
  doesn't know if it's a working one) is now stated in the handover
  narrative itself, not only the harness's own README, since it
  directly bears on how much to trust any corpus-wide number the
  tool produces today.

## [0.6.2] - 2026-08-27

### Added

- **`docs/TAPE_LOADING_HANDOVER.md`**: a synthesis of the multi-day
  tape-loading investigation (fast/turbo/accurate mode characteristics,
  measured corpus results, and open questions), written for whoever
  picks this work up next. Two new register items came out of it:
  T-23 (fast mode's ROM-trap only accelerates the standard LD-BYTES
  routine -- no benefit once a custom/protected loader takes over,
  confirmed with the Chase H.Q. corpus) and T-24 (no mode's
  self-reported completion signal has proven trustworthy on its own --
  turbo, fast, and the corpus report's own heuristic have each been
  observed to report success on a load that was actually wrong).
- **`contrib/tape-corpus-harness/`**: a standalone tool for measuring
  real-world tape-loading coverage and speed, in its own Go module
  with no import relationship to zenzx in either direction (`go list
  ./...` from the repo root does not see it). Downloads its own corpus
  at runtime from a public preservation source
  (archive.org/details/zxspectrum-top-100 -- the same collection
  `cmd/tapereport` already pointed at) rather than assuming one is
  present, and drives `zenzx-headless` entirely as a subprocess via
  `-script`, with no access to internals. Replaces earlier ad hoc
  investigation code that lived in the main package and assumed a
  pre-existing local corpus directory -- that code is removed; nothing
  depending on third-party game content ships as part of zenzx itself
  or its test suite. See that directory's own README for usage and
  the honest limits of its screenshot-diffing completion signal.

## [0.6.1] - 2026-08-26

### Added

- **Permanent cross-emulator trace instrumentation**: `TestTraceHarness`
  (`zzz_trace_harness_test.go`) and `TestPulseDump`
  (`zzz_pulsedump_test.go`), both opt-in and skipped by default. The
  harness records PC/register sequences, memory-write deltas
  (address/old/new), and port-read results against zen80's
  `DebugPCHook`/`DebugMemWriteHook`/`DebugIOInHook`, all filterable by
  step range, PC range, and writing-PC, so a divergence against a
  reference emulator or an independently generated pulse stream can be
  localized to the instruction without ad hoc instrumentation each
  time.

### Fixed

- **A trailing TZX pause could be encoded with an inverted excursion
  and a parity shift**: `genCustomPulses` inlined a pre-fix pause
  pattern that an earlier pause-level fix had missed at this one call
  site, producing a 1ms HIGH excursion where the spec requires LOW and
  flipping the entry level of everything downstream. Verified fixed
  by an independent, from-scratch TZX-spec pulse generator: the
  generated stream now matches expectation pulse-for-pulse across a
  full 828,282-pulse tape, zero mismatches.
- **A zero-length pulse (a pure level flip, no duration) could be
  deferred past a `Tick` call's cycle budget**, leaving its transient
  level observable for one instruction on the next `Tick` before
  collapsing. Zero-length pulses now collapse immediately regardless
  of remaining budget.

## [0.6.0] - 2026-08-25

### Added

- **ULA contention modeling, 48K precise** (closes T-16 -- see
  RESOLVED.md for what was wrong and how it was resolved). contention.go
  implements the WoS FAQ model: memory pattern {6,5,4,3,2,1,0,0} from
  cycle 14335 over 0x4000-0x7FFF, and I/O per the FAQ port table with
  the "Yes" rows as four cascaded C:1 rounds (each round's delay shifts
  the next check's position). Wired through zen80 0.5.0's new
  offset-corrected contention hooks; measured position-error bounds
  ~0.33% (memory) and ~0% (I/O) on real Speedlock-loader execution. The
  128K variant is a documented address-range approximation (T-20).
- **Floating bus** (floatingbus.go): unattached port reads return the
  byte the ULA fetches at the current frame-relative T-state, mirroring
  FUSE's spectrum_unattached_port; idle 0xFF outside fetch cycles;
  gated off for +2A/+3 (no bus there) and TS2068. Granularity and 128K
  caveats filed as T-21.
- **EAR idle feedback** (io.go): with the tape not playing, bit 6 of a
  ULA-port read presents the level fed back from the last ULA OUT --
  Issue 3 (default): OUT bit 4; Issue 2 (new `issue2` flag): bits 4|3;
  +2A/+3: always low. Speedlock-class hardware checks probe exactly
  this. While the tape plays, it drives bit 6 as before.

### Changed

- **Coherent frame timebase**: /INT is asserted at the START of each
  frame for 32 T-states (`InterruptLength`), the PAL frame is the real
  69888 cycles (`ULAFrameCycles`), and a true per-frame origin
  (`frameOrigin`) anchors contention, the floating bus, border
  striping, and snapshot restore. Previously RunFrame ran 70000-cycle
  frames with INT at the END while contention wrapped the cumulative
  counter mod 69888 -- two timebases precessing 112+ T-states per
  frame, leaving the contention window at a continuously rotating
  phase. TS2068 keeps its 58688 frame with the shared pulse length.
- **Test baselines updated for deliberate behaviour changes**: eight
  joystick-test constants (ULA idle now includes bit 5, hardware's
  0xBF); ts2068 timing assertions (per-model end-of-frame threshold no
  longer exists); five tape tests rewritten from pulse-shape
  assertions to semantic ones (pause levels, durations, stop timing,
  next-block-starts-LOW parity).

### Fixed

- **TZX pause blocks now sit LOW per the spec** (and libspectrum's
  END_OF_BLOCK_NEXT_LOW): hold the entry level ~1ms, LOW for the
  remainder, next block starts LOW -- with the entry level derived
  from stream parity and a zero-length flip restoring parity.
  Measured on Batman's stream, the previous SpecIde-shape encoding
  played four of five long pauses entirely HIGH, including the 3587ms
  pause at the exact tape position where its loader was first observed
  stalling. (Not, in the end, that title's root cause -- T-22 tracks
  the continuing investigation -- but a genuine spec violation.)
- **ULA port reads: bits 5 and 7 now read high** (idle 0xBF), matching
  real hardware and FUSE; bit 5 previously read low.

### Release notes

- Dormant guard G-02 (FDC read against a real DSK image) skipped this
  release: its gating image (/mnt/user-data/uploads/artist.dsk, see
  T-05) is absent from this environment. G-01 (GUI link build) is
  exercised by this release's gui-check step.
- The release test gate now excludes the 31-game corpus harness
  (`TestBatchThreeModeCompare`) by name in the manifest: it is
  investigation tooling run in its own batched campaigns, not a unit
  gate, and its ~6-minute runtime was stalling releases. The 135 unit
  tests remain the gate.

## [0.5.1] - 2026-08-24

### Fixed

- **T-19: Turbo tape mode failed on 128K+ titles** (see RESOLVED.md for
  full detail). Root cause: the flat, block-boundary-synced 64K memory
  snapshot could not represent a game that actively pages RAM/ROM banks
  via port 0x7FFD *during ongoing code execution*, not just between
  tape blocks -- confirmed with Cybernoid 2 (47 paging writes in the
  first 4000 turbo frames alone). Replaced with a page-based model
  matching SpectrumMemory's own real layout (12 independently-addressable
  16K pages: 4 ROM banks + 8 RAM banks), synced immediately on every
  paging port write via a new zen80 hook (`FastPortWriteOut`, mirroring
  the existing `FastPortReadIn`) rather than only at block boundaries.
  Verified: Cybernoid 2 now reaches real game code and produces a
  screenshot pixel-identical to accurate mode's own ground truth, via
  the real CLI end to end; Chase HQ (48K) re-verified correct after the
  rewrite. Scope: standard 128K/+2/+3 paging only, not +3's special
  all-RAM mode or TS2068's separate Extension-ROM scheme (both rare for
  tape-based titles).

## [0.5.0] - 2026-08-24

### Added

- **Full TZX block coverage** via zentools' now-complete 25-block-type
  `pkg/tzx`/`pkg/tap` (see zentools 0.6.0): `loadTAP`/`loadTZX` migrated
  to delegate all structural decoding to `tap.Decode()`/`tzx.Decode()`
  rather than zenzx's own, previously partial parsing. `TapeBlock`
  extended with `HeaderType`/`HeaderName`/`DataLength`, populated at
  load time for both `.tap` files and TZX 0x10 blocks via
  `tap.DecodeBlock()`.
- **0x11/0x12/0x13/0x14 wiring**, both fast-load (`Blocks[]`, for
  trapLoad's ROM-address interception) and accurate-mode playback
  (`Pulses[]`, including `genCustomPulses` for 0x11/0x14's per-block
  custom pilot/sync/data timing). **0x2A Stop-The-Tape-If-In-48K**
  wired (`StopPointsIf48K`, checked conditionally on `!is128K`).
- **`cmd/tapereport`**: an HTML tape-compatibility report generator
  (ZSP design system, embedded ZX-Spectrum-style font) -- run against
  the newdiv corpus (303 games, 413 TZX files): 100% fully playable
  (accurate mode) and 100% fully fast-loadable, both figures confirmed
  non-vacuous by deliberately removing 0x2A from the wiring map and
  observing the playable figure correctly drop to 96.9%.
- **`wait-tape-done` zenscript verb**: blocks a script until
  `!zx.tape.st.Playing` or a timeout, falling through either way.
- **Turbo tape mode** (`-tapemode=turbo`): identical correctness to
  accurate mode's real pulse-level EAR-pin emulation (unlike fast
  mode's ROM-trap, which only accelerates while the ROM's own
  `LD-BYTES` is running -- ineffective the moment a game's own custom
  loader takes over, which is common). Two independent layers:
  - **`loadingFrame`**: a leaner `RunFrame` used only while a tape is
    actively playing in Turbo mode -- identical CPU/tape stepping, with
    per-instruction AMX-mouse/audio bookkeeping and per-frame
    border-history rendering skipped (irrelevant while nothing is
    rendering intermediate frames anyway). ~20-27% faster on its own,
    zero correctness risk since nothing is semantically shortened, only
    bookkeeping that has no effect on CPU or tape state is skipped.
  - **zen80's `FastMem`/`FastPort`** (see zen80 0.3.0): a flat 64K
    scratch memory/port array zenzx snapshots real `SpectrumMemory`
    into (`turboSnapshotIn`), runs the CPU against directly (avoiding
    interface-dispatch overhead -- ~3.6x on memory access alone), and
    reconciles back per-block (`turboSyncBlock`, keyed off `loadTZX`'s
    new `BlockBoundaries` tracking) rather than only at the very end --
    correct for the overwhelmingly common case of a 128K loader
    switching RAM banks between blocks, not mid-block. A `FastPortReadIn`
    hook supplies the ULA's live keyboard+EAR-bit port value (which a
    static array can't cheaply represent, given how often the EAR bit
    toggles) by reading `Tape.EarLevel` directly on the rare event (an
    actual `IN`), not the frequent one (an EAR transition).
    `Scheduler.BlockingOnRealMemory()` pauses the fast path whenever a
    barrier that reads real screen/memory content (`wait-boot`,
    `wait-screen`, `wait-attr`, `wait-mem`) is currently active, since
    such a barrier has no way to observe progress in the fast path's
    own unsynced scratch memory.
  - **Confirmed correct for 48K** (`Chase HQ`, an Ocean/"Paul Owens
    loader core" title -- a documented, one-byte-different variant of
    the standard ROM's own bit-sampling routine, confirmed via
    <https://sinclair.wiki.zxnet.co.uk/wiki/Loading_routine_%22cores%22>):
    byte-identical outcome to accurate mode via both direct screenshot
    comparison and a full memory diff at the same checkpoint, at a
    measured ~17-27% wall-clock speedup over accurate mode across
    several runs.
  - **128K known broken -- filed as T-19**, not yet root-caused. Do not
    ship `-tapemode=turbo` for 128K+ titles until resolved; 48K is safe.

### Fixed

- **TAP inter-block pause**: was a flat `pause × 3500` T-states between
  blocks; corrected to the proper 3-pulse `addTZXPause()` pattern
  (`{945, 3500, 3500×ms}`), matching both SpecIDE's and libspectrum's
  independent implementations, which both insert this exact pattern
  after every TAP block.
- **`EnqueueText`'s K-mode keyword typing**: was typing keywords
  letter-by-letter even in 48K K-mode, where a keyword should be a
  single keypress (`LOAD` -> J). `keywordKeys` map added, currently
  covering `LOAD`; extensible for others as needed.
- Five zentools `pkg/tzx` bounds-checking gaps (see zentools 0.6.0)
  found via corpus/fixture testing during this work; fixed upstream.

### Removed

- Seven leaked Amstrad CPC ROM files from `rom/` (unrelated to any
  Spectrum model this project targets).

## [0.4.65] - 2026-08-21

### Added

- **`go-sanity` release gate**, wired into `.repoman.json` right after
  `register` and before the more expensive `gui-check`/`build`/`test`
  steps: `python3 repoman/gomod.py check` fails the release if `go.mod`
  carries a `replace` directive pointing at an absolute local filesystem
  path (the exact shape of a real incident on another project -- a
  hardcoded sandbox path reached a downstream team's committed go.mod),
  and separately confirms `go.sum` is complete enough for a plain `go
  build -mod=readonly` with no extra magic. Verified against this
  repository's own `go.mod`/`go.sum`: `GOMOD CHECK OK`. `repoman/`
  synced to 0.8.0, which introduces this tool.

## [0.4.64] - 2026-08-21

### Added

- **Memory assertions in zenscript** (`wait-mem`, `expect-mem`) and a new
  **`sym`** verb to support them: `sym <path>` loads a pasmo/zenas-format
  `.sym` file into a name -> address table; `wait-mem <symbol|addr> <op>
  <value> [timeout=...]` blocks the timeline until a live memory value
  satisfies the comparison (`= != < > <= >=`), then rebases, exactly the
  same block-and-rebase contract as `wait-screen`/`wait-attr`; `expect-mem`
  is the non-blocking instant-assert counterpart to `expect-screen`. Symbol
  resolution falls back to a literal address when no table is loaded or the
  name isn't found, so both verbs work with or without `sym`. This closes
  the gap the "Recipes: probing machine state" section of `docs/zenscript.md`
  previously worked around with a snapshot-save-plus-manual-byte-offset
  dance -- that recipe remains documented for whole-region dumps, but a
  single named value is now one line: `wait-mem cur_map != 0 timeout=10s`.
- **`shot`'s new `zoom=N` argument** (1-16, default 1): upscales a capture
  by an integer factor via nearest-neighbour sampling before writing the
  PNG -- hard pixel edges, no blending, for inspecting sprite/tile detail
  too small to read at native 256x192.
- **`docs/MANUAL.md`**: a new organised reference (18 sections, full CLI
  flag tables for both binaries pulled directly from `flag.String`/
  `flag.Bool` source rather than transcribed from prose) complementing
  `README.md`'s narrative account. Linked from the README's own top.

Verified end-to-end against a real build (`oakhollow.bin`, not a synthetic
fixture): a `.zen` script loaded 365 symbols via `sym`, `wait-mem kn_x >
120` correctly matched and rebased at frame 113, and a separate `wait-mem
cur_map != 0` correctly timed out and logged clearly rather than hanging
when genuine tree collision blocked the driven character's path -- both
the match and the timeout path exercised, not just the happy case. `shot
... zoom=6` produced pixel-exact nearest-neighbour captures confirmed
against the source framebuffer. `go build`, `go vet -tags headless .`, and
`repoman/selftest.py` (64/64) all pass. `gofmt` flags no new violations
introduced by this change (the pre-existing `scheduler.go`/`script.go`
non-conformance is T-06, already on the register).

### Fixed

- **`relcore.py`'s archive step could write a duplicate `MANIFEST.sha256`
  entry** into the checkpoint zip: the exclude list never treated the
  manifest itself as reserved packaging output, so a working tree that
  already had one on disk (e.g. extracted from a prior checkpoint) got it
  swept in as an ordinary source file, then overwritten a second time by
  the builtin's own fresh manifest -- two same-named entries in one zip,
  found via this release's own packaging run. Added `MANIFEST.sha256` to
  `release.archive.exclude` in `.repoman.json`, alongside the other
  packaging-output exclusions it already lists (`.release-state.json`,
  `release-*.log`). Verified: a re-run of the archive step now produces
  exactly one `MANIFEST.sha256` entry.

### Known gaps carried into this release

- **G-01** (GUI link build) and **G-02** (FDC read against a real DSK
  image) are stale as of this release -- neither exercised, since this
  session's sandbox has no CGO/raylib/ALSA toolchain and no fixture DSK
  image available, and neither guard's subject matter (GUI linking, disk
  I/O) was touched by this release's changes. Recorded here rather than
  silently skipped, per the release gate: run `./build.sh` (G-01) and
  `go test ./... -run TestFDCRead` (G-02, see T-05 for its own sandbox-path
  caveat) to close them out.

## [0.4.63] - 2026-08-19

### Changed

- **The "Spectrum 128" theme is "Spectrum" now**, identifiers and
  display name both: `zenui.ThemeSpectrum128` -> `zenui.ThemeSpectrum`
  (constant value `"Spectrum 128"` -> `"Spectrum"`), `Spectrum128Theme()`
  -> `SpectrumTheme()` (matching `DarkTheme`/`LightTheme`'s own naming
  exactly). Applied everywhere the theme is actually named -- Go
  identifiers throughout `pkg/zenui` and every call site, the
  `-theme` CLI flag's accepted values and usage text, `settings.json`'s
  own schema and stored value, README and Help's own descriptions --
  while deliberately leaving untouched every reference to the real
  Sinclair 128K/+2/+3 hardware that inspired the theme (its own doc
  comment, "Spectrum 128K" snapshot-format labels, "ZX Spectrum 128k"
  Machine-menu model labels): those describe real machines, not the
  theme's own name, and renaming the theme doesn't change what
  hardware it's modelled on. `CHANGELOG.md`'s own history is
  deliberately not rewritten -- past entries correctly describe what
  the theme was called at the time.
- **`-theme=Spectrum 128` (the old, space-containing form) is no
  longer recognised** -- a genuine, deliberate behaviour change, not
  merely updated documentation: it now falls back to Dark with a
  warning, the same as any other unrecognised value, rather than
  still resolving to the Spectrum theme. Caught because a test
  asserting the opposite would have actually failed if left as-is
  (`normalize` strips spaces before comparing, so `"Spectrum 128"`
  normalises to `"spectrum128"`, which no longer matches `"spectrum"`)
  -- confirmed with a dedicated regression test instead of just
  deleting the now-wrong assertion silently.
- Also added, noticed missing while touching this area: `help.txt`
  never documented the `-settings` flag from the previous release.
  Added.

### Fixed

- **A duplicated `[0.4.62]` changelog entry**, found while preparing
  this release: an earlier, truncated session's own account of its
  `machines.json` work had already been committed to this file before
  this session discovered and built on top of it, and a second,
  independently-written `[0.4.62]` entry got appended later without
  checking for the first. Consolidated into one accurate entry with
  verified (not merely asserted) test counts, rather than left as two
  overlapping, partially-double-counted accounts of the same release.

76 substitutions applied via a scripted, ordered rename (most-specific
pattern first, to avoid `ThemeSpectrum128`/`Spectrum128Theme` partial-
matching each other) across 13 files, verified with a full build and
test pass immediately after. 1 new regression test confirms the old
`"Spectrum 128"` input form now correctly falls back to Dark instead
of silently continuing to resolve to the renamed theme; 2 test cases
for the old form's previous behaviour removed from `TestParseThemeFlag`
since that behaviour no longer exists to test.

## [0.4.62] - 2026-08-19

### Added

- **The Machine menu's layout is configurable via `machines.json`**,
  not hardcoded -- a flat list of `separator`/`title`/`model`/
  `submenu` nodes (new `pkg/machineconfig` package), letting groups,
  sub-headings, and one level of real hover-opened submenu be
  arranged however wanted (`submenu` capped at one level of nesting --
  matching what `zenui.MenuBarSelection`'s own `SubIndex` can actually
  report back, a second level would have nowhere to report through).
  Every `model` node's own `id` must be one of the fixed `-model`
  identifiers, and every one of them must appear exactly once across
  the whole file -- structural shape validated against a
  [queryfy](https://github.com/ha1tch/queryfy) schema (new dependency,
  v0.3.2, fetched via the established offline module-proxy procedure
  and confirmed working with a standalone test binary before touching
  the real project); a separate completeness check (a whole-document
  relationship queryfy's own per-node schema can't express) confirms
  the identifier coverage -- together, this is what "rearrange freely,
  but the identifiers stay the same" actually means enforced, catching
  a duplicated or accidentally-omitted model before either becomes a
  confusing runtime symptom. A copy is embedded in the binary as the
  default; a valid `machines.json` next to the executable overrides it
  entirely, an invalid one is reported and skipped in favour of the
  embedded copy rather than treated as fatal (a broken *embedded*
  default fails loudly instead, since that would be a bug in the
  binary itself, not a user configuration problem). `machines.json` at
  the project root is the real default configuration, replicating the
  existing Sinclair/Amstrad/En Español/Timex grouping exactly.
- `menubar_gui.go`'s Machine menu construction rebuilt around this:
  new recursive `buildMachineNodes` (walks a `machineconfig.Node` tree
  into parallel `zenui.Item`/flag slices, recursing once into any
  `submenu` node's own items), new `machineSubmenuFlags` field for
  submenu-level dispatch (`sel.SubIndex` resolves through it), and the
  Machine dispatch logic extended to handle a real submenu selection,
  not just the flat-list case every other node type still produces.
- **Theme, font, font zoom, display scale, and the fixed-menu-bar
  state persist across sessions to `settings.json`** (new
  `pkg/settingsconfig` package; same embedded-default-with-disk-
  override pattern, also queryfy-validated), written atomically (temp
  file + rename) every time one of them changes via a menu. New
  `-settings=PATH` flag points at a different file. An explicitly
  passed `-theme` or `-scale` flag always wins outright over whatever
  settings.json has saved; leave either unset and the persisted value
  is used instead of that flag's own hardcoded default -- resolved via
  two new, deliberately pure and independently testable helpers
  (`resolveExplicitOrPersisted`/`Int`) rather than left as untestable
  logic inline in `main()`. The running `-model` and FPS/border
  visibility are deliberately not persisted (an explicit model should
  never be silently overridden by a stale save; FPS/border are
  session-level diagnostic toggles, not the kind of preference most
  applications carry across restarts).

### Changed

- **Replaced an earlier, more limited implementation of the Machine-
  menu-configuration feature**, discovered mid-session (a
  `groups`/`subgroups`/`models` schema, already using queryfy and
  already wired in, with its own tests): that schema only supported
  "flat models OR flat subgroups" per group, with no real hover-opened
  submenu capability. `pkg/machineconfig`'s node-tree schema supports
  separator/title/model/submenu as independently combinable options in
  any order, which more literally delivers "arrange however you want,
  including submenus, separators, and titles" as three genuinely
  separate, freely-combinable choices rather than two fixed shapes.
  `machines.json` itself was rewritten to the new schema, replicating
  the exact same grouping and labelling as before -- an internal
  representation change, not a menu redesign.

### Fixed

- **`newAppMenuBar`'s font checkmark was still hardcoded to Sinclair**
  regardless of which font actually got loaded at construction --
  caught while wiring settings.json's own initial font through: a
  non-Sinclair starting font (from a loaded `settings.json`) would
  load and render correctly but show the checkmark on the wrong menu
  item. Fixed to check against the actual resolved starting font.
- **A theme-name inconsistency in `pkg/settingsconfig`'s own test
  fixtures**: `"Spectrum128"` (no space) didn't match
  `zenui.ThemeSpectrum128`'s real constant value, `"Spectrum 128"`
  (with a space) -- not a functional bug (the fixtures were
  self-consistent against their own local, synthetic valid-themes
  list), but misleading for anyone reading the tests later. Corrected
  in the fixtures and in this session's own new test that hit the same
  assumption.

42 new tests: 14 in `pkg/machineconfig` (valid config, malformed JSON,
unknown node type, unknown model id, missing required fields, rejected
double-nesting, missing/duplicate model detection including inside a
submenu, and all four `Load` disk/embedded/fallback paths); 16 in
`pkg/settingsconfig` (already complete when discovered mid-session and
re-verified here rather than rewritten: the same shape of coverage
plus atomic-save round-tripping and overwrite behaviour); 2 in
`menubar_checkmarks_test.go` confirming `buildMachineNodes` and the
dispatch logic both handle a real submenu correctly end-to-end (a path
the default `machines.json` itself doesn't exercise, since it has no
submenu nodes); 5 covering the initial-settings-driven construction
path (including the font-checkmark regression above) and
`saveSettings`'s no-op-without-a-path and actually-persists-current-
state behaviour; 5 for the CLI precedence helpers' own cases. 3
existing Machine-menu-structure tests rewritten against the new
node-tree schema (one of them had silently referenced an undeclared
variable and would never have compiled).

## [0.4.61] - 2026-08-19

### Fixed

- **Historical correction: the Machine menu's Spanish-market group no
  longer attributes all four models to Investrónica.** It didn't make
  all of them. Researched properly rather than assumed: the 48k is
  Sinclair's own design, merely distributed and locally assembled/
  localised in Spain by Investrónica; the 128k was genuinely designed
  by Investrónica itself (with Sinclair's permission), launched in
  Spain months before the UK got it; the +2 and +3 are Amstrad's, the
  +2 specifically "the first computer launched under the new owner"
  after Amstrad's 1986 acquisition of Sinclair. The group's own title
  is "En Español" now, not "Investronica S.A. Spain", and each model
  sits under its own indented sub-heading naming its actual maker
  (Sinclair, Investrónica, or Amstrad) -- the one group in this menu
  that spans three manufacturers rather than one. New
  `machineSubManufacturer` type and a `SubGroups` field alongside
  `machineModelGroups`' existing `Models` (exactly one of the two set
  per group; every other group -- Sinclair Research Ltd, Amstrad plc,
  Timex -- still uses the flat `Models` list unchanged), with the
  construction loop and Machine-menu dispatch both extended to walk
  either shape.

5 new tests: each Spanish model is attributed to its correct actual
manufacturer (not just to *a* manufacturer), the three sub-manufacturer
headings render as indented Title items, the old inaccurate group
title no longer appears anywhere in the menu, plus the existing
structure and model-coverage tests extended to walk both the flat and
nested group shapes rather than assuming every group is flat.

## [0.4.60] - 2026-08-19

### Changed

- **The About box sizes its own window height to fit its content**,
  rather than always filling most of the screen the way Help still
  does (Help always needs scrolling regardless of screen size, so
  screen-filling gives it the most room; About is short and fixed-
  length, where that would leave most of the panel empty). New
  `markdownModal.autoHeight bool` (a third `newMarkdownModal`
  parameter, `true` for About, `false` for Help): the panel height
  becomes an estimate -- line count times the body's own line height,
  plus title and padding -- rather than `screenH - 4*pad`. Estimate,
  not a guaranteed exact fit: correct as far as this layout already
  goes (every other size here is already line-count * line-height
  derived too), but real text rendering could in principle wrap or
  round slightly differently. Still capped at the screen height and
  floored at the same minimum Help already has, for the pathological
  case of content longer than the screen.

6 new tests: an auto-height panel is shorter than a full-height one
and doesn't fill the screen, its height matches the content estimate
exactly, it never needs scrolling (visible >= total), it respects the
minimum-height floor for very short content, it's still capped at the
screen height for content too long to fit, and Help's own layout is
completely unchanged by any of this. Plus 2 existing tests extended
to confirm the right autoHeight value is actually wired through
`handleLogoMenuResult` for each of Help and About.

## [0.4.59] - 2026-08-19

### Changed

- **Help/About: client area 85% opaque, backdrop 20% less dark, text
  and widgets stay fully opaque.** New `scaleAlpha` helper and
  `modalPanelOpacity`/`modalBackdropDarkening` constants in
  `markdownmodal_gui.go`, applied only to this modal's own panel and
  backdrop fills -- not a change to `theme.Panel`/`theme.Backdrop`
  themselves, which every other dialog (the disk-image browser, Form/
  Modal, MessageBox) also draws with, and which weren't part of this
  request. Panel: 85% of whatever `theme.Panel`'s own alpha already
  is (currently 0xff/fully opaque across every theme, so effectively
  0xd8 today, but computed relative to the theme rather than
  hardcoded in case that changes). Backdrop: 80% of the theme's own
  alpha, i.e. 20% less dark, relative to each theme's differing
  starting point (0xa0 for Dark/Default, 0x50 for Light, 0xf0 for
  Spectrum128) rather than one fixed value for all four. Title,
  close-box, body text, and the scrollbar are untouched (already
  fully opaque in every theme) -- ordinary alpha compositing means a
  fully-opaque draw on top of a semi-transparent one still renders
  fully solid, so no extra "opacity: 1" needed on those calls, they
  simply weren't touched.

6 new tests: `scaleAlpha`'s own arithmetic (including the zero-factor
edge case), the panel and backdrop alpha values computed correctly
relative to all four themes' own starting alphas, `draw` actually
using the adjusted values rather than the theme's raw ones, and every
text draw call staying at full alpha.

## [0.4.58] - 2026-08-19

### Fixed

- **Critical: `MenuBar.SetItems` recreated the entire open dropdown's
  `Menu` from scratch on every call**, wiping `itemRects` (only
  repopulated by the next `Draw`) and keyboard-selection state every
  single time -- fatal for a host calling `SetItems` every frame a
  dropdown might be open, the established pattern
  `refreshCustomROMItems`/`refreshViewItems` both use. Reported as
  "all the items under the View menu appear disabled, or don't
  respond" -- the real mechanism: each frame, `refreshViewItems`
  called `SetItems`, which called `openLabel`, which built a brand
  new `Menu` with empty `itemRects`; that same frame's own `Update`
  then hit-tested the mouse against those empty rects and always
  missed, so no item could ever register as hovered or clicked, the
  entire time the dropdown was open. `SetItems` now updates the
  already-open `Menu`'s items in place (`mb.menu.cfg.Items = items`)
  instead of recreating it -- `layout()` already recomputes
  `itemRects` fresh from `cfg.Items` on every `Draw` regardless of
  item count, so this is picked up correctly on the very next frame;
  an out-of-range `selected`/`hover` left over from a shorter new
  list was already handled safely elsewhere (`itemEnabled`'s own
  bounds check, and `hover` is recomputed fresh from the current mouse
  position every `Update` regardless). This also silently affected
  Custom ROM the same way whenever its own dropdown was open, though
  that hadn't been reported.

Verified the fix is meaningful, not tautological: temporarily
reverted `SetItems` to the old recreate-on-every-call behaviour and
confirmed the new regression tests actually fail against it,
reproducing the reported symptom exactly (a click on a known-good
item rect gets rejected) before restoring the fix and confirming they
pass again.

3 new tests in `pkg/zenui`: `itemRects` staying populated (and
stable) across repeated `SetItems` calls on an open dropdown without
an intervening `Draw`, a click still correctly registering under
those same conditions, and `SetItems` on a *closed* dropdown updating
its configured items without opening it.

## [0.4.57] - 2026-08-19

### Changed

- **Font is a submenu of Theme now**, not its own top-level bar label
  -- Theme's dropdown holds Dark/Light/Spectrum128, a separator, then
  "Font" with the six bundled faces and (below their own separator)
  Zoom X1/X2/X3 as nested SubItems. Dispatch is a new special case in
  Update (`sel.ItemIndex == len(zenui.Themes)+1`, computed the same
  way at construction and here rather than hardcoded twice) reading
  `sel.SubIndex` into a new `b.fontActions` field, since Font's
  actions no longer live under a `b.actions[barX]` entry the way every
  other menu's still do. The gradient overlay/rainbow/logo hot zone
  all anchor to `MenuBar.LabelsEndX()` -- whatever the last label
  actually is -- rather than to "Font" by name, so they needed no
  code changes at all now that Theme is genuinely the last label;
  verified this directly with a test rather than assumed it. New
  `Menu.Items()` (pkg/zenui) and `MenuBar.LabelCount()` accessors,
  added to make this properly testable.
- **The logo/rainbow dropdown, reordered and with a checkbox.**
  "Fixed menu bar" is a checkbox now (`Checked: &b.fixed`, the same
  direct-pointer pattern the View menu's own checkboxes use), first
  in the list; order is now Fixed menu bar, ZenZX website (renamed
  from "View ZenZX Homepage"), Help, About... (renamed from "About").
  New `applyFixedState` helper supplies the window-resize side effect
  after `Item.Toggle` has already flipped `b.fixed` directly, invoked
  from a new `Toggled` case in the logo menu's own Update handling
  (which, unlike `Accepted`, keeps the menu open -- the point of a
  checkbox).
- **Tape, Floppy Disk, and Snapshot menus reorganised** with
  separators, matching the Machine menu's own earlier precedent:
  Tape splits into playback (Play/Stop, Rewind) and settings/info
  (Accurate/Fast Mode, Show Info); Floppy Disk into mount (Open DSK
  Image..., Insert Blank Disk), save (Save Disk, Save Disk As...),
  and current-disk operations (Eject Disk, Disk Info); Snapshot into
  save operations (Quick Save, Quick Load, Save Timestamped) and
  info/diagnostics (Snapshot Info, Run Diagnostics).

### Added

- **View menu zoom.** New `DisplayManager.SetScale(n)` jumps directly
  to a specific multiplier (validated against
  `maxMultiplierThatFits()`), unlike `ScaleUp`/`ScaleDown`'s one-step-
  at-a-time behaviour. View's dropdown gained a separator and X 1/
  X 2/X 3 items -- checked against the current scale, disabled above
  what the current monitor fits -- rebuilt every frame the bar is
  shown via a new `refreshViewItems` (the same `SetItems`-based
  pattern `refreshCustomROMItems` already established), since
  `Item.Disabled` has no pointer-based live-update mechanism the way
  `Checked` does, and the scale itself can change outside the menu
  entirely (`PgUp`/`PgDn`).

17 new tests this pass, plus 6 existing tests updated for the
restructured mechanics (not counted as new): `SetScale`'s validation
and no-op-when-unchanged behaviour, `refreshViewItems`' structure/
checked-state/disabled-state, the zoom dispatch actually calling
`SetScale`, the logo menu's new order and the Fixed item's checkbox
structure, Font's submenu structure and dispatch-index computation
matching construction, a real font selection through `fontActions`,
and the new separator groupings (and, for Floppy Disk, that the
reordered actions still match their reordered labels) in all three
reorganised menus.

## [0.4.56] - 2026-08-19

### Fixed

- **Critical: the emulated machine silently lost keyboard input any
  time the menu bar was merely visible, whether or not anything was
  actually open.** `appMenuBar.Active()` included `b.state !=
  barHidden` -- true the entire time the bar is shown, sliding, or
  fixed, not just while a dropdown/dialog/menu/modal is genuinely
  open and consuming input. Since `zenzx_gui.go`'s main loop gates
  `zx.HandleInput()` on `!bar.Active()`, this meant: the mouse merely
  drifting near the top edge during normal play (triggering the
  idle-at-edge auto-show) silently stopped keyboard input to the
  Spectrum until the bar auto-hid again -- explaining reports of
  keyboard input working "sometimes" -- and permanently while the bar
  was fixed, since a fixed bar never leaves the shown state at all.
  `zenui.MenuBar.Active()` itself was already correct
  (`openIndex >= 0`, meaning "a dropdown is actually open"); the bug
  was specifically `appMenuBar`'s own wrapper adding the visibility
  check on top. Fixed by removing it -- `Active()` now reflects only
  `b.widget.Active()`/`diskDialog`/`logoMenu`/`activeModal`, matching
  exactly "does something here actually want this frame's input",
  which is what both of `Active()`'s two callers in `zenzx_gui.go`
  (the demo-overlay trigger-key guard and the `HandleInput` gate)
  already wanted. `bar.Update` itself is unconditional every frame
  regardless of `Active()`'s value, so the bar's own hover-to-open
  and click handling are unaffected by this fix -- only whether the
  emulated machine simultaneously receives input changes.

3 new tests: `Active()` correctly false across every bar visibility
state (hidden, sliding in, shown, sliding out) with nothing open,
correctly false even while fixed with nothing open, and correctly
true when the logo menu is open regardless of the bar's own
visibility state.

## [0.4.55] - 2026-08-19

### Added

- **Fixed menu bar grows/shrinks the window to accommodate itself,
  animated.** New `DisplayManager.reservedTopHeight`/
  `SetReservedTopHeight`, reusing the exact `updateTargetSize`/
  `isAnimating`/`UpdateWindowSize` mechanism border-toggling already
  uses -- the window animates to its new size the same way. Getting
  this right meant fixing three separate places in `Render()` that
  assumed content started at Y=0: the destination rect (offset down,
  height reduced, so content shifts rather than stretches into the
  reserved strip), and `borderV` in both the with-border and
  without-border branches (an absolute window-space Y, which needed
  the identical offset to stay aligned with the now-shifted content --
  easy to miss, since it's computed independently of the destination
  rect it needs to match). Wired into the logo menu's Fix/Hide action.
- **The Spectrum 128 rainbow's remaining 1px black seam between
  colour bands is gone.** Switched each scanline segment from
  `DrawLine` to `FillRect` -- unambiguous pixel coverage, where
  raylib's line drawing apparently wasn't (confirmed visually, not
  assumed; this is the second rainbow-adjacent rendering quirk this
  project has hit from `DrawLine` specifically, after the earlier
  checkered-artifact one).
- **A second, horizontal gradient overlaid on the Spectrum 128 bar's
  vertical one**, composited with multiply blend mode, over the same
  right-side region the rainbow occupies: white (no darkening) at
  that region's left edge, black (matching the base gradient's own
  bottom colour) at the window's right edge -- darkening increases
  toward the rainbow's own corner. New `Renderer.
  FillRectGradientHMultiply` method (raylib's `BeginBlendMode(
  BlendMultiplied)` + `DrawRectangleGradientH`), keeping this
  portable through the same interface-extension approach every other
  raylib-specific capability in this widget layer has gone through,
  rather than a host-side escape hatch. New `Theme.UseGradientOverlay`/
  `GradientOverlayLeft`/`GradientOverlayRight` fields; drawn
  immediately after the base gradient and before labels/rainbow/logo,
  so those stay at full brightness rather than also being darkened.
- **The logo/rainbow dropdown is now always right-aligned to the
  window's own right edge**, regardless of the hot zone's own width
  -- a zero-width anchor at `screenW - rightMargin`, the same
  positioning trick `openSubmenu` already uses, rather than `Menu`'s
  own default "left-align within the anchor if it fits" behaviour
  (which is what was actually happening before: the hot zone is wide
  enough that the narrow logo menu always fit starting from its left
  edge).

### Fixed

- **Spectrum 128 theme colours, three changes**: menu separators are
  now grey (new `Theme.SeparatorColour`, applied uniformly across
  every theme, only Spectrum128 actually differing from its own
  Border colour -- black-on-white would have been near invisible);
  checkmarks are a darker, standard-brightness green
  (`0x00c800`, was bright `0x00ff00`); checkboxes get their own fixed
  blue (`0x0000c8`, new `Theme.UseCheckboxColour`/`CheckboxColour`,
  mirroring the existing `UseCheckmarkColour` pattern) so the two
  indicator kinds read as visually different at a glance, not just by
  shape.
- **Spectrum 128 bar gradient one further notch more contrasted**:
  top colour 25% -> 40% grey (`0x40`->`0x66`), the same 15-percentage-
  point "notch" size used throughout this theme's earlier tuning
  passes; the bottom was already pure black, the floor.

14 new tests this pass (3 existing tests also updated for the changed
checkmark/gradient/rainbow-encoding values, not counted as new):
`SetReservedTopHeight`'s target-height math and animation-triggering
(plus its no-op-when-unchanged case), the Fix/Hide menu action
actually reserving/releasing the height, right-alignment holding
across four very different screen widths, the rainbow's adjacent-band
edge-pixel-sharing directly, the new separator/checkbox colours
(including confirming the other three themes don't opt into either
override), and the gradient overlay's values, draw-call presence
(and absence for non-opted-in themes), and draw-ordering relative to
the labels.

## [0.4.54] - 2026-08-19

### Added

- **A dropdown menu associated with the rainbow/logo decoration** on
  the right of the bar -- click it directly (no hover needed): Fix/
  Hide menu bar, Help, About, and View ZenZX Homepage. Hardcoded
  (`appMenuBar.openLogoMenu`/`handleLogoMenuResult`) rather than built
  through the same `zenui.MenuBarItem`/actions machinery every other
  menu uses, since the decoration isn't really a menu bar label and
  there are only ever these four fixed choices -- a `*zenui.Menu` is
  still what actually renders it, just constructed directly instead of
  via `MenuBarConfig`. The click zone is computed from
  `MenuBar.LabelsEndX()` to the screen's own right edge, generously
  covering both the rainbow and the ZSP logo regardless of which
  theme is active, without needing to export either decoration's
  internal geometry functions just for hit-testing.
- **Fix/Hide menu bar**: new `appMenuBar.fixed` field bypasses the
  idle-at-edge auto-hide check entirely when set, keeping the bar
  permanently shown; turning it on also shows the bar immediately.
- **Help and About**, both a scrollable markdown-lite reader
  (`# H1`/`## H2` headings, indented shortcut tables, mouse-wheel/
  drag scrollbar, Escape/close-box/click-outside to dismiss) adapted
  from zenimate's own `helpModal`
  (`cmd/zenimate-gui/helpmodal.go`, same author, likewise Apache
  2.0 -- the same project `pkg/zenui`/`pkg/bdf`/`pkg/zxpalette`/
  `pkg/zenuiraylib` originally came from), generalised
  (`markdownmodal_gui.go`) to take a title and content string
  directly instead of one embedded `help.txt`, so both share a single
  implementation rather than two near-identical copies. Content is
  new: `help.txt` covers every menu, keyboard shortcut, and
  command-line flag; `about.txt` covers version (the `__VERSION__`
  placeholder substituted from the existing build-time `version`
  variable at open time, not a fresh import -- avoided a real naming
  collision with an existing package-level `version` var along the
  way), the project's homepage link, and licence/attribution.
- **View ZenZX Homepage** opens `https://ha1tch.github.io/zsp/
  projects/zenzx` via `rl.OpenURL`.

### Fixed

- **Redundant "Toggle" removed from three menu labels.** "Toggle
  FPS"/"Toggle Border" (View menu, both checkboxes since an earlier
  release -- the checkbox itself already conveys the toggle, the word
  was redundant) are now "FPS Counter"/"Border Display"; "Toggle Mode
  (Accurate/Fast)" (Tape menu) is now "Accurate/Fast Mode", matching
  the "Pause/Resume" convention already established elsewhere in the
  Machine menu.

A real crash surfaced and got fixed during this release's own
testing, not shipped: calling `Draw` with the real, raylib-backed
renderer from a headless test segfaults (no live GL context for its
actual drawing calls) -- something no test in this codebase had done
before. Fixed by adding a small no-op `zenui.Renderer` implementation
scoped to this test file, used wherever a test needs `Draw` called
(to populate layout state like `LabelsEndX`/`ItemRect`) without
touching real raylib drawing calls at all.

7 new tests: the logo hot zone's exact geometry, the logo menu's item
count, Fix/Hide toggling and the bar actually showing when fixed,
Help/About opening with the correct title and (for About) the
version placeholder correctly substituted, the auto-hide bypass
condition itself, and `Active()` correctly reporting true for both
the logo menu and an open modal.

## [0.4.53] - 2026-08-19

### Added

- **Menu: group headings ("tiny titles").** `Item.Title bool` marks an
  item as a small, dim, non-selectable heading -- drawn in
  `theme.DimText` at one scale step smaller than the menu's own
  (`Menu.titleScale`, floored at 1), with a more compact row height
  than a normal item (one `padY`, not two). Never hoverable, clickable,
  or keyboard-navigable, the same way `Separator` already isn't
  (`itemEnabled` now excludes both). Meant to pair with `Separator` to
  divide a menu into labelled groups; indenting the items belonging to
  a group is left to the caller (leading spaces in their own `Label`),
  not something this field does automatically.
- **The Machine menu now groups models by manufacturer**, using the
  new Title/Separator combination instead of the hover-opened
  Standard/Spanish/Timex submenus from an earlier release: Sinclair
  Research Ltd (the original 48k/128k, before Amstrad's 1986
  acquisition), Amstrad plc (+2/+2A/+3), Investronica S.A. Spain (the
  Spanish-localised variants, made under licence), and Timex (the
  US-market Timex Sinclair variant) -- each its own heading and
  separator, with the models listed directly beneath, indented and
  under their own display names ("ZX Spectrum 48k") rather than the
  raw `-model` flag string. New `machineModel` type pairs each flag
  with its display label; `machineModelGroups` is keyed by
  manufacturer now, not the old three-way split. Selection dispatch
  is a direct index lookup into a new parallel `machineModelFlags`
  slice (Title/Separator rows carry `""`) rather than the
  group/sub-index computation submenus needed -- simpler now that
  there's no nesting to resolve.

9 new tests this pass (1 replaces a prior test whose submenu-structure
assumptions no longer applied; 1 existing test updated for the new
`machineModel` struct without changing what it actually asserts):
title row height/non-highlighting/non-clickability/keyboard-skip
behaviour, title text colour and scale (including the floor-at-1
case), the flat Machine menu's exact structure (separator/title/model
ordering and the parallel `machineModelFlags` correctness), and a
direct flag-lookup check for a specific model's row.

## [0.4.52] - 2026-08-19

### Fixed

- **The Spectrum 128 rainbow was still wrong -- right triangles, not
  parallelograms, confirmed directly from a screenshot.** The previous
  fix's real bug: each band was clipped to a fixed-position,
  stripeW-wide column, but the line drawn into it shifted left by up
  to `height` pixels every row -- the clip's own fixed left edge
  truncated the shifted-away portion, and the right edge shrank
  naturally with it, producing a triangle that narrowed toward zero
  width at the bottom row instead of a constant-width parallelogram.
  Rebuilt from an exact specification (an ASCII diagram of the target
  pixel pattern, reproduced pixel-for-pixel in a new test): for each
  row, draw four separate lines, one per colour band, each a fixed
  `RainbowBandWidth` pixels wide, with the whole four-band group
  shifted one pixel further left than the row above. No clipping
  anywhere -- the geometry function now reserves enough room on the
  left up front for the full shift, so every line is already the
  right width and position by construction, nothing needs to be cut
  down afterwards. `RainbowBandWidth` and `RainbowScanlines` are now
  plain exported package variables (defaults 16 and 24, matching the
  bar's own height) rather than values derived from a dynamic
  available-space calculation -- simpler, and directly tunable without
  touching the drawing logic.

### Added

- **Checkmarks are Spectrum green in the Spectrum 128 theme.** New
  `Theme.UseCheckmarkColour`/`CheckmarkColour` fields: when set, a
  checkmark's tick uses this fixed colour instead of the item's own
  computed text/selection colour, so the "current setting" indicator
  reads as a stable colour that doesn't shift with hover. Only
  Spectrum128Theme sets it (bright green, `0x00ff00`, matching the
  palette's existing use of bright variants elsewhere); every other
  theme is unaffected, still using the item's own computed colour as
  before this field existed.
- **Empty (unchecked) checkboxes are grey, uniformly across every
  theme.** New `Theme.CheckboxEmptyColour` field (`0x808080`, neutral
  grey, set the same way on all four themes) -- an unchecked
  checkbox's own box outline now always uses this colour rather than
  the item's computed text colour; a checked box (and its cross) is
  unaffected, still using the computed colour as before.

12 new tests: the rainbow's geometry directly against the exact
example diagram (reconstructed pixel-for-pixel from recorded draw
calls and compared string-for-string against all five example rows),
the fit/no-fit boundary at the shift-reserved margin, confirmation
that no clipping happens anywhere in the new implementation, scanline
count correctly capped by the bar's own height even when
`RainbowScanlines` is set larger, the Spectrum128 checkmark colour
override and every other theme's absence of it, and the checkbox grey
value applying to the empty state only, not the checked one.

## [0.4.51] - 2026-08-19

### Fixed

- **Alt+B and the View menu's Border checkbox no longer disagreed on
  what "toggling the border" means.** `ToggleBorder` doesn't just flip
  a bool -- it also calls `updateTargetSize` (sets `isAnimating =
  true`, driving a real window-resize animation via
  `UpdateWindowSize`). The checkbox's direct-pointer wiring
  (`Item.Toggle` flipping `zx.display.showBorder` directly) only ever
  did the bool half; the animation never fired from the menu. Fixed by
  changing the Border action closure -- already reached by the
  generic dispatch on every `Toggled` event, confirmed, not just
  `Accepted` -- from a no-op to calling `updateTargetSize` itself, so
  Item.Toggle supplies the flip and this closure supplies the second
  half, matching Alt+B exactly rather than just agreeing on the bool.
- **The Spectrum 128 rainbow's diagonal lines produced a checkered
  artifact, not a clean edge** -- confirmed by the person's own numpy
  analysis of the actual rendered output, not assumed. Replaced
  diagonal `DrawLine` calls with dense per-row horizontal scanlines:
  each row draws one full-width horizontal line, shifted left by
  exactly `row` pixels from the row above, clipped per band. Scanline
  rasterisation is unambiguous in a way sparse diagonal lines spaced
  2px apart apparently weren't -- there's no possible gap or overlap
  between adjacent rows to produce a checkerboard.
- **Checkmark and checkbox indicators no longer look identical.**
  `Item.Toggle` distinguishes them properly now: checkboxes
  (non-mutually-exclusive options) always show an outlined box, with
  a cross mark drawn inside when checked; checkmarks (mutually
  exclusive options) show nothing at all when not selected, a tick
  mark when selected. Previously both used the same filled/outlined
  square regardless of which kind of choice the item represented.
- **Submenus now always draw a full border, including the top edge,
  regardless of the active theme's `DropdownBorderSkipTop`.** That
  setting means "this dropdown opens directly under the bar, so a top
  edge would just double the bar's own bottom edge" -- a statement
  about a top-level dropdown's relationship to the bar that's never
  true for a submenu, which opens beside a parent item instead. New
  `Menu.isSubmenu` field, set only by `openSubmenu`, makes the
  distinction.
- **Checkbox/checkmark indicators sat hard against the dropdown
  panel's own left border.** The indicator's X was computed from
  `rec.X` directly, ignoring `padX` entirely. Now positioned after
  `padX` first, then centred within the gutter -- the same left
  margin every item's own text already gets.

### Added

- **Menu separators** (`Item.Separator bool`): a thin horizontal
  divider row, never hoverable-as-selected, clickable, or
  keyboard-navigable (extends the existing `itemEnabled` check the
  same way `Disabled` already does, so every interaction path that
  already respects Disabled correctly skips separators too, with no
  further changes needed). Required rewriting `Menu.layout` to use
  per-item variable row heights instead of a uniform `itemH` -- a
  separator's own row is shorter than a normal item's, not the full
  text-plus-padding height.
- **Submenus open 4px to the left of the parent row's own right
  edge**, overlapping it slightly rather than sitting exactly flush.
- **ZSP logo rotation slowed to 1 second per frame** (was a third of
  a second) -- too fast once actually seen running; a full
  three-arrangement cycle now takes three seconds, not one.
- **Spectrum 128 bar gradient darkened two more notches**: top colour
  55% grey -> 25% grey (`0x8c`->`0x40`, two more 15-percentage-point
  reductions); the bottom was already pure black, the floor.

17 new tests this pass (2 replace prior tests whose assumptions no
longer applied after the checkbox/checkmark rewrite): the Border
animation regression itself, rainbow scanline count/orientation/shift-
per-row, checkmark tick-only-when-checked and checkbox
box-always-cross-when-checked behaviour, the left-margin padding fix,
submenu full-border and 4px-position correctness, and six separator
tests covering row height, non-highlighting, non-clickability,
keyboard-navigation skipping, line-not-text drawing, and total menu
height reduction.

## [0.4.50] - 2026-08-19

### Added

- **ZSP (Zen Spectrum Project) logo on the bar, Dark and Light themes
  only, in the same right-of-labels slot the Spectrum 128 rainbow
  uses.** Two colour-halves, each two ascending three-block "teeth"
  (bottom-left to top-right -- x increases and y decreases together
  as the block level rises, matching the reference exactly), rotating
  through three colour arrangements every third of a second (a full
  cycle takes one second). Colours measured directly from the three
  reference images provided (numpy exact RGB extraction): standard-
  brightness yellow/cyan, then two bright pairs (green/magenta,
  cyan/red) -- reproduced exactly as given rather than normalised to
  one brightness level. Drawn entirely through `FillRect` (new
  `pkg/zenui/zsplogo.go`), not a bitmap or a transparent image -- the
  "prefer procedural rectangle drawing over an image asset" approach
  explicitly requested, and the same approach the Spectrum 128
  rainbow already uses.
- **`Input.DeltaTime float32`**, populated from `rl.GetFrameTime()` at
  the single shared point every widget's input already flows through
  (`zenuiraylib.Input()`) -- benefits any future widget with its own
  animation timing, not just this one. `MenuBar` accumulates it into a
  new `logoElapsed` field across `Update` calls, driving the colour
  rotation; the accumulator runs regardless of which theme is active,
  so switching to Dark/Light mid-session doesn't reset the rotation
  phase to whichever arrangement happened to be first.
- **`Theme.ShowZSPLogo bool`** -- `true` for Dark/Light, explicit
  `false` for Spectrum128Theme and DefaultTheme, following the same
  per-theme-field convention `ShowBarRainbow`/`UseBarGradient` already
  established.

### Verified

- **The Spectrum 128 rainbow's existing geometry and colours
  independently re-confirmed against a newly provided, purpose-built
  reference asset (`spectrum128.png`)** -- pixel-measured the same way
  as the original boot-screenshot verification: four equal-width
  bands, the same red/yellow/green/cyan order, a true 45-degree
  diagonal (shift equal to the strip's own height). Both references
  agree completely; no code changes were needed, only the doc comment
  updated to record the independent confirmation.

9 new tests: ZSP logo colour-triple exact match, block proportion
(1:2 width:height, matching the reference logo's own 45x90 block
ratio), geometry's not-enough-room and too-short-for-blocks refusals,
fill count and per-half colour correctness, the ascending-staircase
direction itself, colour-index advancement from accumulated
`DeltaTime`, and the logo drawing nothing at all when the active
theme doesn't request it.

## [0.4.49] - 2026-08-19

### Fixed

- **Spectrum 128 theme's bar gradient 15% darker.** Top colour 70%
  grey -> 55% grey (`0xb2`->`0x8c`); the bottom was already pure
  black, the floor, so it couldn't get darker.
- **`Menu.subResult` was never initialised, defaulting to Go's zero
  value (0) rather than -1.** Discovered while wiring the Machine
  menu's submenus: a plain top-level item at index 0 could be
  mistaken for "submenu item 0 was chosen" by anything checking
  `SubResult() >= 0`. `NewMenu` now sets `subResult: -1`, matching
  every other "not applicable" field it already initialises the same
  way (`result`, `subOpen`, `hover`, `selected`).
- **`MenuBar.Update` didn't handle `Toggled` at all** -- it fell
  through the switch's cases entirely, returning `ok=false` and
  leaving the host with no way to learn a checkbox item was clicked
  even though the underlying bool had already flipped correctly. Now
  returns the selection with `ok=true`, matching `Accepted`, but
  without calling `Close()` -- the dropdown stays open, since that's
  the entire point of a checkbox. Extended `MenuBarSelection` with a
  new `SubIndex` field (mirroring `Menu.SubResult` one level up) so a
  submenu choice is fully expressible at the bar level too.

### Added

- **Checkmarks on Theme, Font, Font zoom, and Machine's current
  model.** Each menu's items hold a `*bool` into a small map on
  `appMenuBar` (`themeChecked`/`fontChecked`/`zoomChecked`/
  `modelChecked`), built once at construction and updated in place --
  never rebuilt -- whenever `ApplyTheme`/`ApplyFont`/`ApplyZoom`/a
  model switch runs, so a menu re-opened later always shows the
  correct current selection.
- **Machine menu reorganised into Standard/Spanish/Timex submenus**
  (replacing the old flat 10-item list), each carrying its own
  checkmark. Selection dispatch reads the new `MenuBarSelection.
  SubIndex` to resolve which model within which group was picked.
- **View menu's Toggle FPS/Toggle Border are checkboxes now, not
  one-shot actions.** Wired directly to `zx.display.showFPS`/
  `showBorder` themselves (`Item.Checked: &zx.display.showFPS`,
  `Toggle: true`) rather than a separately-synced copy -- the same
  bool the existing `Alt+F`/`Alt+B` shortcuts already flip via
  `ToggleFPS`/`ToggleBorder`, so the checkmark is correct regardless
  of which path changed it, with no refresh step needed anywhere.
  One accepted, documented trade-off: `ToggleBorder`'s own console
  print doesn't fire when toggled from the menu specifically (`Item.
  Toggle` flips the bool directly, bypassing that method) -- judged
  acceptable since the checkbox itself now makes the state visible
  without needing the console at all.
- **`newAppMenuBar` now takes `zx *ZenZX` and the initial model**,
  needed for the View checkboxes' direct wiring and the Machine
  menu's initial checkmark respectively. Confirmed safe before
  making the change: `zx` exists and the `-model` flag's own value is
  stable well before `newAppMenuBar` is called in `zenzx_gui.go`'s
  `main`, even though the actual ROM-loading switch statement runs
  later.
- **`zenui.MenuBar.ItemsFor(barIndex) []Item`** -- a small new
  accessor for a host that needs to inspect an item's `Checked`
  pointer or `Toggle` flag directly, used throughout this release's
  own tests; mirrors `SetItems`' existing bounds-checking convention.

14 new tests: Machine's submenu structure and model coverage, initial
and post-switch checkmark correctness for Theme/Font/Zoom/Machine,
the View checkboxes' direct-pointer wiring (including a same-address
check, not just same-value), `Toggle` vs. plain-item distinction, a
safety check that the otherwise-unreachable `barView` action-table
entries are no-ops rather than nil (confirmed necessary once
`MenuBar.Update` started returning `ok=true` for `Toggled`), and
`MenuBar`'s own `Toggled`-keeps-dropdown-open behaviour directly.

## [0.4.48] - 2026-08-19

### Added

- **Form: a grid-laid-out widget for labelled fields, checkboxes, and
  text input.** `FormField.Row`/`Col` place a field in grid cells (not
  pixels); `ColSpan`/`RowSpan` let one field cover more than one cell.
  Column widths and row heights are computed from the fields that
  occupy them (a field's own label, or `MinWidthCells` when that needs
  to be wider than the label implies) and snapped up to
  `FormConfig.GridSize` -- default 8px, as specified. A deliberate
  simplification, documented rather than left to be discovered by
  surprise: a field spanning multiple columns doesn't widen the
  columns it spans, it gets the sum of whatever they already are --
  a full constraint solver is more machinery than a form of a handful
  of fields needs. Three field types: `FieldLabel` (static text),
  `FieldText` (editable, append/backspace only -- the same convention
  `Dialog`'s own filename field already established, no mid-string
  cursor), `FieldCheckbox`. Tab cycles focus forward through
  focusable fields, wrapping, skipping labels and disabled fields;
  clicking a text field focuses it, clicking a checkbox toggles and
  focuses it. Form itself never Accepts or Cancels -- it always
  reports `Active`; that decision belongs to whatever hosts it.
- **Modal: a generic dialog wrapping a Form with a backdrop and a
  configurable button row.** `ModalConfig.Buttons` is an arbitrary
  list of labels, left to right -- not limited to any fixed set.
  `CancelButtonIndex`, if set, is the button Escape also triggers.
  Genuine two-pass layout (`Form.layout` is pure computation, safe to
  call twice): first learns the form's natural size, then repositions
  it so the form-plus-buttons combination is centred on screen as a
  whole, not just the form alone with buttons trailing off wherever
  they land.
- **MessageBox and MessageDialog: generic and purpose-specific
  message/choice dialogs, built on Modal.** `NewMessageBox` is fully
  generic (arbitrary button labels, for anything from a save-changes
  prompt to a "pick a format" choice); `NewOKMessageBox`/
  `NewYesNoMessageBox`/`NewYesNoCancelMessageBox`/
  `NewOKCancelMessageBox` are convenience constructors with the
  buttons and `CancelButtonIndex` pre-set. `MessageDialog` is a
  straight type alias for `MessageBox` -- documented plainly as such,
  not a distinct implementation pretending to a difference that
  wasn't asked for -- with a parallel `NewXMessageDialog` constructor
  set, for callers who know this concept by the other name. Both
  names share every method (`Draw`/`Update`/`Status`/`Result`) via the
  same embedded `*Modal`.

### Fixed

- A real bug caught during Menu's own checkbox/submenu testing (not
  from this release, but worth restating here since it's the same
  class of issue Form/Modal's layered Update calls could easily have
  repeated): a status returned from a nested widget's `Update` must
  not be allowed to fall through into the parent's own key handling
  in the same call. Modal's `Update` (delegates to Form, then checks
  its own Escape/button handling) and Menu's submenu delegation both
  follow the same return-immediately discipline established when that
  bug was fixed.

All four widgets confirmed to have zero raylib references anywhere in
their source -- grepped directly, not just built successfully -- the
same portability standard every existing zenui widget already meets.

35 new tests across `form_test.go` (18), `modal_test.go` (9), and
`messagebox_test.go` (8): grid sizing and snapping, multi-column/multi-row
layout, `ColSpan`, `MinWidthCells`, text typing/backspace/control-
character filtering, tab navigation order, checkbox and text-field
click behaviour, anchor vs. auto-centring, button positioning and
click handling, `CancelButtonIndex` via both click and Escape, Form
fields working correctly through Modal, a closed modal drawing
nothing, every purpose-specific constructor's exact button
configuration, and the `MessageDialog`/`MessageBox` constructor sets
producing identical results.

## [0.4.47] - 2026-08-19

### Added

- **Menu: checkmarks, checkboxes, and submenus.** All three extend
  `Item` additively -- a menu that uses none of them draws and behaves
  exactly as before.
  - **Checkmarks**: `Item.Checked *bool` draws a small filled (checked)
    or outlined (unchecked) square before the label, built from
    `FillRect`/`StrokeRect` rather than a font glyph, since the
    bundled bitmap fonts don't reliably have a dedicated check
    character. Display-only -- clicking still selects and closes the
    menu, the same as any item without `Checked` set. Gutter space is
    reserved only if at least one item in the menu actually sets
    `Checked`, so checkable and non-checkable items in the same menu
    align consistently, and a menu using neither is laid out
    identically to before this existed.
  - **Checkboxes**: `Item.Toggle bool` (alongside `Checked`) makes
    clicking flip `*Checked` and keep the menu `Active` rather than
    closing it -- a new `Toggled` status, returned from `Update` for
    that one frame without ever being stored into the menu's own
    `Status()` (which stays `Active`), which is what lets a checkbox
    be clicked more than once in the same menu-opening.
  - **Submenus**: `Item.SubItems []Item` opens a nested `*Menu`
    on hover -- not click, matching `MenuBar`'s own established
    hover-opens/hover-switches behaviour -- positioned to the right of
    its row (or left, with no new positioning logic: a zero-size
    anchor placed at the row's right edge reuses `layout`'s existing
    anchor-fallback maths unchanged). A parent row with `SubItems` is
    never itself directly selectable, by hover or by click. New
    `Menu.SubResult()` reports which submenu item was chosen,
    separate from `Result()` (the parent row's own index) -- mirroring
    how `MenuBarSelection` already separates bar-index from
    item-index one level up. One level of nesting only; a submenu
    item's own `SubItems`, if set, are ignored.

A real bug turned up during testing and got fixed: Escape inside an
open submenu correctly cancelled the submenu, but the same key press
then fell through to the parent menu's own Escape check in the same
`Update` call, cancelling the parent too. Fixed by returning
immediately after closing a cancelled submenu rather than falling
through -- hover detection catching up costs one extra frame, a better
trade than double-consuming the same key press.

11 new tests: checkmark/checkbox rendering and gutter reservation,
toggle-stays-open and repeated-toggle behaviour, non-toggle items
unaffected, submenu open-on-hover, parent-not-directly-selectable,
choice propagation to the parent, the Escape-inside-submenu regression
itself, and hovering a different submenu-having item switching
correctly.

## [0.4.46] - 2026-08-19

### Fixed

- **Architecture review: eliminated every remaining direct raylib call
  from the UI widget layer.** The bar's gradient and rainbow
  decorations previously bypassed the `Renderer` abstraction entirely
  -- `menubar_gui.go` called `rl.DrawRectangleGradientV`/
  `rl.BeginScissorMode`/`rl.DrawLine` directly, with `pkg/zenui`'s
  `MenuBar.Draw` skipping its own background fill and trusting the
  host to have already painted it first. `Renderer` gained two new
  methods, `FillRectGradientV` and `DrawLine`, implemented in
  `pkg/zenuiraylib.Renderer` and in every test double
  (`noopRenderer`, `drawRecorder`); `MenuBar.Draw` now draws both
  decorations itself, entirely through the interface -- the existing
  `Clip`/`ClipEnd` methods already covered the rainbow's per-band
  scissoring, no further interface change needed there. The rainbow's
  geometry/colour logic and drawing (`rainbowGeometry`,
  `rainbowColours`, `drawRainbow`) moved from `menubar_gui.go` into a
  new `pkg/zenui/rainbow.go`; the host file lost roughly 100 lines and
  every raylib call it used to make for either decoration.
  `gui_demo_overlay.go`'s darkening backdrop had the same problem on a
  smaller scale (`rl.DrawRectangle` called directly instead of through
  `d.renderer.FillRect`) -- fixed the same way.
  A full audit of every file that integrates with a zenui widget
  (`menubar_gui.go`, `gui_demo_overlay.go`, `custom_roms_menu_gui.go`)
  confirms the only raylib calls remaining anywhere in that layer are
  input polling (`GetMouseX/Y`, `IsKeyPressed`) and frame/window
  lifecycle (`BeginDrawing`, `WindowShouldClose`, `GetScreenWidth/
  Height`) -- never widget content drawing. `display.go`,
  `videorender_gpu.go`, and `input.go` are unaffected by this review
  by design: they're the emulator's own screen-rendering and
  emulated-keyboard pipeline, a genuinely separate concern from the
  zenui widget system, not something this abstraction was ever meant
  to cover.

Existing tests migrated to their new home
(`pkg/zenui/rainbow_test.go`) plus 2 new ones confirming `drawRainbow`
actually clips per band and draws nothing when there's insufficient
room -- both were previously untestable from the host side without a
live GL context.

## [0.4.45] - 2026-08-19

### Added

- **"Open DSK Image..." in the Floppy Disk menu** -- a live file
  browser (`zenui.Dialog`, filtered to `.dsk`), mounted the same way
  `-disk` does at startup but without needing to restart. Gated on
  `zx.io.hasFDC` (true only for +3/Spanish +3, tracked live by
  `switchModelLive` already): on any other model it prints a message
  and does nothing rather than opening a dialog that would fail. The
  dialog is driven independently of the bar's own state (`appMenuBar.
  Update`/`Draw` both check it first, before the normal bar/dropdown
  logic), matching the same "own its own lifecycle across frames"
  pattern the Custom ROM two-step flow already established --
  `Active()` also reports true while it's open, so the emulated
  machine receives no keystrokes meant for the file browser.
- **Disk Info now reports the actual floppy controller state**
  (loaded filename, modified-but-unsaved) instead of a static
  "restart with -disk=filename.dsk" message that's been outdated
  since Open DSK Image made a restart unnecessary.

4 new tests: Open DSK Image correctly refusing without a floppy
controller and correctly opening with one, `Active()` reflecting the
open dialog, and Disk Info not panicking across its different states.

## [0.4.44] - 2026-08-19

### Fixed

- **The rainbow was wrong in both colour count and direction --
  verified this time with numpy pixel analysis of an actual +3 boot
  screenshot, not visual estimation.** The real title strip's rainbow
  is four bands (red/yellow/green/cyan), not six -- there is no blue
  or magenta band at all. Confirmed by classifying every pixel across
  a wide row and column range: exactly four 16px-wide bands of pure
  bright RGB (255,0,0 / 255,255,0 / 0,255,0 / 0,255,255), matching
  this project's own `zxPalette` bright variants exactly. The diagonal
  direction was also backwards: measuring the same colour boundary's
  x-position across 16 rows showed it shifting *left* as y increases
  (a "/" lean), not right -- `drawBarRainbow`'s line endpoints added
  `height` to x instead of subtracting it. Both fixed;
  `rainbowColours` is now 4 entries, `x2 := x1-height` not `x1+height`.
  A first analysis pass had actually found the same numbers but missed
  the shift entirely, because a debug filter suppressing long runs of
  background colour also suppressed the one value (the shifting
  black-prefix run length) that would have shown it.
- **Spectrum 128 theme, further tuning:** row height down another 2px
  at the default zoom (`ItemPadYPercent` 14 -> 7); dropdown bottom
  padding increased via new `Theme.DropdownBottomPadding` (6px, every
  other theme explicit 0) -- left/right padding was already correct
  and untouched.

### Added

- **Vertical gradient bar background, Spectrum 128 theme only:** 70%
  grey at the top fading to black at the bottom, via
  `rl.DrawRectangleGradientV`, replacing the flat (twice-lightened)
  Sidebar fill this theme used before. New `Theme.UseBarGradient`/
  `BarGradientTop`/`BarGradientBottom` fields; `MenuBar.Draw` skips its
  own flat background fill when `UseBarGradient` is set, expecting the
  host to have already painted the gradient first (the same
  raylib-in-the-host, signal-in-the-portable-package split
  `ShowBarRainbow`/`drawBarRainbow` already established). New
  `drawBarGradient` (`menubar_gui.go`), called from `appMenuBar.Draw`
  before the widget itself so labels/border/dropdown layer on top of
  the gradient rather than a flat fill covering it back up.

8 new tests: the corrected rainbow colours against the measured
reference values, the gradient skip/no-skip fill behaviour in both
directions, the third-pass Spectrum 128 values, and a Dark/Light
regression guard for the two new fields.

## [0.4.43] - 2026-08-19

### Fixed

- **Spectrum 128 theme, third pass against a direct screenshot
  comparison.** Border thickness 2px (down from the second pass's
  still-heavy 3px). Bar's resting colour lightened a further step.
  Item padding: a side-by-side pixel comparison against the real
  "128 +3" menu box visible in the same screenshot showed its rows
  barely taller than the text itself -- nothing close to the second
  pass's 42%, which had overshot toward "generous" based on
  recollection rather than the actual reference. Vertical padding
  dropped to 14% (tighter than even this project's own 25% default
  for every other theme); horizontal widened further to 75% per
  direct feedback, in the opposite direction from vertical.

### Added

- **The classic Sinclair diagonal-stripe rainbow, on the right side of
  the bar strip, Spectrum 128 theme only.** Six coloured bands of
  45-degree parallel lines (`rl.DrawLine`, scissor-clipped per band --
  not a bitmap, not a single filled shape), using this project's own
  bright-palette RGB values (`zxPalette`, `videorender.go`) for
  consistency with the emulator's own screen rendering. Drawn only
  when there's at least 60px of free space after the last label, and
  capped at 160px total width even on very wide windows, so it reads
  as a compact decoration rather than stretching to fill whatever room
  exists. New `Theme.ShowBarRainbow` field (Spectrum128Theme only;
  every other theme explicit `false`) is the portable on/off signal
  `pkg/zenui` carries; the actual raylib drawing
  (`drawBarRainbow`/`rainbowGeometry`, `menubar_gui.go`) lives entirely
  in zenzx's own GUI code, split into a pure geometry/decision
  function and a thin raylib-calling wrapper so the "is there enough
  room, and where do the bands land" logic is directly testable
  without a live GL context. New `zenui.MenuBar.LabelsEndX()` exposes
  where the last label ends, so a host can make exactly this kind of
  "only if there's room" decision.

10 new tests: `rainbowGeometry`'s space-check, sizing, and width-cap
behaviour in isolation; `LabelsEndX` correctness; and the third-pass
Spectrum 128 padding/border values alongside updated Dark/Light
regression guards.

## [0.4.42] - 2026-08-19

### Fixed

- **The GUI never actually started in Dark theme -- it started in
  `zenui.DefaultTheme()`, a separate, older palette that predates the
  Dark/Light/Spectrum128 preset system.** `newAppMenuBar` set
  `themeName: zenui.ThemeDark` correctly, but initialized the actual
  `theme` value from `zenui.DefaultTheme()` instead of
  `zenui.LoadTheme(zenui.ThemeDark)` -- the two have diverged since
  DarkTheme's own adjustments in 0.4.38 (a more distinct panel/field
  pairing, a richer blue-violet selection). The bar looked right only
  after explicitly picking a theme from the Theme menu once; before
  that, `DefaultTheme()`'s original, unadjusted colours showed instead.
  Fixed: `newAppMenuBar` now takes the initial theme as a parameter and
  resolves it through `zenui.LoadTheme`, the same function the Theme
  menu itself uses.
- **Spectrum 128 theme, further tuning against direct feedback:**
  border thickness brought down from 3px to 2px (the first pass read
  too heavy); the bar's resting colour lightened a second time (from
  the first pass's ~10%-lightened-from-black to a further lightening on
  top of that); dropdown item padding widened from the tight
  25%/50% (vertical/horizontal, matching every other theme) to
  42%/65% -- a middle ground between that and the real menu's own more
  generous spacing, not a full match (which would crowd the dropdown
  vertically at short menus). New `Theme.ItemPadYPercent`/
  `ItemPadXPercent` fields replace the previously-fixed `lh/4`/`lh/2`
  formula; every other theme sets these to 25/50 explicitly, guarded
  by a new regression test confirming Dark and Light are unchanged.

### Added

- **`-theme` command-line flag** (`Dark`/`Light`/`Spectrum128`,
  case-insensitive, the space in "Spectrum 128" optional) sets the
  startup theme directly, rather than always starting Dark and
  requiring the Theme menu for anything else. An unrecognised value
  warns and falls back to Dark rather than failing silently.

6 new tests: the startup theme actually matching `zenui.LoadTheme`'s
own value (not `DefaultTheme`), `-theme` flag parsing (case,
whitespace, the optional space in "Spectrum 128", and the
unrecognised-value fallback), and the Spectrum 128 padding/border
adjustments alongside a Dark/Light regression guard.

## [0.4.41] - 2026-08-19

### Fixed

- **Spectrum 128 theme, second pass on direct feedback against the
  real 128K/+2/+3 menu.** Dark and Light are unchanged (confirmed
  correct as shipped in 0.4.40).
  - The top bar's "which menu is open" indicator was using the
    dropdown's own cyan selection colour -- the real title strip is
    always black, it never shows that highlight. New `Theme.BarSelFill`/
    `BarSelText` fields, separate from `SelFill`/`SelText`, let the bar
    and the dropdown use different highlight treatments where a theme
    needs it. Spectrum 128: bar's resting colour lightened from pure
    black to a ~10%-lighter near-black; the open label's own
    background is pure black (darker than that resting colour), text
    stays white. Dark and Light: `BarSelFill`/`BarSelText` set equal to
    `SelFill`/`SelText`, so their bar's appearance is byte-for-byte
    unchanged from before these fields existed -- guarded by a new
    regression test (`TestDarkAndLightBarSelMatchesSelUnchanged`).
  - Dropdown border was a flat 1px `StrokeRect` for every theme, no
    thicker than the original menu's own noticeably heavier border
    weight, and drawn on all four sides even though a bar-opened
    dropdown always sits flush under the bar. New
    `Theme.BorderThickness`/`DropdownBorderSkipTop` fields; new
    `drawBorder` helper (four independent edge fills, letting the top
    edge be selectively omitted -- `StrokeRect` draws all four sides in
    one call with no way to skip just one). Spectrum 128:
    `BorderThickness: 3`, `DropdownBorderSkipTop: true`. Dark and
    Light: `BorderThickness: 1`, `DropdownBorderSkipTop: false` --
    identical to the previous fixed `StrokeRect(..., 1)` call, guarded
    by `TestDarkAndLightBorderUnchanged`.

9 new tests: `drawBorder`'s own edge/thickness/skip-top behaviour in
isolation, `Menu.Draw` correctly reading `BorderThickness`/
`DropdownBorderSkipTop` from the active theme, `MenuBar.Draw` using
`BarSelFill`/`BarSelText` for the open label, and the Dark/Light
regression guards and Spectrum 128 correctness checks described
above. `TestMenuBorderDrawnAfterSelectionFill` updated to identify
border edges by colour rather than by a `StrokeRect` call that no
longer exists in the drawing path.

## [0.4.40] - 2026-08-19

### Fixed

- **Spectrum 128 theme: the cyan selection highlight overlapped the
  dropdown's own black border.** `Menu.Draw` and `MenuBar.Draw` both
  drew the border stroke before the item/label rows, and a highlighted
  row's fill spans the same full width the border traces -- so the
  fill painted over the border at the left/right edges. Fixed by
  drawing the border last in both, on top of everything else, so it
  always reads as an unbroken frame regardless of what's highlighted
  underneath.
- **Spectrum 128 theme: the top menu bar was white-on-black instead of
  black-on-white, unlike the real 128K/+2/+3 menu's own title strip.**
  `MenuBar.Draw` used `theme.Panel`/`Text` (the same pair the dropdown
  panel below it uses) for the bar's own background/label colours.
  Fixed: the bar now uses `theme.Sidebar`/`SideText` instead -- a pair
  every theme already had, `Spectrum128Theme` already set correctly
  (near-black/white) when the theme shipped, it just wasn't wired to
  where it needed to be.
- **Light theme: the selected menu item's text was unreadable against
  the blue highlight.** Selected-row text used `theme.Text` (near-black
  in Light) regardless of what it was drawn over. New `Theme.SelText`
  field, used whenever a row is the highlighted one: white for Light,
  matching its selection blue; a near-white already appropriate for
  Dark's selection colour, made explicit; black for Spectrum 128,
  matching the real menu's own cyan-highlighted row.

### Added

- **Font zoom: X1/X2/X3, three new items in the existing Font menu.**
  Applies only to dropdown text/layout, never the bar strip itself,
  which always draws at X1. `MenuConfig.Scale` and
  `MenuBarConfig.Scale` (new, both optional, defaulting to the
  package's existing fixed scale when unset) thread a per-dropdown
  scale through `Menu`/`MenuBar` in place of the previously-fixed
  `dlgScale` constant; `MenuBar.SetScale` changes it live, applied the
  next time a dropdown opens rather than resizing one already open out
  from under an in-progress interaction. X2 is the default -- picking
  nothing changes nothing from what every dropdown already drew at.

16 new tests across `pkg/zenui` and the menu bar wiring cover all of
the above: border-drawn-after-fill ordering (both `Menu` and
`MenuBar`), `SelText` actually being used for highlighted rows,
`MenuBar` using `Sidebar`/`SideText` rather than `Panel`/`Text`, scale
threading from `MenuBarConfig`/`SetScale` through to an opened
dropdown, and the zoom menu's three actions each applying their own
correct level.

## [0.4.39] - 2026-08-18

### Fixed

- **Menu options could fire before the bar had fully unrolled,** opening
  a dropdown anchored to the bar's mid-animation position rather than
  its resting one -- the reported "menu floating mid-air while the bar
  hasn't unrolled yet" behaviour. `appMenuBar.Update` called
  `zenui.MenuBar.Update` (hover-to-open, click-to-select) unconditionally
  every frame; hit-testing during `barSlidingIn`/`barSlidingOut` ran
  against `labelRects` from whatever Y the bar's last, still-animating
  Draw call had rendered at, not its final position. Fixed: widget
  interaction is now gated on `b.state == barShown` -- fully, restingly
  visible, not merely on screen. 2 new tests
  (`TestUpdateSkipsWidgetInteractionWhileAnimating`,
  `TestUpdateAllowsWidgetInteractionWhenShown`) confirm the gate holds
  across every non-`barShown` state without panicking and doesn't block
  the `barShown` path itself.

## [0.4.38] - 2026-08-18

### Added

- **Switchable UI themes: Dark, Light, and Spectrum 128.** Two new
  menus in the top menu bar, `Theme` and `Font`, both applying live --
  every dropdown reads the current theme/font fresh each frame, so a
  change takes effect everywhere immediately, no restart needed.
  - **Dark** (`zenui.DarkTheme`, the default): the same scheme the bar
    has always used, with two deliberate adjustments -- the dialog
    panel/field pairing reads more distinct from each other, and the
    selection highlight shifts from a flat blue to a richer
    blue-violet.
  - **Light** (`zenui.LightTheme`): a macOS Aqua-inspired scheme --
    light grey panels, near-white fields, near-black text, the classic
    Aqua selection blue.
  - **Spectrum 128** (`zenui.Spectrum128Theme`): the real Sinclair
    128K/+2/+3 boot menu's own palette, not a generic "retro" scheme --
    white panel, black text, bright cyan selection, verified directly
    against this project's own `docs/zenzx-model-catalog.pdf`.
  - Font choices: the bundled Sinclair face (the default) plus five
    more BDF faces newly bundled into `pkg/fonts` -- TomThumb, Spleen,
    Cozette, Creep, and HaxorMedium.
- New: `pkg/zenui/theme_presets.go` (`ThemeName`, `Themes`, `LoadTheme`,
  `DarkTheme`/`LightTheme`/`Spectrum128Theme`) and `pkg/fonts`'s
  `Name`/`All`/`Load` (mirroring the theme API's shape). 9 new tests
  across both packages plus `menubar_gui_test.go`'s theme/font
  application tests.

### Fixed

- **A real colour-multiplication bug in `pkg/zenuiraylib.BDFText`,
  found while building theme switching.** Glyph textures were cached
  with a fixed ink colour baked into their pixels at construction
  time, then `Draw`'s separate tint parameter multiplied on top of
  that at draw time. Harmless with the previous single, near-white
  Dark theme, but would have been catastrophic with Spectrum 128's
  black text as the baked-in ink -- every tint multiplying against
  black renders black regardless of what colour was actually
  requested. Fixed: glyphs now cache as white-on-transparent masks,
  with colour coming entirely from the draw-time tint, the way a
  tintable glyph atlas is supposed to work. `NewBDFText`'s signature
  dropped its now-meaningless `ink` parameter accordingly.

## [0.4.37] - 2026-08-18

### Fixed

- **Hi-colour mode now honours FLASH, matching real hardware.** Both
  `videorender_hicolour.go`'s `Decode` and `videorender_gpu.go`'s
  `hicolourVideoRenderer.RenderGPU` now swap ink/paper per 8x1 row on
  the same `screen.flashEnabled`/`screen.flashTickTock` timer standard
  mode already uses. The previous "FLASH is standard-only" design
  comment was a mistaken description of a scope decision as if it
  reflected real hardware behaviour -- it didn't. Corroborated by the
  ZX-Uno manual's own attribute description for this mode
  ("paper/ink/bright/flash attribute per each 8x1 pixels block") and
  this project's own `docs/timex-modes.md`, both describing the
  ordinary Spectrum attribute layout, FLASH bit included. Nothing
  found suggests real hi-colour hardware disables or ignores it.
  `docs/video-architecture.md` corrected to match.
  `TestHicolourFlashIgnored` replaced with
  `TestHicolourFlashSwapsInkAndPaper` and
  `TestHicolourFlashDisabledNeverSwaps`, mirroring the equivalent
  standard-mode guards added in 0.4.35.

## [0.4.36] - 2026-08-17

### Fixed

- **Restored the original GPU texture-blitting renderer for the GUI's
  live display, lost when the VideoRenderer abstraction (needed for
  hi-colour and headless screenshot support) replaced it with a
  portable CPU-side path.** The portable `Decode()` path -- still used
  unconditionally by the headless build, and by any renderer without a
  fast path -- is unchanged. But the GUI's live rendering now draws
  through `FastGUIRenderer` (`videorender_gpu.go`) when the active
  renderer implements it: 256 pre-baked bit-pattern textures (one per
  possible bitmap byte, an 8x1 mask of which pixels are "on") and 16
  pre-baked solid-colour textures, all GPU-resident and baked exactly
  once in `InitializeAfterWindow` -- never re-uploaded per frame. Each
  frame issues draw calls only: one tinted texture blit per attribute
  cell (paper) and per bitmap byte (ink), composited directly onto the
  window, no CPU-side pixel buffer and no intermediate render-texture
  hop. This restores the exact technique this project's own v0.4.2
  release used, verified directly against that release's source.
  `standardVideoRenderer` and `hicolourVideoRenderer` both implement
  it -- hi-colour's version is new, extending the same technique to
  its own per-byte (rather than per-cell) attribute granularity.
  Compile-time checks (`var _ FastGUIRenderer = ...`) guard both
  against a future accidental signature drift silently falling back to
  the slower path.
- `DisplayManager.Render`'s signature changed from a pre-decoded
  `*image.RGBA` to raw `(mem, screen)`, so the decision of whether to
  decode at all now lives inside `Render` itself -- the CPU-side
  `Decode()` no longer runs (and is immediately discarded) on frames
  where the fast path is used.

### Note on verification

Headless build, all existing tests, and both build targets are
confirmed clean, including a real CLI run of both standard and
hi-colour modes through the still-unconditional `Decode()` path
headless uses. The fast path itself draws directly via raylib calls
with no CPU-readable intermediate, so its actual on-screen output
needs visual confirmation with a real display, which this environment
doesn't have -- verify by running the GUI build directly.

## [0.4.35] - 2026-08-17

### Fixed

- **T-18: FLASH stopped blinking on every model, not just TS2068.**
  `updateFlash()` was fully correct in isolation, but nothing called
  it -- `display.go`'s `Render()` was missing the call its own design
  comment documented as existing. Fixed by adding
  `dm.screen.updateFlash()` as `Render`'s first line. 4 new tests
  (`flash_regression_test.go`): standard-mode ink/paper swap
  correctness, `flashEnabled=false` fully disabling FLASH rather than
  just leaving it un-ticked, and `updateFlash`'s own 320ms timing
  directly. See `RESOLVED.md` for the full record.

## [0.4.34] - 2026-08-17

### Changed

- **Menu bar reveal distance widened from 5px to 10px** from the top
  window edge -- still 100ms idle to trigger.
- **Machine and Custom ROM now open on hover, not click** -- the bar's
  dropdowns follow a traditional desktop menu bar's behaviour once any
  dropdown is engaged: hovering a different label switches directly to
  it, no click needed.
- **The bar and its dropdowns are now general, reusable components,
  not zenzx-specific code.** `pkg/zenui.MenuBar` (new) is a
  renderer-agnostic horizontal strip of labels, each opening a `Menu`
  (already shipped) -- it has no opinion about *when* or *how* it
  appears on screen; that presentation policy (the idle-at-edge
  trigger, the eased slide) stays entirely in zenzx's own
  `menubar_gui.go` as the host's choice, not the widget's. 7 new tests
  (`pkg/zenui/menubar_test.go`) covering label layout, hover-opens,
  hover-switches-without-a-click, hovering the same label not
  rebuilding its dropdown, accepted-selection correctness, forced
  Close, and drawing at a non-zero (mid-slide) Y.

### Added

- **Four new populated menus: Tape, Floppy Disk, Snapshot, View** --
  every one wired to the exact same functions the existing keyboard
  shortcuts already call, not reimplemented:
  - Tape: Play/Stop, Rewind, toggle Accurate/Fast, show info
    (`Alt+P/R/T/I`).
  - Floppy Disk: Save, Insert Blank, Eject, Save As, disk-loading info
    (`F4`-`F8`).
  - Snapshot: Quick Save/Load, Save Timestamped, snapshot info, run
    diagnostics (`F9`-`F12`).
  - View: toggle FPS, toggle border, show status (`Alt+F`, `Alt+B`,
    `F3`).
  - Machine also gained Reset and Pause/Resume alongside its existing
    model list (`F1`/`F2`).
  `ToggleTapeMode`/`PlayStopTape`/`RewindTape`/`ShowTapeInfo`/
  `TogglePause` (`input.go`) are new, callable methods extracted from
  what was previously inline logic inside `handleAltCombinations`/
  `handleFunctionKeys` -- the keyboard shortcuts call the exact same
  methods the menu does now, rather than the menu duplicating their
  logic.
- `zenui.MenuBar.SetItems`: replaces one top-level item's dropdown
  contents at runtime (rebuilding it immediately if that item is the
  one currently open) -- what makes Custom ROM's directory-listing-
  dependent menu, and its own ROM-then-bank two-step flow, possible
  without the general widget knowing anything about ROMs or banks.

## [0.4.33] - 2026-08-17

### Added

- **A real, always-available top menu bar: live Machine switching and
  live Custom ROM selection.** Hold the mouse still within 5px of the
  top window edge for 100ms and the bar slides in (eased in-out over
  220ms, not linear); move away with nothing open and it slides back
  out. Two menus:
  - **Machine** -- switches the running model live (`switchModelLive`,
    `modelswitch_gui.go`): reloads the target's standard ROM set,
    resets, and resets the joystick to that model's own default.
    Mirrors the startup `-model` switch's ROM loading and +3 FDC
    handling exactly, minus the CLI-only flags (`-disk`, `-noFdc`,
    `-debugFdc`) that have no live equivalent. FDC is unconditionally
    disabled first, then re-enabled only for plus3/spanishplus3, so
    switching away from +3 doesn't leave it running against a ROM set
    that no longer expects it.
  - **Custom ROM** -- same two-step ROM-then-bank flow as
    `-custom-roms-menu`, applied live (`applyCustomROMLive`) rather
    than only at startup.
  Unlike `-custom-roms-menu` and the demo overlay (`Shift+F1-3`), both
  of which block their own frame loop, the bar runs inside the main
  loop itself -- CPU stepping and the Spectrum's own screen never
  pause while it's open. Only `zx.HandleInput` is skipped for that
  frame, withholding keyboard input from the emulated machine without
  touching anything else.
  New: `menubar_gui.go` (`menuBar`, `easeInOutCubic`, idle-at-edge
  detection). 5 new tests (`menubar_gui_test.go`) covering the easing
  curve's endpoints/monotonicity/ease shape, hit-testing, and the
  bar-label-to-dropdown index mapping.

## [0.4.32] - 2026-08-17

### Added

- **A demo overlay proving the widget system runs live over the running
  emulator, not just at startup.** `Shift+F1`/`F2`/`F3` show a bogus
  dropdown menu, animated notification, and file-open dialog
  respectively, drawn over a darkened emulator screen while the
  Spectrum keeps running underneath -- CPU stepping and its own screen
  updates never pause. Only keyboard input to the emulated machine is
  withheld while a demo widget is active (`zx.HandleInput` is skipped
  outright for that frame, not filtered), so nothing leaks through by
  accident.
- `DisplayManager.SetPreEndDrawHook` (`display.go`): lets a caller draw
  on top of a frame `Render` just produced, still inside its
  `BeginDrawing`/`EndDrawing` bracket. Real, necessary fix along the
  way -- an initial version called the overlay's draw after `Render`
  returned, which is outside that bracket and doesn't land on the
  frame actually presented in raylib. The hook exists specifically so
  callers don't have to duplicate `Render`'s own begin/end handling.
- New: `gui_demo_overlay.go` (`demoOverlay`, `newDemoOverlay`,
  `TriggerMenu`/`TriggerDialog`/`TriggerNotification`) -- built on the
  `pkg/zenui`/`pkg/zenuiraylib` widgets already shipped in 0.4.30/
  0.4.31, darkening via `zenui.Theme.Backdrop`, the same dim layer
  `Dialog`/`Menu` already draw against when used normally.

## [0.4.31] - 2026-08-17

### Added

- **`pkg/zenuiraylib.OSD`: an animated on-screen status caption.** A
  message rises from the bottom-right corner, holds, then fades over
  a fixed travel distance, with small animated spark sprites scattered
  around it -- the same confirmation style used for menu selections.
  `Start`/`Update`/`Draw`/`Active` mirror the calling convention
  `zenui.Menu` already uses (drive it once per frame, check status).
  Wired into the GUI build's custom ROM menu: picking a ROM now plays
  a brief "Loaded X -> bank N" caption after the menu closes, reusing
  the same font/renderer the menu itself just drew with rather than
  loading a second copy.

## [0.4.30] - 2026-08-17

### Added

- **The GUI build's custom ROM menu (`-custom-roms-menu`) is now drawn
  on screen instead of prompted at the terminal.** Same two-step flow
  as before -- pick a ROM, then (for multi-bank models) pick which
  bank -- but rendered as an actual on-screen menu through the
  Sinclair bitmap face (the ZX Spectrum 48K ROM's own 8x8 character
  set), matching the visual language the rest of the toolchain uses.
  The headless build has no window to draw into and keeps the stdin
  prompt (`custom_roms_menu.go`, unchanged).
- New packages, all under `pkg/`: `zenui` (renderer-agnostic dialog/
  menu widgets -- imports nothing beyond the standard library and
  `pkg/zxpalette`), `bdf` (a standalone BDF bitmap font reader),
  `zxpalette` (the ZX Spectrum colour palette and attribute-byte
  encoding), `fonts` (embeds the Sinclair bitmap face), and
  `zenuiraylib` (the raylib-specific text renderer and `zenui.Renderer`/
  `Input` adapter -- the only one of these that imports raylib, kept
  separate so the headless build never needs to link against it).
  `pkg/zenui`/`pkg/bdf`/`pkg/zxpalette` carry real test coverage
  already; `pkg/fonts` gets 2 new tests confirming the embedded font
  decodes correctly.
- `custom_roms_menu_gui.go`: the GUI-side driver (`runGraphicalCustomROMMenu`,
  `runMenuLoop`) wiring the above into the same `OverrideROMBank`
  mechanism `-rom0` through `-rom3` and the headless menu already use.

## [0.4.29] - 2026-08-17

### Added

- **`custom-roms/` and `-custom-roms-menu`: an interactive selector for
  alternate ROM variants.** `custom-roms/` is for language-translated
  and other regional variants of the standard set (empty by default,
  distinct from `rom/`, the standard embedded set). `-custom-roms-menu`
  scans it, lists every `.rom` file found with its size, prompts for a
  selection, and -- for multi-bank models -- asks which bank it should
  replace, applying it through the same `OverrideROMBank` mechanism
  `-rom0` through `-rom3` already use. Applied last, after every other
  ROM-loading path, so an interactive selection always wins over
  whatever `-model`/`-rom`/`-romN` already set up.
  `-custom-roms-dir <path>` points the selector elsewhere if needed.
  An empty directory, a skipped selection (0), or invalid input all
  leave the standard ROM set in place rather than erroring out --
  none of these are failure states.
  New: `custom_roms_menu.go` (`listCustomROMs`, `runCustomROMMenu`,
  `readInt`). 7 new tests (`custom_roms_menu_test.go`), including
  end-to-end CLI verification across single-bank and multi-bank models,
  an empty directory, and an out-of-range selection.
- `custom-roms/README.md`: usage for the new directory and selector.

## [0.4.28] - 2026-08-17

### Added

- **The standard ROM set is now embedded directly in the binary.**
  Every `.rom` file in `rom/` (`go:embed rom/*.rom`, `embedded_roms.go`)
  is compiled into the executable, so a distributed release no longer
  needs a separate `rom/` folder shipped alongside it. Verified by
  booting all 10 models to a screenshot from a completely empty
  directory with `-romdir` pointed at a path that doesn't exist --
  every one falls back to the embedded copy correctly.
- `resolveROMBytes` (`embedded_roms.go`): filesystem-first, embedded-
  fallback resolution for a named ROM. Deliberately filesystem-first,
  not embed-first -- this keeps every existing local development and
  test workflow that already points `-romdir` at a real checkout
  working completely unchanged; the embed is consulted only when a
  named file isn't found on disk.
- `LoadROMBytes`/`Load128KROMBytes`/`LoadPlus3ROMBytes`/
  `LoadTS2068ROMBytes`: byte-data primitives behind the existing
  path-based loaders, which are now thin wrappers around them. This is
  what let the standard `-model` loading path and the embed share one
  code path instead of two parallel ones.
- Not embedded, deliberately: documentation (`.txt`), the reference hex
  dump (`48.rom.hex`), and the Z80 exerciser test programs (`zexall`/
  `zexdoc` `.com`/`.z80`) -- none of these are consulted by `-model`'s
  own loading path, so bundling them would only grow the binary for no
  runtime benefit.

### Fixed

- **`zenzx_gui.go`'s `plus2a` case was silently broken.** It pointed at
  `plus2a-0.rom` through `plus2a-3.rom`, files that never existed in
  `rom/` -- every +2A launch via the GUI binary was silently falling
  through to a generic +2 ROM with a "not fully accurate" warning,
  never the real thing. Fixed to load the genuine +3 ROM set (the
  correct approach, matching real +2A hardware sharing the +3's
  motherboard, and matching what `zenzx_headless.go` already did
  correctly). Also fixes two `Load128KROM` calls in the same file that
  mixed `./rom/` and `../rom/` prefixes for their two file arguments --
  a bug that only ever worked by accident depending on the working
  directory the GUI binary happened to be launched from.

## [0.4.27] - 2026-08-17

### Added

- **Per-bank ROM overrides: `-rom` and `-rom0` through `-rom3`.**
  Previously `-rom` was a single-file, all-or-nothing override that
  replaced `-model`'s own ROM loading entirely -- it had no way to swap
  just one bank (e.g. only +3DOS on an otherwise-standard +3) and no
  way to specify more than one file at all. Now `-model`'s own standard
  ROM set always loads first, and overrides layer on top of it:
  - `-rom` takes a comma-separated list of paths, positionally mapped
    to bank 0, 1, 2, 3 (up to however many banks the selected model
    actually has).
  - `-rom0`/`-rom1`/`-rom2`/`-rom3` each override exactly one bank,
    leaving every other bank as `-model` loaded it -- the mechanism
    for the "just swap +3DOS" case directly.
  - Both compose: `-rom` fills in from bank 0 first, then any `-romN`
    individually overrides that specific bank regardless of what `-rom`
    already did.
  - Per-model bank-count validation (`zenzx.maxROMBank`): a 48K model
    only has bank 0, 128K/+2 has banks 0-1, +2A/+3 has banks 0-3,
    TS2068 has its own two banks (bank 0 = 16K Home, bank 1 = 8K
    Extension -- validated at their own real sizes, not assumed
    uniform). An out-of-range or wrong-sized override is a warning, not
    a fatal error -- the standard ROM for that bank stays in place and
    the emulator keeps running.
  New: `memory.go`'s `SetROMBank` (the in-place single-bank primitive)
  and `zenzx.go`'s `OverrideROMBank`/`maxROMBank`. 9 new tests
  (`rombank_test.go`).

## [0.4.26] - 2026-08-17

### Fixed

- **Investigated and confirmed: Kempston Mouse does not take over
  either Kempston joystick port, and all three can coexist.** Kempston
  Mouse is a genuinely separate physical device from Kempston Joystick
  -- confirmed by a first-hand account on Spectrum Computing's forums
  from someone who owned one: "a custom interface" with its own
  dedicated ports, not something that plugs into a joystick port at
  all. Its ports (`0xFADF`/`0xFBDF`/`0xFFDF`, low byte `0xDF`) share no
  address with either Kempston joystick port (`0x1F`, `0x37`). The
  precise reason this can go wrong on *some* real hardware but not
  others: the ZX-VGA-JOY interface's own design documentation names it
  directly -- interfaces using incomplete address decoding (checking
  only a few bits, not the full byte) could accidentally collide with a
  Kempston Mouse plugged in alongside one; "Best is use full 8-bit
  address decoding" avoids it entirely. zenzx already does exactly
  that for every relevant port, confirmed by reading the actual
  dispatch code, not assumed. `TestKempstonMouseCoexistsWithDualKempstonJoysticks`
  (`joystick_test.go`) proves it: both Kempston joystick ports and a
  Kempston mouse, all active simultaneously, all independently correct.
- **Found and fixed a real gap while investigating this**: the startup
  validation rejecting `-joystick kempston` with `-mouse amx` (both
  genuinely use port `0x1F`) never covered `-joystick kempston-both`,
  which also uses `0x1F` for its first sub-port and has the identical
  conflict. Extended the check in both `zenzx_headless.go` and
  `zenzx_gui.go`. `kempston2` alone (port `0x37` only) is correctly
  unaffected -- confirmed no real conflict exists there.
- `mouse.go`'s package documentation extended with the full
  investigation and its sources, so this doesn't need re-deriving next
  time the question comes up.
  2 new tests (`joystick_test.go`).

## [0.4.25] - 2026-08-17

### Added

- **Second Kempston port** (`JoystickKempston2`/`JoystickKempstonBoth`,
  port `0x37`) -- confirmed real, but genuinely different in kind from
  the +2/+3's built-in dual Sinclair ports added in 0.4.24: no stock
  Sinclair/Amstrad/Timex machine ever had a second Kempston port, so
  `-model`'s `auto` default never selects it. It belongs to modern
  "neo-Spectrum" platforms -- ZX-Uno, ZX-Tres, the Omni, and the ZX
  Spectrum Next (originally codenamed TBBlue) -- confirmed directly
  against the Next's own official I/O port register documentation
  (`specnext.com/tbblue-io-port-system`: register 0x05, "Kempston 2
  (port 0x37)"), cross-checked against a separate, independent
  source's own port numbering (the modern "KEMPSTON_MAX 2" hobbyist
  interface's documented "Kempston Port 55" -- 55 decimal = `0x37`
  hex). `kempston2` selects the second port alone; `kempston-both`
  drives both simultaneously from two host gamepads, mirroring
  `sinclair-both`'s existing pattern. Explicit-choice only, the same
  category `-ns-graphics`/`-ns-storage` already occupy for exactly this
  kind of platform-specific enhancement.
  6 new tests in `joystick_test.go`, including one confirming no
  `-model` value ever implies either new mode.

### Fixed

- Corrected the TS2068 joystick-compatibility framing from the previous
  entry: its built-in ports are architecturally "Kempston-style" (a
  dedicated digital port read, not a keyboard-remapping mechanism like
  Sinclair/Cursor) but not protocol-compatible with either Kempston
  port modelled here -- `ts2068.go`'s own AY-register-14 mechanism,
  unchanged, remains correct. `joystick.go`'s package documentation now
  states this precisely rather than leaving "Kempston-style" ambiguous
  between the category and the literal protocol.
- Fixed a dangling, incomplete doc comment left over from an earlier
  edit this session (`SetJoystickMode`'s own comment had lost its first
  line) while rewriting the file for the additions above.
- Flagged, rather than silently resolved, a genuine discrepancy found
  while researching the above: the ZX Spectrum Next's own register
  documentation labels the Sinclair ports the opposite way round from
  this project's existing numbering ("Sinclair 1 (12345)" there vs.
  here). Kept the existing numbering, which has two independent,
  physically-grounded sources (Wikipedia's explicit statement on which
  keys map to "Player 1," plus the Interface 2's own dimpled-socket
  identifies-Joystick-1 hardware detail) against the Next's one
  internal firmware-labelling convention, which may simply be that
  board's own choice rather than a disagreement about the underlying
  keys. Documented in `joystick.go` for whoever revisits this next.

## [0.4.24] - 2026-08-17

### Added

- **`-joystick` now defaults to `auto`: the selected `-model`'s own real
  built-in configuration, not a flag the user must separately know to
  set.** Verified against real hardware history (searched, not assumed)
  rather than taken at face value from a plausible-sounding claim:
  - **48K, and the original Sinclair 128K** ("Toastrack", pre-Amstrad):
    confirmed no built-in joystick port of any kind existed --
    Sinclair-compatible ports were, per spectrumforeveryone.com's own
    +2 history, "a feature that really should have been part of the 128
    specification from the start," only arriving with Amstrad's +2.
  - **+2 (grey), +2A, +3** (and Spanish variants): two built-in
    Sinclair-protocol ports ("SJS"), confirmed genuinely Sinclair
    Interface 2's own keyboard-mapping mechanism electrically, not
    Kempston -- cross-checked against the ZX Interface 2 circuitry
    reference and sharedmemorydump.net's own +2/+3 port testing ("Where
    a Kempston joystick can be read from I/O-port 31... the Sinclair
    joystick just emulates keyboard keys").
  - **TS2068**: confirmed its built-in ports are *not*
    Kempston-compatible, directly against timexsinclair.com (the
    Timex/Sinclair preservation project): "the Timex joystick ports
    were not compatible with the most popular ZX Spectrum standard, the
    Kempston joystick." Corrects an assumption stated earlier in this
    session's conversation before this check -- the genuinely
    Kempston-equipped Timex machine is the TC2048, a different, related
    computer this project does not emulate. TS2068's own mechanism
    (AY register 14, already implemented and correct) remains active
    for that model regardless of `-joystick` entirely, since the
    canonical-Spectrum mechanism this flag configures doesn't apply to
    it -- `auto` correctly resolves to `none` there, not a guess.
  - `-mouse` deliberately keeps its universal `none` default across
    every model: no stock Sinclair/Amstrad/Timex machine ever shipped
    with a built-in mouse (both AMX and Kempston Mouse were always
    third-party add-ons, TS2068 included, per this session's earlier
    T-14 research) -- verified as already correct rather than assumed
    to need a matching change.
  An explicit `-joystick` value always overrides the model default --
  third-party interfaces (Kempston most commonly) remained a legitimate
  choice historically even on models with their own built-in ports, and
  remain one here.
- **Sinclair Joystick 2** (`JoystickSinclair2`, `joystick.go`) and a new
  **`JoystickSinclairBoth`** mode driving both of the +2/+2A/+3's real
  built-in ports simultaneously from two independent host gamepads --
  completing part of T-13 sooner than planned, once it turned out to be
  required (not merely a nice-to-have) to correctly represent hardware
  that genuinely has two ports, not one. `input.go`'s
  `handleJoystickInput` now polls gamepad 0 for port 1 and gamepad 1 for
  port 2 when `SinclairBoth` is configured, mirroring the same
  dual-gamepad convention already established for TS2068's own two
  built-in ports.
  10 new/updated tests (`joystick_test.go`): both key matrices, the
  dual-port state application, `defaultJoystickModeForModel` against
  every supported `-model` value, and `resolveJoystickMode`'s `auto`/
  explicit-override behaviour.
- **T-17 filed**: a related but pre-existing gap, noticed while
  auditing port dispatch for this work, not introduced by it -- the
  128K-style AY-3-8912 ports (`0xFFFD`/`0xBFFD`) respond regardless of
  `-model`, including on 48K, which has no AY chip at all (beeper
  only). Same underlying principle (`-model` should imply which
  hardware genuinely exists) applied to a different subsystem; recorded
  rather than silently expanding this session's scope to fix it.

### Fixed

- **T-13 updated**: Sinclair Joystick 2 done (see above). Cursor/AGF/
  Protek joystick support and headless zenscript joystick driving
  remain open.
- `README.md`'s `-joystick` documentation rewritten with the full
  per-model mapping and citations, replacing the earlier, incomplete
  note that only mentioned TS2068.
- `docs/KNOWN_ISSUES.md`'s TS2068 entry extended with the corrected
  joystick-compatibility fact and its source.

## [0.4.23] - 2026-08-17

### Fixed

- **T-12 closed: TS2068 promoted from "code exists" to "documented,
  supported model"** (see `docs/RESOLVED.md` for the closure record).
  5 of 6 stages complete; Stage 6 (memory contention) postponed as a
  deliberate, recorded decision rather than left looking like stalled
  work -- spun out into a general, non-TS2068-scoped item (T-16) once
  investigation found no contention modeling exists for *any* model to
  diverge from. Concrete gaps closed, not just the tracking document:
  - `README.md`: the opening description and the `-model` flag table
    didn't mention TS2068 at all. Fixed, along with a broader staleness
    the search turned up while in there -- `-non-standard`, `-ns-graphics`,
    `-ns-storage`, `-joystick`, `-mouse`, `-disk`, `-nofdc`, and `-script`
    were all missing from the flags table too, some since earlier this
    session. Also replaced the "Known issues" entry describing the old,
    unhardened fast tape loader (T-02, closed) with an accurate current
    one for TS2068 itself.
  - `docs/KNOWN_ISSUES.md`: TS2068's real, intentional scope limits
    (no Dock/cartridge banking, no TS2040 printer, dynamic port-driven
    hi-colour switching instead of the documented `CHNG_VID` service,
    no tape-save capture, no contention) recorded formally, not left
    implicit in `TRACKING.md`/`RESOLVED.md` history.
  - `smoke_headless.sh`: the release gate's own "does the real compiled
    binary actually boot" check only ever exercised `-model=48k` --
    TS2068 was covered by unit tests but not by this layer. Now boots
    both.
  - `docs/TS2068_TRACKING.md`: Stage 6 marked `✗` (dropped) with a
    deviation entry explaining why, rather than left `☐` looking like
    still-pending work; its header `Version:` field, stale since the
    document's creation (0.4.11), synced to the current release.

## [0.4.22] - 2026-08-17

### Added

- **`-model ts2068` Stage 5: tape I/O, both fast and accurate mode.**
  Direct disassembly of `rom/ts2068-1.rom` found `R_TAPE`/`W_TAPE` are a
  byte-for-byte relocated copy of the standard 48K ROM's `LD-BYTES`/
  `SA-BYTES` (a constant offset of `0x045A` -- `0x0556-0x00FC` =
  `0x04C2-0x0068`), with an identical register contract -- so T-02's
  freshly-hardened `trapLoad` is reused completely unchanged for TS2068;
  only the trap addresses (`0x0112`, the same "after real ROM
  housekeeping, before the slow pulse call" positioning `0x056C` uses)
  and a new context guard differ. `isTS2068ExtensionROMActive`
  (`tape.go`) confirms chunk 0 is genuinely switched to Extension ROM
  before trusting the trap -- these addresses are otherwise ordinary
  low addresses that mean nothing in particular during normal Home Bank
  execution. `TestTS2068FastLoaderRealROMIntegration` proves the whole
  chain: a hand-assembled program engages chunk-0 banking itself (the
  same port sequence real Extension ROM Interface Routine calls use)
  and `CALL`s the genuine `R_TAPE` entry directly.
  Accurate mode required no new code at all -- confirmed, not assumed:
  `TestTS2068AccurateModeRealROMIntegration` runs a complete,
  real, pulse-by-pulse load through the actual ROM using the same
  `genPulses` the ordinary `.tap` loading pipeline uses, with the fast
  loader deliberately absent so the full polling loop has to run to
  completion. Confirmed practical rather than assumed impractically
  slow: 809,629 real CPU steps, about 30ms wall-clock.
  Save-side (`W_TAPE`) correctly declines rather than misfire, matching
  T-02's scope for the standard models: zenzx has no tape-save capture
  to hook into for either.
  6 new tests in `tape_fastload_test.go`.

## [0.4.21] - 2026-08-17

### Fixed

- **T-02 closed: hardened the ROM-trap fast tape loader from first
  principles** (see `docs/RESOLVED.md` for the closure record), prompted by a direct challenge that the existing
  implementation was likely to misbehave and should be checked against
  a real reference before trusting it for TS2068 work. Cloned
  MartianGirl's SpecIde (github.com/MartianGirl/SpecIde, already local
  from the AMX mouse research) and read its own `LD-BYTES`/`SA-BYTES`
  trap implementation directly. The gap was real and significant, not
  just theoretical:
  - **No ROM-context check** -- fired on PC match alone, regardless of
    which ROM bank was actually paged in on 128K/+3.
  - **Wrong trap address**: the old code trapped at `0x0556`, LD-BYTES's
    literal entry point. Verified directly against `rom/48.rom`'s own
    bytes that the real routine's first instruction there is
    `EX AF,AF'`, followed by a border flash and a genuine BREAK-key
    check -- all real ROM housekeeping the old trap silently skipped.
    The correct trap point, also verified against the real ROM, is
    `0x056C`: immediately before the slow pulse-reading call
    (`CALL 0x05E7`) this replaces, letting everything before it run as
    genuine ROM code.
  - **No VERIFY mode** -- the Carry flag (LOAD vs VERIFY) was read into
    a comment but never actually checked; every call was treated as
    LOAD.
  - **No flag-byte matching, no checksum** -- Carry was hardcoded to
    always indicate success, regardless of whether the "loaded" block
    actually matched what the caller asked for.
  - **A second, looser fallback**: any PC in a 175-byte range
    (`0x0556`-`0x0605`) triggered an instant block injection outright,
    and a third path injected every CODE block the instant tape
    playback started, with no relationship to whether the CPU had
    asked to load anything at all.
  - **`Playing` cleared after every single block**, breaking ordinary
    multi-block loading (header then data each need their own separate
    call into `LD-BYTES`) -- confirmed as a real bug via a new
    regression, `TestFastLoaderMultiBlockContinues`.

  Rewrote `tape.go`'s fast-load path around a single, precise trap:
  `is48KROMActive()` (new, `memory.go`) confirms the right ROM bank is
  genuinely paged in -- verified against the actual shipped ROM bytes,
  not assumed, for every model (48K's only bank; 128K/+2's bank 1;
  +3/+2A's bank 3; TS2068 explicitly excluded, since its tape routines
  are entirely different Extension ROM code). `trapLoad` replicates the
  real routine's exact contract: reads the expected flag byte and
  LOAD/VERIFY carry from the shadow registers (confirmed via direct
  disassembly that `EX AF,AF'` has already run by the trap point),
  matches it against the tape block's real flag, computes a genuine
  running XOR checksum, handles VERIFY (compare, abort on first
  mismatch -- confirmed against the real ROM's own verify loop) as a
  first-class case rather than always writing, and sets exit registers
  matching SpecIde's own independently-derived conventions for this
  same routine. The two heuristic fallback paths and their
  now-superseded helper functions (`fastInjectNextBlock`,
  `fastInjectAll`, `instantLoadHeaderAndData`) are gone entirely --
  fast mode is now a precise short-circuit for the standard ROM loader
  and nothing else; non-standard/custom loaders need `-tapemode
  accurate`, not a guess.

  17 tests (`tape_fastload_test.go`), including
  `TestFastLoaderRealROMIntegration`: a hand-assembled program executed
  via genuine `zx.cpu.Step()` calls that `CALL`s the real `rom/48.rom`
  LD-BYTES entry point directly, proving the entire path end to end --
  real ROM preamble runs for real, the trap fires naturally when
  execution reaches it, and control returns correctly to the original
  caller -- not a synthetic Go-level call into the trap function.
  Save-side (`SA-BYTES`) capture remains unimplemented (zenzx has no
  existing tape-save capability to harden) -- the trap point is
  checked and declines cleanly rather than risk misreading real ROM
  code at that address as something else.

## [0.4.20] - 2026-08-17

### Added

- **`-model ts2068` Stage 4: AY sound chip ports and joystick**
  (`ts2068.go`, `input.go`). `F5H` (register select, write-only per the
  Technical Manual's port map) / `F6H` (data, R/W) dispatch to the same
  underlying AY-3-8912 emulation and `ayRegister` field the existing
  128K-style ports (`0xFFFD`/`0xBFFD`) already use -- purely a new
  dispatch path, no changes to the AY chip itself. Confirmed: the low
  byte alone is matched regardless of what's in the upper address byte
  during a single-byte `OUT`/`IN` (matching the SCLD's full 8-bit port
  decode, not the partial decode the standard ULA ports rely on), and a
  real AY register-width quirk (register 3, Coarse Tune B, is genuinely
  only 4 bits wide on real AY-3-8912 hardware) flows through the new
  ports correctly rather than being masked over by the test.
  Joystick: TS2068's own built-in ports, read via AY register 14 --
  write `0x0E` to `F5H`, then read `F6H` with address bit 8 (port 1) or
  bit 9 (port 2) set. Bits 0-3 up/down/left/right, bit 7 fire, active
  low (Table 2.4.4-1's `*DIR1`/`*BUTTON` naming) -- a different polarity
  from Kempston Joystick's active-high and a different bit layout again
  from AMX Mouse's buttons, both already in the codebase. Always active
  for this model, independent of `-joystick`: these are the machine's
  own hardware ports, not an optional peripheral the way Kempston/
  Sinclair interfaces are for the canonical Spectrum. `input.go`'s
  gamepad-reading logic was factored into a shared `readGamepadState`
  helper, reused by both the existing Kempston/Sinclair path and the new
  `handleTS2068JoystickInput` (gamepad 0 -> port 1, gamepad 1 -> port 2).
  2 tests (`TestTS2068AYPorts`, `TestTS2068JoystickPorts`).

## [0.4.19] - 2026-08-17

### Added

- **`-model ts2068` Stage 3, rescoped from the original plan.** Real
  TS2068 software engaged hi-colour mode with a direct `OUT` to port
  `FFH`, not the documented Extension ROM `CHNG_VID` service call --
  and the hardware has no mechanism to clear the screen for you when
  you do; whatever was already in RAM stays there until software
  clears it itself. Timex BASIC's `PLOT`/`DRAW` never touch the
  hi-colour attribute plane either, only the standard one, so real
  hi-colour software always poked pixels and attributes by hand.
  Implemented what "CHNG_VID support" means in that corrected scope:
  `SpectrumIO.onTS2068VideoModeChange`, a closure set by `NewZenZX`,
  fires whenever TS2068 guest code writes video-mode bits to port
  `FFH` and dynamically switches the active renderer (standard <->
  `mode-timex-001-hicolour`) -- genuine guest-triggered switching,
  where previously only the static `-ns-graphics` host flag could
  select it. `TestTS2068Stage3DynamicVideoMode` seeds the hi-colour
  plane with `0xAA` garbage before running the test program, to prove
  the program's own clear is real and not merely untested; the program
  clears both planes by hand, writes mode 2 directly to port `FFH`,
  pokes 8 stripes (not via BASIC), and the result is confirmed both by
  memory assertion and by rendering through the real, unchanged
  `mode-timex-001-hicolour` renderer.
- **T-15 filed**: the original plan's target -- verifying the real
  Extension ROM `CHNG_VID` service end to end via the documented
  `IFRTN` calling convention -- was attempted first. Confirmed working
  correctly via direct register tracing: chunk-0 banking itself, `CHG_V`'s
  early logic, the nested `CALL_BANK`/`REMGSZ` round-trip into Home
  ROM (a real, bounded, correctly-terminating loop), and `OPDFIL`'s
  relocation fix-up table walk (61 real entries, correctly
  zero-terminated, confirmed progressing to completion via direct `HL`
  tracing -- an early misreading of this as "stuck" was a debugging
  mistake, caught by checking actual register values instead of
  trusting a repeating-address pattern that was actually still making
  progress). Genuine divergence found partway through `OPDFIL`'s
  screen-clear loop, landing in what looks like Home ROM print
  machinery with chunk-0 banking fully reverted -- root cause not
  diagnosed. Not pursued further since real software didn't depend on
  that path; left open in T-15 in case genuine software requiring the
  real service call (e.g. dual-screen mode) is ever found.

## [0.4.18] - 2026-08-17

### Added

- **`-model ts2068` Stages 1-2** (`ts2068.go`, `ts2068_test.go`,
  `docs/TS2068_DEVELOPMENT_PLAN.md`/`docs/TS2068_TRACKING.md`).
  **Stage 1**: model registered in both mains; `LoadTS2068ROM` loads the
  16K Home + 8K Extension ROM pair with size validation; chunk-0
  Home/Extension banking via ports `F4H` (HSR bit 0) and `FFH` (bit 7,
  EXROM/Dock select) -- both ports had to be intercepted *before* the
  existing ULA keyboard/border catch-all (`port&0x01==0`), since `F4H`
  is even and would otherwise be silently swallowed by logic that's
  correct for the standard ULA's incomplete decode but wrong for
  TS2068's SCLD, which decodes `F4H` specifically. RAM addressing
  deliberately reuses the existing 48K path unchanged (`is128K` stays
  false) rather than building new banking logic, since TS2068's RAM
  organisation is structurally identical. Verified booting to the real
  copyright screen and, confirmed by injecting an actual keypress
  (not just a static screenshot, which looks the same whether the
  machine is correctly idling at the ready prompt or genuinely
  stalled), to a fully responsive BASIC prompt: `PRINT 9` [ENTER]
  produces `9` and the genuine `0 OK, 0:1` ready line.
  **Stage 2**: `cyclesPerFrame`/`interruptCycles` are now per-`ZenZX`-
  instance fields (58688/58594 for TS2068, computed from the real
  3.528MHz/60.1145Hz, not the plan's rough estimate), threaded through
  `RunFrame`, `display.go`'s border-stripe timing (both GUI and
  headless `DisplayManager` needed the signature change), and snapshot
  cycle bookkeeping. Every other model verified unaffected -- identical
  values to the PAL constants they replace. `CPUFrequency`/
  `FramesPerSecond` and `audio_oto.go` deliberately left untouched:
  TS2068 audio isn't implemented yet (Stage 4), so nothing currently
  needs them correct for TS2068 -- revisit when Stage 4 lands.
  4 tests (`ts2068_test.go`).
- **T-12 updated to partial (◐)**: 2 of 6 stages done, detail in
  `docs/TS2068_TRACKING.md`.

## [0.4.17] - 2026-08-17

### Added

- **`-mouse amx` implemented** (`mouse.go`, `io.go`, `zenzx.go`,
  `zenzx_headless.go`, `zenzx_gui.go`). Buttons: port `0xDF`, bits 5/6/7,
  active high, polled directly with no interrupt needed -- matches
  Kempston Mouse's simplicity. X/Y: genuinely interrupt-driven, per the
  disassembly (T-14) -- one CPU interrupt per step of movement, IM2-
  vectored (`I=0xE9`, vector `0xD0`->Y at table entry `0xE9D0`, vector
  `0xD2`->X at `0xE9D2`), delivered by implementing zen80's optional
  `InterruptController` interface (`GetInterruptVector`/
  `GetMode0Instruction`) on `SpectrumIO` -- confirmed wireable with zero
  CPU-core changes, since `z80.New(memory, io)` already passes `zx.io`
  as `z.IO`, and zen80 already had a complete, tested IM2 implementation
  (`im2_vectoring_test.go`) supporting a settable per-request vector
  byte, not a hardcoded one. `ZenZX.RunFrame`'s existing frame-interrupt
  mechanism (`zx.cpu.INT = true`/`= false`, already in place for the
  standard 50Hz interrupt) is reused for AMX steps too, via a new
  `checkAMXInterrupt` call before each `cpu.Step()` -- no new CPU
  infrastructure, one more trigger condition on the existing one. A
  bounded FIFO (`amxQueueCap = 64`) queues pending steps so a burst of
  fast mouse movement or a guest with interrupts briefly disabled
  doesn't lose or corrupt events. `-joystick kempston` and `-mouse amx`
  together is a hard startup error (both use port `0x1F` on real
  hardware, confirmed in the disassembly).
  17 tests (`mouse_test.go`), including `TestAMXEndToEndInterrupt`: a
  hand-assembled Z80 program installed at the real AMX vector table
  addresses, run on a genuine `ZenZX` (not a bare `SpectrumIO`), proving
  the full pipeline -- `SetMouseState` queues a step, `checkAMXInterrupt`
  asserts the CPU interrupt line, zen80 accepts it and calls
  `GetInterruptVector`, the correct handler runs at the correct address,
  and reads the correct port value -- for both axes, not just asserted
  in isolation.
- **T-14 closed**: `-mouse amx` fully implemented and verified end to
  end (see "Added" above and `docs/RESOLVED.md`).

## [0.4.16] - 2026-08-17

### Fixed

- **T-14 (AMX mouse) resolved from a primary source, not documentation.**
  A genuine AMX Art tape was supplied this session; extracted every CODE
  block to its correct load address and disassembled with the `z80dis`
  package (confirmed independently against two copies of the same
  routine -- the dedicated "AMX" driver block and `ART 1.1`'s own
  inline copy -- both agree exactly). The mechanism is materially
  simpler than assumed: not software-side quadrature phase-decoding,
  but one CPU interrupt per step, direction pre-decoded into a single
  bit by the PIO hardware. IM2 vector setup (`I=0xE9`), PIO control
  writes at ports `0x5F`/`0x7F` (a separate control-port pair from the
  `0x1F`/`0x3F` data ports), X/Y handlers that each do exactly one `IN`
  read and one increment-or-decrement per interrupt, and buttons
  (port `0xDF`, bits 5-7 -- not Kempston Mouse's bits 0-2) polled
  separately with a 10-retry hardware debounce pattern. What's needed
  to implement this is now scoped precisely: a second, IM2-vectored
  interrupt source triggered by host mouse movement, not a new decode
  algorithm -- zenzx's Z80 core already supports IM2 for other
  purposes.
- Caught and fixed a privacy slip while writing this up: briefly wrote
  the real name into `docs/TRACKING.md` (a file that ships in the
  repo) before catching and correcting it in the same pass.

## [0.4.15] - 2026-08-17

### Fixed

- **T-14 (AMX mouse) updated with real evidence, not assumption.** Cloned
  and directly searched the source of four actively-maintained,
  accuracy-focused open-source Spectrum emulators for a working AMX
  Mouse reference implementation: Fuse, ZEsarUX, SpecEmu, and SpecIde
  (MartianGirl). None of the four contains real AMX emulation code.
  Fuse's shared `libspectrum` library has a `.szx` snapshot-format
  chunk reserved for AMX state (`read_amxm_chunk`/`write_amxm_chunk` in
  `szx.c`) that is parsed-and-discarded on read and never written on
  save -- confirming the format has a slot for it that no checked
  implementation actually fills. ZEsarUX's only "AMX" reference is a
  hardware-name label in a TZX metadata display table, not emulation.
  SpecEmu and SpecIde have no AMX or Kempston-mouse-adjacent code at
  all in the relevant files. Cross-checked against SpecEmu's own live
  changelog (five years, through April 2026 builds) to rule out "the
  checked mirror is just incomplete" as an explanation. The register
  item's conclusion is now grounded in having checked real code, not
  in an assumption that it would be hard to find.

## [0.4.14] - 2026-08-17

### Added

- **`-mouse kempston` flag**, both GUI and headless binaries (`mouse.go`,
  `io.go`, `input.go`, `zenzx_gui.go`, `zenzx_headless.go`). Three ports,
  confirmed against four independent sources (gamedev.net, zxpress.ru,
  8bit.yarek.pl, and the ZX_BUS_Mouse hardware-clone project, all
  agreeing exactly): `0xFADF` buttons (bit0=right, bit1=left,
  bit2=middle, active low -- polarity confirmed for the CPC-bus variant
  of the same interface family, cpcwiki.eu, not separately re-confirmed
  against a ZX-specific source for polarity specifically, though the
  port/bit assignment itself is independently ZX-confirmed), `0xFBDF` X
  and `0xFFDF` Y as 8-bit wrapping *relative* counters -- not absolute
  screen coordinates (k1.spdns.de's Kempston Mouse Interface page states
  this outright: software must track its own cursor position and
  recalibrate at screen edges). Partial address decode (upper 4 bits
  don't-care) matched via a 12-bit mask, per the bit table documented at
  spectrumcomputing.co.uk, not just the three literal port values.
  GUI-side input reads `rl.GetMouseDelta()` (already in host
  window-client pixels) divided by the current display magnification
  (`zx.screen.GetMultiplier()`) before being handed to the
  hardware-independent translation layer -- border margin does not enter
  into it, since the device is relative, not absolute; a real physical
  mouse reports ball rotation with no notion of cursor position. Fractional
  deltas accumulate across frames rather than truncating to zero every
  frame, so slow movement at high magnification isn't silently lost.
  8 tests (`mouse_test.go`): mode parsing (including AMX's explanatory
  rejection, not a bare "invalid value"), the button byte polarity and
  bit assignment, the default button state (0xFF, not Go's zero-value
  0x00, which would mean "everything pressed" under active-low
  semantics -- the specific bug this default guards against), the full
  port-decode path including an aliased address, 8-bit wraparound, and
  fractional-delta accumulation.
- **T-14 filed**: AMX Mouse deliberately not implemented. Its actual
  protocol turned out to be a raw quadrature encoder through a Z80 PIO
  (Sinclair Wiki), not a position register like Kempston -- a materially
  different, timing-sensitive emulation problem needing further research
  before attempting, not a same-pass approximation that might not
  actually drive real AMX-aware software correctly.

## [0.4.13] - 2026-08-17

### Added

- **`-joystick kempston|sinclair` flag**, both GUI and headless binaries
  (`joystick.go`, `io.go`, `input.go`, `zenzx_gui.go`,
  `zenzx_headless.go`). Kempston: dedicated port `0x1F`, active high,
  bit layout confirmed against the original Kempston Joystick Interface
  instruction sheet (bit0=right, bit1=left, bit2=down, bit3=up,
  bit4=fire). Sinclair: Interface 2 Joystick 1's keyboard-remapping
  mechanism, confirmed against multiple independent sources (Wikipedia's
  ZX Interface 2 article, the Interface 2 circuitry reference, Fuse's
  libretro docs) -- keys 6/7/8/9/0 as left/right/down/up/fire, applied
  via the existing `PressKey`/`ReleaseKey` matrix API, composing
  correctly with ordinary keyboard use of the same keys. An invalid
  `-joystick` value is a hard startup error (`ParseJoystickMode`),
  matching the `-ns-graphics`/`-non-standard` validation convention, not
  the older `-tapemode` silent-fallback style. GUI-side input reads
  raylib's own gamepad API (`IsGamepadButtonDown`,
  `GetGamepadAxisMovement`) -- present in both raylib-go backends,
  confirmed directly in the vendored source, so no new dependency and no
  risk to the cgo-free darwin/windows cross-build established in 0.4.7.
  Deliberately did not adopt a third-party joystick library evaluated
  for this (`0xcafed00d/joystick`): its darwin backend requires cgo (a
  `.c` file in the package), which would have broken exactly the
  cross-build property raylib's own API already satisfies for free.
  10 tests (`joystick_test.go`): mode parsing (including the hard-error
  case), the Kempston bit layout, the Sinclair row/col mapping against
  input.go's own key-1..0 matrix positions, both mechanisms' full path
  through the real `SpectrumIO.ReadPort`, and confirmation that
  `JoystickNone` (the default) touches neither the Kempston byte nor the
  keyboard matrix.
- **Register item filed** for the deliberately deferred pieces: Sinclair
  Joystick 2, Cursor/AGF/Protek, and a headless zenscript joystick-drive
  verb -- none requested this pass, all cheap to add later given
  `JoystickState`/`SetJoystickState`'s deliberate separation from any
  particular input source.

## [0.4.12] - 2026-08-17

### Added

- **`docs/TS2068_DEVELOPMENT_PLAN.md`**: frozen 6-stage plan for T-12
  (`-model ts2068`) -- model skeleton/ROM loading/chunk-0 banking/boot;
  NTSC frame timing; `CHNG_VID` (the sole deferred-list item now in
  scope, per direction; everything else deferrable -- full 8-chunk
  Dock/cartridge banking, the TS2040 printer, composite/RF video --
  stays unimplemented indefinitely); AY sound-chip ports and joystick;
  tape I/O; memory contention. States a guiding principle up front:
  because zenzx runs the real ROM images, most of what reads as "new
  system-software feature" is actually "correct the substrate and let
  real ROM code do it" -- reshapes `CHNG_VID` from a reimplementation
  task into a substrate-correctness verification task.
- **`docs/TS2068_TRACKING.md`**: stage-status tracker for the plan
  (all 6 stages ☐, ready for execution).
- **T-12 condensed** in `docs/TRACKING.md` to reference the plan/tracker
  rather than duplicating their content.

## [0.4.11] - 2026-08-17

### Fixed

- **`rom/TIMEX.txt` rewritten with a properly researched provenance
  note**, replacing 0.4.10's "no known statement" framing. Checked the
  actual sources rather than assuming: Cliff Lawson of Amstrad, quoted
  directly from the 1999 Usenet thread that is SINCLAIR.txt's own
  citation, scopes Amstrad's permission to "code that was written by
  Sinclair or Amstrad" and states he has "no clue" about the separate
  Timex licensing deal -- so Amstrad's grant does not, on its own
  wording, extend to Timex's own additions in the TS2068 ROM (T/S 2000
  BASIC's extra keywords, SCLD/cartridge support). World of Spectrum's
  Sinclair BASIC licence page, by contrast, states directly that "the
  TS2000 BASIC rights stayed with Timex" and "Timex permits
  distribution" -- real support from the field's long-standing
  reference archive, applying the same standard it uses for the other
  three rights-holders on that page, though not as strong as Amstrad's
  directly quotable, dated statement. The note now cites both by name
  rather than asserting either "fully covered" or "no permission
  exists."

## [0.4.10] - 2026-08-17

### Added

- **TS2068 ROM images added to `./rom`**, ahead of T-12: `ts2068-0.rom`
  (16384 bytes, Home ROM) and `ts2068-1.rom` (8192 bytes, Extension
  ROM). Verified: sizes match the T/S 2068 Technical Manual exactly,
  sensible Z80 code at the reset vectors, expected embedded copyright
  strings present. `rom/TIMEX.txt` records provenance, hashes, and
  copyright status -- flagged there that, unlike the Sinclair ROMs
  (`rom/SINCLAIR.txt`, an explicit Amstrad redistribution grant), no
  equivalent documented blanket permission from a current TS2068 rights
  holder is known.
- **T-12 updated**: the "needs its own ROM files" blocker is resolved;
  the bank-switching/I/O-port/memory-map work remains open.

## [0.4.9] - 2026-08-17

### Added

- **`mode-timex-001-hicolour` implemented** (`videorender_hicolour.go`):
  the first non-standard `-ns-graphics` renderer. Screen 0's bitmap
  supplies pixels exactly as standard mode; screen 1, at the same
  non-linear byte offset plus `0x2000`, supplies one FLASH/BRIGHT/PAPER/
  INK attribute byte per 8x1 pixel row instead of the standard 8x8
  block -- read straight through `mem.Read()`, since `$6000-$77FF` is
  already ordinary bank-5 RAM in zenzx's memory model, no new storage
  needed. FLASH is deliberately not honoured, per the standard-only-
  FLASH decision (0.4.7): the real attribute byte's bit 7 is never read.
  7 tests (`videorender_hicolour_test.go`): registration, the exact
  pixel/attribute address correspondence from the T/S 2068 Technical
  Manual's own worked example (`$4000`->`$6000`, `$47FF`->`$67FF`,
  `$57FF`->`$77FF`), basic decode, BRIGHT, FLASH-is-ignored, screen 0's
  own attribute block being correctly unused, and an end-to-end test
  through the real `ZenZX.SelectVideoRenderer`/`DecodeDisplay` path
  (the same one both front ends call) that also writes a PNG for visual
  inspection -- eight different ink colours in eight consecutive 8x1
  rows of one character cell, a picture standard mode cannot produce.
- **T-11 closed**: `mode-timex-001-hicolour` implemented and verified end
  to end (see "Added" above and `docs/RESOLVED.md`).
- **T-12 filed**: `-model ts2068` (own ROM pair, 8K-chunk bank
  switching, dedicated AY ports) -- a separate, larger feature,
  deliberately not a prerequisite for hi-colour mode.

### Fixed

- **`docs/timex-modes.md`'s memory-layout section corrected a second
  time**, now against the T/S 2068 Technical Manual's actual
  `CHNG_VID`/`OPDFIL`/`CLDFIL` Z80 source rather than inference: there
  is a real ROM-level relocation (2112 bytes of OS-resident code and the
  machine stack, moved `$6000`->`$F7C0`), but it moves only the OS's own
  internal plumbing, not the user's BASIC program, and it happens only
  when the `CHNG_VID` service is called -- not from the raw port write.
  zenzx has no equivalent service (it doesn't emulate the T/S 2068 ROM),
  so this resolves the design question rather than leaving it open: read
  `$6000-$77FF` as ordinary memory, matching what that range already is
  on real hardware outside of a video mode reading it.

## [0.4.8] - 2026-08-17

### Fixed

- **Personal name removed from docs and code comments.** 0.4.7 introduced
  nine instances of a real name in `display.go`, `videorender.go`,
  `docs/video-architecture.md`, `docs/TRACKING.md`, and
  `docs/KNOWN_ISSUES.md`, attributing design decisions and reports.
  Replaced with neutral phrasing (dates retained, attribution dropped).
  Violated this codebase's own privacy rule against putting identifying
  information into project artifacts.

## [0.4.7] - 2026-08-17

Refactor: the standard video display is now pluggable, in both the GUI and
headless builds. No visual change for the standard mode -- see
`docs/video-architecture.md` for the design and the byte-for-byte
regression evidence.

### Added

- **`VideoRenderer` interface** (`videorender.go`): `Name`, `Decode`,
  `Dimensions`, `BorderMargins`. A registry (`RegisterVideoRenderer`,
  `LookupVideoRenderer`) resolves the active renderer from `-ns-graphics`
  at startup; selecting a valid-but-unimplemented mode is now a startup
  error instead of silently rendering standard while claiming otherwise.
- **`docs/video-architecture.md`**: the interface, three design decisions
  (FLASH is standard-only; border is optional per mode; magnification is
  shared but not every zoom level fits every mode), and what moved where.
- **`screen.go`**: `SpectrumScreen` (display-file storage: bitmap,
  attributes, multiplier, flash state) as one definition shared by both
  builds, replacing two independent per-build-tag definitions.
- 4 new tests (`videorender_test.go`) covering renderer lookup, the
  unregistered-mode error path, duplicate-registration panics, and
  `ZenZX.SelectVideoRenderer`.

### Changed

- **GUI rendering**: replaced the 256-pre-baked-texture-per-bit-pattern
  fast path with a single `image.RGBA` upload per frame
  (`rl.UpdateTexture`), decoded by whichever `VideoRenderer` is active.
  Window, texture, and border sizing now come from the active renderer's
  `Dimensions()`/`BorderMargins()` rather than hardcoded 256x192/
  32-32-24-32 constants.
- **`DisplayManager.ScaleUp` and `InitDisplay`** now clamp the window to
  the current monitor's size (`maxMultiplierThatFits`), since a
  higher-resolution mode's window at a high multiplier may not fit every
  display. Not yet exercised against a real non-256x192 renderer (none
  exists yet, see T-09) -- reasoned through, not verified in practice.
- **`writeScreenPNG`/`writePNG`** consolidated into one function
  (scheduler.go), both call sites now go through `zx.DecodeDisplay()`.

### Fixed

- **FLASH/paper-ink swap divergence**: the GUI's screenshot decoder
  checked `flashEnabled` before swapping ink and paper on a FLASH cell;
  the headless decoder didn't. Both now use the GUI's (correct) check.
  Unlikely to have been observed in practice (headless never advanced
  FLASH automatically), but a real, provable behavioural difference
  between the two builds prior to this fix.

## [0.4.6] - 2026-08-17

### Added

- **`docs/timex-modes.md`**: reference documentation for the Timex SCLD
  extended video hardware -- the port `0xFF` mode-select protocol,
  screen 0/1, hi-res, and hi-colour (Extended Colour Mode) in detail,
  including the open memory-layout question for implementation.

### Changed

- **`-ns-graphics mode-timex` renamed to `-ns-graphics
  mode-timex-001-hicolour`**, adopting a numbered scheme
  (`mode-timex-NNN-name`) that leaves room for hi-res and dual-screen as
  separate values once designed, rather than one flag value standing in
  for three distinct hardware modes. `mode-timex` (bare) is no longer a
  valid value.

## [0.4.5] - 2026-08-17

### Added

- **`-non-standard on|off`**, a master switch gating a family of `-ns-*`
  sub-switches (`nonstandard.go`). With `-non-standard off` (the default),
  any `-ns-*` flag set to a non-empty value is a startup error rather than
  being silently ignored. With `-non-standard on`, each `-ns-*` flag
  independently accepts its default (standard behaviour) or one of a
  fixed set of recognised values.
  - `-ns-graphics`: `mode-timex`, `mode-zenzx-01` (256x192, 3px/byte,
    linear framebuffer, no attribute clash), `mode-zenzx-02` (512x384,
    double resolution).
  - `-ns-storage`: `storage-zenzx-posix`.
  - Validated values are stored on `zx.nonStandard` for future subsystems
    to consume; no mode is implemented yet (see T-09, T-10).
  - 19 table-driven test cases (`nonstandard_test.go`) cover the gate,
    each recognised value, invalid values, and the startup summary line.

## [0.4.4] - 2026-08-17

### Added

- **CI** (`.github/workflows/ci.yml`), modelled on `github.com/ha1tch/zenimate`'s:
  gofmt/vet/test on the cgo-free core; the GUI cross-compiled cgo-free for
  darwin and windows (amd64 and arm64) via raylib-go's `purego` backend;
  the GUI built natively with cgo for linux/amd64 and linux/arm64 (the
  latter on GitHub's `ubuntu-24.04-arm` hosted runner).
- **Release workflow** (`.github/workflows/release.yml`): on a `v*` tag,
  builds and publishes headless binaries for linux/darwin/windows
  (amd64+arm64) plus freebsd/amd64, and GUI binaries for the same matrix
  as CI, each archived with the ROM set, README, CHANGELOG, and LICENSE,
  as GitHub release assets.
- **README**: "Continuous integration and cross-platform builds" section.
- **`docs/KNOWN_ISSUES.md`**: "CI and cross-platform builds" section
  documenting why darwin/windows are cgo-free and Linux is not (oto's
  Linux backend requires ALSA via cgo; raylib-go's `purego` backend and
  oto's darwin/windows backends do not need cgo), and that BSD has no
  purego path and is not covered by CI.

### Fixed

- **`docs/KNOWN_ISSUES.md`** corrected: the GUI is now built (not merely
  type-checked) in CI, superseding the note this section carried since
  0.4.3.

## [0.4.3] - 2026-08-17

Repository-discipline release: no emulation behaviour changes.

### Added

- **repoman toolset in-tree** (`repoman/`, from github.com/ha1tch/repoman
  0.7.0) with `.repoman.json` at the root: journaled editing (`ed.py`),
  the work register (`register.py`), the dormant-guard registry
  (`guards.py`), version sync (`syncver.py`), and manifest-driven,
  resumable release orchestration (`relcore.py`).
- **Tracking documents:** `docs/TRACKING.md` (live register, T-01..T-06),
  `docs/RESOLVED.md` (append-only resolution record), and
  `docs/KNOWN_ISSUES.md` (intentional limits plus dormant guards G-01 GUI
  link build and G-02 FDC read against a real DSK). The README's Known
  issues section now points at these instead of doubling as the register.
- **Release manifest** in `.repoman.json`: version sync, changelog and
  register gates, GUI type-check (`check_gui.sh`), headless build, tests,
  headless smoke test (`smoke_headless.sh`), and a checkpoint zip with an
  embedded SHA-256 manifest, contamination scan and binary sniff.

### Changed

- **README corrected** (T-01): removed the stale claims that `.dsk` images
  do not load (floppy read/write/format shipped in 0.3.x) and that the
  beeper is unfiltered (DC blocker and 14 kHz lowpass shipped in 0.3.3);
  the versioning section documents the repoman workflow.

### Removed

- `release.sh` and `syncver.sh`, superseded by `repoman/relcore.py` and
  `repoman/syncver.py`. The GUI-check and smoke-test logic they carried
  now lives in `check_gui.sh` and `smoke_headless.sh`.

### Dormant guards

- G-01 (GUI link build) not exercised in this cycle: the sandbox has no
  raylib/oto host libraries; handed off. G-02 (FDC read against a real
  DSK) not exercised: no DSK image available in-session (see T-05).

## [0.4.2] - 2026-06-30

### Fixed

- **Snapshots saved with a blank screen.** After loading a screen directly (e.g.
  a `.scr` image) and saving a `.sna` or `.z80`, reloading that snapshot restored
  the border but left the screen blank. The display is held authoritatively in
  the render buffers (`screen.bitmap`/`attributes`); a directly loaded screen
  populates those buffers without going through the RAM write path, so the bank
  5 screen region stayed stale and the snapshot captured an empty screen. The
  border survived because it is a single restored value. `toMachineState` now
  copies the display buffers into the displayed bank's screen region before
  encoding (the mirror of `resyncAfterLoad`, which copies the other way on load),
  so a snapshot always captures what is actually on screen. The round-trip tests
  now set and check screen-region markers through the display buffers, matching
  how the screen is really stored, and reproduce the reported scenario.

## [0.4.1] - 2026-06-29

### Added

- **Headless `-model plus2a`**: the +2A is now selectable in the headless build,
  not only the GUI. It loads the +3 ROM set (the +2A shares the +3's ROM,
  including the unused +3DOS) but does not enable the floppy controller, since
  the +2A has no drive. A measured boot baseline (frame 55) was added for the
  `wait-boot` action; the +2A settles far sooner than the +3 (frame 135)
  precisely because it has no FDC seeking for a boot disk.

### Fixed

- **README fast-tape note corrected.** The previous note stated flatly that fast
  tape loading does not work. In fact the instant-inject path (used when a tape
  is loaded with `-tape` in fast mode) places `.tap` and `.tzx` CODE blocks into
  memory byte-identically to the source. The note now distinguishes this working
  path from the separate, still-unverified ROM-trap path that would intercept a
  guest's own `LOAD ""`, and records that a CODE tape is loaded but not run.

## [0.4.0] - 2026-06-29

Migrates the standard snapshot codecs (`.sna`, `.z80`) onto the shared
`zentools/pkg/snapshot` library, replacing ZenZX's in-tree implementations with
thin delegations over a neutral machine-state adapter. The proprietary `.zxs`
format stays native to ZenZX. This both removes duplicated format code and fixes
real defects the in-tree versions carried.

### Added

- **`snapshot_adapter.go`**: `toMachineState` / `fromMachineState` map ZenZX's
  live emulator state to and from `zentools`' neutral `MachineState`. The format
  codecs never see ZenZX's types; this adapter is the only coupling point.
- **Snapshot regression suite** validated against real third-party artifacts:
  round-trip tests for 48K/128K `.sna` and `.z80`, plus sentinel tests that load
  genuine game snapshots (Jet Set Willy v1, Manic Miner v2, Z80 Attack v3 128K)
  and a load -> re-save -> independent-decode check confirming byte-identical
  memory across a v1-to-v3 conversion.

### Changed

- **`.z80` save now writes version 3** (extended header, 128K-capable) rather
  than the previous version 1. Loading still accepts v1, v2, and v3.
- `SaveSNA` / `LoadSNA` / `SaveZ80` / `LoadZ80` are now thin wrappers over
  `zentools`; roughly 460 lines of in-tree codec were removed.

### Fixed

- **`.sna` 48K save lost the program counter.** The previous in-tree `SaveSNA`
  never pushed PC onto the stack for 48K snapshots (the 48K SNA header has no PC
  field, so PC must be saved on the stack), silently losing it. The `zentools`
  codec does this correctly; a regression test pins PC survival across a round
  trip.
- **`.sna` and `.z80` snapshots now load correctly**, resolving the prior
  known-issue. Verified by running loaded game snapshots headlessly to their
  title/menu screens.

## [0.3.5] - 2026-06-28

### Fixed

- **Headless fast-load**: a tape loaded in the headless runner is now played
  automatically, so fast-load injection actually fires. `LoadFile` leaves the
  tape stopped, and the headless main set the tape mode but never called
  `Play()`; the tape `Tick` returns early unless the tape is playing, so in fast
  mode no block was ever injected. The headless main now calls `Play()` after
  loading, matching the GUI path. Verified by loading a CODE tape and reading the
  target memory directly: the block's bytes are placed at the address its header
  specifies.

## [0.3.4] - 2026-06-27

Smooths AY-3-8912 sound. The chip was clocked accurately but its output was
read once per audio sample (point sampling), which aliases the square and noise
harmonics into the audible band before any output filter can act -- the
roughness left after the beeper was cleaned up in 0.3.3.

### Fixed

- **AY anti-aliasing**: the chip's mixed output is now area-sampled --
  accumulated at every AY clock within each sample window and averaged -- rather
  than read once per sample. This is first-order anti-aliasing at the source,
  matching the beeper's duty-cycle approach. In a synthetic test this reduced
  energy above 14 kHz by roughly forty times with no loss of the tone's
  fundamental. The internal chip logic (tone, noise LFSR, envelope, channel
  mixing) is unchanged; only the sampling of the result differs.

## [0.3.3] - 2026-06-27

Smooths beeper audio. The square wave was generated by per-sample duty-cycle
averaging, which leaves residual high-frequency content that folds back as
aliasing -- the jagged, buzzy edge of the sound.

### Fixed

- **Beeper anti-aliasing**: the mixed audio output now passes through a DC
  blocker (removing the offset of the unipolar beeper signal) and a 14 kHz
  Butterworth lowpass that removes the aliased high harmonics while leaving the
  beeper's tone intact. In a synthetic test this reduced energy above 14 kHz by
  roughly two and a half orders of magnitude with no measurable loss of the
  fundamental. The filtering is on by default and can be toggled off for a
  deliberately harsher sound.

## [0.3.2] - 2026-06-27

Makes disk writes persist automatically. Previously, modifications to a disk
(such as a BASIC SAVE) lived only in memory and were written to the .dsk file
only on a manual save (F4) or eject; exiting discarded them.

### Added

- **Save on exit**: a modified disk loaded from a file is flushed back to that
  file when the emulator exits, in both the windowed and headless front-ends.
  A blank in-memory disk with no filename is left untouched (use Save As).
- **Debounced auto-commit**: a modified disk is committed to its file
  automatically once writing has settled -- roughly 30 seconds after the last
  write, and only while the controller is idle (never mid-command). Each new
  write restarts the window, so a multi-block save commits after the final
  write rather than partway through, avoiding a torn image. An idle or
  read-only session writes nothing.

Manual save (F4) and save-on-eject (F6) continue to work for an immediate
commit.

## [0.3.1] - 2026-06-27

Reworks +3 floppy handling to read, write, and format real DSK images
correctly, and fixes a +3 memory-paging crash. Includes a freshly formatted
DSK image for experimentation.

### Added

- **DSK image parsing** (standard and extended CPC/+3 formats): disks are now
  parsed into a track/sector model where each sector carries its own CHRN
  address (cylinder, head, record ID, size code), recorded FDC status, and
  data. This replaces a flat fixed-geometry model that could not represent real
  disks. Verified against a range of commercial images, including ones with
  irregular sector sizes, non-sequential high-numbered sector IDs, and sparse
  formatting.
- **DSK serialization and save-back**: the parsed structure can be written back
  to a valid extended-DSK file, so modifications persist. Verified to
  round-trip losslessly.
- **Sector write and Format Track**: the FDC write path stores data into the
  parsed structure by CHRN, and Format Track (previously unimplemented) builds
  a track's sectors with the requested geometry and filler byte. Both persist
  via save-back.
- A freshly formatted blank DSK image, `zenzx-formatted.dsk`, is bundled for
  experimentation with disk writing.

### Fixed

- **FDC read path** now locates sectors by matching the requested record ID
  against the track's sector list (correct CHRN addressing), with multi-sector
  reads that advance the record ID up to the end-of-track parameter. Read ID
  returns real sector headers rotating through the track. Previously the read
  path used flat-offset arithmetic against an unpopulated buffer, so a parsed
  disk mounted but returned no data.
- **+3 special paging crash**: writing certain values to port 0x1FFD could
  index the paging configuration table out of bounds and panic. The special
  paging state is now tracked correctly (an active flag separate from the 0-3
  configuration index), and the four all-RAM configurations are mapped
  correctly. The +3 no longer crashes when boot code triggers special paging.
- **Keyboard injection**: corrected the matrix position for the colon
  character, and lengthened key hold timing so the ROM keyboard scan registers
  presses reliably, including consecutive identical keys.

## [0.3.0] - 2026-06-27

Adds zenscript, a timestamped automation format for driving the emulator, and
the screen-reading and keyboard-injection primitives needed to write
self-synchronising automated tests. Scripts work in both the windowed and
headless front-ends via a new `-script` flag.

### Added

- **zenscript (`.zen`) action scripts** (`-script <file>`): a line-oriented,
  subtitle-style format of `<offset> <verb> [args]` driving the machine
  frame-by-frame. Offsets are frame counts from power-on, with `f`/`s`/`ms`
  suffixes (50 fps). A frame-driven scheduler, shared by both front-ends, fires
  actions against an effective clock; blocking verbs rebase that clock so
  scripts are portable across models. Documented in `docs/zenscript.md`.
- **Media and state verbs**: `snapshot`, `snapshot-save`, `bin`, `scr`,
  `tape-play`, `tape-stop`, `reset`, `quit`, and `shot` (named or auto-named
  PNG capture). While a script drives a run, the headless front-end's own
  interval/final-frame capture is disabled so the two do not compete.
- **`wait-boot`**: blocks until the current model has finished booting, then
  rebases the timeline, so one script adapts to each model's boot time. Boot is
  detected by a combination of the 128-family rainbow band, the bottom two text
  rows being written, and screen stabilisation. Measured per-model baselines
  (48K 1.70 s, 128K 1.22 s, +2 1.24 s, +3 2.70 s, Spanish 48K 1.70 s, Spanish
  128K 0.88 s) are recorded with a +-5% regression guard.
- **Keyboard injection**: `type <text>` (with SYMBOL SHIFT applied for
  symbols), `key <name>` (enter, delete, cursors, break, and so on), and
  `press`/`release` for held keys. Keypresses are held across several frames
  (default 5-frame hold, 3-frame gap) so the ROM keyboard scan registers them;
  shifted keys are staged (shift settles first, then the key) so consecutive
  shifted characters are not merged.
- **Screen text recognition** (`dump-screen`, `expect-screen`): reads
  characters off the display by matching each cell against the ROM font, the
  way Sinclair BASIC's `SCREEN$` does, as a pure non-invasive read. Inverse
  video is handled; only the standard ROM font is recognised.
- **Condition waits** (`wait-screen`, `wait-attr`): block the timeline until a
  row contains given text, or until a region's attributes match a colour
  predicate (`ink=`, `paper=`, `bright`, with a cell-count threshold), each
  with a timeout. These let a script synchronise on what the machine actually
  did rather than on a guessed frame, including recognising menus by colour
  when their characters use a custom font.

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
