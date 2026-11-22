; fuji - clouds behind Mount Fuji, for the ZX Spectrum Next
;
; Hokusai's "Fine Wind, Clear Morning" taken apart: the clouds drift behind the mountain,
; the higher ones faster, the title stays where the print has it, and the day turns to
; dusk and back.
;
; What of the Next it uses:
;   Layer 2    the sky, 320x256, which fills the border too. It scrolls sideways with a
;              nine-bit offset and goes round. It is the .nex file's loading screen.
;   Copper     the scroll offset is changed at four lines of the frame, so the sky moves
;              in four bands at four speeds.
;   Tilemap    the mountain and the forest: 428 tiles of 16 colours, drawn over Layer 2,
;              the sky's place in them transparent. Mirrored tiles are stored once.
;   Sprites    the title cartouche, 2 by 7 sprites of 256 colours, over the border.
;   Palette    the sky's colours are nine bits each; they are moved a step at a time
;              towards dusk colours and back.
;
; Assemble with:  zenas assemble fuji.asm build/fuji.bin --next
; (from this folder: INCBIN paths are taken from the folder zenas is run in)

                ORG     0x8000

SKY_COLOURS     EQU     64              ; the sky uses fewer; the rest of the palette is left
TITLE_PATTERNS  EQU     14

start:          di
                ld      sp,stack
                nextreg 0x07,3          ; 28 MHz
                xor     a
                out     (0xFE),a

; The tilemap. Its map and tiles are in bank 5, loaded there from the .nex file, where
; the ULA's screen would be: the ULA is turned off.
                nextreg 0x68,0x80       ; ULA off
                nextreg 0x6E,0x00       ; the map is at 0x4000: 40x32 entries of two bytes
                nextreg 0x6F,0x0A       ; the tiles at 0x4A00
                nextreg 0x4C,0x0F       ; colour 15 of a tile is transparent
                nextreg 0x2F,0
                nextreg 0x30,0
                nextreg 0x31,0
                nextreg 0x6B,0x83       ; on, 40 columns, 512 tiles, over the ULA
                nextreg 0x43,0x30       ; the tilemap's palette
                nextreg 0x40,0
                ld      hl,mountain_palette
                ld      b,32
                call    palette_bytes

; Layer 2, 320x256.
                nextreg 0x12,9          ; in banks 9 to 13, where the loader put it
                nextreg 0x70,0x10       ; 320x256, 256 colours
                nextreg 0x1C,0x01       ; its clip window: all of it
                nextreg 0x18,0
                nextreg 0x18,159
                nextreg 0x18,0
                nextreg 0x18,255
                nextreg 0x69,0x80       ; on

; The sky's colours as day and as dusk, a byte for each of red, green and blue.
                ld      hl,sky_palette
                ld      ix,day
                ld      b,SKY_COLOURS
unpack:         ld      a,(hl)          ; RRRGGGBB
                rlca
                rlca
                rlca
                and     7
                ld      (ix+0),a        ; red
                ld      a,(hl)
                rrca
                rrca
                and     7
                ld      (ix+1),a        ; green
                ld      a,(hl)
                and     3
                add     a,a
                inc     hl
                or      (hl)            ; the ninth bit: blue's lowest
                inc     hl
                ld      (ix+2),a        ; blue
                inc     ix
                inc     ix
                inc     ix
                djnz    unpack
                ld      hl,day
                ld      de,now
                ld      bc,SKY_COLOURS*3
                ldir
                ld      hl,day
                ld      de,dusk
                ld      b,SKY_COLOURS
make_dusk:      ld      a,(hl)          ; red: up by three
                add     a,3
                cp      8
                jr      c,dusk_red
                ld      a,7
dusk_red:       ld      (de),a
                inc     hl
                inc     de
                ld      a,(hl)          ; green: down by one
                or      a
                jr      z,dusk_green
                dec     a
dusk_green:     ld      (de),a
                inc     hl
                inc     de
                ld      a,(hl)          ; blue: halved
                srl     a
                ld      (de),a
                inc     hl
                inc     de
                djnz    make_dusk
                call    sky_write

; The title: its patterns to the pattern memory, and fourteen sprites to show them.
                ld      bc,0x303B
                xor     a
                out     (c),a
                ld      hl,title
                ld      bc,0x005B
                ld      a,TITLE_PATTERNS
patterns:       otir                    ; B is 0: 256 bytes
                dec     a
                jr      nz,patterns
                ld      d,0             ; pattern
                ld      e,24            ; Y
                ld      bc,0x0057
title_row:      ld      a,16
                out     (c),a           ; X
                out     (c),e           ; Y
                xor     a
                out     (c),a
                ld      a,d
                or      0x80
                out     (c),a           ; visible, pattern
                inc     d
                ld      a,32
                out     (c),a
                out     (c),e
                xor     a
                out     (c),a
                ld      a,d
                or      0x80
                out     (c),a
                inc     d
                ld      a,e
                add     a,16
                ld      e,a
                ld      a,d
                cp      TITLE_PATTERNS
                jr      nz,title_row
                nextreg 0x15,0x0B       ; sprites on and over the border; sprites over the tilemap over Layer 2
                nextreg 0x64,32         ; line 0 for the copper is the first line of the frame

main:           call    wait_frame

; The four bands of sky, each moved by its own fraction of a pixel.
                ld      ix,bands
                ld      b,4
move_band:      ld      a,(ix+2)        ; the fraction
                add     a,(ix+3)        ; the speed, in 256ths of a pixel a frame
                ld      (ix+2),a
                jr      nc,band_done
                ld      l,(ix+0)
                ld      h,(ix+1)
                inc     hl
                ld      a,h
                dec     a
                jr      nz,band_keep    ; below 256
                ld      a,l
                cp      64              ; 256+64 = 320: round again
                jr      nz,band_keep
                ld      hl,0
band_keep:      ld      (ix+0),l
                ld      (ix+1),h
band_done:      ld      de,5
                add     ix,de
                djnz    move_band

; The copper's list: at each band's first line, its offset to Layer 2's scroll registers.
                nextreg 0x61,0
                nextreg 0x62,0xC0       ; from the start of the list at the top of each frame
                ld      ix,bands
                ld      b,4
list_band:      nextreg 0x60,0x80       ; WAIT for the band's line
                ld      a,(ix+4)
                nextreg 0x60,a
                nextreg 0x60,0x16       ; MOVE the low byte to NextREG 0x16
                ld      a,(ix+0)
                nextreg 0x60,a
                nextreg 0x60,0x71       ; MOVE bit 8 to NextREG 0x71
                ld      a,(ix+1)
                nextreg 0x60,a
                ld      de,5
                add     ix,de
                djnz    list_band
                nextreg 0x60,0xFF       ; WAIT for a line that never comes
                nextreg 0x60,0xFF

; Day and dusk: every eighth frame the colours go one step towards where they are going;
; when they are there, they rest, and then go back.
                ld      a,(tick)
                inc     a
                ld      (tick),a
                and     7
                jp      nz,main
                ld      hl,(rest)
                ld      a,h
                or      l
                jr      z,fade
                dec     hl
                ld      (rest),hl
                jp      main
fade:           ld      de,day
                ld      a,(towards)
                or      a
                jr      z,fade_to
                ld      de,dusk
fade_to:        ld      hl,now
                ld      bc,SKY_COLOURS*3
                ld      ixl,0           ; set when anything moved
fade_loop:      ld      a,(de)
                cp      (hl)
                jr      z,fade_next
                jr      c,fade_down
                inc     (hl)
                jr      fade_moved
fade_down:      dec     (hl)
fade_moved:     ld      ixl,1
fade_next:      inc     hl
                inc     de
                dec     bc
                ld      a,b
                or      c
                jr      nz,fade_loop
                ld      a,ixl
                or      a
                jr      nz,fade_show
                ld      hl,30           ; there: rest for 30 eighths of frames, about five seconds
                ld      (rest),hl
                ld      a,(towards)
                xor     1
                ld      (towards),a
                jp      main
fade_show:      call    sky_write
                jp      main

; sky_write: the colours in now to Layer 2's palette.
sky_write:      nextreg 0x43,0x10       ; Layer 2's palette
                nextreg 0x40,0
                ld      hl,now
                ld      b,SKY_COLOURS
sw_loop:        ld      a,(hl)          ; red
                rrca
                rrca
                rrca
                ld      c,a
                inc     hl
                ld      a,(hl)          ; green
                add     a,a
                add     a,a
                or      c
                ld      c,a
                inc     hl
                ld      a,(hl)          ; blue
                srl     a
                or      c
                nextreg 0x44,a          ; RRRGGGBB
                ld      a,(hl)
                and     1
                nextreg 0x44,a          ; and blue's lowest bit
                inc     hl
                djnz    sw_loop
                ret

; palette_bytes: B bytes at HL to NextREG 0x44, as zx wrote them.
palette_bytes:  ld      a,(hl)
                nextreg 0x44,a
                inc     hl
                djnz    palette_bytes
                ret

; wait_frame: until the display is on the last line of the frame.
wait_frame:     ld      bc,0x243B
                ld      a,0x1F
                out     (c),a
                inc     b
wf_leave:       in      a,(c)
                cp      255
                jr      z,wf_leave
wf_reach:       in      a,(c)
                cp      255
                jr      nz,wf_reach
                ret

; The bands: offset (two bytes), fraction, speed, first line.
bands:          DB      0, 0, 0, 160, 0
                DB      0, 0, 0, 104, 52
                DB      0, 0, 0, 64, 104
                DB      0, 0, 0, 32, 150

tick:           DB      0
towards:        DB      1               ; 1 towards dusk, 0 towards day
rest:           DW      40

mountain_palette: INCBIN "build/mountain.nxp"
sky_palette:    INCBIN  "build/sky.nxp", 0, SKY_COLOURS*2
title:          INCBIN  "build/title.spr"

day:            DS      SKY_COLOURS*3
dusk:           DS      SKY_COLOURS*3
now:            DS      SKY_COLOURS*3

                DS      128
stack:
