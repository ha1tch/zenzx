#!/usr/bin/env python3
"""Prepare the pictures of the gallop example from the originals on Wikimedia Commons.

    python3 tools/prepare_gallop.py ORIGINALS_DIR

ORIGINALS_DIR holds the two files as downloaded (see README.md for where from). This is
kept as the record of how gallop/art/*.png were made; building the example needs only
those PNGs, zenas and zx, not this script or Python.
"""
import os
import sys

from PIL import Image

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from nextart import dither333, colours

src = sys.argv[1]
out = "gallop/art/"

# The background: the middle of the painting, 4:3, at Layer 2's 256x192.
p = Image.open(src + "/Albert_Bierstadt_-_Among_the_Sierra_Nevada,_California_-_Google_Art_Project.jpg").convert("RGB")
w, h = p.size
cw = h * 4 // 3
x0 = int((w - cw) * 0.55)
bg = dither333(p.crop((x0, 0, x0 + cw, h)).resize((256, 192), Image.LANCZOS))
bg.save(out + "sierra.png")
print("sierra.png:", colours(bg), "colours")

# The horse: the first eleven pictures of the card are one stride. Each is cut out as a
# silhouette (the darkest connected shape of its cell), put on a 64x48 cell with the body
# in the same place in every frame, and coloured chestnut with a dark edge.
card = Image.open(src + "/The_Horse_in_Motion.jpg").convert("L")
W, H = card.size
sx, sy = W / 960.0, H / 595.0
cols = [(17, 240), (251, 474), (485, 708), (718, 941)]
rows = [(22, 160), (172, 312), (324, 464)]
frames = []
for i in range(11):
    (cx0, cx1), (cy0, cy1) = cols[i % 4], rows[i // 4]
    cell = card.crop((int(cx0 * sx), int(cy0 * sy), int(cx1 * sx), int(cy1 * sy)))
    dark = cell.point(lambda v: 255 if v < 70 else 0)
    # The track's lines go: dark pixels in a long horizontal run that is only a few pixels
    # thick. The legs, which are thin but not long, stay.
    dp = dark.load(); dw, dh = dark.size
    mask = dark.copy(); kp = mask.load()
    for y in range(dh):
        x = 0
        while x < dw:
            if not dp[x, y]:
                x += 1; continue
            x1 = x
            while x1 < dw and dp[x1, y]:
                x1 += 1
            if x1 - x > 60 * sx:
                for xx in range(x, x1):
                    t0 = y
                    while t0 > 0 and dp[xx, t0 - 1]:
                        t0 -= 1
                    t1 = y
                    while t1 < dh - 1 and dp[xx, t1 + 1]:
                        t1 += 1
                    if t1 - t0 + 1 <= 4 * sx:
                        kp[xx, y] = 0
            x = x1
    # Keep the largest connected shape.
    px = mask.load(); mw, mh = mask.size
    seen = [[False] * mw for _ in range(mh)]
    best = []
    for y in range(mh):
        for x in range(mw):
            if px[x, y] and not seen[y][x]:
                stack, comp = [(x, y)], []
                seen[y][x] = True
                while stack:
                    a, b = stack.pop(); comp.append((a, b))
                    for da, db in ((1, 0), (-1, 0), (0, 1), (0, -1)):
                        na, nb = a + da, b + db
                        if 0 <= na < mw and 0 <= nb < mh and px[na, nb] and not seen[nb][na]:
                            seen[nb][na] = True; stack.append((na, nb))
                if len(comp) > len(best):
                    best = comp
    shape = Image.new("L", mask.size, 0)
    sp = shape.load()
    for a, b in best:
        sp[a, b] = 255
    mx = sum(a for a, _ in best) / len(best)
    my = sum(b for _, b in best) / len(best)
    frames.append((cell, shape, mx, my))

scale = 64 / (190 * sx)
sheet = Image.new("RGBA", (64, 48 * 11), (0, 0, 0, 0))
outline = (36, 0, 0)
chestnut = (146, 73, 36)  # both are already colours the Next has
for i, (cell, shape, mx, my) in enumerate(frames):
    # A box of 190x142 card pixels around the centre of mass, a little above the middle.
    bw, bh = 190 * sx, 142 * sx
    box = (int(mx - bw / 2), int(my - bh * 0.52), int(mx + bw / 2), int(my + bh * 0.48))
    m = shape.crop(box).resize((64, 48), Image.LANCZOS)
    fr = Image.new("RGBA", (64, 48), (0, 0, 0, 0))
    fp, mp = fr.load(), m.load()
    inside = lambda x, y: 0 <= x < 64 and 0 <= y < 48 and mp[x, y] >= 128
    for y in range(48):
        for x in range(64):
            if not inside(x, y):
                continue
            if not (inside(x - 1, y) and inside(x + 1, y) and inside(x, y - 1) and inside(x, y + 1)):
                fp[x, y] = outline + (255,)  # the edge, so the shape reads on dark and on light
            else:
                fp[x, y] = chestnut + (255,)
    sheet.paste(fr, (0, 48 * i))
sheet.save(out + "horse.png")
print("wrote", out + "sierra.png", "and", out + "horse.png")
