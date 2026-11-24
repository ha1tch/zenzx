; gallop - The Horse in Motion, for the ZX Spectrum Next
;
; Two riders cross Bierstadt's "Among the Sierra Nevada, California": a far one at the
; sprites' own size and a near one at twice it. Both are the eleven photographs of one
; stride that Eadweard Muybridge took in 1878.
;
; What of the Next it uses:
;   Layer 2    the painting, 256x192 in 256 colours. It is the .nex file's loading screen,
;              so the loader has put it and its palette in place before this code runs.
;   Sprites    each rider is 4 by 3 sprites of 16 colours; the near one is scaled by two.
;   DMA        the twelve patterns of the stride's next picture are sent to the sprite
;              pattern port in one transfer, every other frame, so the whole stride never
;              has to fit in the pattern memory.
;   Copper     a new colour for the border on each of its lines, above and below the
;              painting; the list is written again each frame to make the bands move.
;   Tilemap    the captions, in its text mode with the ROM's character set, drawn in the
;              border where the ULA cannot draw.
;
; Assemble with:  zenas assemble gallop.asm build/gallop.bin --next
; (from this folder: INCBIN paths are taken from the folder zenas is run in)

                ORG     0x8000

MAP             EQU     0x5B00          ; the tilemap: 40x32 bytes, in bank 5
TILES           EQU     0x6000          ; its characters: 8 bytes each
FRAME_BYTES     EQU     12*128          ; one picture of the stride: 12 patterns of 4 bits
FRAMES          EQU     11

start:          di
                ld      sp,stack
                nextreg 0x07,3          ; 28 MHz
                xor     a
                out     (0xFE),a        ; border 0: ULA palette entry 16

; The ULA's screen is under Layer 2 and is not used: clear it.
                ld      hl,0x4000
                ld      de,0x4001
                ld      bc,0x1AFF
                ld      (hl),0
                ldir

; The tilemap: clear the map, take the characters from the ROM, write the captions.
                ld      hl,MAP
                ld      de,MAP+1
                ld      bc,40*32-1
                ld      (hl),0
                ldir
                ld      hl,TILES
                ld      de,TILES+1
                ld      bc,255
                ld      (hl),0
                ldir
                ld      hl,0x3D00       ; the ROM's characters, from the space on
                ld      de,TILES+32*8
                ld      bc,96*8
                ldir
                ld      hl,text1
                ld      de,MAP+1*40+10
                call    print
                ld      hl,text2
                ld      de,MAP+2*40+8
                call    print
                ld      hl,text3
                ld      de,MAP+29*40+2
                call    print
                ld      hl,text4
                ld      de,MAP+30*40+9
                call    print
                nextreg 0x6E,0x1B       ; the map is at 0x5B00
                nextreg 0x6F,0x20       ; the characters at 0x6000
                nextreg 0x6C,0          ; every cell: palette entries 0 and 1
                nextreg 0x6B,0xA9       ; on, 40 columns, no attribute bytes, text mode, over the ULA

; Palettes. The tilemap's: 0 the transparent colour, 1 white.
                nextreg 0x43,0x30
                nextreg 0x40,0
                nextreg 0x41,0xE3
                nextreg 0x41,0xFF
; The sprites': sixteen colours of nine bits, as zx wrote them.
                nextreg 0x43,0x20
                nextreg 0x40,0
                ld      hl,horse_palette
                ld      b,32
pal_loop:       ld      a,(hl)
                nextreg 0x44,a
                inc     hl
                djnz    pal_loop
; The ULA's, left selected at the border's entry and not moving on after a write: the
; copper then needs one instruction a line.
                nextreg 0x43,0x80
                nextreg 0x40,16

                nextreg 0x12,9          ; Layer 2 is in banks 9 to 11, where the loader put it
                nextreg 0x70,0          ; 256x192
                nextreg 0x16,0
                nextreg 0x17,0
                nextreg 0x69,0x80       ; Layer 2 on
                nextreg 0x15,0x01       ; sprites on, in the picture only; sprites over Layer 2 over the ULA
                nextreg 0x64,32         ; line 0 for the copper is the first line of the top border

main:           call    wait_frame
                call    copper_write

; The stride: every other frame, the next picture's patterns by DMA.
                ld      bc,0x303B
                xor     a
                out     (c),a           ; pattern 0 and sprite 0 are the next to be written
                ld      a,(tick)
                inc     a
                ld      (tick),a
                and     1
                jr      nz,no_step
                ld      a,(frame)
                inc     a
                cp      FRAMES
                jr      c,frame_ok
                xor     a
frame_ok:       ld      (frame),a
                add     a,a
                ld      hl,frame_table
                add     hl,a
                ld      a,(hl)
                inc     hl
                ld      h,(hl)
                ld      l,a
                ld      (dma_source),hl
                ld      hl,dma_program
                ld      b,dma_end-dma_program
                ld      c,0x6B
                otir
no_step:

; The riders: the far one a pixel a frame, the near one three.
                ld      hl,(far_x)
                inc     hl
                ld      a,h
                and     1
                ld      h,a
                ld      (far_x),hl
                ld      d,32+92
                ld      e,0
                ld      a,16
                call    rider
                ld      hl,(near_x)
                inc     hl
                inc     hl
                inc     hl
                ld      a,h
                and     1
                ld      h,a
                ld      (near_x),hl
                ld      d,32+96
                ld      e,0x0A          ; twice the size, both ways
                ld      a,32
                call    rider
                jp      main

; wait_frame: until the display is on the last line of the bottom border.
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

; print: the string at HL, ended by 0, to the map at DE.
print:          ld      a,(hl)
                or      a
                ret     z
                ld      (de),a
                inc     hl
                inc     de
                jr      print

; rider: twelve sprites, 4 by 3, with the patterns 0 to 11.
;   HL = X of the left edge (0-511), D = Y of the top, E = the scale bits of attribute 4,
;   A = pixels from one sprite to the next (16, or 32 at twice the size)
rider:          ld      (r_step),a
                ld      (r_x),hl
                ld      a,d
                ld      (r_y),a
                xor     a
                ld      (r_n),a
                ld      b,3
r_row:          push    bc
                ld      hl,(r_x)
                ld      b,4
r_col:          push    bc
                ld      bc,0x0057
                out     (c),l           ; X, low byte
                ld      a,(r_y)
                out     (c),a           ; Y
                ld      a,h
                and     1
                out     (c),a           ; X bit 8; no mirror, no rotation
                ld      a,(r_n)
                srl     a
                or      0xC0
                out     (c),a           ; visible, a fifth byte follows, pattern
                ld      a,(r_n)
                rrca
                rrca
                and     0x40
                or      0x80
                or      e
                out     (c),a           ; 4-bit pattern, which half of it, scale
                ld      a,(r_n)
                inc     a
                ld      (r_n),a
                ld      a,(r_step)
                add     hl,a
                ld      a,h
                and     1
                ld      h,a
                pop     bc
                djnz    r_col
                ld      a,(r_step)
                ld      b,a
                ld      a,(r_y)
                add     a,b
                ld      (r_y),a
                pop     bc
                djnz    r_row
                ret

; copper_write: the copper's list, from its start. For each line of the top border and of
; the bottom one, wait for the line and write its colour to the border's palette entry.
copper_write:   nextreg 0x61,0
                nextreg 0x62,0xC0       ; from the start of the list at the top of each frame
                ld      a,(tick)
                srl     a
                ld      d,a             ; the bands move a line every other frame
                ld      c,0
cw_top:         ld      a,0x80
                nextreg 0x60,a          ; WAIT for line C
                ld      a,c
                nextreg 0x60,a
                ld      a,0x41
                nextreg 0x60,a          ; MOVE to NextREG 0x41
                ld      a,c
                add     a,d
                call    ramp_colour
                nextreg 0x60,a
                inc     c
                ld      a,c
                cp      32
                jr      nz,cw_top
                nextreg 0x60,0x80       ; beside the painting: black
                nextreg 0x60,32
                nextreg 0x60,0x41
                nextreg 0x60,0x00
                ld      c,224
cw_bottom:      ld      a,0x80
                nextreg 0x60,a
                ld      a,c
                nextreg 0x60,a
                ld      a,0x41
                nextreg 0x60,a
                ld      a,c
                cpl                     ; the bottom mirrors the top
                add     a,d
                call    ramp_colour
                nextreg 0x60,a
                inc     c
                jr      nz,cw_bottom
                nextreg 0x60,0xFF       ; WAIT for a line that never comes
                nextreg 0x60,0xFF
                ret

ramp_colour:    and     63
                ld      hl,ramp
                add     hl,a
                ld      a,(hl)
                ret

; The DMA's program: FRAME_BYTES bytes from memory, going up, to port 0x5B.
dma_program:    DB      0x83            ; stop
                DB      0x7D            ; A to B; A's address and the length follow
dma_source:     DW      horse
                DW      FRAME_BYTES
                DB      0x14            ; A is memory, counting up
                DB      0x28            ; B is a port, fixed
                DB      0xAD            ; continuous; B's address follows
                DW      0x005B
                DB      0x82            ; stop at the end
                DB      0xCF            ; load
                DB      0x87            ; go
dma_end:

frame_table:    DW      horse+0*FRAME_BYTES, horse+1*FRAME_BYTES, horse+2*FRAME_BYTES
                DW      horse+3*FRAME_BYTES, horse+4*FRAME_BYTES, horse+5*FRAME_BYTES
                DW      horse+6*FRAME_BYTES, horse+7*FRAME_BYTES, horse+8*FRAME_BYTES
                DW      horse+9*FRAME_BYTES, horse+10*FRAME_BYTES

; A ramp of 64 colours that ends where it starts: from dark brown to orange and back.
ramp:           DB      0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x24, 0x24
                DB      0x44, 0x44, 0x44, 0x44, 0x44, 0x44, 0x44, 0x64
                DB      0x64, 0x64, 0x64, 0x64, 0x64, 0x88, 0x88, 0x88
                DB      0x88, 0x88, 0x88, 0x88, 0x88, 0xA8, 0xA8, 0xA8
                DB      0xA8, 0xA8, 0xA8, 0xA8, 0x88, 0x88, 0x88, 0x88
                DB      0x88, 0x88, 0x88, 0x88, 0x64, 0x64, 0x64, 0x64
                DB      0x64, 0x64, 0x44, 0x44, 0x44, 0x44, 0x44, 0x44
                DB      0x44, 0x24, 0x24, 0x20, 0x20, 0x20, 0x20, 0x20

text1:          DB      "THE HORSE IN MOTION", 0
text2:          DB      "EADWEARD MUYBRIDGE, 1878", 0
text3:          DB      "AMONG THE SIERRA NEVADA, CALIFORNIA", 0
text4:          DB      "ALBERT BIERSTADT, 1868", 0

tick:           DB      0
frame:          DB      0
far_x:          DW      200
near_x:         DW      400
r_x:            DW      0
r_y:            DB      0
r_n:            DB      0
r_step:         DB      0

horse_palette:  INCBIN  "build/horse.nxp"
horse:          INCBIN  "build/horse.spr"

                DS      128
stack:
