; orbit - the Moon round the Earth, for the ZX Spectrum Next
;
; The Earth as Apollo 17 photographed it, the Moon going round it - behind it, small, and
; in front of it, large - with Phobos, borrowed from Mars, tumbling round the Moon. The
; stars are behind them all.
;
; What of the Next it uses:
;   Layer 2    the Earth, 256x192; the space round it is the transparent colour, so the
;              layer below shows through. It is the .nex file's loading screen.
;   ULA        the stars, plotted with the Z80N's PIXELAD and SETAE, and the caption. They
;              twinkle: the ULA's palette entries for the bright colours are rewritten.
;   Sprites    the Moon is 2 by 2 sprites, drawn at twice the size when it is in front;
;              Phobos is one, turned by its mirror and rotate bits.
;   Layers     the order is changed as the Moon goes round: sprites above Layer 2 when it
;              is in front of the Earth, Layer 2 above the sprites when it is behind.
;   Interrupts the Next's own mode 2: each source has its own vector. The frame's
;              interrupt draws; a CTC timer, 427 times a second, moves the Moon on.
;
; Assemble with:  zenas assemble orbit.asm build/orbit.bin --next
; (from this folder: INCBIN paths are taken from the folder zenas is run in)

                ORG     0x8000

VECTORS         EQU     0xBE00
STARS           EQU     160

start:          di
                ld      sp,stack
                nextreg 0x07,3          ; 28 MHz
                xor     a
                out     (0xFE),a

; The ULA's screen: black, the stars, the caption.
                ld      hl,0x4000
                ld      de,0x4001
                ld      bc,0x17FF
                ld      (hl),0
                ldir
                ld      hl,0x5800
                ld      de,0x5801
                ld      bc,0x02FF
                ld      (hl),0x07
                ldir
                ld      b,STARS
star:           push    bc
star_y:         call    random
                cp      176             ; not on the caption's lines
                jr      nc,star_y
                ld      d,a
                call    random
                ld      e,a
                pixelad                 ; HL = the address of the pixel at D (Y), E (X)
                setae                   ; A = its bit
                or      (hl)
                ld      (hl),a
                push    de
                call    random
                and     7
                or      0x40            ; a bright ink on black
                ld      c,a
                pop     de
                ld      a,d             ; the attribute cell: 0x5800 + (Y/8)*32 + X/8
                and     0xF8
                ld      l,a
                ld      h,0
                add     hl,hl
                add     hl,hl
                ld      a,e
                rrca
                rrca
                rrca
                and     0x1F
                add     hl,a
                ld      a,0x58
                add     a,h
                ld      h,a
                ld      (hl),c
                pop     bc
                djnz    star
                ld      hl,text1
                ld      de,0x50C0+2     ; line 22, column 2
                call    print
                ld      hl,text2
                ld      de,0x50E0+3     ; line 23, column 3
                call    print
                call    twinkle

; Layer 2: the Earth, where the loader put it.
                nextreg 0x12,9
                nextreg 0x70,0
                nextreg 0x16,0
                nextreg 0x17,0
                nextreg 0x14,0xE3       ; the transparent colour
                nextreg 0x69,0x80

; The sprites: their palette and their six patterns.
                nextreg 0x43,0x20
                nextreg 0x40,0
                ld      hl,bodies_palette
                ld      bc,512
sprite_pal:     ld      a,(hl)
                nextreg 0x44,a
                inc     hl
                dec     bc
                ld      a,b
                or      c
                jr      nz,sprite_pal
                ld      bc,0x303B
                xor     a
                out     (c),a
                ld      hl,bodies
                ld      bc,0x005B
                ld      a,6
patterns:       otir                    ; B is 0: 256 bytes
                dec     a
                jr      nz,patterns

; Interrupts. Sixteen vectors, all to a handler that does nothing, then the two used:
; the CTC's first channel is source 3 and the frame is source 11, and a vector is twice
; its source's number.
                ld      hl,VECTORS
                ld      de,idle
                ld      b,16
vector:         ld      (hl),e
                inc     hl
                ld      (hl),d
                inc     hl
                djnz    vector
                ld      hl,on_timer
                ld      (VECTORS+3*2),hl
                ld      hl,on_frame
                ld      (VECTORS+11*2),hl
                ld      a,VECTORS/256
                ld      i,a
                im      2
                nextreg 0xC0,0x01       ; the Next's own mode 2; vectors from 0
                nextreg 0xC4,0x01       ; the frame's interrupt on, the line's off
                nextreg 0xC5,0x01       ; the CTC's first channel on
                nextreg 0xC8,0xFF       ; nothing waiting
                nextreg 0xC9,0xFF
                ld      bc,0x183B       ; CTC channel 0: a timer, 28 MHz / 256 / 256
                ld      a,0xA5
                out     (c),a
                xor     a
                out     (c),a
                ei

; The main program has only the stars to see to: every eighth frame, new colours.
main:           halt
                ld      a,(frames)
                ld      hl,seen
                cp      (hl)
                jr      z,main
                ld      (hl),a
                and     7
                jr      nz,main
                call    twinkle
                jr      main

idle:           ei
                reti

; on_timer: 427 times a second. The Moon's place in its orbit, and Phobos's the other way.
on_timer:       push    hl
                push    de
                ld      hl,(moon_at)
                ld      de,20
                add     hl,de
                ld      (moon_at),hl
                ld      hl,(phobos_at)
                ld      de,-70
                add     hl,de
                ld      (phobos_at),hl
                pop     de
                pop     hl
                ei
                reti

; on_frame: once a frame, while the border is being drawn. Places the sprites.
on_frame:       push    af
                push    bc
                push    de
                push    hl
                ld      hl,frames
                inc     (hl)

; The Moon's centre: 100 pixels either side of the Earth's, and a little up and down.
                ld      a,(moon_at+1)
                call    sine
                ld      hl,32+128
                call    add_signed
                ld      (moon_x),hl
                ld      a,(moon_at+1)
                add     a,64
                call    sine            ; the cosine: positive in front, negative behind
                ld      (depth),a
                sra     a
                sra     a
                sra     a
                add     a,32+96
                ld      (moon_y),a

; In front: twice the size, and the sprites above Layer 2. Behind: Layer 2 above them.
                ld      a,(depth)
                bit     7,a
                jr      nz,behind
                nextreg 0x15,0x01       ; sprites, Layer 2, ULA
                ld      a,32            ; half the Moon's size
                ld      e,0x0A          ; scaled by two both ways
                jr      place
behind:         nextreg 0x15,0x05       ; Layer 2, sprites, ULA
                ld      a,16
                ld      e,0
place:          ld      (half),a
                ld      bc,0x303B
                xor     a
                out     (c),a           ; from sprite 0
                ld      d,0             ; pattern 0: top left
                call    moon_part
                ld      d,1             ; top right
                call    moon_part
                ld      d,2             ; bottom left
                call    moon_part
                ld      d,3             ; bottom right
                call    moon_part

; Phobos: round the Moon, turning over.
                ld      a,(phobos_at+1)
                call    sine
                sra     a
                ld      hl,(moon_x)
                call    add_signed
                ld      de,-8
                add     hl,de
                ld      a,h
                and     1
                ld      h,a
                push    hl
                ld      a,(phobos_at+1)
                add     a,64
                call    sine
                sra     a
                sra     a
                sra     a
                ld      hl,moon_y
                add     a,(hl)
                sub     8
                pop     hl
                ld      bc,0x0057
                out     (c),l           ; X
                out     (c),a           ; Y
                ld      a,(frames)
                rrca
                rrca
                and     0x0E            ; mirror X, mirror Y, rotate: a new turn every eighth frame
                or      h
                out     (c),a
                ld      a,0xC4          ; visible, pattern 4
                out     (c),a
                xor     a
                out     (c),a
                pop     hl
                pop     de
                pop     bc
                pop     af
                ei
                reti

; moon_part: one of the Moon's four sprites. D = its pattern, 0 to 3; E = the scale bits.
moon_part:      ld      hl,(moon_x)
                bit     0,d
                jr      nz,mp_x         ; the right half starts at the centre
                ld      a,(half)
                neg
                call    add_signed      ; the left half half the size before it
mp_x:           ld      a,h
                and     1
                ld      h,a
                ld      a,(half)
                ld      c,a
                ld      a,(moon_y)
                bit     1,d
                jr      nz,mp_y         ; the bottom half starts at the centre
                sub     c
mp_y:           ld      bc,0x0057
                out     (c),l           ; X
                out     (c),a           ; Y
                out     (c),h           ; X bit 8
                ld      a,d
                or      0xC0
                out     (c),a           ; visible, a fifth byte follows, pattern
                out     (c),e           ; scale
                ret

; sine: A = 100 * sin(A * 2 pi / 256), signed.
sine:           ld      hl,sines
                add     hl,a
                ld      a,(hl)
                ret

; add_signed: HL = HL + A, A signed.
add_signed:     push    de
                ld      e,a
                add     a,a
                sbc     a,a
                ld      d,a
                add     hl,de
                pop     de
                ret

; random: A = the next of a sequence of 65535 values.
random:         ld      hl,(seed)
                ld      a,h
                rra
                ld      a,l
                rra
                xor     h
                ld      h,a
                ld      a,l
                rra
                ld      a,h
                rra
                xor     l
                ld      l,a
                xor     h
                ld      h,a
                ld      (seed),hl
                ret

; twinkle: the eight bright colours of the ULA's palette, each the next of a ring of eight.
twinkle:        ld      a,(ring_at)
                inc     a
                ld      (ring_at),a
                ld      c,a
                nextreg 0x43,0x00       ; the ULA's palette
                nextreg 0x40,8          ; from the first bright ink
                ld      b,8
tw_loop:        ld      a,c
                and     7
                ld      hl,ring
                add     hl,a
                ld      a,(hl)
                nextreg 0x41,a
                inc     c
                djnz    tw_loop
                ret

; print: the string at HL, ended by 0, to the screen from the address DE, with the ROM's
; characters.
print:          ld      a,(hl)
                or      a
                ret     z
                push    hl
                ld      l,a
                ld      h,0
                add     hl,hl
                add     hl,hl
                add     hl,hl
                ld      bc,0x3C00
                add     hl,bc
                push    de
                ld      b,8
pr_line:        ld      a,(hl)
                ld      (de),a
                inc     hl
                inc     d
                djnz    pr_line
                pop     de
                inc     e
                pop     hl
                inc     hl
                jr      print

; Star colours: white, pale blue, pale yellow, and dimmer ones.
ring:           DB      0xFF, 0xB6, 0xDB, 0x6D, 0xFE, 0x92, 0xBF, 0x49

text1:          DB      "EARTH: APOLLO 17, 1972. NASA", 0
text2:          DB      "MOON: LRO. PHOBOS: MRO. NASA", 0

seed:           DW      0x1D0B
ring_at:        DB      0
frames:         DB      0
seen:           DB      0
moon_at:        DW      0
phobos_at:      DW      0
moon_x:         DW      0
moon_y:         DB      0
depth:          DB      0
half:           DB      0

sines:
                DB      0x00, 0x02, 0x05, 0x07, 0x0A, 0x0C, 0x0F, 0x11, 0x14, 0x16, 0x18, 0x1B, 0x1D, 0x1F, 0x22, 0x24
                DB      0x26, 0x29, 0x2B, 0x2D, 0x2F, 0x31, 0x33, 0x35, 0x38, 0x3A, 0x3C, 0x3E, 0x3F, 0x41, 0x43, 0x45
                DB      0x47, 0x48, 0x4A, 0x4C, 0x4D, 0x4F, 0x50, 0x52, 0x53, 0x54, 0x56, 0x57, 0x58, 0x59, 0x5A, 0x5B
                DB      0x5C, 0x5D, 0x5E, 0x5F, 0x60, 0x60, 0x61, 0x62, 0x62, 0x63, 0x63, 0x63, 0x64, 0x64, 0x64, 0x64
                DB      0x64, 0x64, 0x64, 0x64, 0x64, 0x63, 0x63, 0x63, 0x62, 0x62, 0x61, 0x60, 0x60, 0x5F, 0x5E, 0x5D
                DB      0x5C, 0x5B, 0x5A, 0x59, 0x58, 0x57, 0x56, 0x54, 0x53, 0x52, 0x50, 0x4F, 0x4D, 0x4C, 0x4A, 0x48
                DB      0x47, 0x45, 0x43, 0x41, 0x3F, 0x3E, 0x3C, 0x3A, 0x38, 0x35, 0x33, 0x31, 0x2F, 0x2D, 0x2B, 0x29
                DB      0x26, 0x24, 0x22, 0x1F, 0x1D, 0x1B, 0x18, 0x16, 0x14, 0x11, 0x0F, 0x0C, 0x0A, 0x07, 0x05, 0x02
                DB      0x00, 0xFE, 0xFB, 0xF9, 0xF6, 0xF4, 0xF1, 0xEF, 0xEC, 0xEA, 0xE8, 0xE5, 0xE3, 0xE1, 0xDE, 0xDC
                DB      0xDA, 0xD7, 0xD5, 0xD3, 0xD1, 0xCF, 0xCD, 0xCB, 0xC8, 0xC6, 0xC4, 0xC2, 0xC1, 0xBF, 0xBD, 0xBB
                DB      0xB9, 0xB8, 0xB6, 0xB4, 0xB3, 0xB1, 0xB0, 0xAE, 0xAD, 0xAC, 0xAA, 0xA9, 0xA8, 0xA7, 0xA6, 0xA5
                DB      0xA4, 0xA3, 0xA2, 0xA1, 0xA0, 0xA0, 0x9F, 0x9E, 0x9E, 0x9D, 0x9D, 0x9D, 0x9C, 0x9C, 0x9C, 0x9C
                DB      0x9C, 0x9C, 0x9C, 0x9C, 0x9C, 0x9D, 0x9D, 0x9D, 0x9E, 0x9E, 0x9F, 0xA0, 0xA0, 0xA1, 0xA2, 0xA3
                DB      0xA4, 0xA5, 0xA6, 0xA7, 0xA8, 0xA9, 0xAA, 0xAC, 0xAD, 0xAE, 0xB0, 0xB1, 0xB3, 0xB4, 0xB6, 0xB8
                DB      0xB9, 0xBB, 0xBD, 0xBF, 0xC1, 0xC2, 0xC4, 0xC6, 0xC8, 0xCB, 0xCD, 0xCF, 0xD1, 0xD3, 0xD5, 0xD7
                DB      0xDA, 0xDC, 0xDE, 0xE1, 0xE3, 0xE5, 0xE8, 0xEA, 0xEC, 0xEF, 0xF1, 0xF4, 0xF6, 0xF9, 0xFB, 0xFE
bodies_palette: INCBIN  "build/bodies.nxp"
bodies:         INCBIN  "build/bodies.spr"

                DS      128
stack:
