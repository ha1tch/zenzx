#!/usr/bin/env python3
"""make_icons.py - the application icon in every size and file format an installer needs.

The icon is the picture in assets/icon/speccy01.zani, frame00 (a 64 by 40 Spectrum), saved as it
is in assets/icon/speccy.png. Every size is a whole-number enlargement with no smoothing, so that
the pixels stay square and sharp: 64 is 1x, 128 is 2x, 256 is 4x, 512 is 8x and 1024 is 16x. The
picture is wider than it is tall, so it sits in the middle of a square, with nothing drawn above
and below it (those pixels are transparent).

  assets/icon/png/zenzx-64.png ... zenzx-1024.png   the square pictures
  assets/icon/zenzx.ico                             Windows: 64, 128 and 256 (PNG entries)
  assets/icon/zenzx.icns                            macOS: 64, 128, 256, 512 and 1024

Linux takes the PNG files (hicolor/<size>x<size>/apps/zenzx.png). No sizes below 64 are made:
there is no whole-number way down from 64 that keeps all of the picture.

Usage: python3 scripts/make_icons.py          write the files
       python3 scripts/make_icons.py check    exit 1 if the files in assets/icon differ from what
                                              this would write

Standard library only (Python 3.8 or later).

Copyright (c) 2026 haitch - h@ual.li - https://oldbytes.space/@haitchfive
Licensed under the Apache License, Version 2.0: https://www.apache.org/licenses/LICENSE-2.0
"""
import os
import struct
import sys
import zlib

HERE = os.path.dirname(os.path.abspath(__file__))
DIR = os.path.join(HERE, "..", "assets", "icon")
MASTER = os.path.join(DIR, "speccy.png")
SCALES = (1, 2, 4, 8, 16)           # of 64: 64, 128, 256, 512, 1024
ICO_SCALES = (1, 2, 4)              # 64, 128, 256
ICNS_TYPES = {1: b"icp6", 2: b"ic07", 4: b"ic08", 8: b"ic09", 16: b"ic10"}


def read_png(data):
    """A non-interlaced 8-bit RGB or RGBA PNG as (width, height, rows of (r, g, b, a))."""
    if data[:8] != b"\x89PNG\r\n\x1a\n":
        raise ValueError("not a PNG")
    pos, idat, ihdr = 8, b"", None
    while pos < len(data):
        n, kind = struct.unpack(">I4s", data[pos:pos + 8])
        body = data[pos + 8:pos + 8 + n]
        pos += 12 + n
        if kind == b"IHDR":
            ihdr = struct.unpack(">IIBBBBB", body)
        elif kind == b"IDAT":
            idat += body
    w, h, depth, ctype, _, _, interlace = ihdr
    if depth != 8 or ctype not in (2, 6) or interlace:
        raise ValueError("only non-interlaced 8-bit RGB or RGBA PNG files are read")
    bpp = 4 if ctype == 6 else 3
    raw = zlib.decompress(idat)
    stride = w * bpp
    rows, prev = [], bytearray(stride)
    for y in range(h):
        f = raw[y * (stride + 1)]
        cur = bytearray(raw[y * (stride + 1) + 1:(y + 1) * (stride + 1)])
        for i in range(stride):
            a = cur[i - bpp] if i >= bpp else 0
            b = prev[i]
            c = prev[i - bpp] if i >= bpp else 0
            if f == 1:
                cur[i] = (cur[i] + a) & 255
            elif f == 2:
                cur[i] = (cur[i] + b) & 255
            elif f == 3:
                cur[i] = (cur[i] + (a + b) // 2) & 255
            elif f == 4:
                p = a + b - c
                pa, pb, pc = abs(p - a), abs(p - b), abs(p - c)
                pr = a if pa <= pb and pa <= pc else (b if pb <= pc else c)
                cur[i] = (cur[i] + pr) & 255
            elif f != 0:
                raise ValueError("bad PNG filter")
        rows.append([tuple(cur[x * bpp:(x + 1) * bpp]) + ((255,) if bpp == 3 else ())
                     for x in range(w)])
        prev = cur
    return w, h, rows


def write_png(rows):
    h, w = len(rows), len(rows[0])
    raw = b"".join(b"\x00" + bytes(v for px in row for v in px) for row in rows)

    def chunk(kind, body):
        c = struct.pack(">I", len(body)) + kind + body
        return c + struct.pack(">I", zlib.crc32(kind + body) & 0xFFFFFFFF)

    return (b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", w, h, 8, 6, 0, 0, 0))
            + chunk(b"IDAT", zlib.compress(raw, 9)) + chunk(b"IEND", b""))


def square(master, k):
    """The master enlarged k times, nearest neighbour, centred on a transparent 64k square."""
    w, h, rows = master
    side = 64 * k
    top = (side - h * k) // 2
    clear = (0, 0, 0, 0)
    out = [[clear] * side for _ in range(side)]
    for y in range(h * k):
        src = rows[y // k]
        out[top + y] = [src[x // k] for x in range(side)]
    return out


def ico(pngs):
    """A Windows icon holding PNG pictures: (side, bytes) pairs."""
    head = struct.pack("<HHH", 0, 1, len(pngs))
    entries, body, offset = b"", b"", 6 + 16 * len(pngs)
    for side, png in pngs:
        entries += struct.pack("<BBBBHHII", side % 256, side % 256, 0, 0, 1, 32, len(png), offset)
        body += png
        offset += len(png)
    return head + entries + body


def icns(items):
    """A macOS icon holding PNG pictures: (type, bytes) pairs."""
    body = b"".join(t + struct.pack(">I", 8 + len(p)) + p for t, p in items)
    return b"icns" + struct.pack(">I", 8 + len(body)) + body


def build():
    """{path relative to assets/icon: bytes} for every file this makes."""
    with open(MASTER, "rb") as f:
        master = read_png(f.read())
    if master[:2] != (64, 40):
        raise SystemExit("make_icons: %s is %dx%d, not 64x40" % (MASTER, master[0], master[1]))
    png = {k: write_png(square(master, k)) for k in SCALES}
    files = {"png/zenzx-%d.png" % (64 * k): png[k] for k in SCALES}
    files["zenzx.ico"] = ico([(64 * k, png[k]) for k in ICO_SCALES])
    files["zenzx.icns"] = icns([(ICNS_TYPES[k], png[k]) for k in SCALES])
    return files


def main(argv):
    files = build()
    if argv[1:] == ["check"]:
        bad = []
        for rel, data in sorted(files.items()):
            try:
                with open(os.path.join(DIR, rel), "rb") as f:
                    same = f.read() == data
            except OSError:
                same = False
            if not same:
                bad.append(rel)
        if bad:
            print("make_icons: out of date: " + ", ".join(bad))
            return 1
        print("make_icons: %d files match" % len(files))
        return 0
    for rel, data in sorted(files.items()):
        path = os.path.join(DIR, rel)
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "wb") as f:
            f.write(data)
        print("wrote %s (%d bytes)" % (os.path.normpath(os.path.join("assets", "icon", rel)), len(data)))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
