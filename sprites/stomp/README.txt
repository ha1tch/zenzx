The pictures of the animation that plays when a machine is stopped (Machine > Stop): a leg
comes down from the top of the window and a boot stamps a small ZX Spectrum flat. The ground, with
the bin and the machine on it, comes up from below the window first and goes back down after.

ZenZX draws its own pictures. To use yours, put PNG files with these names in this folder and
build ZenZX again; a name that is not here keeps ZenZX's own picture of it. Each is drawn
enlarged by a whole number, with no smoothing, so draw them at their smallest size.

  icon.png   The machine, standing under the boot. Transparent where it is not.
             ZenZX's own is 23 by 15 pixels: a black Spectrum with a red badge, rows of keys, and
             a green stripe running down to the left in single pixels. It stands one pixel
             into the top of the bin, hiding the bin's handle (its first two rows). ZenZX draws a
             thin grey line round it, outside its picture, while it is whole.
  flat.png   What is left of it once the boot has come down. Optional: without it, icon.png is
             drawn squashed to a few pixels high. The same width as icon.png, or less.
  boot.png   The boot, in the position it lands in: its sole is the bottom row, and the first
             row that has anything in it is where the leg joins it. The boot is centred on the
             machine; the leg comes down with its middle at the middle of that first row's
             pixels, so a boot need not be symmetrical. The foot ZenZX ships is 25 by 24 (a leg
             eight pixels wide at its top, a foot pointing right), magenta on black.
  leg.png    A piece of the leg, repeated down from the top of the window to the top of the
             boot. Its bottom row meets its top row, so that the repeat does not show. The one
             ZenZX ships is 8 by 8.

  bin.png    What the machine stands on, centred under it: the machine's bottom row is over the bin's
             second row, so its first two rows (the handle) are hidden. ZenZX's own is zenplay's rubbish bin, 16 by 16.
  ground-top.png
             The ground the bin stands on, drawn across the window: a tile whose first row
             with anything in it is the ground's surface, which is where the bin's bottom is
             (rows above it should be transparent). ZenZX's own is 16 by 16: a line of two
             pixels with dither under it, from zenplay's tiles.
  ground.png What is under the surface tile, repeated across and down to the bottom of the
             window. ZenZX's own is a 16 by 16 tile of bricks, from zenplay's tiles.

The pictures the drawn code makes in place of a missing one are ZenZX's own: a symmetrical boot
and leg, and the bin and ground above.
