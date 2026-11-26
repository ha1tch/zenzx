#!/usr/bin/env python3
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0
"""Tests of scripts/check_purego_ffi.py: python3 scripts/test_check_purego_ffi.py"""

import contextlib
import importlib.util
import io
import os
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("check_purego_ffi", os.path.join(HERE, "check_purego_ffi.py"))
cp = importlib.util.module_from_spec(spec)
spec.loader.exec_module(cp)

GOOD = '''package rl

var (
	setWindowSize = dll.MustPrep("SetWindowSize", &ffi.TypeVoid, &ffi.TypeSint32, &ffi.TypeSint32)
	getScreenWidth = dll.MustPrep("GetScreenWidth", &ffi.TypeSint32)
	drawCircleLinesV = dll.MustPrep("DrawCircleLinesV", &ffi.TypeVoid, &typeVector2, &ffi.TypeFloat, &typeColor)
)

func SetWindowSize(width int, height int) {
	w, h := int32(width), int32(height)
	setWindowSize.Call(nil, &w, &h)
}

func GetScreenWidth() int {
	var ret ffi.Arg
	getScreenWidth.Call(&ret)
	return int(ret)
}

func DrawCircleLinesV(center Vector2, radius float32, col color.RGBA) {
	drawCircleLinesV.Call(nil, unsafe.Pointer(&center), &radius, unsafe.Pointer(&col))
}
'''

# SetWindowSize prepared without its void result, as raylib-go v0.60.0 had it.
NO_RESULT = GOOD.replace('"SetWindowSize", &ffi.TypeVoid, ', '"SetWindowSize", ')
# DrawCircleLinesV prepared with four types but called with three values.
TOO_MANY = GOOD.replace("&typeVector2, &ffi.TypeFloat, &typeColor)", "&typeVector2, &ffi.TypeFloat, &ffi.TypeFloat, &typeColor)")


class Audit(unittest.TestCase):
    def test_a_file_that_agrees(self):
        n, bad = cp.audit(GOOD)
        self.assertEqual((n, bad), (3, []))

    def test_the_result_type_left_out(self):
        n, bad = cp.audit(NO_RESULT)
        self.assertEqual(len(bad), 1)
        line, cname, prepared, called = bad[0]
        self.assertEqual((cname, prepared, called), ("SetWindowSize", 1, 2))
        self.assertEqual(GOOD.splitlines()[line - 1].strip(), "setWindowSize.Call(nil, &w, &h)")

    def test_an_argument_type_too_many(self):
        n, bad = cp.audit(TOO_MANY)
        self.assertEqual([(b[1], b[2], b[3]) for b in bad], [("DrawCircleLinesV", 4, 3)])

    def test_a_call_with_nested_brackets_counts_its_arguments_once(self):
        src = 'x = dll.MustPrep("F", &ffi.TypeVoid, &ffi.TypePointer)\nfunc F() { x.Call(nil, unsafe.Pointer(&a[0], b)) }\n'
        self.assertEqual(cp.audit(src), (1, []))

    def test_a_function_with_no_arguments(self):
        src = 'x = dll.MustPrep("F", &ffi.TypeVoid)\nfunc F() { x.Call(nil) }\n'
        self.assertEqual(cp.audit(src), (1, []))


class Main(unittest.TestCase):
    def run_main(self, text):
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "raylib_purego.go")
            with open(p, "w") as f:
                f.write(text)
            out, err = io.StringIO(), io.StringIO()
            with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
                rc = cp.main([p])
            return rc, out.getvalue() + err.getvalue()

    def test_exit_status(self):
        self.assertEqual(self.run_main(GOOD)[0], 0)
        rc, text = self.run_main(NO_RESULT)
        self.assertEqual(rc, 1)
        self.assertIn("SetWindowSize: prepared with 1 arguments, called with 2", text)

    def test_a_file_with_nothing_prepared_is_not_a_pass(self):
        self.assertEqual(self.run_main("package rl\n")[0], 2)

    def test_a_file_that_is_not_there(self):
        with contextlib.redirect_stderr(io.StringIO()):
            self.assertEqual(cp.main(["/no/such/raylib_purego.go"]), 2)

    def test_the_library_this_module_selects_agrees(self):
        path = cp.selected_file()
        if path is None or not os.path.isfile(path):
            self.skipTest("raylib-go is not downloaded")
        with open(path, encoding="utf-8") as f:
            n, bad = cp.audit(f.read())
        self.assertGreater(n, 100)
        self.assertEqual(bad, [])


if __name__ == "__main__":
    unittest.main()
