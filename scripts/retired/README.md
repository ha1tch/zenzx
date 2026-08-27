# Retired build scripts

These predate the verified CI/release pipeline (`.github/workflows/ci.yml`
and `release.yml`) and the project's `Makefile`, and are kept here only for
reference -- none of them are used by anything active in the repository.

- `build.sh`, `build_linux.sh`, `build_windows.sh` -- superseded by
  `make build`/`make build-cross`. `build_windows.sh` in particular assumed
  a mingw-w64 cross-compiler was required; it turned out not to be --
  raylib-go's purego backend cross-compiles Windows and macOS GUI binaries
  cleanly with `CGO_ENABLED=0` and no C toolchain at all, verified directly
  against this project's own code (see `ci.yml`'s `gui-cross` job).
- `build_example_bsd.sh`, `build_bsd.sh`, `build_bsd_improved.sh` -- three
  successive, increasingly complex Docker-based attempts at the same BSD
  cross-compilation problem, none ever cleanly retiring the last. The most
  recent (`build_bsd.sh`, despite its own header calling itself "fixed")
  had a confirmed bug: its own prerequisite check looked for source files
  under a `zenzx/` subdirectory that hasn't existed since the project's
  files moved to the repository root, so it would fail before ever
  attempting a build. BSD is not currently a release target; if it becomes
  one, start from the verified native/purego pattern the other platforms
  now use, not from reviving one of these.

`build_headless.sh` is not here -- it is current, correct, and actively
used by `.repoman.json`'s release pipeline and by `make build-headless`.
