#!/usr/bin/env python3
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0
"""The repository's shell scripts are POSIX sh: they say #!/bin/sh, they parse with a strict sh
(dash, where there is one), they use nothing that only bash or zsh has, and a script is executable
exactly when it is a program (it starts with #!). The sourced findpy.sh has no #! and is not.

    python3 scripts/test_shell_scripts.py
"""
import os
import re
import shutil
import stat
import subprocess
import unittest

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SKIP_DIRS = {".git", "dist", "retired", "python", "bin", "node_modules"}

# What POSIX sh does not have (a line that is a comment is not checked).
BASHISMS = [
    (r"\[\[", "[[ ]]"),
    (r"^\s*function\s", "the function keyword"),
    (r"(^|[;&|]\s*)source\s", "source (use .)"),
    (r"<<<", "here-string"),
    (r"\w=\(", "array"),
    (r"\$\{[A-Za-z_0-9]+\[", "array element"),
    (r"\$\{[A-Za-z_0-9]+(//|,,|\^\^|:[0-9]+:[0-9]*|/)", "bash parameter expansion"),
    (r"\$\{![A-Za-z_]", "indirect expansion"),
    (r"\becho\s+-[en]+\b", "echo -e / -n (use printf)"),
    (r"\$''", "$'...' quoting"),
    (r"&>", "&>"),
    (r"(^|[;&|]\s*)let\s", "let"),
    (r"(^|[;&|]\s*)(pushd|popd|shopt|mapfile|readarray|declare|typeset|local)\s", "a bash builtin"),
    (r"\bread\s+-[pn]", "read -p / -n"),
    (r"\$RANDOM|\$BASH|\$EPOCHSECONDS|\$\{?BASH_SOURCE", "a bash variable"),
    (r"\[\s[^\]]*\s==\s", "== inside [ ]"),
    (r"set\s+-o\s+pipefail", "set -o pipefail"),
    (r"<\(|>\(", "process substitution"),
    (r"\{[a-z0-9]+\.\.[a-z0-9]+\}", "brace range"),
    (r"(^|\s)\{[A-Za-z0-9._-]+,[A-Za-z0-9._,-]+\}", "brace expansion"),
    (r"\$\(<", "$(<file)"),
]


def scripts():
    out = []
    for dp, dn, fn in os.walk(REPO):
        dn[:] = sorted(d for d in dn if d not in SKIP_DIRS)
        for f in sorted(fn):
            if f.endswith((".sh", ".command")):
                out.append(os.path.join(dp, f))
    return out


class ShellScriptTests(unittest.TestCase):
    def test_there_are_scripts_to_check(self):
        self.assertGreater(len(scripts()), 10)

    def test_a_program_says_bin_sh_and_is_executable_and_nothing_else_is(self):
        for p in scripts():
            rel = os.path.relpath(p, REPO)
            with open(p, "rb") as f:
                first = f.readline().rstrip(b"\n")
            executable = bool(os.stat(p).st_mode & stat.S_IXUSR)
            if first.startswith(b"#!"):
                self.assertEqual(first, b"#!/bin/sh", rel + ": the shebang is not /bin/sh")
                self.assertTrue(executable, rel + " starts with #! and is not executable")
            else:
                self.assertFalse(executable, rel + " has no #! and is executable")

    def test_nothing_only_bash_or_zsh_has(self):
        for p in scripts():
            rel = os.path.relpath(p, REPO)
            with open(p, encoding="utf-8") as f:
                for n, line in enumerate(f, 1):
                    if line.lstrip().startswith("#"):
                        continue
                    # What is inside '...' is not code. What is inside "..." is expanded, so the
                    # expansions are looked for there; the rest is looked for with "..." taken out too.
                    no_single = re.sub(r"'[^']*'", "''", line)
                    no_quotes = re.sub(r'"[^"]*"', '""', no_single)
                    for pat, what in BASHISMS:
                        text = no_single if pat.startswith((r"\$''", r"\$\{", r"\$\(<")) else no_quotes
                        if re.search(pat, text):
                            self.fail("%s:%d uses %s: %s" % (rel, n, what, line.strip()))

    def test_every_script_parses_with_a_strict_sh(self):
        shell = shutil.which("dash") or shutil.which("sh")
        for p in scripts():
            r = subprocess.run([shell, "-n", p], capture_output=True, text=True)
            self.assertEqual(r.returncode, 0, "%s: %s" % (os.path.relpath(p, REPO), r.stderr))

    def test_no_caller_forces_bash_on_a_script(self):
        for rel in ("Makefile", "run_next_divmmc.sh"):
            with open(os.path.join(REPO, rel), encoding="utf-8") as f:
                for n, line in enumerate(f, 1):
                    self.assertNotRegex(line, r"(^|\s)bash\s+\S*\.sh", "%s:%d runs a script with bash" % (rel, n))


if __name__ == "__main__":
    unittest.main()
