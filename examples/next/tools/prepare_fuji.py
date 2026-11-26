#!/usr/bin/env python3
"""Prepare the pictures of the fuji example from the original on Wikimedia Commons.

    python3 tools/prepare_fuji.py ORIGINALS_DIR

ORIGINALS_DIR holds Red_Fuji_southern_wind_clear_morning.jpg, at 1920 pixels across or
more (see README.md for where from). The print is taken apart into three pictures:

    fuji/art/sky.png        320x256, the sky and its clouds with the mountain taken out,
                            made to join up left to right so that it can scroll for ever
    fuji/art/mountain.png   320x256, the mountain and the forest alone, the sky transparent,
                            in fifteen colours
    fuji/art/title.png      32x112, the title cartouche

This is the record of how those were made; building the example needs only the PNGs.
"""
import os
import sys

from PIL import Image, ImageFilter

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from nextart import dither333, colours

src = sys.argv[1]
out = "fuji/art/"
W, H = 320, 256
SMOOTH = 3

big = Image.open(src + "/Red_Fuji_southern_wind_clear_morning.jpg").convert("RGB")
bw, bh = big.size

# The frame is 5:4 and the print 3:2: keep the right of it, where the mountain is.
cw = bh * 5 // 4
pic = big.crop((bw - cw, 0, bw, bh)).resize((W, H), Image.LANCZOS)
px = pic.load()


def is_mountain(c):
    r, g, b = c
    return (r + g + b) < 510 and not b > r + 25


def is_white(c):
    return c[0] > 185 and c[1] > 175 and c[2] > 140


# Where the mountain starts in each column: the first place from the top that stays
# mountain-coloured, then up through the snow, which is white like the clouds but joined on.
top = []
for x in range(W):
    y = 0
    while y < H:
        run = sum(1 for k in range(8) if y + k < H and is_mountain(px[x, y + k]))
        if run >= 7:
            break
        y += 1
    while y > 0 and is_white(px[x, y - 1]) and y > 40 and x > 150:
        y -= 1
    top.append(y)
# Smooth out single columns.
top = [sorted(top[max(0, x - 2):x + 3])[len(top[max(0, x - 2):x + 3]) // 2] for x in range(W)]

# The mountain: fifteen colours, the rest transparent.
mount_rgb = Image.new("RGB", (W, H), (0, 0, 0))
mp = mount_rgb.load()
for x in range(W):
    for y in range(top[x], H):
        mp[x, y] = px[x, y]
# No dithering: flat areas of one colour make tiles that repeat, and the tilemap has room
# for about 430 different ones beside its map in bank 5.
# And the paper's grain is smoothed away first and stray pixels after, for the same reason.
q = mount_rgb.filter(ImageFilter.MedianFilter(SMOOTH)).quantize(15, method=Image.MEDIANCUT, dither=Image.NONE)
q = q.filter(ImageFilter.ModeFilter(SMOOTH)).convert("RGB")
qp = q.load()
mountain = Image.new("RGBA", (W, H), (0, 0, 0, 0))
ap = mountain.load()
for x in range(W):
    for y in range(top[x], H):
        ap[x, y] = qp[x, y] + (255,)
mountain.save(out + "mountain.png")

# The sky: where the mountain was, the sky beside it mirrored about the mountain's edge;
# then the ends are joined so that the picture wraps.
sky = pic.copy()
sp = sky.load()
for y in range(H):
    edge = next((x for x in range(W) if top[x] <= y), W)
    for x in range(edge, W):
        mx = 2 * edge - 1 - x
        while mx < 0:
            mx += max(1, edge)
        sp[x, y] = sp[max(0, min(edge - 1, mx)), y] if edge > 0 else sp[x, y - 1]
# The join: the sky is stretched to F columns more than the frame, and those F columns,
# which are what would follow the right-hand edge, are faded into the first F.
F = 40
wide = sky.resize((W + F, H), Image.LANCZOS)
wp = wide.load()
seam = wide.crop((0, 0, W, H))
kp = seam.load()
for y in range(H):
    for i in range(F):
        a = i / (F - 1)
        after, own = wp[W + i, y], wp[i, y]
        kp[i, y] = tuple(round(after[k] * (1 - a) + own[k] * a) for k in range(3))
sky = dither333(seam)
sky.save(out + "sky.png")

# The title: the cartouche's rectangle from the top left of the print.
x0, x1, y0, y1 = 0.0609, 0.0985, 0.0975, 0.3120
cart = big.crop((int(bw * x0), int(bh * y0), int(bw * x1), int(bh * y1)))
tw = round(112 * cart.size[0] / cart.size[1])
small = cart.resize((tw, 112), Image.LANCZOS)
title = Image.new("RGBA", (32, 112), (0, 0, 0, 0))
title.paste(small, ((32 - tw) // 2, 0))
title.save(out + "title.png")
print("sky.png:", colours(sky), "colours; mountain.png:", colours(q), "colours; title", small.size)
