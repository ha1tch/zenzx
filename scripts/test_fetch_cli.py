#!/usr/bin/env python3
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0
"""Tests of `zenzx -fetch`: the downloader that the examples' launchers use to get Python.

It runs the real headless program, against a web server of its own on this computer, so that
nothing leaves it. The program is built with `go build` (skipped when there is no Go), or is
the one named by ZENZX_HEADLESS_BIN.

    python3 scripts/test_fetch_cli.py
"""
import atexit
import hashlib
import http.server
import io
import os
import shutil
import stat
import subprocess
import sys
import tarfile
import tempfile
import threading
import unittest
import zipfile

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(HERE)
POSIX = os.name == "posix"

_binary = None


def program():
    """The path of the headless program, built once."""
    global _binary
    if _binary:
        return _binary
    given = os.environ.get("ZENZX_HEADLESS_BIN")
    if given:
        _binary = given
        return _binary
    go = shutil.which("go")
    if not go:
        raise unittest.SkipTest("no Go to build the program with (or set ZENZX_HEADLESS_BIN)")
    built = tempfile.mkdtemp(prefix="zenzx-fetch-test-")
    atexit.register(shutil.rmtree, built, True)
    out = os.path.join(built, "zenzx-headless" + (".exe" if os.name == "nt" else ""))
    env = dict(os.environ, CGO_ENABLED="0")
    r = subprocess.run([go, "build", "-tags", "headless", "-o", out, "./cmd/zenzx-headless"], cwd=ROOT, env=env,
                       capture_output=True, text=True)
    if r.returncode != 0:
        raise RuntimeError("go build failed:\n" + r.stderr)
    _binary = out
    return _binary


def make_python_tar():
    """A .tar.gz that looks like python-build-standalone's: a folder 'python' with bin/python3 a link to python3.12."""
    buf = io.BytesIO()
    with tarfile.open(fileobj=buf, mode="w:gz") as tf:
        def add(name, data=b"", mode=0o644, typ=tarfile.REGTYPE, link=""):
            ti = tarfile.TarInfo(name)
            ti.mode, ti.type, ti.linkname = mode, typ, link
            if typ == tarfile.REGTYPE:
                ti.size = len(data)
            tf.addfile(ti, io.BytesIO(data) if typ == tarfile.REGTYPE else None)
        add("python", typ=tarfile.DIRTYPE, mode=0o755)
        add("python/bin", typ=tarfile.DIRTYPE, mode=0o755)
        add("python/bin/python3.12", b"#!/bin/sh\nexit 0\n", 0o755)
        add("python/bin/python3", typ=tarfile.SYMTYPE, link="python3.12")
    return buf.getvalue()


def make_zip():
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w") as z:
        z.writestr("top/a.txt", "alpha")
        z.writestr("top/sub/b.txt", "beta")
    return buf.getvalue()


class Server:
    """Serves a few files from memory on a port of this computer."""

    def __init__(self, files):
        files_ = files

        class H(http.server.BaseHTTPRequestHandler):
            def do_GET(self):
                body = files_.get(self.path)
                if body is None:
                    self.send_error(404)
                    return
                self.send_response(200)
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)

            def log_message(self, *a):
                pass

        self.httpd = http.server.ThreadingHTTPServer(("127.0.0.1", 0), H)
        self.thread = threading.Thread(target=self.httpd.serve_forever, daemon=True)

    def __enter__(self):
        self.thread.start()
        return "http://127.0.0.1:%d" % self.httpd.server_address[1]

    def __exit__(self, *a):
        self.httpd.shutdown()
        self.httpd.server_close()


def slurp(path, mode="r"):
    with open(path, mode) as f:
        return f.read()


def sha(b):
    return hashlib.sha256(b).hexdigest()


def run(args, env=None, stdin=subprocess.DEVNULL, timeout=120):
    e = dict(os.environ)
    e.pop("HTTPS_PROXY", None)
    e.pop("https_proxy", None)
    e["NO_PROXY"] = "127.0.0.1"
    e["no_proxy"] = "127.0.0.1"
    e.update(env or {})
    return subprocess.run([program()] + args, capture_output=True, text=True, env=e, stdin=stdin, timeout=timeout)


@unittest.skipUnless(POSIX, "Python is fetched into a bin/ folder with a link: the layout of macOS and Linux")
class FetchPython(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tar = make_python_tar()
        program()  # build once, up front

    def setUp(self):
        self.tmp = tempfile.mkdtemp(prefix="zenzx-fetch-")
        self.addCleanup(shutil.rmtree, self.tmp, True)
        self.dest = os.path.join(self.tmp, "python")

    def fetch_args(self, base, extra=()):
        return ["-fetch", "python", "-fetch-to", self.dest, "-fetch-url", base + "/py.tar.gz",
                "-fetch-sha256", sha(self.tar)] + list(extra)

    def test_list_names_python(self):
        r = run(["-fetch", "list"])
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertIn("python", r.stdout)

    def test_private_copy_is_downloaded_checked_unpacked_and_left_alone_next_time(self):
        with Server({"/py.tar.gz": self.tar}) as base:
            r = run(self.fetch_args(base, ["-fetch-how", "private"]))
            self.assertEqual(r.returncode, 0, r.stderr)
            py = os.path.join(self.dest, "bin", "python3")
            self.assertIn("Ready: " + py, r.stdout)
            self.assertTrue(os.access(py, os.X_OK), "the program is not executable")
            self.assertEqual(subprocess.run([py], capture_output=True).returncode, 0, "the unpacked program does not run")
            self.assertEqual(os.listdir(self.tmp), ["python"], "something was left beside the folder")
            again = run(self.fetch_args(base, ["-fetch-how", "private"]))
            self.assertEqual(again.returncode, 0)
            self.assertIn("Already there", again.stdout)

    def test_wrong_checksum_is_refused_and_leaves_nothing(self):
        with Server({"/py.tar.gz": self.tar}) as base:
            args = self.fetch_args(base, ["-fetch-how", "private"])
            args[args.index("-fetch-sha256") + 1] = sha(b"something else")
            r = run(args)
            self.assertEqual(r.returncode, 1)
            self.assertIn("not the expected file", r.stderr)
            self.assertEqual(os.listdir(self.tmp), [], "a refused download left files behind")

    def test_a_missing_file_says_what_the_server_answered(self):
        with Server({}) as base:
            r = run(self.fetch_args(base, ["-fetch-how", "private"]))
            self.assertEqual(r.returncode, 1)
            self.assertIn("404", r.stderr)

    def test_without_a_terminal_and_without_a_way_it_says_what_to_give(self):
        r = run(["-fetch", "python", "-fetch-to", self.dest])
        self.assertEqual(r.returncode, 2)
        self.assertIn("-fetch-how", r.stderr)
        self.assertIn("-fetch-yes", r.stderr)
        self.assertFalse(os.path.exists(self.dest))

    def test_an_unknown_way_or_name_is_a_usage_error_that_lists_the_choices(self):
        r = run(["-fetch", "python", "-fetch-to", self.dest, "-fetch-how", "carrier-pigeon"])
        self.assertEqual(r.returncode, 2)
        self.assertIn("private", r.stderr)
        r = run(["-fetch", "perl"])
        self.assertEqual(r.returncode, 2)
        self.assertIn("python", r.stderr)

    def test_yes_takes_the_first_way(self):
        with Server({"/py.tar.gz": self.tar}) as base:
            r = run(self.fetch_args(base, ["-fetch-yes"]))
            self.assertEqual(r.returncode, 0, r.stderr)
            self.assertTrue(os.path.exists(os.path.join(self.dest, "bin", "python3")))

    def test_manual_way_makes_nothing_and_says_where_to_go(self):
        r = run(["-fetch", "python", "-fetch-to", self.dest, "-fetch-how", "manual"])
        self.assertEqual(r.returncode, 1)
        self.assertIn("python.org/downloads", r.stdout)
        self.assertFalse(os.path.exists(self.dest))

    def fake_tool(self, name, body):
        d = os.path.join(self.tmp, "fakebin")
        os.makedirs(d, exist_ok=True)
        p = os.path.join(d, name)
        with open(p, "w") as f:
            f.write("#!/bin/sh\n" + body + "\n")
        os.chmod(p, 0o755)
        return d

    def test_homebrew_is_offered_when_it_is_there_and_runs_brew_install(self):
        prefix = os.path.join(self.tmp, "brewprefix")
        os.makedirs(os.path.join(prefix, "bin"))
        with open(os.path.join(prefix, "bin", "python3"), "w") as f:
            f.write("#!/bin/sh\n")
        os.chmod(os.path.join(prefix, "bin", "python3"), 0o755)
        log = os.path.join(self.tmp, "brew.log")
        bindir = self.fake_tool("brew", 'echo "$@" >> "%s"; [ "$1" = --prefix ] && echo "%s"; exit 0' % (log, prefix))
        env = {"PATH": bindir + os.pathsep + os.environ["PATH"]}
        r = run(["-fetch", "python", "-fetch-to", self.dest, "-fetch-how", "brew"], env=env)
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertIn("Ready: " + os.path.join(prefix, "bin", "python3"), r.stdout)
        self.assertIn("install python", slurp(log))
        # and "-fetch-how brew" is not a way where there is no brew
        empty = os.path.join(self.tmp, "empty")
        os.makedirs(empty)
        if not any(os.path.exists(os.path.join(d, "brew")) for d in ("/opt/homebrew/bin", "/usr/local/bin", "/home/linuxbrew/.linuxbrew/bin")):
            r = run(["-fetch", "python", "-fetch-to", self.dest, "-fetch-how", "brew"], env={"PATH": empty})
            self.assertEqual(r.returncode, 2)

    @unittest.skipUnless(sys.platform.startswith("linux"), "the system's package manager is a Linux way")
    def test_the_systems_own_package_is_installed_with_apt(self):
        log = os.path.join(self.tmp, "apt.log")
        self.fake_tool("sudo", 'exec "$@"')  # whoever runs this, there is somebody to ask
        self.fake_tool("python3", "exit 0")
        bindir = self.fake_tool("apt-get", 'echo "apt-get $*" >> "%s"' % log)
        env = {"PATH": bindir + os.pathsep + os.environ["PATH"]}
        r = run(["-fetch", "python", "-fetch-to", self.dest, "-fetch-how", "apt"], env=env)
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertEqual(slurp(log), "apt-get update\napt-get install -y python3\n")
        self.assertIn("Ready: " + os.path.join(bindir, "python3"), r.stdout)
        self.assertFalse(os.path.exists(self.dest))  # it belongs to the whole computer, not to the folder
        # a computer without that manager is not offered it
        empty = os.path.join(self.tmp, "empty-apt")
        os.makedirs(empty)
        if not any(os.path.exists(os.path.join(d, "apt-get")) for d in ("/usr/bin", "/bin", "/usr/local/bin", "/opt/homebrew/bin")):
            r = run(["-fetch", "python", "-fetch-to", self.dest, "-fetch-how", "apt"], env={"PATH": empty})
            self.assertEqual(r.returncode, 2)
            self.assertNotIn("apt", r.stderr.split("there is:")[-1])

    def test_a_separate_environment_is_made_by_uv_in_the_folder(self):
        bindir = self.fake_tool("uv", 'mkdir -p "$4/bin" && : > "$4/bin/python3" && chmod +x "$4/bin/python3"')
        env = {"PATH": bindir + os.pathsep + os.environ["PATH"]}
        r = run(["-fetch", "python", "-fetch-to", self.dest, "-fetch-how", "uv"], env=env)
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertIn("Ready: " + os.path.join(self.dest, "bin", "python3"), r.stdout)

    def test_the_menu_in_a_terminal_lists_what_there_is_and_takes_the_answer(self):
        import pty
        bindir = self.fake_tool("brew", "exit 0")
        env = dict(os.environ, PATH=bindir + os.pathsep + os.environ["PATH"], NO_PROXY="127.0.0.1")
        with Server({"/py.tar.gz": self.tar}) as base:
            for answer, expect_code, expect_dir in (("\r", 0, True), ("q\r", 1, False)):
                shutil.rmtree(self.dest, True)
                master, slave = pty.openpty()
                p = subprocess.Popen([program()] + self.fetch_args(base), stdin=slave, stdout=slave, stderr=slave, env=env, close_fds=True)
                os.close(slave)
                seen = b""
                os.set_blocking(master, True)
                while b"press Return" not in seen and b"Type a number" not in seen:
                    chunk = os.read(master, 4096)
                    if not chunk:
                        break
                    seen += chunk
                os.write(master, answer.encode())
                try:
                    while True:
                        chunk = os.read(master, 4096)
                        if not chunk:
                            break
                        seen += chunk
                except OSError:
                    pass  # the other end closed
                p.wait(timeout=60)
                os.close(master)
                text = seen.decode("utf-8", "replace")
                self.assertEqual(p.returncode, expect_code, text)
                self.assertIn("1. A private copy of Python", text)
                self.assertIn("Install it with Homebrew", text)
                self.assertIn("Install it yourself", text)
                self.assertEqual(os.path.exists(os.path.join(self.dest, "bin", "python3")), expect_dir, text)


class FetchAddress(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp(prefix="zenzx-fetch-")
        self.addCleanup(shutil.rmtree, self.tmp, True)

    def test_a_file_by_its_address(self):
        data = b"hello " * 1000
        with Server({"/dir/hello.txt": data}) as base:
            dest = os.path.join(self.tmp, "out.txt")
            r = run(["-fetch", base + "/dir/hello.txt", "-fetch-to", dest, "-fetch-sha256", sha(data)])
            self.assertEqual(r.returncode, 0, r.stderr)
            self.assertEqual(slurp(dest, "rb"), data)
            bad = run(["-fetch", base + "/dir/hello.txt", "-fetch-to", os.path.join(self.tmp, "bad"), "-fetch-sha256", sha(b"x")])
            self.assertEqual(bad.returncode, 1)
            self.assertFalse(os.path.exists(os.path.join(self.tmp, "bad")))
            self.assertEqual(sorted(os.listdir(self.tmp)), ["out.txt"])

    def test_a_zip_by_its_address_taken_apart_with_a_folder_dropped(self):
        z = make_zip()
        with Server({"/p.zip": z}) as base:
            out = os.path.join(self.tmp, "unpacked")
            r = run(["-fetch", base + "/p.zip", "-fetch-sha256", sha(z), "-fetch-unpack", out, "-fetch-strip", "1"])
            self.assertEqual(r.returncode, 0, r.stderr)
            self.assertEqual(slurp(os.path.join(out, "a.txt")), "alpha")
            self.assertEqual(slurp(os.path.join(out, "sub", "b.txt")), "beta")
            self.assertFalse(os.path.exists(os.path.join(out, "top")))

    def test_something_that_is_not_an_address(self):
        r = run(["-fetch", "https://"])
        self.assertEqual(r.returncode, 2)


if __name__ == "__main__":
    unittest.main(verbosity=2)
