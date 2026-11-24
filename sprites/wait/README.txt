The figure that waits in a ZenZX window with no machine in it, as if showing impatience. It
blinks, taps a foot three times, looks to its right, and blinks twice, over and over, with a long
rest between one gesture and the next. When it appears it first looks to its right, once, and rests.

  frame00.png, frame01.png, ...   The pictures. Use two digits, so that frame10 comes after
                                  frame09. Transparent where the figure is not.
  sequence.txt                    What it does with them: which picture, for how many seconds.
  waiting.zani                    The pictures as drawn, a ZCUT collection that zentools reads
                                  (24 by 32 pixels, ink on paper, as the Spectrum draws them).

The pictures are numbered from 0 in the order of their file names, and sequence.txt has a line
for each step: the picture's number and the seconds it is shown. The steps before the line "loop"
are played once, when the figure appears; those after it go round and round. "#" starts a comment.
The file in this folder says which of the pictures is which. Without a sequence.txt, or with one
that does not fit the pictures (a picture that is not there, a time that is not a number), the
pictures are shown in turn, a fifth of a second each.

To change the figure, put other frame*.png files in this folder, and a sequence.txt for them if
they are to do more than go in turn, and build ZenZX again. Any number of pictures from one up.
Each is drawn enlarged by a whole number, with no smoothing, so draw them at their smallest
size. They may differ in size; they are drawn centred and standing on the same line, the figure
about a third of the window's height. If one of the files is not a picture, none are used and
the window shows only its background.
