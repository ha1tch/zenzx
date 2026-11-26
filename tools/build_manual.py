#!/usr/bin/env python3
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0
"""Make the PDF of the manual (docs/MANUAL.md) that a release carries beside its programs.

    build_manual.py --version 0.50.0 --out dist [--repo .] [--chrome PATH]

writes dist/zenzx-0.50.0-manual.pdf. The Markdown becomes a page of HTML with a contents list
and a print style (A4, the tables kept whole where they fit, code wrapped), and a headless
Chrome prints it. The pictures are taken from the repository, so they are in the file. A link
to another document of docs/ (gui.md, ROMS.md ...) becomes a link to that file as the tag
v<version> has it on GitHub, since the PDF holds the manual alone.

Needs the `markdown` package (pip install markdown) and Chrome or Chromium: --chrome, the
CHROME variable, or google-chrome, chromium and the like on the PATH. It stops, and writes
nothing, when either is missing, or when what came out is not a PDF of a plausible length.
"""
import argparse
import html
import os
import posixpath
import re
import shutil
import subprocess
import sys
import tempfile

REPO_URL = "https://github.com/ha1tch/zenzx"
MIN_PAGES = 10
CHROME_NAMES = ("google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome", "microsoft-edge")
CHROME_PATHS = (
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
    "/Applications/Chromium.app/Contents/MacOS/Chromium",
)

CSS = """
@page { size: A4; margin: 18mm 16mm 20mm 16mm; }
html { font-family: "Helvetica Neue", Helvetica, Arial, "Liberation Sans", "DejaVu Sans", sans-serif;
       font-size: 10pt; line-height: 1.45; color: #1a1a1a; }
body { margin: 0; }
h1, h2, h3, h4 { line-height: 1.2; color: #111; break-after: avoid; }
h1 { font-size: 20pt; margin: 0 0 6mm; padding-bottom: 2mm; border-bottom: 1.5pt solid #444; }
h1.part { break-before: page; }
h2 { font-size: 14pt; margin: 8mm 0 3mm; padding-bottom: 1mm; border-bottom: 0.5pt solid #999; }
h3 { font-size: 11.5pt; margin: 6mm 0 2mm; }
h4 { font-size: 10pt; margin: 4mm 0 1mm; }
p { margin: 0 0 3mm; orphans: 3; widows: 3; }
a { color: #0b4f9c; text-decoration: none; }
code, pre { font-family: "DejaVu Sans Mono", "Liberation Mono", Menlo, Consolas, monospace; }
code { font-size: 8.8pt; background: #f1f1f1; padding: 0 1.2pt; border-radius: 2pt; }
pre { font-size: 8.2pt; line-height: 1.35; background: #f6f6f6; border: 0.5pt solid #ccc; padding: 2.5mm;
      white-space: pre-wrap; overflow-wrap: anywhere; break-inside: avoid-page; margin: 0 0 4mm; }
pre code { background: none; padding: 0; font-size: inherit; }
table { border-collapse: collapse; width: 100%; margin: 0 0 4mm; font-size: 8.8pt; }
th, td { border: 0.5pt solid #aaa; padding: 1.2mm 2mm; vertical-align: top; text-align: left; }
th { background: #eaeaea; }
tr { break-inside: avoid; }
thead { display: table-header-group; }
img { max-width: 100%; height: auto; display: block; margin: 2mm auto; break-inside: avoid; }
blockquote { margin: 0 0 3mm; padding-left: 4mm; border-left: 1.5pt solid #bbb; color: #333; }
ul, ol { margin: 0 0 3mm; padding-left: 6mm; }
li { margin: 0 0 1mm; }
.cover { padding: 38mm 0 0; break-after: page; }
.cover h1 { font-size: 34pt; border: none; margin: 0 0 4mm; }
.cover p { font-size: 13pt; color: #444; }
.contents { break-after: page; }
.contents h2 { border: none; margin-top: 0; }
.contents ul { list-style: none; padding-left: 0; }
.contents ul ul { padding-left: 6mm; }
.contents li { margin: 0 0 0.8mm; }
"""


class BuildError(Exception):
    pass


def find_chrome(explicit=None):
    """The path of a Chrome or Chromium, or None."""
    for cand in (explicit, os.environ.get("CHROME")):
        if cand:
            found = shutil.which(cand) or (cand if os.path.isfile(cand) else None)
            if found:
                return found
    for name in CHROME_NAMES:
        found = shutil.which(name)
        if found:
            return found
    for path in CHROME_PATHS:
        if os.path.isfile(path):
            return path
    return None


def rewrite_links(body, version, docs_dir, repo):
    """Relative links of the manual: images by their file, documents by their place on GitHub."""
    blob = "%s/blob/v%s/" % (REPO_URL, version)

    def fix_img(m):
        src = m.group(2)
        if re.match(r"^[a-z][a-z0-9+.-]*:", src, re.I):
            return m.group(0)
        path = os.path.normpath(os.path.join(docs_dir, src))
        if not os.path.isfile(path):
            raise BuildError("the manual shows %s, which is not in the repository" % src)
        return m.group(1) + "file://" + path.replace(os.sep, "/") + m.group(3)

    def fix_href(m):
        href = html.unescape(m.group(2))
        if href.startswith("#") or re.match(r"^[a-z][a-z0-9+.-]*:", href, re.I):
            return m.group(0)
        target, _, frag = href.partition("#")
        rel = posixpath.normpath(posixpath.join("docs", target))
        if not os.path.exists(os.path.join(repo, rel)):
            raise BuildError("the manual links to %s, which is not in the repository" % target)
        url = blob + rel + ("#" + frag if frag else "")
        return m.group(1) + html.escape(url, quote=True) + m.group(3)

    body = re.sub(r'(<img\b[^>]*?\bsrc=")([^"]+)(")', fix_img, body)
    body = re.sub(r'(<a\b[^>]*?\bhref=")([^"]+)(")', fix_href, body)
    return body


def contents(tokens):
    """The contents list from the toc tokens: the parts and their sections."""
    def walk(items, depth):
        out = ["<ul>"]
        for t in items:
            out.append('<li><a href="#%s">%s</a>' % (html.escape(t["id"]), html.escape(t["name"])))
            kids = [c for c in t.get("children", []) if depth < 2]
            if kids:
                out.append(walk(kids, depth + 1))
            out.append("</li>")
        out.append("</ul>")
        return "".join(out)

    return walk(tokens, 1)


def manual_html(version, repo):
    """The manual as one page of HTML."""
    try:
        import markdown
    except ImportError:
        raise BuildError("the markdown package is needed: pip install markdown")
    docs_dir = os.path.join(repo, "docs")
    src = os.path.join(docs_dir, "MANUAL.md")
    if not os.path.isfile(src):
        raise BuildError("docs/MANUAL.md is missing")
    with open(src, encoding="utf-8") as f:
        text = f.read()
    md = markdown.Markdown(extensions=["tables", "fenced_code", "sane_lists", "toc"],
                           extension_configs={"toc": {"toc_depth": "1-3"}})
    body = md.convert(text)
    toc_tokens = md.toc_tokens
    body = rewrite_links(body, version, docs_dir, repo)
    # The title (the h1) is on the cover, with what the manual says before its first chapter; the
    # manual's own list of contents gives way to the one made here, whose links work.
    first = body.find("<h2")
    if first < 0 or len(toc_tokens) != 1:
        raise BuildError("docs/MANUAL.md is not a title and chapters any more")
    intro = re.sub(r"<h1\b.*?</h1>", "", body[:first], flags=re.S)
    body = re.sub(r'<h2 id="contents">.*?(?=<h2\b)', "", body[first:], count=1, flags=re.S)
    top = [t for t in toc_tokens[0].get("children", []) if t["id"] != "contents"]
    title = html.escape(toc_tokens[0]["name"])
    return (
        '<!doctype html><html lang="en"><head><meta charset="utf-8">'
        "<title>ZenZX %s manual</title><style>%s</style></head><body>"
        '<div class="cover"><h1>%s</h1><p>Version %s</p>%s'
        "<p>The documents beyond this manual, and the source, are at "
        '<a href="%s/tree/v%s">%s</a>.</p></div>'
        '<div class="contents"><h2>Contents</h2>%s</div>%s</body></html>'
        % (html.escape(version), CSS, title, html.escape(version), intro, REPO_URL, html.escape(version),
           REPO_URL, contents(top), body)
    )


def pdf_pages(path):
    with open(path, "rb") as f:
        data = f.read()
    if not data.startswith(b"%PDF-"):
        raise BuildError("what Chrome wrote is not a PDF")
    return len(re.findall(rb"/Type\s*/Page[^s]", data))


def build(version, repo, outdir, chrome=None):
    """Write the PDF; (its path, its pages)."""
    exe = find_chrome(chrome)
    if not exe:
        raise BuildError("no Chrome or Chromium found: give --chrome, set CHROME, or put one on the PATH")
    page = manual_html(version, repo)
    os.makedirs(outdir, exist_ok=True)
    out = os.path.join(outdir, "zenzx-%s-manual.pdf" % version)
    with tempfile.TemporaryDirectory() as tmp:
        src = os.path.join(tmp, "manual.html")
        with open(src, "w", encoding="utf-8") as f:
            f.write(page)
        part = os.path.join(tmp, "manual.pdf")
        cmd = [exe, "--headless=new", "--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage",
               "--no-pdf-header-footer", "--generate-pdf-document-outline",
               "--user-data-dir=" + os.path.join(tmp, "profile"),
               "--print-to-pdf=" + part, "file://" + src]
        r = subprocess.run(cmd, capture_output=True, text=True, timeout=300)
        if not os.path.isfile(part) or os.path.getsize(part) == 0:
            raise BuildError("Chrome wrote no PDF (status %d): %s" % (r.returncode, r.stderr.strip()[-400:]))
        pages = pdf_pages(part)
        if pages < MIN_PAGES:
            raise BuildError("the PDF has %d pages, expected at least %d" % (pages, MIN_PAGES))
        shutil.copyfile(part, out)
    return out, pages


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--version", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--repo", default=".")
    ap.add_argument("--chrome", default=None)
    args = ap.parse_args()
    try:
        path, pages = build(args.version, os.path.abspath(args.repo), args.out, args.chrome)
    except BuildError as e:
        print("build_manual.py: " + str(e), file=sys.stderr)
        return 1
    print("%s: %d pages, %.1f MB" % (path, pages, os.path.getsize(path) / 1e6))
    return 0


if __name__ == "__main__":
    sys.exit(main())
