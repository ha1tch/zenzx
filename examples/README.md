# ZenZX examples

Three examples that go with the ZenZX emulator. Each has its own folder and its own README.

| Folder | What it is | Needs |
|---|---|---|
| `world/` | Nine ZX Spectrums in a grid, wired into one world: a character walks off one screen onto the next, and a ball, a pair of scissors and a plant pot drift between them | ZenZX, Python 3 |
| `link/` | Four machines (two 48K, a 128K, a +3) in a chat room, and a host that relays each line to the others | ZenZX, Python 3 |
| `next/` | Three programs for the ZX Spectrum Next, `gallop`, `fuji` and `orbit`, as `.nex` files with their sources and pictures | ZenZX (to run them in it), or a real Next |

## Starting them

ZenZX itself is a separate download (the page of programs, <https://ha1tch.github.io/zenzx/>).
Put `zenzx.exe` (Windows) or `zenzx` in a folder called `bin` beside this `examples` folder, or in
the folder itself, or anywhere on the PATH, or set the `ZENZX` variable to its path. When a launcher
cannot find it, it offers to download the right one for the computer, checks it against the SHA-256
the page lists and puts it in `bin`.

| | Windows | macOS | Linux |
|---|---|---|---|
| The connected world | `examples\world\run.cmd` | `examples/world/run.command` | `sh examples/world/run.sh` |
| The chat room | `examples\link\chat.cmd` | `examples/link/chat.command` | `sh examples/link/chat.sh` |
| The Next and its examples | `examples\next\run.cmd` | `examples/next/run.command` | `sh examples/next/run.sh` |

Double-click the `.cmd` file on Windows, or the `.command` file on a Mac (it opens in a Terminal
window; the first time macOS may refuse a file that came from the internet: right-click it, choose
Open, then Open again). All of them start `launch.py`, which is the same on every system:
`python3 examples/launch.py world` (or `chat`, or `next`) does what the table says anywhere, and
`--yes` makes it download ZenZX without asking.

## Python

`world` and `link` have a host program written in Python, a free programming language (the
language, not the snake) that a great many programs use in the background. It needs Python 3.6
or later and nothing that is not in Python. A person who has never heard of Python does not need
to do anything: when the launchers find none, they say what it is, and pressing Enter downloads a
private copy of it.

The download is done by ZenZX itself (`zenzx -fetch python`), so the launchers need no curl, wget or
PowerShell, and it works on a Windows 7. For that the launchers have to find ZenZX: in a folder
called `bin` beside the examples, or next to them, or named by the `ZENZX` variable. It checks what
it downloaded against a SHA-256 written in the program, picks up a download that was cut off where
it stopped, and installs nothing until the whole of it is there and checked.

- **Windows:** the copy (about 10 MB) is python.org's embeddable Python, put in a folder called
  `python` beside the examples. Nothing is installed, and deleting the folder removes it. Typing
  `O` instead opens <https://www.python.org/downloads/windows/> to install Python yourself (tick
  "Add python.exe to PATH" in its installer). Windows 7 gets Python 3.8, the last that runs there.
- **macOS and Linux:** in a terminal ZenZX asks how, and lists only what the computer has:
  1. a private copy (about 35 MB; a build of Python 3.12 from the python-build-standalone project,
     <https://github.com/astral-sh/python-build-standalone>), in the same `python` folder, installing
     nothing: what Return takes;
  2. Homebrew, where `brew` is there (`brew install python`);
  3. on Linux, the system's own package: `apt` (Debian, Ubuntu), `dnf` (Fedora, Red Hat) or `yum`
     (older Red Hat). It runs as the administrator, or through `sudo`, which asks for the password,
     and it is not offered where neither is possible;
  4. a separate environment in the `python` folder, made by `uv`, or by `conda`, `mamba` or
     `micromamba`, where one of them is there, leaving nothing outside the folder;
  5. installing Python yourself (<https://www.python.org/downloads/>; macOS does not come with
     Python 3: running `python3` offers to install Apple's developer tools, which have one).

  `ZENZX_EXAMPLES_YES=1` takes the private copy without asking, for a launcher that has no terminal.
  `zenzx -fetch python -fetch-how apt` (or `private`, `brew`, `dnf`, `yum`, `uv`, `conda`, `manual`)
  chooses without a menu.

The `next` example does not need Python itself, only its launcher does; to start it without Python,
run `zenzx -next-examples` and then `zenzx -model=next`, as `next/README.md` says.

## What is in the folders

| File | What it is |
|---|---|
| `launch.py` | Starts an example with the parameters it needs, on every system |
| `tools/findpy.cmd` | Windows: find Python, or fetch it (with ZenZX) |
| `tools/findpy.sh` | macOS and Linux: find Python, or fetch it (with ZenZX) |
| `world/run.cmd`, `world/run.command`, `world/run.sh` | Start the connected world (Windows; macOS; Linux) |
| `link/chat.cmd`, `link/chat.command`, `link/chat.sh` | Start the chat room |
| `next/run.cmd`, `next/run.command`, `next/run.sh` | Start the Next with the examples |
| `world/world.tap` | The game, assembled, so that no assembler is needed to run it |

Building the examples from source (`zenas` for the Z80 code, `zx` for the Next's pictures and
`.nex` files) is described in each folder's README; it is only needed after changing a source.
