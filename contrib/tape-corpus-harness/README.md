# tape-corpus-harness

Optional, standalone tool for measuring zenzx's real-world tape-loading
coverage and speed against a corpus of actual commercial ZX Spectrum
releases -- not a test suite, not part of zenzx's build, not part of
its release process.

## Why this is separate

**zenzx itself ships and distributes nothing here.** This directory is
its own Go module (`go.mod`), with no import relationship to the main
`zenzx` package in either direction. `go build ./...` / `go test
./...` run from the repository root never see it. It exists purely so
someone who *wants* to run a real-corpus check can, without that
check's dependencies becoming part of what zenzx ships.

**This tool does not bundle any tape or game data.** Commercial ZX
Spectrum software is copyrighted; distributing copies alongside zenzx
would be wrong regardless of how the software is used. On first run,
this tool downloads a corpus archive from a legitimate public
preservation source (below) into a local cache directory that
`.gitignore` excludes from version control. If you already have a
`.tap`/`.tzx` corpus locally, point `-cache-dir` at it directly and
the download is skipped.

## Corpus source

[archive.org/details/zxspectrum-top-100](https://archive.org/details/zxspectrum-top-100)
-- the "ZX Spectrum Top 100" collection curated by akeley.
`ZXSpectrumTop100-noDoc.zip` (~39MB) is the smallest archive that
still has every game's `.tap`/`.tzx` files. This is the same
collection `cmd/tapereport` in the main zenzx repo documents and was
originally built against -- also, incidentally, the same set of games
behind the informally-named "newdiv" distribution some earlier
zenzx development sessions tested against locally. Using the
documented public source here instead means this tool is
self-sufficient: it doesn't assume that or any other private corpus
copy exists on the machine running it.

## Requirements

- A built `zenzx-headless` binary (`go build -tags headless -o
  zenzx-headless .` from the main repo root). This tool drives it
  entirely as a subprocess via `-tape`/`-tapemode`/`-model`/`-script`
  -- it does not link against zenzx's internals at all, which is what
  keeps it a genuinely separate module rather than needing zenzx
  refactored into an importable library.
- Network access on first run (or a pre-populated `-cache-dir`).

## Usage

```
go run . -zenzx-bin /path/to/zenzx-headless -games "Chase H.Q,Cybernoid 2" -out report.json
```

`-games` accepts a comma-separated list of directory names as they
appear in the corpus archive, or `all` for the full collection (slow --
100 games x 3 modes is a multi-hour run; see `-modes` to narrow it).

Flags:

| Flag | Default | Meaning |
|------|---------|---------|
| `-zenzx-bin` | (required) | Path to a built `zenzx-headless` binary |
| `-cache-dir` | `./cache` | Where the corpus is downloaded/extracted, or an existing corpus directory |
| `-games` | `Chase H.Q,Cybernoid 2` | Comma-separated game directory names, or `all` |
| `-modes` | `fast,accurate,turbo` | Comma-separated subset of the three tape modes |
| `-out` | `report.json` | Output report path |
| `-max-wait` | `15m` | Per-tape wall-clock timeout before declaring a run stuck |

## How success is determined

This tool has no access to zenzx's internal tape/CPU state -- by
design, since that's what keeps it decoupled. It drives each run via a
generated `.zen` script (see `docs/zenscript.md` in the main repo)
that boots the model, types `LOAD ""`, and takes periodic screenshots
(`shot`) until the tape's own reported state settles or `-max-wait`
elapses. It then does the same kind of analysis externally, on the
saved PNG files, that earlier ad hoc investigation did internally:
find the last frame at which the screen meaningfully changed, treat
that as the real completion point rather than trusting "the tape
finished playing" (which this project's own history -- see
`docs/TAPE_LOADING_HANDOVER.md` and register items T-19/T-24 in the
main repo -- has repeatedly shown can be true while a load is
completely wrong).

**This is a real limitation, not a solved problem.** External
screenshot-diffing is a weaker signal than the memory/checksum
verification T-24 calls for, precisely because nothing like that
exists yet in a form this arm's-length tool could call into. Treat
this tool's reports as a speed/coverage *survey*, not a correctness
*proof* -- exactly the caveat T-24 raises about every completion
signal this project has tried so far.

## What this replaces

Earlier investigation (see `docs/TAPE_LOADING_HANDOVER.md`) used
Go test files directly inside the main zenzx package
(`zzz_chq_compare_test.go`, `zzz_chq_threemode_test.go`) with a
hardcoded path into a pre-existing local corpus directory. Those are
removed from the main tree -- their findings are preserved in the
handover document and the register, and their *method* is
reimplemented here at arm's length instead, self-fetching its own
corpus rather than assuming one is already present.
