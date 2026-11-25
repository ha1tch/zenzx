#!/usr/bin/env python3
"""Tests for make_icons.py: every size is an exact whole-number enlargement, the picture is
centred on a transparent square, and the .ico and .icns files hold what they say they do.
Run: python3 -W error scripts/test_make_icons.py

Copyright (c) 2026 haitch - h@ual.li - https://oldbytes.space/@haitchfive
Licensed under the Apache License, Version 2.0: https://www.apache.org/licenses/LICENSE-2.0
"""
import os
import struct
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import make_icons as mi  # noqa: E402


class Icons(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        with open(mi.MASTER, "rb") as f:
            cls.master = mi.read_png(f.read())
        cls.files = mi.build()

    def test_master_is_the_64_by_40_picture(self):
        self.assertEqual(self.master[:2], (64, 40))
        colours = {px for row in self.master[2] for px in row}
        self.assertEqual(colours, {(0, 0, 0, 255), (200, 200, 200, 255)})

    def test_every_size_is_a_nearest_neighbour_enlargement(self):
        for k in mi.SCALES:
            side = 64 * k
            w, h, rows = mi.read_png(self.files["png/zenzx-%d.png" % side])
            self.assertEqual((w, h), (side, side))
            top = (side - 40 * k) // 2
            for y in range(side):
                for x in range(side):
                    inside = top <= y < top + 40 * k
                    want = self.master[2][(y - top) // k][x // k] if inside else (0, 0, 0, 0)
                    if rows[y][x] != want:
                        self.fail("%d px: pixel (%d, %d) is %r, wanted %r" % (side, x, y, rows[y][x], want))

    def test_no_colour_is_made_up(self):
        # An enlargement with smoothing would make greys between black and the keys' grey.
        for name, data in self.files.items():
            if name.endswith(".png"):
                seen = {px for row in mi.read_png(data)[2] for px in row}
                self.assertEqual(seen, {(0, 0, 0, 0), (0, 0, 0, 255), (200, 200, 200, 255)}, name)

    def test_ico_holds_64_128_256(self):
        d = self.files["zenzx.ico"]
        self.assertEqual(struct.unpack("<HHH", d[:6]), (0, 1, 3))
        for i, side in enumerate((64, 128, 256)):
            w, h, _, _, planes, bits, size, off = struct.unpack("<BBBBHHII", d[6 + 16 * i:22 + 16 * i])
            self.assertEqual((w, h), (side % 256, side % 256))
            self.assertEqual(d[off:off + size], self.files["png/zenzx-%d.png" % side])

    def test_icns_holds_five_pictures(self):
        d = self.files["zenzx.icns"]
        self.assertEqual(d[:4], b"icns")
        self.assertEqual(struct.unpack(">I", d[4:8])[0], len(d))
        pos, seen = 8, []
        while pos < len(d):
            kind, n = struct.unpack(">4sI", d[pos:pos + 8])
            seen.append(kind)
            self.assertEqual(d[pos + 8:pos + n][:8], b"\x89PNG\r\n\x1a\n")
            pos += n
        self.assertEqual(seen, [b"icp6", b"ic07", b"ic08", b"ic09", b"ic10"])
        self.assertEqual(pos, len(d))

    def test_the_files_in_the_tree_are_what_the_script_makes(self):
        for rel, data in self.files.items():
            with open(os.path.join(mi.DIR, rel), "rb") as f:
                self.assertEqual(f.read(), data, rel)


if __name__ == "__main__":
    unittest.main()
