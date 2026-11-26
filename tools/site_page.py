# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0
"""The download page of ZenZX, as one HTML string: made by build_site.py, which has the files.

The page is drawn the way the program is: the Spectrum theme (a bar that fades from grey to
black with the rainbow at its end, white panels, cyan where something is chosen), Spleen, the
bitmap face of the menu bar, the Spectrum of the icon, and the figure that waits in a window
with no machine in it. Where a visitor has to choose, the page chooses first: a script reads
what the browser says about the computer and marks the download that fits it; with no script
the three systems are listed one after another, each with how to tell which of its builds is
the right one. Whatever is not on the release (an installer that failed to build) is not
mentioned at all.

Python's standard library only.
"""
import html
import json

OS_ORDER = ("windows", "darwin", "linux")
OS_TITLE = {"windows": "Windows", "darwin": "macOS", "linux": "Linux"}
OS_ID = {"windows": "windows", "darwin": "macos", "linux": "linux"}

# What a build is for, in the words of a person who does not know what amd64 is.
FOR_PROGRAM = {
    ("windows", "amd64"): "Windows 10 or 11 on an Intel or AMD PC (64-bit), the usual kind",
    ("windows", "arm64"): "Windows 10 or 11 on an ARM PC (Windows on ARM)",
    ("darwin", "arm64"): "A Mac with Apple silicon (M1 or later)",
    ("darwin", "amd64"): "A Mac with an Intel processor",
    ("linux", "amd64"): "Linux on an Intel or AMD PC (64-bit)",
    ("linux", "arm64"): "Linux on ARM, 64-bit (a Raspberry Pi running a 64-bit system, for one)",
}

HOW_TO_TELL = {
    "windows": (
        "<p>Open <em>Settings</em>, then <em>System</em>, then <em>About</em>, and read <em>System type</em>. "
        "<em>64-bit operating system, x64-based processor</em> is the usual kind: take the 64-bit Intel or AMD build. "
        "<em>ARM-based processor</em> is Windows on ARM. ZenZX needs Windows 10 or later, and has no 32-bit build.</p>"
    ),
    "darwin": (
        "<p>Choose the Apple menu, then <em>About This Mac</em>. If it says <em>Chip</em> followed by Apple M1, "
        "M2 or a later one, the Mac has Apple silicon. If it says <em>Processor</em> followed by Intel, it is an Intel Mac.</p>"
    ),
    "linux": (
        "<p>In a terminal, <code>uname -m</code> says <code>x86_64</code> for an Intel or AMD PC, "
        "and <code>aarch64</code> for ARM, 64-bit. There is no 32-bit build.</p>"
    ),
}


def human(n):
    return f"{n / 1048576:.1f} MiB"


def e(s):
    return html.escape(str(s), quote=True)


def release_url(repo, version, name):
    return f"https://github.com/{repo}/releases/download/v{version}/{name}"


class Page:
    """What the release has, ready to be laid out. runners: the programs, as in runners.json;
    rows: the installers and the rest, as find_extras makes them."""

    def __init__(self, version, repo, runners, rows, figure):
        self.version, self.repo = version, repo
        self.runners = runners
        self.rows = rows
        self.figure = figure          # {"frames": [...], "intro": [...], "loop": [...], "rest": n}

    def extra(self, *needles):
        """The row whose file name has every one of needles in it, or None."""
        for r in self.rows:
            if all(n in r["name"] for n in needles):
                return r
        return None

    def program(self, os_, arch):
        for r in self.runners:
            if r["os"] == os_ and r["arch"] == arch:
                return r
        return None

    # --- one download, as a row of a system's panel --------------------------------------------

    def choice(self, os_, arch, kind, title, what, href, name, size):
        label = what if kind == "program" else what
        return (
            f'<li class="choice"><div class="what"><strong>{e(title)}</strong>'
            f'<span class="dim">{e(what)}</span></div>'
            f'<a class="dl" href="{e(href)}" data-os="{e(os_)}" data-arch="{e(arch)}" data-kind="{e(kind)}" '
            f'data-label="{e(title.lower() + ", " + what[0].lower() + what[1:])}" data-size="{human(size)}">'
            f'<span class="fn">{e(name)}</span><small>{human(size)}</small></a></li>'
        )

    def extra_choice(self, row, os_, arch, title, what):
        return self.choice(os_, arch, "installer", title, what,
                           release_url(self.repo, self.version, row["name"]), row["name"], row["size"])

    def program_choice(self, os_, arch):
        r = self.program(os_, arch)
        return self.choice(os_, arch, "program", "The program on its own", FOR_PROGRAM[(os_, arch)],
                           r["file"], r["file"].split("/")[-1], r["size"])

    # --- a system's panel ----------------------------------------------------------------------

    def windows_panel(self):
        msi = {a: self.extra(f"-windows-{a}.msi") for a in ("amd64", "arm64")}
        items = []
        for a in ("amd64", "arm64"):
            if msi[a]:
                items.append(self.extra_choice(msi[a], "windows", a, "Installer", FOR_PROGRAM[("windows", a)]))
        for a in ("amd64", "arm64"):
            items.append(self.program_choice("windows", a))
        if any(msi.values()):
            lead = ("<p>Most people want the <strong>installer</strong>: open it and ZenZX is in the Start menu. "
                    "The program on its own is a single <code>.exe</code> that needs no installing: keep it anywhere and "
                    "double-click it.</p>")
        else:
            lead = ("<p>ZenZX is a single <code>.exe</code> that needs no installing: keep it anywhere and "
                    "double-click it.</p>")
        out = lead + '<ul class="choices">' + "".join(items) + "</ul>"
        if any(msi.values()):
            out += ('<p class="dim">The installers are not signed, so Windows SmartScreen says it protected the PC: '
                    "choose <em>More info</em>, then <em>Run anyway</em>. "
                    "The installer is for one person and needs no administrator; <em>Apps</em> in Windows Settings removes it.</p>")
        w7 = [self.extra("windows7", ".msi"), self.extra("windows7", ".zip")]
        if any(w7):
            w7items = []
            if w7[0]:
                w7items.append(self.extra_choice(w7[0], "windows7", "amd64", "Installer",
                                                 "Windows 7 on a 64-bit PC"))
            if w7[1]:
                w7items.append(self.extra_choice(w7[1], "windows7", "amd64", "A folder to unzip",
                                                 "Windows 7 on a 64-bit PC"))
            out += ('<h3>On Windows 7</h3><p>The builds above need Windows 10 or later. Windows 7 has a build of its own, '
                    "for a 64-bit PC.</p>" '<ul class="choices">' + "".join(w7items) + "</ul>")
        out += '<details><summary>How do I tell which one my PC is?</summary>' + HOW_TO_TELL["windows"] + "</details>"
        return out

    def macos_panel(self):
        dmg = self.extra(".dmg")
        items = []
        if dmg:
            items.append(self.extra_choice(dmg, "darwin", "universal", "Installer (a disk image)",
                                           "Any Mac, Intel or Apple silicon"))
        for a in ("arm64", "amd64"):
            items.append(self.program_choice("darwin", a))
        if dmg:
            lead = ("<p>Most people want the <strong>disk image</strong>, which suits every Mac: open it and drag ZenZX "
                    "onto Applications. The program on its own is one file, made for the processor of one kind of Mac.</p>")
        else:
            lead = "<p>ZenZX is one file, made for the processor of one kind of Mac, which the choices below name.</p>"
        out = lead + '<ul class="choices">' + "".join(items) + "</ul>"
        out += ('<p class="dim">'
                + ("The disk image is not signed, so macOS refuses the first opening: choose <em>Open Anyway</em> in System Settings, "
                   "Privacy &amp; Security. " if dmg else "")
                + "A browser marks a file it downloads, and macOS then asks for permission to open it on its own: "
                  "<code>chmod +x</code> the file, and <code>xattr -d com.apple.quarantine</code> and its name clears the mark.</p>")
        out += '<details><summary>How do I tell which Mac I have?</summary>' + HOW_TO_TELL["darwin"] + "</details>"
        return out

    def linux_panel(self):
        debs = {a: self.extra(f"_{a}.deb") for a in ("amd64", "arm64")}
        items = []
        for a in ("amd64", "arm64"):
            items.append(self.program_choice("linux", a))
        for a in ("amd64", "arm64"):
            if debs[a]:
                items.append(self.extra_choice(debs[a], "linux", a, "Installer for Debian and Ubuntu",
                                               FOR_PROGRAM[("linux", a)]))
        if any(debs.values()):
            lead = ("<p>The <strong>program on its own</strong> is one file that runs on most Linux systems: "
                    "<code>chmod +x</code> it and start it. On Debian, Ubuntu and their relatives the "
                    "<strong>installer</strong> also puts ZenZX in the application menu.</p>")
        else:
            lead = ("<p>ZenZX is one file that runs on most Linux systems: <code>chmod +x</code> it and start it.</p>")
        out = lead + '<ul class="choices">' + "".join(items) + "</ul>"
        if any(debs.values()):
            out += ("<p class=\"dim\">To install: <code>sudo apt install ./</code> and the file's name, for the file that matches "
                    "<code>dpkg --print-architecture</code>.</p>")
        out += '<details><summary>How do I tell which one my computer is?</summary>' + HOW_TO_TELL["linux"] + "</details>"
        return out

    # --- the page -----------------------------------------------------------------------------

    def chooser(self):
        panels = {"windows": self.windows_panel(), "darwin": self.macos_panel(), "linux": self.linux_panel()}
        out = []
        for os_ in OS_ORDER:
            out.append(f'<section class="os" id="os-{OS_ID[os_]}" data-os="{os_}">'
                       f"<h3>{OS_TITLE[os_]}</h3>{panels[os_]}</section>")
        return "\n".join(out)

    def every_file(self):
        """The long list, for a person who wants a file by name or its checksum."""
        def prog_rows():
            body = []
            for r in self.runners:
                body.append(
                    '<tr><td>{os}</td><td>{arch}</td><td class="n"><a href="{file}">{name}</a></td>'
                    '<td class="n">{size}</td><td class="sum"><code>{sha}</code></td></tr>'.format(
                        os=e(r["os"]), arch=e(r["arch"]), file=e(r["file"]),
                        name=e(r["file"].split("/")[-1]), size=human(r["size"]), sha=r["sha256"]))
            return "\n".join(body)

        def table(group):
            body = "\n".join(
                '<tr><td>{system}</td><td>{detail}</td><td class="n"><a href="{url}">{name}</a></td>'
                '<td class="n">{size}</td><td class="sum"><code>{sha}</code></td></tr>'.format(
                    system=e(r["system"]), detail=e(r["detail"]),
                    url=e(release_url(self.repo, self.version, r["name"])),
                    name=e(r["name"]), size=human(r["size"]), sha=r["sha256"])
                for r in self.rows if r["group"] == group)
            return ("<table>\n<tr><th>System</th><th>For</th><th>Download</th><th>Size</th>"
                    '<th class="sum">SHA-256</th></tr>\n' + body + "\n</table>\n")

        names = [r["name"] for r in self.rows]
        out = (
            "<h2>Programs</h2>\n"
            "<p>One file for each system, with the ROMs inside it.</p>\n"
            "<table>\n<tr><th>System</th><th>Arch</th><th>Download</th><th>Size</th>"
            '<th class="sum">SHA-256</th></tr>\n' + prog_rows() + "\n</table>\n"
            "<p>Check a download with <code>sha256sum -c checksums.txt</code> (<code>shasum -a 256 -c</code> on a Mac), "
            'in the folder that has both. <a href="bin/checksums.txt">checksums.txt</a></p>\n'
        )
        if any(r["group"] == "installer" for r in self.rows):
            out += ("\n<h2>Installers</h2>\n"
                    "<p>For a normal install, with a menu entry and a way to remove it. The programs above are the "
                    "same programs, as bare files.</p>\n" + table("installer"))
            notes = []
            if any(n.endswith(".deb") for n in names):
                notes.append("Linux: <code>sudo apt install ./</code> and the file's name, for the file that matches "
                             "<code>dpkg --print-architecture</code>.")
            if any(n.endswith(".msi") and "windows7" not in n for n in names):
                notes.append("Windows: open the <code>.msi</code>. It installs for the person who runs it, needs no administrator "
                             "and puts ZenZX in the Start menu.")
            if any(n.endswith(".dmg") for n in names):
                notes.append("macOS: open the <code>.dmg</code> and drag ZenZX onto Applications.")
            warn = []
            if any(n.endswith(".msi") and "windows7" not in n for n in names):
                warn.append("Windows SmartScreen says it protected the PC: choose <em>More info</em>, then <em>Run anyway</em>.")
            if any(n.endswith(".dmg") for n in names):
                warn.append("macOS refuses the first opening: choose <em>Open Anyway</em> in System Settings, "
                            "Privacy &amp; Security.")
            out += "<ul>" + "".join(f"<li>{n}</li>" for n in notes) + "</ul>\n"
            if warn:
                out += '<p class="dim">The installers are not signed. ' + " ".join(warn) + "</p>\n"
        if any(r["group"] == "windows7" for r in self.rows):
            out += ("\n<h2>Windows 7</h2>\n"
                    "<p>The Windows programs above need Windows 10 or later. For Windows 7 there is a separate 64-bit "
                    "program, built with a patched Go compiler because Go itself has not supported Windows 7 since "
                    "version 1.21. The zip holds the program and the manual.</p>\n" + table("windows7"))
        if any(r["group"] == "documents" for r in self.rows):
            out += ("\n<h2>Manual and examples</h2>\n"
                    "<p>The manual as a PDF, the same text as <code>docs/MANUAL.md</code>. The examples are a "
                    "connected world of nine machines, a chat room of four and three programs for the Spectrum Next. "
                    "The first two need Python 3, and each has a launcher for Windows (<code>.cmd</code>) and one for "
                    "macOS and Linux (<code>.sh</code>) that say so, and offer to download Python (Windows) or ZenZX "
                    "when either is missing. Unzip the examples beside ZenZX, or anywhere, and start one from its "
                    "folder; <code>README.md</code> in the zip says how.</p>\n" + table("documents"))
        return out

    def documents_strip(self):
        """The manual and the examples, as two links near the top: what a new person looks for next."""
        bits = []
        man, ex = self.extra("-manual.pdf"), self.extra("-examples.zip")
        if man:
            bits.append(f'<a href="{e(release_url(self.repo, self.version, man["name"]))}">The manual (PDF, {human(man["size"])})</a>')
        if ex:
            bits.append(f'<a href="{e(release_url(self.repo, self.version, ex["name"]))}">The examples ({human(ex["size"])})</a>')
        return "".join(f"<li>{b}</li>" for b in bits)

    def manual_panel(self):
        """The manual on a panel of its own, between the download and every file: a person who wants the
        manual should not have to look for it among the installers, and the program's Help menu points here."""
        man = self.extra("-manual.pdf")
        if not man:
            return ""
        url = e(release_url(self.repo, self.version, man["name"]))
        return (
            '<section class="panel" id="manual">\n'
            '<div class="title"><h2>The manual</h2><span class="rainbow" aria-hidden="true"><i></i><i></i><i></i><i></i></span></div>\n'
            '<div class="body">\n'
            f'<p>What the program does, menu by menu and flag by flag, in one PDF: <a href="{url}">{e(man["name"])}</a> '
            f'(A4, {human(man["size"])}, with its pictures). It is the same text as <code>docs/MANUAL.md</code>. '
            'Put it in the folder of the program and <em>Help &gt; ZenZX Manual</em> opens it from there; '
            'without it, that menu row opens this copy on the web.</p>\n'
            f'<p class="dim hash">SHA-256: <code>{man["sha256"]}</code></p>\n'
            '</div>\n'
            '</section>\n')

    def render(self):
        v = e(self.version)
        docs = self.documents_strip()
        man = self.extra("-manual.pdf")
        nav_docs = ((f'<a href="{e(release_url(self.repo, self.version, man["name"]))}">Manual</a>' if man else "")
                    + '<a href="#files">Every file</a>')
        return (TEMPLATE
                .replace("@@CSS@@", CSS)
                .replace("@@JS@@", JS)
                .replace("@@VERSION@@", v)
                .replace("@@REPO@@", e(self.repo))
                .replace("@@FIGURE@@", e(self.figure["frames"][self.figure["rest"]]))
                .replace("@@FIGURE_JSON@@", json.dumps(self.figure).replace("</", "<\\/"))
                .replace("@@CHOOSER@@", self.chooser())
                .replace("@@MANUAL@@", self.manual_panel())
                .replace("@@EVERY@@", self.every_file())
                .replace("@@DOCS@@", ("<ul class=\"next\">" + docs + "</ul>") if docs else "")
                .replace("@@NAVDOCS@@", nav_docs))


TEMPLATE = """<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>ZenZX @@VERSION@@</title>
<meta name="description" content="ZenZX, a ZX Spectrum and Spectrum Next emulator, for Windows, macOS and Linux.">
<link rel="icon" type="image/png" href="assets/zenzx-64.png">
<style>
@@CSS@@
</style>
</head>
<body>
<header class="bar">
<a class="brand" href="#top"><img src="assets/speccy.png" width="32" height="20" alt="">ZenZX</a>
<nav><a href="#get">Download</a>@@NAVDOCS@@<a href="https://github.com/@@REPO@@">Source</a></nav>
<span class="rainbow" aria-hidden="true"><i></i><i></i><i></i><i></i></span>
</header>
<main id="top">

<section class="panel hero">
<img class="icon" src="assets/speccy.png" width="64" height="40" alt="A ZX Spectrum, drawn in pixels">
<div>
<h1>ZenZX @@VERSION@@</h1>
<p>A ZX Spectrum emulator: 48K, 128K, +2, +2A, +3, the Spanish variants, the Timex Sinclair 2068
and the ZX Spectrum Next. One program, with its ROMs inside it.</p>
@@DOCS@@
</div>
</section>

<section class="panel" id="get">
<div class="title"><h2>Get ZenZX</h2><span class="rainbow" aria-hidden="true"><i></i><i></i><i></i><i></i></span></div>
<div class="body">
<div class="screen">
<img id="figure" class="figure" src="@@FIGURE@@" width="24" height="32" alt="" aria-hidden="true">
<div class="say">
<p id="you" class="you">Choose your system. Each has a note on how to tell which build is the right one.</p>
<div id="pick" class="pick" hidden></div>
</div>
</div>
<div id="tabs" class="tabs" role="tablist" hidden></div>
<div id="panels">
@@CHOOSER@@
</div>
</div>
</section>

@@MANUAL@@
<section class="panel" id="files">
<div class="title"><h2>Every file</h2><span class="rainbow" aria-hidden="true"><i></i><i></i><i></i><i></i></span></div>
<div class="body files">
@@EVERY@@
<h2>For a program</h2>
<p><a href="runners.json">runners.json</a> lists the same programs for a ZenZX client, with the
size and SHA-256 of each; <code>file</code> is relative to the manifest's own address.</p>

<h2>More</h2>
<ul>
<li><a href="https://github.com/@@REPO@@/releases/tag/v@@VERSION@@">Release @@VERSION@@</a>:
the same programs in archives, with the ROMs, the manual and the Next demo card beside them,
and the programs without a window.</li>
<li><a href="https://github.com/@@REPO@@">Source</a>, Apache License 2.0.</li>
</ul>
</div>
</section>

</main>
<footer>ZenZX @@VERSION@@. The face is Spleen 5x8 by Frederic Cambus (BSD 2-Clause, <a href="assets/LICENSE-Spleen.txt">licence</a>).</footer>
<script type="application/json" id="figure-data">@@FIGURE_JSON@@</script>
<script>
@@JS@@
</script>
</body>
</html>
"""

CSS = """
:root {
  --black: #000; --white: #fff; --ink: #000; --dim: #4d4d4d;
  --blue: #0000d7; --red: #d70000; --yellow: #ff0; --green: #00d700; --cyan: #0ff;
  --bar-top: #4d4d4d; --frame-top: #333; --frame-bottom: #1a1a1a;
}
* { box-sizing: border-box; }
html { background: var(--frame-bottom); scroll-behavior: smooth; }
body {
  margin: 0; min-height: 100vh; color: var(--ink);
  background: linear-gradient(var(--frame-top), var(--frame-bottom)) fixed;
  font: 16px/24px "Spleen 5x8 Proportional", "Spleen 5x8", ui-monospace, "SF Mono", Menlo, Consolas, monospace;
  -webkit-font-smoothing: none; font-smooth: never; text-rendering: optimizeSpeed;
}
@font-face { font-family: "Spleen 5x8"; src: url(assets/spleen-5x8.woff) format("woff"); font-display: swap; }
@font-face { font-family: "Spleen 5x8 Proportional"; src: url(assets/spleen-5x8-proportional.woff) format("woff"); font-display: swap; }
img { image-rendering: pixelated; }
a { color: var(--blue); }
a:hover, a:focus-visible { background: var(--cyan); color: var(--black); outline: none; }
code, .sum, pre { font-family: "Spleen 5x8", ui-monospace, "SF Mono", Menlo, Consolas, monospace; }
code { background: #e8e8e8; padding: 0 4px; }
em { font-style: normal; background: #e8e8e8; padding: 0 4px; }
h1, h2, h3, strong { margin: 0; font-weight: normal; }
strong { text-decoration: underline; text-decoration-thickness: 2px; text-underline-offset: 4px; }
.choice .what strong { text-decoration: none; text-transform: uppercase; }
h1 { font-size: 32px; line-height: 40px; }
h2 { font-size: 24px; line-height: 32px; }
h3 { font-size: 16px; line-height: 24px; text-transform: uppercase; margin: 16px 0 8px; }
p { margin: 8px 0; }
.dim { color: var(--dim); }

/* The bar: grey fading to black, the rainbow at its end, white text. */
.bar {
  position: sticky; top: 0; z-index: 5; display: flex; align-items: center; gap: 24px;
  height: 40px; padding: 0 0 0 16px; color: var(--white); overflow: hidden;
  background: linear-gradient(var(--bar-top), var(--black)); border-bottom: 2px solid var(--black);
}
.bar a { color: var(--white); text-decoration: none; padding: 0 8px; line-height: 40px; }
.bar a:hover, .bar a:focus-visible { background: var(--cyan); color: var(--black); }
.bar .brand { display: flex; align-items: center; gap: 8px; padding-left: 0; font-size: 24px; }
.bar .brand img { display: block; }
.bar nav { display: flex; gap: 8px; flex: 1; overflow: hidden; }
.bar nav a { white-space: nowrap; }
.rainbow { display: inline-flex; flex: none; height: 100%; transform: skewX(-35deg); margin-right: 12px; }
.rainbow i { display: block; width: 12px; height: 100%; }
.rainbow i:nth-child(1) { background: var(--red); }
.rainbow i:nth-child(2) { background: #d7d700; }
.rainbow i:nth-child(3) { background: var(--green); }
.rainbow i:nth-child(4) { background: #00d7d7; }

main { max-width: 904px; margin: 0 auto; padding: 24px 16px 8px; }
.panel { background: var(--white); border: 2px solid var(--black); box-shadow: 8px 8px 0 rgba(0, 0, 0, .55); margin: 0 0 32px; }
.panel .title {
  display: flex; align-items: center; justify-content: space-between; height: 40px; padding-left: 16px;
  color: var(--white); background: linear-gradient(var(--bar-top), var(--black)); border-bottom: 2px solid var(--black);
}
.panel .title .rainbow { margin-right: 12px; }
.panel .body { padding: 16px; position: relative; }

.hero { display: flex; gap: 24px; align-items: center; padding: 24px; }
.hero .icon { width: 192px; height: 120px; flex: none; }
.hash { overflow-wrap: anywhere; }
.next { display: flex; flex-wrap: wrap; gap: 8px 24px; list-style: none; padding: 0; margin: 8px 0 0; }

/* Choosing. */
/* The black box is the colour of the figure's own window: it stands in the box, on its floor. */
.screen { display: flex; align-items: flex-end; gap: 16px; padding: 16px 16px 0; background: var(--black); color: var(--white); }
.say { flex: 1; min-width: 0; padding-bottom: 16px; }
.figure { display: block; flex: none; width: 72px; height: 96px; }
.you { margin: 0; }
.pick { margin: 16px 0 0; }
.pick .big {
  display: inline-block; padding: 8px 16px; background: var(--yellow); color: var(--black);
  border: 2px solid var(--white); text-decoration: none; font-size: 24px; line-height: 32px;
}
.pick .big:hover, .pick .big:focus-visible { background: var(--cyan); }
.pick p { margin: 8px 0 0; color: var(--white); }
.panel .body > .screen { margin: -16px -16px 16px; }
.tabs { display: flex; flex-wrap: wrap; gap: 0; margin: 16px 0 0; border-bottom: 2px solid var(--black); }
.tabs button {
  font: inherit; color: var(--black); background: #d7d7d7; border: 2px solid var(--black); border-bottom: 0;
  margin: 0 4px 0 0; padding: 0 16px; height: 32px; cursor: pointer;
}
.tabs button:hover { background: #e8e8e8; }
.tabs button[aria-selected="true"] { background: var(--cyan); height: 34px; margin-bottom: -2px; }
.tabs button:focus-visible { outline: 2px solid var(--blue); outline-offset: 2px; }
.js .os[hidden] { display: none; }
.js .os > h3 { display: none; }
.choices { list-style: none; margin: 8px 0; padding: 0; border: 2px solid var(--black); }
.choice { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 8px 12px; border-bottom: 2px solid var(--black); }
.choice:last-child { border-bottom: 0; }
.choice .what { display: flex; flex-direction: column; min-width: 0; }
.choice .what .dim { font-size: 16px; }
.choice.rec { background: var(--yellow); }
.choice.rec .dim { color: var(--black); }
.choice.rec .tag { background: var(--blue); color: var(--white); padding: 0 8px; margin-left: 8px; }
a.dl {
  display: inline-flex; flex-direction: column; align-items: flex-end; flex: none; text-align: right;
  padding: 4px 12px; background: var(--black); color: var(--white); text-decoration: none; border: 2px solid var(--black);
}
a.dl small { font-size: 16px; color: #d7d7d7; }
a.dl:hover, a.dl:focus-visible { background: var(--cyan); color: var(--black); }
a.dl:hover small, a.dl:focus-visible small { color: var(--black); }
details { margin: 8px 0; border: 2px solid var(--black); }
summary { padding: 4px 12px; cursor: pointer; background: #d7d7d7; }
summary:hover { background: var(--cyan); }
details[open] summary { border-bottom: 2px solid var(--black); }
details > p { margin: 8px 12px; }

/* The long list. */
.files h2 { margin: 24px 0 8px; }
.files h2:first-child { margin-top: 0; }
table { border-collapse: collapse; width: 100%; }
th, td { text-align: left; padding: 4px 8px; border-bottom: 2px solid var(--black); vertical-align: top; }
th { background: #d7d7d7; }
td.n { white-space: nowrap; }
.sum { font-size: 16px; word-break: break-all; }
.sum code { font-size: 16px; background: none; padding: 0; }
.files ul { padding-left: 24px; }
footer { max-width: 904px; margin: 0 auto; padding: 8px 16px 32px; color: #d7d7d7; }
footer a { color: var(--cyan); }
footer a:hover { color: var(--black); }

@media (max-width: 40rem) {
  .bar nav a:nth-child(n+3) { display: none; }
  .hero { flex-direction: column; align-items: flex-start; padding: 16px; }
  .hero .icon { width: 128px; height: 80px; }
  .figure { width: 48px; height: 64px; }
  .choice { flex-direction: column; align-items: stretch; gap: 8px; }
  a.dl { align-items: flex-start; text-align: left; }
  .sum { display: none; }
  td.n { white-space: normal; overflow-wrap: anywhere; }
  th, td { padding: 4px; }
  .pick .big { font-size: 16px; }
}
@media (prefers-reduced-motion: reduce) { html { scroll-behavior: auto; } }
"""

JS = r"""
(function () {
  "use strict";
  var doc = document, root = doc.documentElement;
  var VERSION = "@@VERSION@@";
  root.className += " js";

  var NAMES = { windows: "Windows", darwin: "macOS", linux: "Linux" };
  var ARCH = {
    windows: { amd64: "an Intel or AMD processor", arm64: "an ARM processor" },
    linux: { amd64: "an Intel or AMD processor", arm64: "an ARM processor" },
    darwin: { amd64: "an Intel processor", arm64: "Apple silicon" }
  };
  var ua = navigator.userAgent || "";

  // What the browser says about the computer. A phone or a tablet is no machine for this.
  function detect() {
    var d = { os: "", arch: "", old: false, mobile: false, x86: false };
    if (/Android|iPhone|iPad|iPod/.test(ua)) { d.mobile = true; return d; }
    if (/Windows/.test(ua)) {
      d.os = "windows";
      d.arch = /ARM64|ARM;/.test(ua) ? "arm64" : "amd64";
      var m = /Windows NT (\d+)\./.exec(ua);
      if (m && +m[1] < 10) { d.old = true; }
      if (!/Win64|x64|WOW64|ARM/.test(ua)) { d.x86 = true; d.arch = ""; }
    } else if (/Macintosh|Mac OS X/.test(ua)) {
      d.os = "darwin";
    } else if (/Linux|X11/.test(ua) && !/CrOS/.test(ua)) {
      d.os = "linux";
      d.arch = /aarch64|arm64|armv8/.test(ua) ? "arm64" : (/x86_64|x86-64|amd64/.test(ua) ? "amd64" : "");
    }
    return d;
  }

  function find(os, kind, arch) {
    var sel = 'a.dl[data-os="' + os + '"][data-kind="' + kind + '"]' + (arch ? '[data-arch="' + arch + '"]' : "");
    return doc.querySelector(sel);
  }

  // The download that fits, or null when the computer does not say enough to choose one.
  function choose(d) {
    if (!d.os || d.x86) { return null; }
    if (d.os === "windows" && d.old) {
      return find("windows7", "installer") || find("windows7", "program");
    }
    if (d.os === "linux") { return d.arch ? find("linux", "program", d.arch) : null; }
    if (d.os === "darwin") { return find("darwin", "installer", "universal") || (d.arch ? find("darwin", "program", d.arch) : null); }
    return d.arch ? (find(d.os, "installer", d.arch) || find(d.os, "program", d.arch)) : null;
  }

  var you = doc.getElementById("you"), pick = doc.getElementById("pick");
  var tabs = doc.getElementById("tabs");
  var panels = [].slice.call(doc.querySelectorAll(".os"));

  function show(id) {
    panels.forEach(function (p) { p.hidden = p.id !== id; });
    [].slice.call(tabs.children).forEach(function (b) {
      b.setAttribute("aria-selected", b.getAttribute("aria-controls") === id ? "true" : "false");
    });
  }

  function makeTabs(first) {
    panels.forEach(function (p) {
      var b = doc.createElement("button");
      b.type = "button"; b.setAttribute("role", "tab"); b.setAttribute("aria-controls", p.id);
      b.textContent = NAMES[p.getAttribute("data-os")];
      b.addEventListener("click", function () { show(p.id); });
      tabs.appendChild(b);
    });
    tabs.hidden = false;
    show(first);
  }

  function say(text) { you.textContent = text; }

  function recommend(d) {
    var link = choose(d), where = d.os ? "os-" + (d.os === "darwin" ? "macos" : d.os) : "os-windows";
    makeTabs(where);
    if (d.mobile) {
      say("This looks like a phone or a tablet. ZenZX is for a computer: you can fetch it from here for one, or come back from the computer.");
      return;
    }
    if (!d.os) { say("We could not tell what system this is. Choose yours below."); return; }
    if (d.x86) {
      say("This looks like 32-bit Windows, and ZenZX has no 32-bit build. If the PC is 64-bit, choose its build below.");
      return;
    }
    if (!link) {
      say("This looks like " + NAMES[d.os] + (d.os === "darwin" ? ", Intel or Apple silicon" : "") +
          ". Choose the build for your processor below; the note under the list says how to tell.");
      return;
    }
    var row = link.parentNode;
    row.className += " rec";
    var tag = doc.createElement("span"); tag.className = "tag"; tag.textContent = "Recommended for you";
    row.querySelector("strong").appendChild(tag);
    var oname = link.getAttribute("data-os") === "windows7" ? "Windows 7" : NAMES[d.os];
    say("This looks like " + oname + (d.arch ? " on " + ARCH[d.os][d.arch] : "") + ". The build that fits is marked below, and offered here.");
    var big = doc.createElement("a");
    big.className = "big"; big.href = link.href; big.textContent = "Download ZenZX " + VERSION + " for " + oname;
    var p = doc.createElement("p");
    p.textContent = link.getAttribute("data-label") + ", " + link.getAttribute("data-size");
    pick.appendChild(big); pick.appendChild(p); pick.hidden = false;
  }

  var d = detect();
  recommend(d);
  // Chromium says more, when asked: Windows on ARM and an Apple silicon Mac look like Intel to the user agent string.
  var uad = navigator.userAgentData;
  if (uad && uad.getHighEntropyValues && (d.os === "windows" || d.os === "darwin" || d.os === "linux") && !d.old) {
    uad.getHighEntropyValues(["architecture", "bitness"]).then(function (h) {
      var arch = h.architecture === "arm" ? "arm64" : (h.architecture === "x86" ? "amd64" : "");
      if (arch && arch !== d.arch) {
        d.arch = arch;
        [].slice.call(doc.querySelectorAll(".choice.rec")).forEach(function (r) {
          r.className = r.className.replace(" rec", ""); var t = r.querySelector(".tag"); if (t) { t.parentNode.removeChild(t); }
        });
        pick.hidden = true; pick.textContent = ""; tabs.textContent = "";
        recommend(d);
      }
    }, function () {});
  }

  // The figure that waits, as the program has it, with the steps it is given.
  var fig = doc.getElementById("figure"), data = null;
  try { data = JSON.parse(doc.getElementById("figure-data").textContent); } catch (err) { data = null; }
  var still = window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  if (fig && data && !still) {
    data.frames.forEach(function (f) { var im = new Image(); im.src = f; });
    var steps = data.intro.concat(data.loop), i = 0, first = data.intro.length;
    var next = function () {
      var s = steps[i];
      fig.src = data.frames[s[0]];
      i += 1;
      if (i >= steps.length) { i = data.loop.length ? first : 0; }
      if (steps.length > 1) { setTimeout(next, Math.max(60, s[1] * 1000)); }
    };
    if (steps.length) { next(); }
  }
})();
"""
