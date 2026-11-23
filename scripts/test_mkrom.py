#!/usr/bin/env python3
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0
"""Tests of mkrom.py: python3 scripts/test_mkrom.py"""

import contextlib
import io
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import mkrom  # noqa: E402

ORIGINAL = mkrom.DEFAULT_ROM


def run(*argv):
    """Run mkrom.main; returns (exit code, stdout, stderr)."""
    out, err = io.StringIO(), io.StringIO()
    with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
        code = mkrom.main(list(argv))
    return code, out.getvalue(), err.getvalue()


class MkromTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)

    def path(self, name):
        return os.path.join(self.tmp.name, name)

    def write(self, name, data):
        with open(self.path(name), "wb") as f:
            f.write(data)
        return self.path(name)

    def read(self, name):
        with open(self.path(name), "rb") as f:
            return f.read()

    def test_default_is_zero_fill_at_zero(self):
        src = self.write("boot.bin", bytes(range(1, 101)))
        code, out, err = run(src)
        self.assertEqual(code, 0, err)
        rom = self.read("boot.rom")
        self.assertEqual(len(rom), 0x4000)
        self.assertEqual(rom[:100], bytes(range(1, 101)))
        self.assertEqual(rom[100:], bytes(0x4000 - 100))
        self.assertEqual(err, "")

    def test_any_address_and_notations(self):
        data = b"\xAA\xBB\xCC"
        for text, want in (("0x1234", 0x1234), ("$1234", 0x1234), ("4660", 0x1234), ("0", 0), ("010", 10)):
            with self.subTest(address=text):
                src = self.write("p.bin", data)
                code, _, err = run(src, "-a", text, "-o", self.path("o.rom"))
                self.assertEqual(code, 0, err)
                rom = self.read("o.rom")
                self.assertEqual(rom[want : want + 3], data)
                self.assertEqual(len(rom), 0x4000)
                self.assertEqual(rom[:want], bytes(want))

    def test_overlay_on_the_original_rom_changes_only_the_input(self):
        original = self.read_original()
        data = bytes([0xFF] * 0x20)
        src = self.write("patch.bin", data)
        code, _, err = run(src, "-a", "0x0038", "--fill", "rom", "-o", self.path("o.rom"))
        self.assertEqual(code, 0, err)
        rom = self.read("o.rom")
        self.assertEqual(len(rom), 0x4000)
        self.assertEqual(rom[0x38:0x58], data)
        self.assertEqual(rom[:0x38], original[:0x38])
        self.assertEqual(rom[0x58:], original[0x58:])

    def read_original(self):
        with open(ORIGINAL, "rb") as f:
            return f.read()

    def test_other_rom_to_fill_from(self):
        other = self.write("other.rom", bytes([7]) * 0x4000)
        src = self.write("p.bin", b"\x01\x02")
        code, _, err = run(src, "-f", "rom", "-r", other, "-a", "2", "-o", self.path("o.rom"))
        self.assertEqual(code, 0, err)
        self.assertEqual(self.read("o.rom"), bytes([7, 7, 1, 2]) + bytes([7]) * (0x4000 - 4))

    def test_beyond_the_end_warns_and_cuts_at_exactly_16k(self):
        src = self.write("big.bin", bytes([0x55]) * 0x300)
        code, out, err = run(src, "-a", "0x3F00", "-o", self.path("o.rom"))
        self.assertEqual(code, 0)
        self.assertIn("warning", err)
        self.assertIn("512 bytes left out", err)
        rom = self.read("o.rom")
        self.assertEqual(len(rom), 0x4000)
        self.assertEqual(rom[0x3F00:], bytes([0x55]) * 0x100)
        self.assertEqual(rom[:0x3F00], bytes(0x3F00))

    def test_input_larger_than_the_rom_at_zero_is_cut(self):
        src = self.write("huge.bin", bytes(range(256)) * 200)  # 51200 bytes
        code, _, err = run(src, "-o", self.path("o.rom"))
        self.assertEqual(code, 0)
        self.assertIn("warning", err)
        rom = self.read("o.rom")
        self.assertEqual(len(rom), 0x4000)
        self.assertEqual(rom, (bytes(range(256)) * 200)[:0x4000])

    def test_exact_fit_has_no_warning(self):
        src = self.write("fit.bin", bytes([9]) * 0x100)
        code, _, err = run(src, "-a", "0x3F00", "-o", self.path("o.rom"))
        self.assertEqual(code, 0)
        self.assertEqual(err, "")
        self.assertEqual(len(self.read("o.rom")), 0x4000)

    def test_last_byte(self):
        src = self.write("one.bin", b"\xEE")
        code, _, err = run(src, "-a", "0x3FFF", "-o", self.path("o.rom"))
        self.assertEqual(code, 0, err)
        rom = self.read("o.rom")
        self.assertEqual(rom[0x3FFF], 0xEE)
        self.assertEqual(err, "")

    def test_address_outside_the_rom_is_an_error_and_writes_nothing(self):
        src = self.write("p.bin", b"\x01")
        for text in ("0x4000", "65535", "-1"):
            with self.subTest(address=text):
                try:
                    code, _, err = run(src, "-a=" + text, "-o", self.path("o.rom"))
                except SystemExit as e:  # argparse rejecting it
                    code, err = e.code, ""
                self.assertNotEqual(code, 0)
                self.assertFalse(os.path.exists(self.path("o.rom")))

    def test_bad_address_text_is_rejected(self):
        src = self.write("p.bin", b"\x01")
        with contextlib.redirect_stderr(io.StringIO()):
            with self.assertRaises(SystemExit) as cm:
                mkrom.main([src, "-a", "banana"])
        self.assertEqual(cm.exception.code, 2)

    def test_empty_input_warns_and_gives_the_fill(self):
        src = self.write("empty.bin", b"")
        code, _, err = run(src, "-o", self.path("o.rom"))
        self.assertEqual(code, 0)
        self.assertIn("empty", err)
        self.assertEqual(self.read("o.rom"), bytes(0x4000))

    def test_missing_input_is_an_error(self):
        code, _, err = run(self.path("nowhere.bin"))
        self.assertEqual(code, 1)
        self.assertIn("cannot read", err)

    def test_rom_to_fill_from_must_be_16k(self):
        short = self.write("short.rom", bytes(100))
        src = self.write("p.bin", b"\x01")
        code, _, err = run(src, "-f", "rom", "-r", short, "-o", self.path("o.rom"))
        self.assertEqual(code, 1)
        self.assertIn("exactly 16384", err)
        self.assertFalse(os.path.exists(self.path("o.rom")))

    def test_will_not_overwrite_its_own_input(self):
        src = self.write("same.rom", b"\x01\x02")
        code, _, err = run(src)  # the default output name is same.rom
        self.assertEqual(code, 1)
        self.assertIn("is the input", err)
        self.assertEqual(self.read("same.rom"), b"\x01\x02")

    def test_default_output_name(self):
        src = self.write("thing.bin", b"\x01")
        code, out, err = run(src)
        self.assertEqual(code, 0, err)
        self.assertTrue(os.path.exists(self.path("thing.rom")))


if __name__ == "__main__":
    unittest.main()
