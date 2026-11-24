"""Shared steps for preparing the examples' pictures."""
from PIL import Image


def dither333(img):
    """Floyd-Steinberg dither an RGB image to the Next's nine-bit colours.

    The Next has three bits for each of red, green and blue. A painting's soft gradients
    turn into flat bands when each pixel is simply rounded to them. zx does not yet dither
    against the three-bit levels (zentools' register, T-09), so it is done here and zx is
    given a picture that already holds only colours the Next has.
    """
    img = img.convert("RGB")
    w, h = img.size
    src = img.load()
    out = Image.new("RGB", (w, h))
    dst = out.load()
    err = [[[0.0, 0.0, 0.0] for _ in range(w + 2)] for _ in range(2)]
    for y in range(h):
        cur, nxt = err[y & 1], err[(y + 1) & 1]
        for x in range(w + 2):
            nxt[x] = [0.0, 0.0, 0.0]
        for x in range(w):
            px = []
            for k in range(3):
                want = src[x, y][k] + cur[x + 1][k]
                level = min(7, max(0, int(want * 7 / 255 + 0.5)))
                got = round(level * 255 / 7)
                e = want - got
                cur[x + 2][k] += e * 7 / 16
                nxt[x][k] += e * 3 / 16
                nxt[x + 1][k] += e * 5 / 16
                nxt[x + 2][k] += e * 1 / 16
                px.append(got)
            dst[x, y] = tuple(px)
    return out


def colours(img):
    return len(set(img.convert("RGB").get_flattened_data() if hasattr(img, "get_flattened_data") else img.convert("RGB").getdata()))
