The application icon: a Spectrum, 64 by 40 pixels.

  speccy01.zani   The picture as drawn (a ZCUT collection; frame00 is the icon, the other
                  frames are empty).
  speccy.png      frame00 as it is, 64 by 40, black and grey (200, 200, 200), no smoothing.
  png/            Square pictures at 64, 128, 256, 512 and 1024 pixels. Each is the picture
                  enlarged by a whole number (1, 2, 4, 8, 16) with no smoothing, in the middle of
                  a transparent square.
  zenzx.ico       For Windows: 64, 128 and 256.
  zenzx.icns      For macOS: 64, 128, 256, 512 and 1024.

Everything except the first two files is made by `python3 scripts/make_icons.py` from speccy.png,
and `python3 scripts/make_icons.py check` says whether they are up to date. To change the icon,
change speccy.png (and speccy01.zani with it) and run the script.
