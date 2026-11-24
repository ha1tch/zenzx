#!/usr/bin/env python3
"""Prepare the pictures of the orbit example from the originals on Wikimedia Commons.

    python3 tools/prepare_orbit.py ORIGINALS_DIR

ORIGINALS_DIR holds The_Earth_seen_from_Apollo_17.jpg, Moon_nearside_LRO.jpg and
Phobos_colour_2008.jpg (see README.md for where from). It writes

    orbit/art/earth.png    256x192, the Earth 160 pixels across in the middle, space transparent
    orbit/art/bodies.png   32x48: the Moon, 32x32, and under it Phobos, 16x16, space transparent

This is the record of how those were made; building the example needs only the PNGs.
"""
import os
import sys

from PIL import Image, ImageDraw

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from nextart import dither333

src = sys.argv[1]
out = "orbit/art/"


def disc(name, size, threshold=40, fills_frame=False):
    """The lit body of a picture taken against black space, scaled to size x size, with
    everything outside its outline transparent. fills_frame is for a picture whose body
    touches all four edges: its outline is then the circle that fits the frame."""
    img = Image.open(src + "/" + name).convert("RGB")
    if fills_frame:
        body = img.resize((size, size), Image.LANCZOS)
        alpha = Image.new("L", (size, size), 0)
        ImageDraw.Draw(alpha).ellipse((0, 0, size - 1, size - 1), fill=255)
    else:
        box = img.convert("L").point(lambda v: 255 if v > threshold else 0).getbbox()
        body = img.crop(box).resize((size, size), Image.LANCZOS)
        lit = body.convert("L").point(lambda v: 255 if v > threshold * 0.6 else 0)
        # Space is the dark that can be reached from the corners; dark inside the body stays.
        for corner in ((0, 0), (size - 1, 0), (0, size - 1), (size - 1, size - 1)):
            if lit.getpixel(corner) == 0:
                ImageDraw.floodfill(lit, corner, 128)
        alpha = lit.point(lambda v: 0 if v == 128 else 255)
    rgba = dither333(body).convert("RGBA")
    rgba.putalpha(alpha)
    return rgba


earth = Image.new("RGBA", (256, 192), (0, 0, 0, 0))
earth.paste(disc("The_Earth_seen_from_Apollo_17.jpg", 160), (48, 16))
earth.save(out + "earth.png")

bodies = Image.new("RGBA", (32, 48), (0, 0, 0, 0))
bodies.paste(disc("Moon_nearside_LRO.jpg", 32, fills_frame=True), (0, 0))
bodies.paste(disc("Phobos_colour_2008.jpg", 16, threshold=30), (0, 32))
bodies.save(out + "bodies.png")
print("wrote", out + "earth.png", "and", out + "bodies.png")
