#!/usr/bin/env python3
"""check_binaries.py -- structural checks on the programs a release builds.

    python3 tools/check_binaries.py dist/                  # every binary under dist/
    python3 tools/check_binaries.py zenzx-windows-arm64.exe
    python3 tools/check_binaries.py --list                 # the checks, and their names

It reads ELF (Linux, FreeBSD), Mach-O (macOS) and PE (Windows) files with LIEF
(`pip install lief`), and for each says whether it is the program the file name says it
is: the right format and architecture, the right Windows subsystem, only the libraries
and imports it is meant to need, a macOS minimum version that has not crept up, a code
signature where Apple Silicon wants one, and no build machine's paths inside it.

It looks at structure. It does not run anything, so it cannot say that a program starts
on the system it is built for; that is for a runner of that system (the release
workflow's `smoke` job, or a person with the machine).

What each binary is meant to be comes from its path: a name that has linux, darwin,
windows or freebsd and amd64 or arm64 in it (zenzx-windows-arm64.exe, or
zenzx-darwin-amd64/zenzx), and "headless" in the file name for the program without a
window. A name without them is read from the file itself, and the checks that need
the name are then not made. --os, --arch and --kind say it outright.

Every expectation can be overridden (see --help), a check can be skipped by name
(--skip), and --warn-only reports failures and still exits 0.

Exit status: 0 every check passed (or --warn-only), 1 a check failed, 2 the tool could
not run (no LIEF, a path that is not there, a bad option).
"""

import argparse
import os
import re
import sys

try:
    import lief
except ImportError:  # reported in main(), so that --list and --help still work
    lief = None

# The checks, in the order they run, with what each says.
CHECKS = {
    "format": "the file is the executable format its system uses (ELF, Mach-O or PE)",
    "arch": "the processor is the one the name says",
    "type": "it is a program, not a library or an object file",
    "os": "the operating system marked in the file is the one the name says",
    "subsystem": "Windows: the GUI subsystem for the program with a window, the console one for headless",
    "imports": "Windows: it imports only the DLLs it is allowed to (kernel32.dll)",
    "libs": "Linux and macOS: it needs only the libraries it is allowed to; headless needs none",
    "minos": "macOS: the minimum system version has not risen above the limit",
    "signature": "macOS on Apple Silicon: a code signature is present, as the system requires",
    "glibc": "Linux: the newest glibc symbol version it needs is not above the limit",
    "strings": "none of the forbidden strings (a build machine's paths) is inside it",
}

# What the Linux GUI program is allowed to link against, as it is built with cgo for ALSA and
# X11 (raylib's GL is loaded at run time, not linked). The glibc parts are always allowed,
# and that includes the dynamic loader, which the linker lists as a needed library on some
# processors (ld-linux-aarch64.so.1 on arm64) and not on others.
LINUX_GUI_LIBS = {
    "libc.so.6", "libm.so.6", "libdl.so.2", "libpthread.so.0", "librt.so.1",
    "libasound.so.2", "libX11.so.6",
    "ld-linux-aarch64.so.1", "ld-linux-x86-64.so.2",
}
# Where a Mach-O program may find its libraries: the system's own.
MACOS_LIB_PREFIXES = ("/usr/lib/", "/System/Library/")
# The strings a program must not carry: the path of the machine it was built on, which
# `go build -trimpath` takes out.
DEFAULT_FORBIDDEN = ["/home/", "/root/", "/Users/", "C:\\Users\\"]
# Paths that contain one of those and are not the build machine's: Homebrew's fixed prefix on
# Linux, which a program that looks for programs there (pkg/fetch) has to name.
DEFAULT_ALLOWED = ["/home/linuxbrew/"]

NAME_RE = re.compile(r"(linux|darwin|windows|freebsd)[-_](amd64|arm64)")
OS_NAMES = ("linux", "darwin", "windows", "freebsd")
ARCH_NAMES = ("amd64", "arm64")

# How LIEF names each processor, for each of the architectures a name can say.
ARCH_OF = {
    "ELF": {"X86_64": "amd64", "AARCH64": "arm64"},
    "MACHO": {"X86_64": "amd64", "ARM64": "arm64"},
    "PE": {"AMD64": "amd64", "ARM64": "arm64"},
}
FORMAT_OF_OS = {"linux": "ELF", "freebsd": "ELF", "darwin": "MACHO", "windows": "PE"}
FORMAT_WORD = {"ELF": "ELF", "MACHO": "Mach-O", "PE": "PE"}
OS_OF_FORMAT = {"MACHO": "darwin", "PE": "windows"}  # ELF is linux or freebsd: see detect()

MAGICS = (b"\x7fELF", b"MZ", b"\xfe\xed\xfa\xce", b"\xfe\xed\xfa\xcf", b"\xce\xfa\xed\xfe",
          b"\xcf\xfa\xed\xfe", b"\xca\xfe\xba\xbe")


class Result:
    """One line of the report: a check, whether it passed, and what it found."""

    def __init__(self, path, check, ok, detail):
        self.path, self.check, self.ok, self.detail = path, check, ok, detail


def version_tuple(text):
    """'12.0' or 'GLIBC_2.38' as (12, 0) or (2, 38); None when it is not a version."""
    nums = re.findall(r"\d+", text)
    return tuple(int(n) for n in nums) if nums else None


def version_text(t):
    return ".".join(str(n) for n in t)


def looks_binary(path):
    try:
        with open(path, "rb") as f:
            head = f.read(4)
    except OSError:
        return False
    return any(head.startswith(m) for m in MAGICS)


def collect(paths):
    """The files to check: a file as given, a directory as every program inside it."""
    files = []
    for p in paths:
        if os.path.isdir(p):
            for root, _dirs, names in os.walk(p):
                for n in sorted(names):
                    full = os.path.join(root, n)
                    if os.path.isfile(full) and looks_binary(full):
                        files.append(full)
        elif os.path.isfile(p):
            files.append(p)
        else:
            raise FileNotFoundError(p)
    return sorted(files)


def expectations(path, opts):
    """What the path (and the options) say the file should be: (os, arch, kind), any may be None."""
    norm = path.replace("\\", "/")
    m = list(NAME_RE.finditer(norm.lower()))
    name_os = name_arch = None
    if m:
        name_os, name_arch = m[-1].group(1), m[-1].group(2)
    kind = "headless" if "headless" in os.path.basename(norm).lower() else "gui"
    return (opts.os or name_os, opts.arch or name_arch, opts.kind or kind)


def parse(path):
    """The file read by LIEF: (family, binary), family being ELF, MACHO or PE, or (None, why)."""
    try:
        if lief.is_elf(path):
            return "ELF", lief.ELF.parse(path)
        if lief.is_pe(path):
            return "PE", lief.PE.parse(path)
        if lief.is_macho(path):
            fat = lief.MachO.parse(path)
            if fat is None or len(fat) == 0:
                return None, "LIEF found no Mach-O slice in it"
            if len(fat) > 1:
                return None, "a universal Mach-O with %d slices; release builds are one slice each" % len(fat)
            return "MACHO", fat.at(0)
    except Exception as e:  # LIEF raises on files it cannot read
        return None, "LIEF could not read it: %s" % e
    return None, "not an ELF, Mach-O or PE file"


def check_file(path, opts, skip):
    """Run the checks on one file: a list of Results."""
    out = []

    def add(check, ok, detail):
        if check not in skip:
            out.append(Result(path, check, ok, detail))

    want_os, want_arch, kind = expectations(path, opts)
    family, bin_ = parse(path)
    if family is None:
        add("format", False, bin_)
        return out

    # The system the file is for, as the file itself says.
    got_os = OS_OF_FORMAT.get(family)
    if family == "ELF":
        abi = bin_.header.identity_os_abi.name
        got_os = "freebsd" if abi == "FREEBSD" else "linux"

    if want_os:
        want_family = FORMAT_OF_OS[want_os]
        add("format", family == want_family,
            FORMAT_WORD[family] if family == want_family
            else "%s, but a %s program is %s" % (FORMAT_WORD[family], want_os, FORMAT_WORD[want_family]))
    else:
        add("format", True, FORMAT_WORD[family])

    # Processor.
    if family == "ELF":
        proc = bin_.header.machine_type.name
    elif family == "MACHO":
        proc = bin_.header.cpu_type.name
    else:
        proc = bin_.header.machine.name
    got_arch = ARCH_OF[family].get(proc)
    if got_arch is None:
        add("arch", False, "processor %s is not amd64 or arm64" % proc)
    elif want_arch:
        add("arch", got_arch == want_arch, "%s" % got_arch if got_arch == want_arch
            else "%s, but the name says %s" % (got_arch, want_arch))
    else:
        add("arch", True, got_arch)

    # Program, not library.
    if family == "ELF":
        ftype = bin_.header.file_type.name
        add("type", ftype in ("EXEC", "DYN"), ftype.lower())
    elif family == "MACHO":
        ftype = bin_.header.file_type.name
        add("type", ftype == "EXECUTE", ftype.lower())
    else:
        is_dll = "DLL" in [c.name for c in bin_.header.characteristics_list]
        add("type", not is_dll, "a library (DLL)" if is_dll else "a program")

    # Operating system.
    if want_os:
        add("os", got_os == want_os, got_os if got_os == want_os
            else "%s, but the name says %s" % (got_os, want_os))
    if family == "MACHO":
        plat = bin_.build_version.platform.name if bin_.has_build_version else "none"
        add("os", plat == "MACOS", "platform %s" % plat.lower())

    if family == "PE":
        sub = bin_.optional_header.subsystem.name
        mode = opts.subsystem or ("gui" if kind == "gui" else "console")
        if mode == "any":
            add("subsystem", True, "%s (not checked)" % sub.lower())
        else:
            wanted = {"gui": "WINDOWS_GUI", "console": "WINDOWS_CUI"}[mode]
            add("subsystem", sub == wanted, sub.lower() if sub == wanted
                else "%s, but %s is wanted (%s program)" % (sub.lower(), wanted.lower(), mode))
        allowed = {d.lower() for d in (opts.allow_import if opts.allow_import else ["kernel32.dll"])}
        dlls = sorted({i.name.lower() for i in bin_.imports})
        extra = [d for d in dlls if d not in allowed]
        add("imports", not extra, ("imports " + ", ".join(dlls)) if not extra
            else "imports %s, which %s not allowed" % (", ".join(extra), "is" if len(extra) == 1 else "are"))

    if family == "MACHO":
        libs = [lib.name for lib in bin_.libraries]
        bad = [n for n in libs if not n.startswith(tuple(opts.allow_lib or MACOS_LIB_PREFIXES))]
        add("libs", not bad, "%d system libraries" % len(libs) if not bad
            else "outside the system: " + ", ".join(bad))
        limit = version_tuple(opts.max_min_macos)
        if not bin_.has_build_version:
            add("minos", False, "no build version in the file")
        else:
            minos = tuple(bin_.build_version.minos)
            while len(minos) > 2 and minos[-1] == 0:
                minos = minos[:-1]
            add("minos", minos <= limit, "macOS %s or later" % version_text(minos) if minos <= limit
                else "macOS %s or later, above the limit %s" % (version_text(minos), version_text(limit)))
        if got_arch == "arm64":
            add("signature", bin_.has_code_signature,
                "present" if bin_.has_code_signature else "absent; Apple Silicon will not run it")

    if family == "ELF":
        needed = [str(n) for n in bin_.libraries]
        if kind == "headless":
            allowed_libs = set(opts.allow_lib or [])
        else:
            allowed_libs = set(opts.allow_lib) if opts.allow_lib else LINUX_GUI_LIBS
        bad = [n for n in needed if n not in allowed_libs]
        if got_os == "freebsd" and not needed:
            bad = []
        add("libs", not bad, ("needs " + ", ".join(needed)) if needed and not bad
            else "needs no libraries" if not needed and not bad
            else "not allowed: " + ", ".join(bad))
        newest = None
        for req in bin_.symbols_version_requirement:
            for aux in req.get_auxiliary_symbols():
                if aux.name.startswith("GLIBC_"):
                    v = version_tuple(aux.name)
                    if v and (newest is None or v > newest):
                        newest = v
        if opts.max_glibc:
            limit = version_tuple(opts.max_glibc)
            if newest is None:
                add("glibc", True, "needs no glibc")
            else:
                add("glibc", newest <= limit, "needs glibc %s" % version_text(newest) if newest <= limit
                    else "needs glibc %s, above the limit %s" % (version_text(newest), version_text(limit)))
        else:
            add("glibc", True, ("needs glibc %s (no limit set: --max-glibc)" % version_text(newest))
                if newest else "needs no glibc")

    # Strings.
    forbidden = list(DEFAULT_FORBIDDEN) if not opts.no_default_forbid else []
    forbidden += opts.forbid or []
    try:
        with open(path, "rb") as f:
            data = f.read()
    except OSError as e:
        add("strings", False, "cannot read it: %s" % e)
        return out
    allowed = ([] if opts.no_default_forbid else list(DEFAULT_ALLOWED)) + (opts.allow or [])
    found = []
    for s in forbidden:
        needle = s.encode("utf-8")
        at = data.find(needle)
        while at >= 0 and excused(data, at, needle, allowed):
            at = data.find(needle, at + 1)
        if at >= 0:
            around = data[max(0, at - 8):at + len(s) + 40]
            snippet = "".join(chr(c) if 32 <= c < 127 else "." for c in around)
            found.append("%r at byte %d (%s)" % (s, at, snippet))
    add("strings", not found, "none of %d" % len(forbidden) if not found else "; ".join(found))
    return out


def excused(data, at, needle, allowed):
    """Whether the forbidden string at data[at:] is part of one of the allowed strings that contain it."""
    for a in allowed:
        raw = a.encode("utf-8")
        i = raw.find(needle)
        while i >= 0:
            if at >= i and data[at - i:at - i + len(raw)] == raw:
                return True
            i = raw.find(needle, i + 1)
    return False


def colour(text, code, on):
    return "\033[%sm%s\033[0m" % (code, text) if on else text


def report(results, files, opts, tty):
    """Print the results, a block a file; returns the number of failures."""
    fails = 0
    by_file = {}
    for r in results:
        by_file.setdefault(r.path, []).append(r)
    for f in files:
        rs = by_file.get(f, [])
        bad = [r for r in rs if not r.ok]
        if opts.quiet and not bad:
            continue
        print(f)
        for r in rs:
            if r.ok and opts.quiet:
                continue
            mark = "ok  " if r.ok else colour("FAIL", "31", tty)
            print("  %s  %-10s %s" % (mark, r.check, r.detail))
        fails += len(bad)
    return fails


def build_parser():
    p = argparse.ArgumentParser(
        prog="check_binaries.py", description="Structural checks on the ELF, Mach-O and PE programs a release builds.",
        epilog="Run with --list for the checks and what each says.")
    p.add_argument("paths", nargs="*", help="programs, or directories to search for them")
    p.add_argument("--list", action="store_true", help="list the checks and exit")
    p.add_argument("--skip", action="append", default=[], metavar="CHECK",
                   help="do not make this check (repeatable): " + ", ".join(CHECKS))
    p.add_argument("--warn-only", action="store_true", help="report failures, but exit 0")
    p.add_argument("-q", "--quiet", action="store_true", help="print only the files with a failure, and only the failures")
    g = p.add_argument_group("what the programs are meant to be (read from the path unless given)")
    g.add_argument("--os", choices=OS_NAMES, help="the system every program is for")
    g.add_argument("--arch", choices=ARCH_NAMES, help="the processor every program is for")
    g.add_argument("--kind", choices=("gui", "headless"), help="the program with a window, or without (default: headless when the file name says so)")
    g = p.add_argument_group("limits")
    g.add_argument("--subsystem", choices=("gui", "console", "any"), help="Windows subsystem (default: gui, or console for headless)")
    g.add_argument("--allow-import", action="append", metavar="DLL", help="Windows: a DLL it may import, instead of the default kernel32.dll (repeatable)")
    g.add_argument("--allow-lib", action="append", metavar="LIB", help="a library it may need, instead of the defaults (a name for Linux, a path prefix for macOS; repeatable)")
    g.add_argument("--max-min-macos", default="12.0", metavar="VER", help="macOS: the highest minimum system version allowed (default 12.0)")
    g.add_argument("--max-glibc", metavar="VER", help="Linux: the highest glibc version it may need, such as 2.35 (default: reported, not limited)")
    g.add_argument("--forbid", action="append", metavar="STRING", help="a string it must not contain, besides the defaults " + " ".join(DEFAULT_FORBIDDEN) + " (repeatable)")
    g.add_argument("--no-default-forbid", action="store_true", help="forbid only the --forbid strings (and allow nothing the defaults allow)")
    g.add_argument("--allow", action="append", metavar="STRING", help="a string that holds a forbidden one and is let through, besides the defaults " + " ".join(DEFAULT_ALLOWED) + " (repeatable)")
    return p


def main(argv=None):
    p = build_parser()
    opts = p.parse_args(argv)
    if opts.list:
        for name, what in CHECKS.items():
            print("%-10s %s" % (name, what))
        return 0
    unknown = [s for s in opts.skip if s not in CHECKS]
    if unknown:
        print("check_binaries: no such check: %s (see --list)" % ", ".join(unknown), file=sys.stderr)
        return 2
    if not opts.paths:
        p.print_usage(sys.stderr)
        print("check_binaries: give a program, or a directory of them", file=sys.stderr)
        return 2
    if lief is None:
        print("check_binaries: needs LIEF: pip install lief (on a Mac, pip3 install lief)", file=sys.stderr)
        return 2
    if hasattr(lief, "logging"):
        lief.logging.disable()
    try:
        files = collect(opts.paths)
    except FileNotFoundError as e:
        print("check_binaries: %s is not there" % e, file=sys.stderr)
        return 2
    if not files:
        print("check_binaries: no programs found in %s" % " ".join(opts.paths), file=sys.stderr)
        return 2

    skip = set(opts.skip)
    results = []
    for f in files:
        results += check_file(f, opts, skip)
    tty = sys.stdout.isatty() and "NO_COLOR" not in os.environ
    fails = report(results, files, opts, tty)
    total = len(results)
    count = colour(str(fails), "31", tty) if fails else str(fails)
    print("%d programs, %d checks, %s failed%s" % (len(files), total, count,
          " (--warn-only: exit status 0)" if fails and opts.warn_only else ""))
    if fails and not opts.warn_only:
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
