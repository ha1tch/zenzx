# zen80 tracing hooks — proposal

Updated: 2026-08-28

Design intent for a zen80-side change requested by ZenZX. Written from
zenzx (the dependent), not from inside zen80 itself — zen80 isn't checked
out in this session, so nothing here has been implemented or tested
against zen80's actual source beyond direct inspection of the v0.5.1
module (`$GOPATH/pkg/mod/github.com/ha1tch/zen80@v0.5.1/z80/z80.go`) to
ground the call sites below. Motivated by `docs/TRACKING.md` T-26 and
`docs/TRACE_GENERALIZATION_DEVELOPMENT_PLAN.md` Stage 4, which is blocked
on this landing.

## Correction to the starting assumption

The original read of the gap was "zen80 needs memory-read, port-write,
*and* interrupt tracing." Checking the actual v0.5.1 source narrowed
that: **interrupt tracing already exists.** `z.M1Hook func(pc uint16,
opcode uint8, context string)`, gated by `DEBUG_M1` (currently `true`,
same "nil-check costs one branch" cost model as the other hooks), fires
on every M1 opcode fetch — normal, and post-CB/DD/ED/FD prefix
continuations — and on every interrupt-acknowledge M1 cycle, with
`context` set to `"NMI"`, `"IM0"`, `"IM0-fallback"`, `"IM1"`, or `"IM2"`.
zenzx's trace harness has simply never wired it up. That's zenzx-side
work (Stage 1 of the generalization plan), not a zen80 change — it needs
nothing from this proposal.

What's actually missing, confirmed by reading `memRead`/`memWrite` and
`ioIn`/`ioOut` side by side: **memory reads and port writes have no hook
at all**, not even an unused one. `memWrite` calls `DebugMemWriteHook`
before the write lands; `ioIn` calls `DebugIOInHook` after device
dispatch. `memRead` and `ioOut` have no equivalent — confirmed by reading
both functions directly, not inferred from their absence in `grep`.

## Proposed additions

### `DebugMemReadHook`

```go
// DebugMemReadHook, when non-nil, is called on every memory read with
// the address and the value read, after the read completes. Fires for
// both opcode-fetch reads (memRead is shared by fetchByte and operand
// reads; instrReadIndex distinguishes them internally but the hook does
// not) and ordinary data reads. Debug/trace instrumentation only: nil
// (the default) costs one branch.
var DebugMemReadHook func(addr uint16, val uint8)
```

Call site: `memRead` (z80.go, currently ends `return val` with no hook
call), immediately before the `return`, mirroring `memWrite`'s
`DebugMemWriteHook` placement exactly.

Known limitation to document, not fix here: because `memRead` backs both
instruction fetch and operand reads, a consumer wanting only "data the
program read" (as opposed to "bytes of the instruction stream") has to
cross-reference against `M1Hook`/`DebugPCHook` step boundaries
themselves. Giving the hook its own fetch-vs-operand flag would need
threading `instrReadIndex`'s state out through the call, which changes
the hook's signature for a distinction every existing hook already
leaves to the caller to reconstruct (same as `DebugMemWriteHook` not
distinguishing stack-push writes from ordinary ones). Not proposed here;
flag if it becomes a real blocker.

### `DebugIOOutHook`

```go
// DebugIOOutHook, when non-nil, is called on every port write with the
// port and the value written, after dispatch to the I/O device (or
// FastPortWriteOut) -- symmetric with DebugIOInHook, which fires after
// dispatch on the read side. Debug/trace instrumentation only: nil (the
// default) costs one branch.
var DebugIOOutHook func(port uint16, val uint8)
```

Call site: `ioOut` (z80.go), before `return` on both the `FastPort`
branch and the `z.IO.Out(port, val)` branch — two call sites in one
function, both needed, mirroring how `ioIn` already has a single call
site after its own two-branch dispatch converges.

## Explicitly not proposed

- **No change to `M1Hook`'s signature or `DEBUG_M1`.** It already covers
  the interrupt case zenzx needs; widening it risks breaking zen80's own
  existing consumers (`common_test.go`, `opcov_runtime_rom_test.go`,
  `pager128_testdouble.go` all set `M1Hook` directly).
- **No new interrupt-specific hook.** Would duplicate `M1Hook`.
- **No change to `DebugPCHook`'s "before fetch and interrupt handling"
  timing.** Correlating a `DebugPCHook` step against an `M1Hook`
  interrupt-context call at the same PC is enough to annotate a trace;
  changing `DebugPCHook` itself would be a breaking change to an
  existing public hook for no added information `M1Hook` doesn't
  already carry.

## Acceptance

- Both hooks documented in zen80's `CHANGELOG.md` under `[Unreleased]`,
  matching how `DebugPCHook`/`DebugMemWriteHook`/`DebugIOInHook` are
  documented there per `docs/hardware-accuracy.md`'s own cross-reference.
- zen80's own selftest/regression suite covers both firing and each
  staying silent (nil) with no observable cost, matching whatever
  pattern already covers the existing three hooks there — not verified
  from this side, since zen80's test suite isn't available in this
  session; confirm on the zen80 side before merging.
- No zenzx-side change ships until a tagged zen80 version carrying both
  hooks exists and `go.mod`'s `github.com/ha1tch/zen80` requirement is
  bumped to it, per this repo's own dependency-bump discipline.
