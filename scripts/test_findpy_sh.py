#!/usr/bin/env python3
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0
"""Tests for examples/tools/findpy.sh: finding a Python for the examples, and getting one, through
`zenzx -fetch python`, for a person who has none. ZenZX is the real headless program (built with
Go, or ZENZX_HEADLESS_BIN); the download comes from a web server of this test's own, so the real
python-build-standalone is not fetched.

    python3 scripts/test_findpy_sh.py
"""
import hashlib
import io
import os
import pty
import shutil
import subprocess
import sys
import tarfile
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import test_fetch_cli as fc  # noqa: E402  (the web server and the program, shared with that test)

REPO = os.path.dirname(HERE)
FINDPY = os.path.join(REPO, "examples", "tools", "findpy.sh")
TOOLS = ("sh", "uname", "cat")
STUB_OK = "#!/bin/sh\nexit 0\n"
STUB_OLD = "#!/bin/sh\nexit 1\n"          # a python older than 3.6, as far as the check can tell


def locate(tool):
    """The system's own copy of a tool: /usr/bin and /bin before the PATH, which may hold wrappers."""
    for d in ("/usr/bin", "/bin"):
        p = os.path.join(d, tool)
        if os.path.isfile(p) and os.access(p, os.X_OK):
            return p
    return shutil.which(tool)


def have_tools():
    return all(locate(t) for t in TOOLS)


@unittest.skipUnless(have_tools(), "a tool findpy.sh uses is missing here")
class FindPyTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.pkg = os.path.join(self.tmp.name, "pkg")           # what a package unpacks to: bin/ and examples/
        self.ex = os.path.join(self.pkg, "examples")
        os.makedirs(os.path.join(self.ex, "tools"))
        shutil.copy(FINDPY, os.path.join(self.ex, "tools", "findpy.sh"))
        # A PATH of only what the script needs, so that this computer's own Python is not found.
        self.bin = os.path.join(self.tmp.name, "toolbox")
        os.makedirs(self.bin)
        for t in TOOLS:
            os.symlink(locate(t), os.path.join(self.bin, t))
        self.env = {"PATH": self.bin, "HOME": self.tmp.name, "ZENZX_PYTHON_DIRS": ""}

    def stub(self, path, body=STUB_OK):
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "w") as f:
            f.write(body)
        os.chmod(path, 0o755)

    def install_zenzx(self):
        """The real headless program, in bin/ beside the examples folder."""
        dest = os.path.join(self.pkg, "bin", "zenzx-headless")
        os.makedirs(os.path.dirname(dest), exist_ok=True)
        shutil.copy(fc.program(), dest)
        os.chmod(dest, 0o755)
        return dest

    def source(self, extra_env=None, stdin=subprocess.DEVNULL, tail='echo "PY=$PY"', head=""):
        env = dict(self.env, **(extra_env or {}))
        script = '%s examples_dir=%s; . "$examples_dir/tools/findpy.sh"; %s' % (head, self.ex, tail)
        return subprocess.run(["sh", "-c", script], env=env, capture_output=True, text=True, timeout=120, stdin=stdin)

    def tarball(self, with_python=True):
        buf = io.BytesIO()
        with tarfile.open(fileobj=buf, mode="w:gz") as t:
            name, data, mode = ("python/bin/python3", STUB_OK.encode(), 0o755) if with_python else ("python/README", b"nothing", 0o644)
            info = tarfile.TarInfo(name)
            info.size, info.mode = len(data), mode
            t.addfile(info, io.BytesIO(data))
        return buf.getvalue(), hashlib.sha256(buf.getvalue()).hexdigest()

    def leftovers(self):
        return [n for n in os.listdir(self.ex) if n.startswith(".python")]

    # --- finding ---------------------------------------------------------------------------

    def test_a_python3_on_the_path_is_used(self):
        self.stub(os.path.join(self.bin, "python3"))
        r = self.source()
        self.assertIn("PY=" + os.path.join(self.bin, "python3"), r.stdout)
        self.assertEqual(r.stderr, "")

    def test_the_private_copy_comes_before_the_path(self):
        self.stub(os.path.join(self.bin, "python3"))
        private = os.path.join(self.ex, "python", "bin", "python3")
        self.stub(private)
        self.assertIn("PY=" + private, self.source().stdout)

    def test_a_separate_environment_that_has_only_python_is_used(self):
        env_python = os.path.join(self.ex, "python", "bin", "python")
        self.stub(env_python)
        self.assertIn("PY=" + env_python, self.source().stdout)

    def test_a_python_that_does_not_run_as_3_6_is_set_aside(self):
        self.stub(os.path.join(self.bin, "python3"), STUB_OLD)
        self.stub(os.path.join(self.bin, "python"), STUB_OK)
        self.assertIn("PY=" + os.path.join(self.bin, "python"), self.source().stdout)

    def test_the_launchers_own_arguments_survive(self):
        self.stub(os.path.join(self.bin, "python3"))
        r = self.source(head='set -- one "two words";', tail='echo "ARGS=$# $1|$2"')
        self.assertIn("ARGS=2 one|two words", r.stdout)

    # --- none, and nobody to ask ---------------------------------------------------------------

    def test_none_and_no_one_to_ask_says_what_python_is_and_where_to_get_it(self):
        self.install_zenzx()
        r = self.source()
        self.assertIn("PY=\n", r.stdout)
        for words in ("free helper program called Python", "not the snake", "https://www.python.org/downloads/",
                      "ZENZX_EXAMPLES_YES=1"):
            self.assertIn(words, r.stdout)
        self.assertFalse(os.path.exists(os.path.join(self.ex, "python")))
        self.assertEqual(self.leftovers(), [])

    def test_none_and_no_zenzx_says_where_zenzx_goes(self):
        r = self.source({"ZENZX_EXAMPLES_YES": "1"})
        self.assertIn("PY=\n", r.stdout)
        self.assertIn("ZenZX, which fetches it, is not where it can be found", r.stdout)
        self.assertIn("https://ha1tch.github.io/zenzx/", r.stdout)
        self.assertIn("https://www.python.org/downloads/", r.stdout)

    def test_zenzx_is_looked_for_where_launch_py_looks(self):
        record = os.path.join(self.tmp.name, "args")
        fake = '#!/bin/sh\necho "$0" > %s\nexit 1\n' % record
        for where in (os.path.join(self.pkg, "bin"), self.pkg, os.path.join(self.pkg, "dist"), os.path.dirname(self.pkg)):
            if os.path.exists(record):
                os.remove(record)
            self.stub(os.path.join(where, "zenzx"), fake)
            self.source({"ZENZX_EXAMPLES_YES": "1"})
            self.assertEqual(fc.slurp(record).strip() if os.path.exists(record) else None, os.path.join(where, "zenzx"), where)
            os.remove(os.path.join(where, "zenzx"))
        # the headless build is preferred to the window one, which needs a graphics library
        self.stub(os.path.join(self.pkg, "bin", "zenzx"), fake)
        self.stub(os.path.join(self.pkg, "bin", "zenzx-headless"), fake)
        self.source({"ZENZX_EXAMPLES_YES": "1"})
        self.assertTrue(fc.slurp(record).strip().endswith("zenzx-headless"))
        # $ZENZX names it outright
        mine = os.path.join(self.tmp.name, "mine", "zz")
        self.stub(mine, fake)
        self.source({"ZENZX_EXAMPLES_YES": "1", "ZENZX": mine})
        self.assertEqual(fc.slurp(record).strip(), mine)

    def test_zenzx_is_told_what_was_asked(self):
        record = os.path.join(self.tmp.name, "args")
        self.stub(os.path.join(self.pkg, "bin", "zenzx-headless"), '#!/bin/sh\nprintf "%%s\\n" "$@" > %s\nexit 1\n' % record)
        self.source({"ZENZX_EXAMPLES_YES": "1", "ZENZX_PYTHON_URL": "http://x/y.tgz", "ZENZX_PYTHON_SHA256": "ab" * 32})
        self.assertEqual(fc.slurp(record).split("\n")[:-1],
                         ["-fetch", "python", "-fetch-to", os.path.join(self.ex, "python"), "-fetch-yes=true",
                          "-fetch-url=http://x/y.tgz", "-fetch-sha256=" + "ab" * 32])

    # --- getting ---------------------------------------------------------------------------------

    def download_env(self, base, sha, extra=None):
        return dict({"ZENZX_EXAMPLES_YES": "1", "ZENZX_PYTHON_URL": base + "/py.tar.gz", "ZENZX_PYTHON_SHA256": sha}, **(extra or {}))

    def test_the_download_is_checked_unpacked_and_used(self):
        self.install_zenzx()
        tgz, sha = self.tarball()
        with fc.Server({"/py.tar.gz": tgz}) as base:
            r = self.source(self.download_env(base, sha))
        private = os.path.join(self.ex, "python", "bin", "python3")
        self.assertIn("PY=" + private, r.stdout, r.stdout + r.stderr)
        self.assertIn("Python is ready.", r.stdout)
        self.assertEqual(self.leftovers(), [])

    def test_a_download_that_does_not_match_is_thrown_away(self):
        self.install_zenzx()
        tgz, _ = self.tarball()
        with fc.Server({"/py.tar.gz": tgz}) as base:
            r = self.source(self.download_env(base, "0" * 64))
        self.assertIn("PY=\n", r.stdout)
        self.assertIn("not the expected file", r.stderr)
        self.assertIn("https://www.python.org/downloads/", r.stdout)
        self.assertFalse(os.path.exists(os.path.join(self.ex, "python")))
        self.assertEqual(self.leftovers(), [])

    def test_an_archive_without_a_python_in_it_is_not_kept(self):
        self.install_zenzx()
        tgz, sha = self.tarball(with_python=False)
        with fc.Server({"/py.tar.gz": tgz}) as base:
            r = self.source(self.download_env(base, sha))
        self.assertIn("PY=\n", r.stdout)
        self.assertFalse(os.path.exists(os.path.join(self.ex, "python")))
        self.assertEqual(self.leftovers(), [])

    def test_a_download_that_is_not_there_says_what_the_server_answered(self):
        self.install_zenzx()
        with fc.Server({}) as base:
            r = self.source(self.download_env(base, "0" * 64))
        self.assertIn("PY=\n", r.stdout)
        self.assertIn("404", r.stderr)
        self.assertEqual(self.leftovers(), [])

    # --- asking, with a terminal -------------------------------------------------------------------

    def ask(self, answer, extra_env):
        """Run with a pseudo-terminal as stdin and type `answer`; the output."""
        master, slave = pty.openpty()
        try:
            env = dict(self.env, **extra_env)
            script = 'examples_dir=%s; . "$examples_dir/tools/findpy.sh"; echo "PY=$PY"' % self.ex
            proc = subprocess.Popen(["sh", "-c", script], env=env, stdin=slave, stdout=subprocess.PIPE,
                                    stderr=subprocess.STDOUT, text=True)
            os.write(master, answer.encode())
            out, _ = proc.communicate(timeout=120)
        finally:
            os.close(master)
            os.close(slave)
        return out

    def test_zenzx_asks_how_and_the_answer_decides(self):
        self.install_zenzx()
        tgz, sha = self.tarball()
        with fc.Server({"/py.tar.gz": tgz}) as base:
            env = {"ZENZX_PYTHON_URL": base + "/py.tar.gz", "ZENZX_PYTHON_SHA256": sha}
            out = self.ask("q\n", env)
            self.assertIn("How would you like to get it?", out)
            self.assertIn("1. A private copy of Python", out)
            self.assertIn("Install it yourself, from python.org", out)
            self.assertIn("PY=\n", out)
            self.assertFalse(os.path.exists(os.path.join(self.ex, "python")))
            out = self.ask("\n", env)
            self.assertIn("PY=" + os.path.join(self.ex, "python", "bin", "python3"), out, out)

    def test_homebrew_is_offered_where_there_is_brew(self):
        self.install_zenzx()
        self.stub(os.path.join(self.bin, "brew"), "#!/bin/sh\nexit 0\n")
        out = self.ask("q\n", {})
        self.assertIn("Install it with Homebrew (brew install python)", out)


if __name__ == "__main__":
    unittest.main()
