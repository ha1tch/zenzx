#!/usr/bin/env python3
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0
"""launch.py -- start one of the examples that needs more than a command line.

    python3 examples/launch.py world       nine Spectrums in a grid, one world (examples/world)
    python3 examples/launch.py chat        four machines in a chat room (examples/link)
    python3 examples/launch.py next        a Spectrum Next with ZenZX's three example programs

    --yes                download ZenZX without asking when it cannot be found
    --                   what follows goes to zenzx as it is: python3 launch.py world -- -scale 1

This is what run.cmd and chat.cmd call on Windows, and it does what run.sh and chat.sh do on
Linux and macOS, with the same arguments to zenzx. It needs Python 3.6 or later and nothing
that is not in Python.

Where ZenZX is looked for, in order: the ZENZX environment variable, bin/ in the folder that
holds examples/, that folder itself, its dist/ folder, the folder above it, a zenzx-* folder
in either (where an archive of the program unpacks), and the PATH. When it is not there it
offers to fetch it from the page of programs (ZENZX_SITE changes the address), checks it
against the SHA-256 in the page's runners.json and puts it in bin/.

Set ZENZX_EXAMPLES_DELAY to the seconds the machines are given to boot before the host starts
(10 by default).
"""
import argparse
import glob
import hashlib
import json
import os
import platform
import shutil
import stat
import subprocess
import sys
import time
import urllib.request

HERE = os.path.dirname(os.path.abspath(__file__))  # .../examples
ROOT = os.path.dirname(HERE)  # the folder that holds examples/: a checkout or an unpacked package
SITE = os.environ.get("ZENZX_SITE", "https://ha1tch.github.io/zenzx/")
WINDOWS = os.name == "nt"
EXE = "zenzx.exe" if WINDOWS else "zenzx"
PYTHON_PAGE = "https://www.python.org/downloads/"


def say(msg):
    print(msg, flush=True)


def fail(msg, code=1):
    print("launch.py: " + msg, file=sys.stderr, flush=True)
    sys.exit(code)


def is_program(path):
    return bool(path) and os.path.isfile(path) and (WINDOWS or os.access(path, os.X_OK))


def find_zenzx():
    """The path of the zenzx GUI program, or None."""
    cands = [os.environ.get("ZENZX", "")]
    for base in (ROOT, os.path.dirname(ROOT)):
        cands += [
            os.path.join(base, "bin", EXE),
            os.path.join(base, EXE),
            os.path.join(base, "dist", EXE),
        ]
        cands += sorted(glob.glob(os.path.join(base, "zenzx-*", EXE)))
    for c in cands:
        if is_program(c):
            return c
    return shutil.which("zenzx")


def host_key():
    system = {"Windows": "windows", "Darwin": "darwin", "Linux": "linux"}.get(platform.system(), "")
    machine = platform.machine().lower()
    arch = {"x86_64": "amd64", "amd64": "amd64", "arm64": "arm64", "aarch64": "arm64"}.get(machine, "")
    return system, arch


def fetch(url, dest, expect_sha=None):
    """Download url to dest (through a .part file), checking its SHA-256 when one is given."""
    part = dest + ".part"
    h = hashlib.sha256()
    try:
        with urllib.request.urlopen(url, timeout=60) as r, open(part, "wb") as f:
            while True:
                block = r.read(1 << 16)
                if not block:
                    break
                h.update(block)
                f.write(block)
        if expect_sha and h.hexdigest() != expect_sha.lower():
            raise OSError("the SHA-256 of what came is %s, the page says %s" % (h.hexdigest(), expect_sha))
        os.replace(part, dest)
    finally:
        if os.path.exists(part):
            os.remove(part)


def download_zenzx(assume_yes):
    """Fetch ZenZX for this system from the page of programs into bin/; its path, or None."""
    system, arch = host_key()
    if not system or not arch:
        say("There is no ZenZX program on the page for this system (%s, %s)." % (platform.system(), platform.machine()))
        return None
    base = SITE if SITE.endswith("/") else SITE + "/"
    try:
        with urllib.request.urlopen(base + "runners.json", timeout=30) as r:
            manifest = json.load(r)
    except (OSError, ValueError) as e:
        say("The page of programs (%s) cannot be read: %s" % (base, e))
        return None
    pick = [x for x in manifest.get("runners", []) if x.get("os") == system and x.get("arch") == arch]
    if not pick:
        say("The page has no program for %s/%s." % (system, arch))
        return None
    run = pick[0]
    size = run.get("size", 0)
    say("")
    say("ZenZX, the program that runs these examples, is not on this computer yet.")
    say("It is free. Version %s for %s/%s (%.1f MB) is on %s" % (manifest.get("version", "?"), system, arch, size / 1e6, base))
    if not assume_yes:
        if not sys.stdin or not sys.stdin.isatty():
            say("Run this again in a terminal window to be asked, or with --yes to have it downloaded without asking.")
            return None
        say("Nothing is installed: it is saved as one file in the folder %s, and deleting that file removes it." % os.path.join(ROOT, "bin"))
        say("")
        if input("Press Enter to download it now, or type Q to quit: ").strip().lower()[:1] in ("q", "n"):
            return None
    dest_dir = os.path.join(ROOT, "bin")
    os.makedirs(dest_dir, exist_ok=True)
    dest = os.path.join(dest_dir, EXE)
    try:
        fetch(base + run["file"], dest, run.get("sha256"))
    except (OSError, KeyError) as e:
        say("The download failed: %s" % e)
        return None
    if not WINDOWS:
        os.chmod(dest, os.stat(dest).st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
    say("Checked against the page's SHA-256 and saved as " + dest)
    return dest


def need_zenzx(assume_yes):
    path = find_zenzx() or download_zenzx(assume_yes)
    if not path:
        fail(
            "cannot find the zenzx program.\n"
            "  ZenZX is the program that runs the examples. It is free: get it from %s\n"
            "  and save it in the folder %s,\n"
            "  or set ZENZX to the path of zenzx." % (SITE, os.path.join(ROOT, "bin")),
            2,
        )
    return path


def delay():
    try:
        return float(os.environ.get("ZENZX_EXAMPLES_DELAY", "10"))
    except ValueError:
        return 10.0


def stop(proc):
    if proc and proc.poll() is None:
        proc.terminate()
        try:
            proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()


def run_world(zenzx, extra):
    """examples/world/run.sh: nine 48K machines in a grid, linked, each loading the game."""
    here = os.path.join(HERE, "world")
    tap = os.path.join(here, "world.tap")
    if not os.path.isfile(tap):
        fail("world.tap is not in %s (zenas builds it from world.asm)" % here, 2)
    ports = [str(16384 + i) for i in range(9)]
    cmd = [zenzx, "-scale", "2", "-layout", "grid", "-focus-follow", "-multi", "9:48k",
           "-listen", ",".join(ports), "-autorun", ",".join([tap] * 9)] + extra
    gui = subprocess.Popen(cmd)
    host = None
    try:
        time.sleep(delay())
        host = subprocess.Popen([sys.executable, os.path.join(here, "host.py"), "--rows", "3", "--cols", "3"] + ports)
        gui.wait()
    except KeyboardInterrupt:
        pass
    finally:
        stop(host)
        stop(gui)


def run_chat(zenzx, extra):
    """examples/link/chat.sh: four machines in a chat room, and a host that relays the lines."""
    here = os.path.join(HERE, "link")
    ports = [str(16384 + i) for i in range(4)]
    clients = [os.path.join(here, n) for n in ("chat.bas", "chat-ada.bas", "chat-clive.bas", "chat-grace.bas")]
    cmd = [zenzx, "-scale", "3", "-layout", "grid", "-multi", "4:48k,48k,128k,plus3",
           "-listen", ",".join(ports), "-autorun", ",".join(clients)] + extra
    gui = subprocess.Popen(cmd)
    try:
        time.sleep(delay())
        say("Host ready. Type a line and press Enter to chat; Ctrl-Z then Enter (Ctrl-D on Linux and macOS) to leave.")
        subprocess.call([sys.executable, os.path.join(here, "chatroom.py")] + ports)
    except KeyboardInterrupt:
        pass
    finally:
        stop(gui)


def run_next(zenzx, extra):
    """The Next with ZenZX's three examples on the card in its second slot (examples/next/README.md)."""
    say("Putting the examples on ZenZX's card (zenzx-card.img, beside the Next's boot files)...")
    subprocess.call([zenzx, "-next-examples"])
    say("Starting the Next. The first time, a window offers to download the firmware (about 70 MB).")
    say("In the Browser press D to change drive, then ENTER on gallop.nex, fuji.nex or orbit.nex.")
    subprocess.call([zenzx, "-model=next"] + extra)


def main():
    ap = argparse.ArgumentParser(description="Start one of the ZenZX examples.")
    ap.add_argument("example", choices=("world", "chat", "next"))
    ap.add_argument("--yes", action="store_true", help="download ZenZX without asking if it is not found")
    ap.add_argument("extra", nargs="*", help="after --: arguments for zenzx")
    args = ap.parse_args()
    if sys.version_info < (3, 6):
        fail("Python 3.6 or later is needed; this is %s. Get one at %s" % (platform.python_version(), PYTHON_PAGE), 2)
    zenzx = need_zenzx(args.yes)
    {"world": run_world, "chat": run_chat, "next": run_next}[args.example](zenzx, args.extra)


if __name__ == "__main__":
    main()
