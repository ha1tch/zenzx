# Tape loading: handover

Updated: 2026-08-27 — describes zenzx as of v0.6.2.

Addressed to whichever Claude session picks this up next. Goal, stated
by Horatio and unchanged across every session that touched this:
**load as many tapes as possible, as fast as possible.** This document
is the synthesis — what exists, what was learned, what's still wrong,
and a proposed order of work. The register (`TRACKING.md`) is the
source of truth for open items; this is the narrative that connects
them.

## The three modes, honestly characterised

**Accurate** — real pulse-level EAR-pin emulation, every instruction
genuinely executed, tape delivers data at the tape's own recorded
speed. The best-verified mode in the codebase: this project's own
hardware-fidelity work (contention, frame timebase, EAR idle feedback,
floating bus, TZX pulse generation — the last checked pulse-for-pulse
against an independent from-scratch spec implementation, 828,282
pulses, zero mismatches) is either accurate-mode-specific or shared by
all three modes at the pulse-generation layer. Slow by nature: real
1980s tape speed is genuinely 3-12+ minutes for a large title, not a
zenzx artifact — independently verified from raw physics (byte count ×
real ROM bit-cell timing) matching measured results within ~1.5%.

**Turbo** (`-tapemode=turbo`) — the same CPU instance, same
instruction stream, same tape-pulse timeline as accurate, with
per-frame rendering and audio/mouse bookkeeping skipped while the tape
plays (`loadingFrame`), and zen80's `FastMem` flat-array replacing
interface-dispatch memory access for speed (~3.6x on memory access
alone). Wall-clock speedup over accurate: ~15-27% in this codebase's
own measurements, not orders of magnitude — the real bottleneck
profiling found was Go interface dispatch, not the 50Hz frame
throttling that motivated the original design idea. Confirmed correct
for 48K and, after T-19's fix (see below), for 128K bank-switching
titles too, via direct screenshot and full memory-diff comparison
against accurate mode. **Known scope gaps, never tested:** +3's
special all-RAM paging mode (0x1FFD) and TS2068's Extension-ROM
banking. **Known failure history:** T-19 (RESOLVED.md) — turbo's own
completion signal (`tape.Position` reaching the exact tape end)
reported success while the screen was completely blank and no game
code had ever been read, root-caused to a flat 64K snapshot unable to
represent Cybernoid 2's genuine mid-block bank switching. Fixed via a
proper 12-page pool (`turbo_pages.go`) synced immediately on every
paging `OUT`, not just at block boundaries. **One behavioural property
worth knowing before instrumenting anything:** turbo's screen buffer
is frozen throughout the fast path by design — the real render only
happens on the first call *after* `Playing` clears. Any tooling that
samples the screen mid-turbo-run will see nothing change and must not
mistake that for a stall.

**Fast** (`-tapemode=fast`) — a ROM-trap: the instant execution hits
the standard ROM's `LD-BYTES` entry point (`tape.go`'s
`TryIntercept`), the requested block is injected directly from the
decoded tape data, bypassing pulse-by-pulse execution entirely for
that block. Near-instant when it fires. **Structural limitation,
confirmed this session with the Chase H.Q. corpus (T-23):** it can
*only* fire while PC is literally inside that one ROM routine. The
moment a tape's bulk data is delivered by a custom/protected loader
(non-standard flag bytes, which is the common case for real commercial
releases — Speedlock is one large example, T-22), PC never revisits
the trap point and fast mode provides zero benefit for the rest of the
load. Measured directly: 2 of 4 confirmed-working Chase H.Q. tape
variants never engaged the trap at all and would have taken the full
5-12 minutes fast mode is supposed to eliminate.

## What actually works, measured

The 31-game corpus (`zzz_batch_test.go`, run this session): **19/27
tracked titles verify in accurate mode.** The 8 failures split into two
groups worth treating differently: 5 share a mechanism (Speedlock-class
protection — Cobra, WEC Le Mans, Robocop, The Great Escape, Batman),
investigated in real depth for Batman (T-22) without a resolved root
cause; 3 use unrelated custom loaders never investigated (Sabre Wulf,
Lotus Esprit Turbo Challenge, R-Type).

The Chase H.Q. deep-dive (this session, all four working tape variants
in all three modes) is the clearest evidence of the actual shape of
the problem: **the tape file you happen to have matters enormously.**
Of seven candidate Chase H.Q. rips tested, three never completed at
all (stuck at the title screen indefinitely, confirmed by screen-hash
timeline, not a timeout artifact), and the four that worked split into
two genuinely different correct behaviours (a "STOP THE TAPE / PRESS
ANY KEY" manual-intervention prompt for the two 48K-targeted rips, vs.
a direct load into gameplay for the two 128K-capable rips) — meaning
"does zenzx load Chase H.Q." doesn't have one answer; it depends which
of at least seven archived dumps you're holding, and that's very
likely true across the wider corpus too, not a one-title peculiarity.

## The load-bearing problem: T-24

Everything above measures *outcomes* by trusting some mode's own
bookkeeping about itself — and that bookkeeping has been wrong, more
than once, in ways that looked like success:

- Turbo's `Position` reached the exact tape end while the screen was
  blank and nothing had loaded (T-19's original bug).
- This session's own "verified" heuristic (screen hash differs from
  boot + has visible content) reported success for a stuck Chase H.Q.
  title screen for eleven straight minutes before deeper checking
  found it was never going to progress.
- Fast mode's trap completed and produced a screen that "differed from
  boot and had content" for `CHASHQ48.TAP` — but with visibly
  corrupted icon graphics and a stray crack-signature line, which the
  existing heuristic would have called a pass.

**This is the actual bottleneck for "load as many tapes as possible,
as fast as possible," not the concurrency plumbing.** Any scheme that
tries several approaches and picks a winner — a fast-mode-first
fallback, a turbo/fast race, an unattended corpus-wide speed pass —
needs a winner-determination step that doesn't trust the mode's own
self-report, and nothing in the codebase currently provides one. The
one technique this whole multi-day effort found that *did* work
without trusting any mode's internal state was the TZX pulse-stream
audit against an independently written spec implementation — because
it checked the *content*, not whether the tape engine claimed success.

## Proposed order of work

1. **T-24 first, before anything else on this list.** A real verifier:
   given a tape and a model, can the load state be checked against
   something that doesn't originate from the mode being tested? Candidate
   starting point: for tapes with a known, fixed load address (most
   headers declare one), checksum the actually-written memory region
   against the tape's own declared checksum bytes — independent of
   which mode wrote it, and independent of whether the screen looks
   different. This is scoped, buildable, and turns every downstream
   idea (racing, fallback, batch confidence scoring) from a guess into
   a measurement.
2. **T-23's natural next step once T-24 exists:** a fast-mode-first,
   turbo-fallback sequential pipeline — try the trap; if it hasn't
   fired within a short, real bound (not "wait the full tape"), fall
   back to turbo, which is the closest thing to a validated fast path
   available. This was proposed mid-session as a sharper alternative
   to a three-way race; it still needs T-24's verifier to be trustworthy
   rather than merely fast.
3. **T-22 (Batman/Speedlock)** remains open on its own track — a
   correctness question (does the R-register-seeded keystream decode
   right), not fundamentally a speed question, though every Speedlock
   title it eventually unblocks is one more tape that currently fails
   outright regardless of mode or speed.
4. **The uninvestigated 3** (Sabre Wulf, Lotus, R-Type) — custom
   loaders, never looked at. Possibly several small, unrelated fixes
   rather than one mechanism; worth a first triage pass to find out
   which.
5. **Corpus-wide re-verification once T-24 exists** — the current
   19/27 figure and the Chase H.Q. multi-variant results were both
   measured with the same heuristic just shown to produce false
   positives. Worth confirming which passes are real once a trustworthy
   check exists, not assuming the current numbers survive it.
   `contrib/tape-corpus-harness/` (below) is the tool to run this
   with, once it exists and once the harness's own variant-picking
   limitation is addressed — running it as-is today would produce a
   number confounded by both problems at once, not a clean measurement
   of either.

## Corpus-backed investigation tooling — now shipped, in v0.6.2

The Chase H.Q. multi-variant findings above were originally produced
with ad hoc Go test files living directly in the main zenzx package,
hardcoded to a pre-existing local corpus directory
(`/tmp/newdiv_corpus`). That's gone: anything depending on newdiv-style
content cannot be part of mainline zenzx testing, since it's
copyrighted commercial software zenzx must not bundle or assume access
to. The investigation method survives, reimplemented properly and
**built, run, and verified end to end (not just designed)** in
`contrib/tape-corpus-harness/` — a genuinely separate Go module (no
import relationship with zenzx in either direction; `go list ./...`
from the repo root doesn't even see it) that downloads its own corpus
at runtime from a legitimate public preservation source
(archive.org/details/zxspectrum-top-100 — the same collection
`cmd/tapereport`'s own doc comment already pointed at) and drives
zenzx-headless entirely as a subprocess via a generated `.zen` script,
with no access to internals. See that directory's own README for
usage, and its "How success is determined" section for the honest
limits of screenshot-diffing as a completion signal — the same T-24
caveat applies to this tool as to everything else that's tried to
answer "did it load."

Two real bugs were found and fixed getting it working, worth knowing
before extending it: `-shot-interval` is a documented no-op once a
script drives the run (screenshots only come from explicit `shot`
verbs written *into* the script), and every non-`wait-boot` script
line needs an explicit frame offset. Verified against a real
`zenzx-headless` run afterward — 56 genuine periodic screenshots,
correctly tracking a game's actual attract-mode screen progression via
plain external pixel-diffing.

**Known limitation carried over into this shipped version, not yet
fixed:** `findTape` picks one tape variant per game directory (first
`.tzx`, else first `.tap`, alphabetically) and says so in its own
result note — but does not know whether the picked variant is one of
the working ones. This session's own Chase H.Q. corpus had 3 of 7
variants that never load at all; a naive alphabetical pick has real
odds of landing on a dead one. Any corpus-wide number this tool
produces today undercounts real coverage until variant selection gets
smarter — treat its output as a lower bound, not the true figure.

## Provenance note

This document was assembled by reading a full multi-day chat export
(PDF, provided by Horatio) covering both an unrelated project
(Oakhollow) and the zenzx tape-loading work proper, cross-checked
against the live register, `RESOLVED.md`, and the actual current
source (`tape.go`, `turbo.go`, `tape_types.go`) rather than trusting
the transcript's prose alone — the T-19 and T-02 summaries above are
taken from `RESOLVED.md`'s resolution records, not paraphrased from
chat. T-23 and T-24 are new, filed this session; everything else
referenced was already in `TRACKING.md`/`RESOLVED.md` before this
document was written.
