# Installers

Updated: 2026-11-25 · Status: built, not yet run in CI · Register: T-93, T-94, T-95

A person who does not want a bare executable gets a normal installer: a `.deb` for Linux, a `.msi`
for Windows and a `.dmg` for macOS. The programs stay on the page and in the release as they are
(the export stubs, `runners.json` and the smoke workflow use them); the installers are extra
release assets.

## What each one does

| System | File | What it installs | Removal | Signed |
|---|---|---|---|---|
| Linux (amd64, arm64) | `zenzx_<version>_<arch>.deb` | the program in `/usr/bin`, a menu entry, the icon at 64, 128, 256 and 512 pixels, the licence notice; depends on glibc (the version the program names), ALSA and X11, recommends libgl1 | `apt remove zenzx` | no (apt does not need it for a file installed by hand) |
| Windows (amd64, arm64) | `zenzx-<version>-windows-<arch>.msi` | for the person who runs it, no administrator: `%LocalAppData%\Programs\ZenZX\zenzx.exe`, a Start menu shortcut, an entry in Apps and features with the icon | Apps and features, or `msiexec /x` | no: SmartScreen warns |
| macOS (Intel and Apple silicon) | `zenzx-<version>-macos-universal.dmg` | a disk image with `ZenZX.app` and a link to /Applications to drag it to; one program for both processors | delete the app | ad hoc only: Gatekeeper blocks the first start (right-click, Open) |

None of them registers file types: ZenZX ignores a file named on its command line (T-94).
None of them makes the ordinary Windows program run on Windows 7: that is a separate program and `.msi`, built with a patched Go (`.github/workflows/windows7.yml`, T-95).

## How they are made and checked

`tools/package.py` makes the `.deb` (written with the standard library), the app bundle and the
WiX source from the release's own archives. `.github/workflows/installers.yml` runs after the release
is published, or by hand for an existing tag (Actions > Installers > Run workflow). Each installer is
installed, run with `-version` and removed on a runner of its own system before anything is
attached:

- `.deb`: an Ubuntu runner for each processor; `desktop-file-validate`; nothing left after removal.
- `.msi`: made with WiX 3.14 on a Windows runner (a pinned download, checked by SHA-256); installed
  on `windows-latest` (amd64) and `windows-11-arm` (arm64); the program, the shortcut and the
  removal are checked.
- `.dmg`: made on a macOS runner (`lipo`, `codesign -s -`, `hdiutil`); mounted, signature checked,
  the program run from the image.

All five are attached to the release with a checksum file only if all of them passed. A failure
leaves the release as it was.

## What is not shown

The runners have no screen, so no installed program is seen to open its window. The `.msi` and
`.dmg` were not built where the session that wrote them could run the tools; the first run of the
workflow is their first build.
