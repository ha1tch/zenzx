#!/usr/bin/env python3
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0
"""
scripts/mkrom.py

Make a 16 KiB ZX Spectrum ROM image out of a binary file of any size.

The input is placed in the ROM at an address (0x0000 by default); the rest of
the ROM is filled with zeroes, or with the original 48K ROM, so that the input
acts as an overlay on it. The output is always exactly 16384 bytes.

    mkrom.py boot.bin                      # boot.rom: boot.bin at 0x0000, zeroes after it
    mkrom.py patch.bin -a 0x0038 --fill rom    # patch.bin over the original ROM at 0x0038
    mkrom.py patch.bin -a 0x3D00 --fill rom --rom rom/48-spanish.rom -o es-patched.rom

An input that does not fit between its address and the end of the ROM is cut
at the end of the ROM, with a warning. Python 3.8 or later, standard library
only.
"""

import argparse
import os
import sys

ROM_SIZE = 0x4000  # 16 KiB

# The ROM that --fill rom takes unless --rom says another: the ZenZX 48K ROM,
# found from this script's own place (scripts/ in the repository).
DEFAULT_ROM = os.path.join(os.path.dirname(os.path.abspath(__file__)), os.pardir, "rom", "48.rom")


class RomError(Exception):
    """A reason to stop, with the message to give."""


def parse_address(text):
    """An address as 0x1234, $1234 or decimal."""
    t = text.strip()
    try:
        if t.startswith("$"):
            return int(t[1:], 16)
        if t.lower().startswith(("0x", "0b", "0o")):
            return int(t, 0)
        return int(t, 10)
    except ValueError:
        raise argparse.ArgumentTypeError("%r is not an address (0x1234, $1234 or decimal)" % text)


def read_file(path, what):
    try:
        with open(path, "rb") as f:
            return f.read()
    except OSError as e:
        raise RomError("cannot read %s %s: %s" % (what, path, e.strerror or e))


def background(fill, rom_path):
    """The 16 KiB the input is placed over."""
    if fill == "zero":
        return bytes(ROM_SIZE)
    data = read_file(rom_path, "the ROM to fill from,")
    if len(data) != ROM_SIZE:
        raise RomError("%s is %d bytes; a 16 KiB ROM to fill from must be exactly %d" % (rom_path, len(data), ROM_SIZE))
    return data


def build(payload, address, base):
    """Place payload at address over base. Returns (the ROM, how many bytes of payload were cut off)."""
    if not 0 <= address < ROM_SIZE:
        raise RomError("address 0x%04X is outside the ROM (0x0000 to 0x%04X)" % (address, ROM_SIZE - 1))
    if len(base) != ROM_SIZE:
        raise RomError("the base image is %d bytes, not %d" % (len(base), ROM_SIZE))
    kept = payload[: ROM_SIZE - address]
    rom = bytearray(base)
    rom[address : address + len(kept)] = kept
    return bytes(rom), len(payload) - len(kept)


def main(argv=None):
    ap = argparse.ArgumentParser(
        prog="mkrom.py",
        description="Make a 16 KiB ZX Spectrum ROM from a binary file placed at an address; the rest is zeroes or the original ROM.",
    )
    ap.add_argument("input", help="the binary file, of any size")
    ap.add_argument("-o", "--output", help="the ROM to write (default: the input's name with .rom)")
    ap.add_argument("-a", "--address", type=parse_address, default=0, metavar="ADDR",
                    help="where in the ROM the input goes: 0x1234, $1234 or decimal (default 0x0000)")
    ap.add_argument("-f", "--fill", choices=("zero", "rom"), default="zero",
                    help="what is around the input: zero (default) or rom, the original ROM, so that the input is an overlay")
    ap.add_argument("-r", "--rom", default=DEFAULT_ROM, metavar="PATH",
                    help="the 16 KiB ROM that --fill rom uses (default: rom/48.rom of this repository)")
    args = ap.parse_args(argv)

    try:
        payload = read_file(args.input, "the input")
        base = background(args.fill, args.rom)
        rom, cut = build(payload, args.address, base)

        out = args.output
        if out is None:
            out = os.path.splitext(args.input)[0] + ".rom"
        if os.path.exists(out) and os.path.samefile(out, args.input):
            raise RomError("%s is the input; give another name with -o" % out)
        try:
            with open(out, "wb") as f:
                f.write(rom)
        except OSError as e:
            raise RomError("cannot write %s: %s" % (out, e.strerror or e))
    except RomError as e:
        print("mkrom: error: %s" % e, file=sys.stderr)
        return 1

    placed = len(payload) - cut
    if not payload:
        print("mkrom: warning: the input is empty; the ROM is only the fill", file=sys.stderr)
    if cut:
        print("mkrom: warning: %d bytes at 0x%04X end at 0x%04X, past the 16 KiB ROM (0x%04X); cut at 0x%04X, %d bytes left out"
              % (len(payload), args.address, args.address + len(payload) - 1, ROM_SIZE - 1, ROM_SIZE - 1, cut),
              file=sys.stderr)
    where = "nothing placed" if placed == 0 else "input at 0x%04X-0x%04X (%d bytes)" % (args.address, args.address + placed - 1, placed)
    print("%s: %d bytes, %s, around it %s" % (out, len(rom), where, "zeroes" if args.fill == "zero" else "the ROM " + os.path.basename(args.rom)))
    return 0


if __name__ == "__main__":
    sys.exit(main())
