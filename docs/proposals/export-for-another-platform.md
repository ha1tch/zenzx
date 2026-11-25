# Export for another platform

Updated: 2026-11-24 · Status: scoped, not started · Register: T-91

## What it is

File > Export makes a standalone program: ZenZX with a machine inside it. Today the program is a
copy of the ZenZX that is running, so it runs on the same system and processor as the person who
made it. This proposal lets a person make the program for another system: from Linux, a program
for Windows or macOS, for Intel/AMD or ARM.

## What the code already gives

- A program is `stub | payload | length | trailer` (`standalone.go`). The payload is the
  machine as a `.zxs` snapshot, a JSON header and the window's settings, and none of it depends
  on the system. `ExportProgram(stub, out, p)` already takes the stub as a path. A program for
  another system is therefore the other system's stub with the same payload after it.
- CI already builds the six GUI programs (Linux, macOS, Windows, each amd64 and arm64) and
  publishes them as raw executables with checksums and `runners.json` on the Pages site. The
  site holds the newest tag only: each deployment replaces the last.
- Windows and macOS stubs are cgo-free, so a test can build them on any machine. The Linux arm64
  stub is a cgo build, and a test uses another ELF arm64 program in its place.

## What is missing

| Part | Gap |
|---|---|
| Stub source | Nothing fetches or accepts another system's stub. |
| Trust | `runners.json` and the programs come from one host, so a matching hash shows the download is intact and does not show the page is genuine. |
| Version | A stub must be the same version as the ZenZX that exports. The site keeps only the newest. |
| Form | Export asks for a name and a folder, with no choice of system. |
| Naming | `.exe` for Windows. |
| CI | No signing, no per-version files, no check of what was published. |

## Decisions

| # | Decision | Why, and what was rejected |
|---|---|---|
| D1 | The stubs for a version are release assets (`zenzx-<version>-stub-<os>-<arch>[.exe]`), beside the manifest and its signature. The Pages site stays the page for people and shows the same files. | Release assets are permanent for each tag, so any ZenZX can find its own version's stubs. Bundling the six stubs in every program would add about 75 MB to each. |
| D2 | A manifest per version, `runners-<version>.json`, signed with Ed25519. The public key is in the source; the private key is a CI secret. The manifest holds each stub's size and SHA-256. | Hashes inside the binaries are circular (each of six programs would hold the others' hashes). Sigstore needs a heavy client. Hash and signature from the same host prove nothing against a changed page. |
| D3 | The manifest names the key. The program accepts two public keys, the current one and the next, so that a key can be replaced without stranding old programs. | A lost or leaked key otherwise needs a new release of every program. |
| D4 | A stub is used only when its version equals the exporting ZenZX's (`pkg/version`). A development build, which has no release, offers only "a ZenZX program file". | A stub of another version reads the payload differently, or not at all. |
| D5 | Before use, the stub's own headers are read (`debug/elf`, `debug/pe`, `debug/macho`): the format and processor must be the chosen target's, and it must carry no payload of its own. | Stops a wrong file, or a program that is already an export, from being used. |
| D6 | Downloads are cached in the user's cache folder, by version and system, and the hash is checked again each time the cache is used. | A second export for the same system needs no network. |
| D7 | The network is touched only when the person chooses another system. HTTPS only, hosts limited to GitHub's release hosts, 30 s timeouts, cancel, a plain message when offline, and the size is shown before the download. No other traffic. | Export for this computer works with no network, as it does now. |
| D8 | The Export form gets a row "Runs on": This computer (default), Windows (Intel/AMD), Windows (ARM), macOS (Intel), macOS (Apple silicon), Linux (Intel/AMD), Linux (ARM), and "A ZenZX program file..." for those with no network. | The last choice also covers a person who holds a stub from a release. |
| D9 | The file gets the target's extension (`.exe` for Windows) and the program is marked executable. | |

## Open until a spike settles it

- **macOS.** The program carries data after its ad-hoc code signature. Whether Apple silicon and
  Intel start it, with and without the quarantine flag, is not known; it needs a real Mac. The
  same question applies to a same-system export from macOS today (T-89).
- **Windows.** An overlay after the PE sections should run; the SmartScreen warning for an
  unsigned program is expected and unsolved.
- **Linux.** The stubs are dynamic glibc programs (glibc 2.38 or later, ALSA and X11 libraries).
  A program exported for an older distribution will not start there. The manual says so.
- **Key custody.** Making the key and storing it as a CI secret is a one-time action for the
  repository's owner; nothing else in the plan needs one.

## Stages

| Stage | Contents | Needs |
|---|---|---|
| S0 Spikes | Run an exported program on a Mac (both processors), on Windows and on a Linux arm64 machine; record what ran, where, when. Decide macOS in or out. | hardware |
| S1 Core, offline | A target type (system, processor); stub validation (D5); output naming (D9); "use a program file" path; a command-line form for scripts and CI (`-export-for`, `-stub`); tests with stubs built locally. | none |
| S2 Manifest and signing | Manifest schema 2 (version, key, per-stub URL, size, SHA-256); `tools/signmanifest` (a small Go program: the standard library has Ed25519 and Python's does not); `build_site.py` reads the manifest; tests with a throwaway key. | none |
| S3 Client and form | Fetch, verify signature and hash, cache, offline and failure messages; the "Runs on" row, progress and cancel; tests against a local test server. The public key goes into the source. | S1, S2 |
| S4 CI | The stages below. | S2 |
| S5 Documents | Manual, GUI document, changelog, register closure; the dormant guards for what only hardware shows. | S0, S3, S4 |

## CI stages

The release workflow today is: build the programs (headless and six GUI) → publish the release
and, separately, build the page and deploy it. The new order has a gate between building and
publishing, because a release cannot be un-published cleanly once people have downloaded from it.

1. **build** (unchanged): the six GUI programs and the headless ones, as archives.
2. **manifest**: needs every build. Takes the six stubs out of the archives (as `build_site.py`
   does), writes the manifest, signs it with the `ZENZX_MANIFEST_KEY` secret. The key sits in a
   repository environment (`release-signing`), so that only tag builds can read it. The output is
   one workflow artifact: stubs, manifest, signature, checksums.
3. **draft release**: needs `manifest`. Creates the release as a draft with the archives, the
   stubs, the manifest and the signature.
4. **verify-release**: needs the draft. A matrix on Linux, macOS and Windows, each on Intel and
   ARM where GitHub offers a runner. Each one downloads the draft's stubs through the client
   code path, checks the signature and every hash, reads each stub's headers (D5), and runs
   `tools/check_binaries.py` on them. One runner then makes a cross-system export of a fixed
   snapshot and checks that the program part equals the stub and that the payload reads back
   (`readProgram`).
5. **publish**: needs `verify-release`. Makes the draft public, then builds and deploys the page
   from the same manifest. A failure at stage 4 leaves a draft and nothing public.
6. **On every push** (the existing CI workflow): the signature round trip with a throwaway key,
   the manifest schema, the client against a local test server, the cross-system export with
   locally built stubs, and the checks that already run.

Optional, and not needed for the first release of the feature: a `-selftest-frames N` option
that starts the program, runs N frames without showing a window and exits 0. With it, stage 4
could start each exported program on its own system's runner, which proves more than reading
headers. It does not replace T-89, which is about a real display.

## Order and size

S1 and S2 do not depend on each other and are the cheapest; S3 is the largest because of the
form and the failure cases; S4 is the most intricate because of the draft-release gate and the
runners. S0 can start now and settles whether macOS is in the first release.

## Not in this proposal

- The headless program as a target, and FreeBSD (the page has no GUI stub for either).
- One export that makes the program for every system at once.
- Signing the programs for Windows or macOS (Authenticode, notarisation).
