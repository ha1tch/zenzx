# Retired scripts

These predate the verified CI/release pipeline (`.github/workflows/ci.yml`
and `release.yml`) and the project's `Makefile`, and are kept here only for
reference -- none of them are used by anything active in the repository.

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

- `_bsd.sh` and `Dockerfile` -- what those BSD scripts ran: the Docker image
  and the script inside it. The `Dockerfile` has unresolved template
  variables and never built as committed (register item T-07). They were in
  the repository's root until 0.27.0.
- `release.sh`, `syncver.sh` and `zxspectrum.txt` -- the release script
  from before `repoman relcore` (`.repoman.json`), the version script it
  called, and the banner it printed. `repoman syncver` and `repoman
  relcore` do both jobs now.

`build.sh`, `build_linux.sh`, `build_windows.sh` and `build_headless.sh` are
not here: they are current, in the repository's root, and are what the
README, the manual and `.github/workflows/ci.yml` call. Copies of the first
three stood here until 0.27.0 with a note that called them superseded; they
were the same files as the live ones, and the note was wrong.
