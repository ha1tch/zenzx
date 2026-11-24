#!/usr/bin/env python3
"""next_card.py - make a bootable SD card image for -model=next from the official
emulator distribution.

Usage:
    python3 scripts/next_card.py sn-emulator-26.11.zip thirdparty/
    python3 scripts/next_card.py --repair thirdparty/next-card.img

scripts/get_next_card.sh (or `make next-card`) downloads the zip and runs the first form.
The second form corrects the free space record (point 4 below) of a card made by an earlier
version of this script, in place, keeping whatever has been saved on it.

The zip is the "emulator" distribution from https://www.specnext.com/latestdistro/ (about
70 MB). It holds a 1 GB card image and the two ROMs that match it. This script writes, into
the output folder:

    next-card.img    the card, rebuilt (see below)
    enNxtmmc.rom     the DivMMC ROM of the same distribution, for -divmmc-rom
    enNextZX.rom     the machine ROM of the same distribution (the firmware loads its own
                     copy from the card; this one is kept for reference)

The ROMs and the card must come from the same distribution: NextZXOS stops on a mismatch.
The boot ROM (nextboot.rom, for -bootrom) is not in the distribution; it has to be supplied
separately.

What "rebuilt" means, and why:

  1. The distribution's image is FAT32 with 32K clusters, which gives it about 32,750
     clusters. FAT32 needs at least 65,525, and the Next's firmware checks. The card is
     therefore formatted again with 4K clusters, same size and same partition table, and
     every file copied across.
  2. The distribution's /machines/next/config.ini says timing=8, which makes the firmware
     run its video-mode test and wait for a key before it writes the answer back to the
     card. The copy says timing=0 (VGA, 50 Hz), so the firmware goes straight on.
  3. If /machines/next/zx81.rom is the 16K file of older distributions it is doubled to the
     32K the firmware's menu loads.
  4. The free space record (the FSInfo sector) is written from the finished FAT. The
     library that formats the card leaves the empty volume's values there, and NextZXOS
     then reports the whole card as free.

This is the same preparation the other Next emulators do before they boot a card; the
script was written from their documentation and from the firmware's own source, not from
their code.

Needs pyfatfs:  pip install pyfatfs   (Linux and macOS)

Copyright (c) 2026 haitch - h@ual.li - https://oldbytes.space/@haitchfive
Licensed under the Apache License, Version 2.0: https://www.apache.org/licenses/LICENSE-2.0
"""
import os
import re
import struct
import sys
import tempfile
import warnings
import zipfile

SECTOR = 512
MIN_FAT32_CLUSTERS = 65525


def die(msg):
    sys.stderr.write("next_card.py: " + msg + "\n")
    sys.exit(1)


def cluster_count(path, start):
    with open(path, "rb") as f:
        f.seek(start * SECTOR)
        b = f.read(SECTOR)
    _, spc, reserved, nfats = struct.unpack("<HBHB", b[11:17])
    total = struct.unpack("<I", b[32:36])[0]
    fatsz = struct.unpack("<I", b[36:40])[0]
    return (total - reserved - nfats * fatsz) // spc, spc


def write_fsinfo(path, start):
    """Count the free clusters in the FAT and write the count and the first free cluster to
    the FSInfo sector and its backup. The library leaves the values of the empty volume
    there, and NextZXOS believes them."""
    with open(path, "r+b") as f:
        f.seek(start * SECTOR)
        b = f.read(SECTOR)
        _, spc, reserved, nfats = struct.unpack("<HBHB", b[11:17])
        total = struct.unpack("<I", b[32:36])[0]
        fatsz = struct.unpack("<I", b[36:40])[0]
        fsinfo, backup = struct.unpack("<HH", b[48:52])
        clusters = (total - reserved - nfats * fatsz) // spc
        f.seek((start + reserved) * SECTOR)
        entries = struct.unpack("<%dI" % (clusters + 2), f.read((clusters + 2) * 4))
        free = [n for n in range(2, clusters + 2) if entries[n] & 0x0FFFFFFF == 0]
        count, first = len(free), (free[0] if free else 0xFFFFFFFF)
        sectors = [fsinfo] + ([backup + fsinfo] if backup else [])
        for s in sectors:
            f.seek((start + s) * SECTOR)
            info = bytearray(f.read(SECTOR))
            if info[0:4] != b"RRaA" or info[484:488] != b"rrAa":
                die("sector %d of the partition is not an FSInfo sector" % s)
            info[488:496] = struct.pack("<II", count, first)
            f.seek((start + s) * SECTOR)
            f.write(info)
    return count, first


def rebuild(src, dst):
    from pyfatfs.PyFat import PyFat
    from pyfatfs.PyFatFS import PyFatFS

    with open(src, "rb") as f:
        mbr = f.read(SECTOR)
    if mbr[510:512] != b"\x55\xaa" or mbr[446 + 4] not in (0x0B, 0x0C):
        die("the image has no FAT32 partition in its first partition slot")
    start, sectors = struct.unpack("<II", mbr[446 + 8:446 + 16])
    before, spc = cluster_count(src, start)
    print("source: %d clusters of %d sectors" % (before, spc))

    with open(dst, "wb") as f:
        f.truncate(os.path.getsize(src))
        f.write(mbr)
    fat = PyFat(offset=start * SECTOR)
    fat.mkfs(dst, PyFat.FAT_TYPE_FAT32, size=sectors * SECTOR, label="NEXT")
    fat.close()

    s = PyFatFS(src, offset=start * SECTOR, read_only=True)
    d = PyFatFS(dst, offset=start * SECTOR)
    files = dirs = size = 0
    for path, subdirs, names in s.walk("/"):
        base = path.rstrip("/")
        for x in subdirs:
            d.makedirs(base + "/" + x.name, recreate=True)
            dirs += 1
        for x in names:
            p = base + "/" + x.name
            data = s.readbytes(p)
            low = p.lower()
            if low == "/machines/next/config.ini":
                data, n = re.subn(rb"(?im)^timing=\d+", b"timing=0", data)
                if n != 1:
                    die("config.ini has %d timing lines, expected 1" % n)
                print("config.ini: timing=0")
            elif low == "/machines/next/zx81.rom" and len(data) == 16384:
                data = data + data
                print("zx81.rom: doubled to 32K")
            d.writebytes(p, data)
            files += 1
            size += len(data)
    s.close()
    d.close()

    free, first = write_fsinfo(dst, start)
    print("free space record: %d clusters free, first free cluster %d" % (free, first))

    after, spc = cluster_count(dst, start)
    print("card: %d files, %d folders, %d bytes; %d clusters of %d sectors" % (files, dirs, size, after, spc))
    if after < MIN_FAT32_CLUSTERS:
        die("the rebuilt card still has too few clusters for FAT32")


def repair(path):
    """Bring the free space record of an existing card up to date, in place."""
    with open(path, "rb") as f:
        mbr = f.read(SECTOR)
    if mbr[510:512] != b"\x55\xaa" or mbr[446 + 4] not in (0x0B, 0x0C):
        die("the image has no FAT32 partition in its first partition slot")
    start = struct.unpack("<I", mbr[446 + 8:446 + 12])[0]
    free, first = write_fsinfo(path, start)
    print("%s: free space record set to %d clusters free, first free cluster %d" % (path, free, first))


def main():
    if len(sys.argv) == 3 and sys.argv[1] == "--repair":
        repair(sys.argv[2])
        return
    if len(sys.argv) != 3:
        die("usage: python3 scripts/next_card.py sn-emulator-NN.NN.zip OUTPUT_FOLDER\n"
            "       python3 scripts/next_card.py --repair next-card.img")
    try:
        import pyfatfs  # noqa: F401
    except ImportError:
        die("pyfatfs is not installed: pip install pyfatfs")
    warnings.simplefilter("ignore")
    zpath, out = sys.argv[1], sys.argv[2]
    os.makedirs(out, exist_ok=True)
    with zipfile.ZipFile(zpath) as z, tempfile.TemporaryDirectory() as tmp:
        names = {os.path.basename(n).lower(): n for n in z.namelist()}
        img = next((names[n] for n in names if n.endswith(".img")), None)
        if img is None:
            die("no .img file in " + zpath)
        for rom in ("ennxtmmc.rom", "ennextzx.rom"):
            if rom not in names:
                die("no " + rom + " in " + zpath)
            target = os.path.join(out, os.path.basename(names[rom]))
            with open(target, "wb") as f:
                f.write(z.read(names[rom]))
            print("wrote", target)
        z.extract(img, tmp)
        dst = os.path.join(out, "next-card.img")
        rebuild(os.path.join(tmp, img), dst)
        print("wrote", dst)


if __name__ == "__main__":
    main()
