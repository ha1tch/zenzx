#!/usr/bin/env python3
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0
"""Make the download page's font: ZenZX's own Spleen 5x8, as a web font.

    python3 tools/make_site_font.py            writes assets/site/spleen-5x8.woff (a 5-pixel cell for every glyph, as
                                               the program has it) and spleen-5x8-proportional.woff (each glyph as
                                               wide as its ink, for running text)
    python3 tools/make_site_font.py check      says whether those files hold what this would make

pkg/fonts/spleen-5x8.bdf is the bitmap face the menu bar is drawn in. This turns each of its
pixels into a square of a TrueType outline, 100 units to the pixel and 800 to the em, so the
page's text is the menu bar's text: at a font size that is a multiple of 8 CSS pixels (16, 24,
32) every pixel of it is a whole number of screen pixels, and the edges are as hard as the
program's. A row of pixels is one rectangle, so that no seam shows between two.

It needs fontTools (pip install fonttools), which only this script uses: build_site.py, which
copies the result into the page, needs nothing but Python. The face is Spleen, by Frederic
Cambus, under the BSD 2-Clause licence (assets/site/LICENSE-Spleen.txt travels with it).
"""
import io
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
BDF = os.path.join(ROOT, "pkg", "fonts", "spleen-5x8.bdf")
OUT = os.path.join(ROOT, "assets", "site", "spleen-5x8.woff")                # every glyph in a 5-pixel cell
OUT_PROP = os.path.join(ROOT, "assets", "site", "spleen-5x8-proportional.woff")  # each glyph as wide as its ink
PX = 100          # font units to a pixel of the bitmap
EM = 800          # eight pixels
ASCENT, DESCENT = 700, 100   # the bitmap has seven rows over the baseline and one under it


def parse_bdf(text):
    """{code point: (xoff, yoff, [row strings of 0/1])} for each glyph of the font."""
    glyphs = {}
    for block in text.split("STARTCHAR")[1:]:
        enc = int(re.search(r"^ENCODING (\d+)", block, re.M).group(1))
        w, h, xo, yo = (int(n) for n in re.search(r"^BBX (-?\d+) (-?\d+) (-?\d+) (-?\d+)", block, re.M).groups())
        hexrows = block.split("BITMAP")[1].split("ENDCHAR")[0].split()
        rows = []
        for hx in hexrows:
            bits = bin(int(hx, 16))[2:].zfill(len(hx) * 4)
            rows.append(bits[:w])
        glyphs[enc] = (xo, yo, rows)
    return glyphs


def build(proportional=False):
    """The font as WOFF bytes, and its glyph count.

    Spleen is a monospaced face: every glyph has a cell of 5 pixels, so an "i" sits in a cell as wide
    as an "m", and running text in it looks gappy. The proportional version gives each glyph the width
    of its ink and one pixel after it (a space is 3 pixels wide), and keeps the digits in cells of 5,
    so that columns of figures still line up. The pixels are the same pixels."""
    from fontTools.fontBuilder import FontBuilder
    from fontTools.pens.ttGlyphPen import TTGlyphPen

    glyphs = parse_bdf(open(BDF, encoding="utf-8").read())
    names = {}
    order = [".notdef"]
    cmap = {}
    for code in sorted(glyphs):
        if code == 0x7f or 0x80 <= code < 0xa0:
            continue
        name = "space" if code == 32 else f"uni{code:04X}"
        names[code] = name
        order.append(name)
        cmap[code] = name

    pens = {}
    metrics = {".notdef": (5 * PX, 0)}
    for code, name in names.items():
        xo, yo, rows = glyphs[code]
        runs = []                                   # (row, first column, end column), columns from the left of the box
        for r, bits in enumerate(rows):
            for m in re.finditer(r"1+", bits):
                runs.append((r, m.start(), m.end()))
        left = min((a for _, a, _ in runs), default=0)
        right = max((b for _, _, b in runs), default=0)
        mono_cell = not proportional or chr(code).isdigit()
        # Where the glyph's first ink column goes, in pixels from the glyph's origin, and how far on the next glyph starts.
        if mono_cell:
            shift, advance = xo, 5
        else:
            shift, advance = -left, (right - left + 1) if runs else 3
        pen = TTGlyphPen(None)
        h = len(rows)
        for r, a, b in runs:
            y0 = (yo + h - 1 - r) * PX              # the bottom of this row, from the baseline
            x0, x1 = (shift + a) * PX, (shift + b) * PX
            # clockwise, which is the outline direction TrueType fills
            pen.moveTo((x0, y0))
            pen.lineTo((x0, y0 + PX))
            pen.lineTo((x1, y0 + PX))
            pen.lineTo((x1, y0))
            pen.closePath()
        pens[name] = pen.glyph()
        # The left side bearing is the first ink pixel's place: a renderer that uses it must not move the glyph.
        metrics[name] = (advance * PX, (shift + left) * PX if runs else 0)
    notdef = TTGlyphPen(None)
    for (a, b, c, d) in ((0, 0, 50, 700), (400, 0, 450, 700), (50, 0, 400, 50), (50, 650, 400, 700)):
        notdef.moveTo((a, b)); notdef.lineTo((a, d)); notdef.lineTo((c, d)); notdef.lineTo((c, b)); notdef.closePath()
    pens[".notdef"] = notdef.glyph()
    metrics[".notdef"] = (5 * PX, 0)

    family = "Spleen 5x8 Proportional" if proportional else "Spleen 5x8"
    fb = FontBuilder(EM, isTTF=True)
    fb.setupGlyphOrder(order)
    fb.setupCharacterMap(cmap)
    fb.setupGlyf(pens)
    fb.setupHorizontalMetrics(metrics)
    fb.setupHorizontalHeader(ascent=ASCENT, descent=-DESCENT)
    fb.setupNameTable({
        "familyName": family, "styleName": "Regular",
        "copyright": "Copyright (c) 2018-2019, Frederic Cambus. BSD 2-Clause licence.",
        "licenseDescription": "BSD 2-Clause",
        "description": f"{family}, from pkg/fonts/spleen-5x8.bdf, as outlines (tools/make_site_font.py)",
    })
    fb.setupOS2(sTypoAscender=ASCENT, sTypoDescender=-DESCENT, usWinAscent=ASCENT, usWinDescent=DESCENT,
                sxHeight=500, sCapHeight=600)
    fb.setupPost(isFixedPitch=0 if proportional else 1)
    fb.font.flavor = "woff"
    # WOFF keeps the table directory's own order and dates; a fixed head date makes the bytes steady.
    fb.font["head"].created = fb.font["head"].modified = 0
    buf = io.BytesIO()
    fb.font.save(buf)
    return buf.getvalue(), len(names)


def main(argv):
    try:
        made = [(OUT, build(False)), (OUT_PROP, build(True))]
    except ImportError:
        print("make_site_font: needs fontTools (pip install fonttools)", file=sys.stderr)
        return 2
    if argv[1:] == ["check"]:
        stale = [p for p, (data, _) in made if not os.path.isfile(p) or open(p, "rb").read() != data]
        print("the fonts are current" if not stale else "NOT what the script makes: " + ", ".join(stale))
        return 1 if stale else 0
    os.makedirs(os.path.dirname(OUT), exist_ok=True)
    for path, (data, n) in made:
        with open(path, "wb") as f:
            f.write(data)
        print(f"{path}: {n} glyphs, {len(data)} bytes")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
