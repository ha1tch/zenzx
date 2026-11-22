; ============================================================================
;  assets.asm -- tiles and sprite frames for the connected-world example
;
;  Tiles: 14 x 16x16, 1bpp, 32 bytes each (2 bytes per row, 16 rows).
;  Sprites: Render_Sprite_RT format -- DB width(=2), height, then per row
;           gfx_hi gfx_lo mask_hi mask_lo. mask bit 1 = keep background
;           (transparent), 0 = draw; the source mask (solid where 1) is
;           inverted here. The character has right- and left-facing walk
;           frames; the ball, scissors and pot are single frames. The ball
;           is centred in its frame so it reads at the right place as it
;           bounces. Scissors is the brickything tile; the pot is drawn for
;           this example.
;
;  Copyright (c) 2026 haitch
;  SPDX-License-Identifier: Apache-2.0
; ============================================================================

; ---- tiles: 14 x 32 bytes, indexed 0..13 --------------------------------
TileData:
; tile 0
                DB      $00,$00, $00,$00, $00,$00, $00,$00
                DB      $00,$00, $00,$00, $00,$00, $00,$00
                DB      $00,$00, $00,$00, $00,$00, $00,$00
                DB      $00,$00, $00,$00, $00,$00, $00,$00
; tile 1
                DB      $FF,$FF, $10,$20, $10,$20, $10,$20
                DB      $FF,$FF, $02,$04, $02,$04, $02,$04
                DB      $FF,$FF, $10,$20, $10,$20, $10,$20
                DB      $FF,$FF, $02,$04, $02,$04, $02,$04
; tile 2
                DB      $00,$00, $60,$30, $00,$00, $03,$00
                DB      $00,$06, $30,$00, $00,$60, $00,$00
                DB      $00,$00, $0C,$18, $00,$00, $00,$00
                DB      $01,$80, $30,$06, $00,$00, $03,$00
; tile 3
                DB      $00,$00, $60,$30, $00,$00, $03,$00
                DB      $00,$06, $10,$00, $00,$60, $00,$00
                DB      $01,$00, $00,$18, $00,$00, $00,$00
                DB      $00,$20, $00,$06, $00,$00, $00,$00
; tile 4
                DB      $00,$00, $0C,$06, $00,$00, $00,$C0
                DB      $60,$00, $00,$08, $06,$00, $00,$00
                DB      $00,$80, $18,$00, $00,$00, $00,$00
                DB      $04,$00, $60,$00, $00,$00, $00,$00
; tile 5
                DB      $00,$00, $00,$00, $60,$00, $04,$00
                DB      $00,$00, $00,$00, $18,$00, $00,$80
                DB      $00,$00, $06,$00, $00,$08, $60,$00
                DB      $00,$C0, $00,$00, $0C,$06, $00,$00
; tile 6
                DB      $00,$00, $00,$00, $00,$06, $00,$20
                DB      $00,$00, $00,$00, $00,$18, $01,$00
                DB      $00,$00, $00,$60, $10,$00, $00,$06
                DB      $03,$00, $00,$00, $60,$30, $00,$00
; tile 7
                DB      $00,$00, $00,$00, $00,$00, $00,$00
                DB      $00,$00, $00,$00, $00,$00, $00,$00
                DB      $00,$00, $00,$00, $00,$00, $00,$00
                DB      $00,$00, $00,$00, $00,$00, $00,$00
; tile 8
                DB      $00,$00, $00,$00, $00,$00, $00,$00
                DB      $00,$00, $00,$00, $00,$00, $00,$00
                DB      $00,$00, $00,$00, $00,$00, $00,$00
                DB      $00,$00, $00,$00, $00,$00, $00,$00
; tile 9
                DB      $00,$00, $02,$00, $04,$00, $02,$00
                DB      $04,$00, $00,$00, $7F,$F0, $55,$5C
                DB      $7F,$F2, $7F,$F2, $7F,$F2, $7F,$F6
                DB      $3F,$FC, $3F,$F0, $1F,$E0, $00,$00
; tile 10
                DB      $00,$00, $03,$C0, $0E,$F0, $1E,$F8
                DB      $3C,$7C, $3B,$BC, $7B,$FE, $78,$7E
                DB      $7C,$3E, $7F,$BE, $3B,$BC, $3C,$7C
                DB      $1E,$F8, $0E,$F0, $03,$C0, $00,$00
; tile 11
                DB      $00,$00, $00,$00, $00,$01, $00,$02
                DB      $00,$04, $60,$08, $90,$10, $9F,$A0
                DB      $60,$5F, $00,$80, $01,$00, $0F,$00
                DB      $10,$80, $10,$80, $11,$00, $0E,$00
; tile 12
                DB      $00,$00, $00,$00, $00,$00, $00,$00
                DB      $00,$00, $00,$00, $00,$00, $00,$00
                DB      $00,$00, $00,$00, $00,$00, $00,$00
                DB      $00,$00, $00,$00, $00,$00, $00,$00
; tile 13
                DB      $00,$00, $00,$00, $00,$00, $00,$00
                DB      $00,$00, $00,$00, $00,$00, $00,$00
                DB      $00,$00, $00,$00, $00,$00, $00,$00
                DB      $00,$00, $00,$00, $00,$00, $00,$00

TILE_BYTES:     EQU     32

; ---- player: 7 right-facing walk frames ---------------------------------
PlayerFramesR:
                DW      PlayerFrameR0
                DW      PlayerFrameR1
                DW      PlayerFrameR2
                DW      PlayerFrameR3
                DW      PlayerFrameR4
                DW      PlayerFrameR5
                DW      PlayerFrameR6
PLAYER_FRAMES:  EQU     7

PlayerFrameR0:
                DB      2, 16
                DB      $00,$00, $FF,$FF
                DB      $01,$C0, $FE,$3F
                DB      $03,$30, $FC,$CF
                DB      $02,$08, $FD,$F7
                DB      $02,$CC, $FD,$33
                DB      $03,$46, $FC,$B9
                DB      $03,$62, $FC,$9D
                DB      $01,$72, $FE,$8D
                DB      $01,$4E, $FE,$B1
                DB      $01,$C0, $FE,$3F
                DB      $01,$E0, $FE,$1F
                DB      $03,$30, $FC,$CF
                DB      $1E,$10, $E1,$EF
                DB      $30,$10, $CF,$EF
                DB      $20,$30, $DF,$CF
                DB      $00,$38, $FF,$C7
PlayerFrameR1:
                DB      2, 16
                DB      $00,$00, $FF,$FF
                DB      $01,$C0, $FE,$3F
                DB      $03,$30, $FC,$CF
                DB      $02,$08, $FD,$F7
                DB      $02,$CC, $FD,$33
                DB      $03,$46, $FC,$B9
                DB      $03,$62, $FC,$9D
                DB      $01,$72, $FE,$8D
                DB      $01,$4E, $FE,$B1
                DB      $01,$C0, $FE,$3F
                DB      $01,$C0, $FE,$3F
                DB      $01,$40, $FE,$BF
                DB      $0D,$40, $F2,$BF
                DB      $0B,$40, $F4,$BF
                DB      $08,$40, $F7,$BF
                DB      $00,$60, $FF,$9F
PlayerFrameR2:
                DB      2, 16
                DB      $00,$00, $FF,$FF
                DB      $01,$C0, $FE,$3F
                DB      $03,$30, $FC,$CF
                DB      $02,$08, $FD,$F7
                DB      $02,$CC, $FD,$33
                DB      $03,$46, $FC,$B9
                DB      $03,$62, $FC,$9D
                DB      $01,$72, $FE,$8D
                DB      $01,$4E, $FE,$B1
                DB      $01,$C0, $FE,$3F
                DB      $01,$C0, $FE,$3F
                DB      $01,$C0, $FE,$3F
                DB      $00,$C0, $FF,$3F
                DB      $07,$C0, $F8,$3F
                DB      $06,$00, $F9,$FF
                DB      $00,$C0, $FF,$3F
PlayerFrameR3:
                DB      2, 16
                DB      $00,$00, $FF,$FF
                DB      $01,$C0, $FE,$3F
                DB      $03,$30, $FC,$CF
                DB      $02,$08, $FD,$F7
                DB      $02,$CC, $FD,$33
                DB      $03,$46, $FC,$B9
                DB      $03,$62, $FC,$9D
                DB      $01,$72, $FE,$8D
                DB      $01,$4E, $FE,$B1
                DB      $01,$C0, $FE,$3F
                DB      $01,$E0, $FE,$1F
                DB      $01,$B0, $FE,$4F
                DB      $00,$60, $FF,$9F
                DB      $00,$80, $FF,$7F
                DB      $02,$E0, $FD,$1F
                DB      $01,$00, $FE,$FF
PlayerFrameR4:
                DB      2, 16
                DB      $00,$00, $FF,$FF
                DB      $01,$C0, $FE,$3F
                DB      $03,$30, $FC,$CF
                DB      $02,$08, $FD,$F7
                DB      $02,$CC, $FD,$33
                DB      $03,$46, $FC,$B9
                DB      $03,$62, $FC,$9D
                DB      $01,$72, $FE,$8D
                DB      $01,$4E, $FE,$B1
                DB      $01,$C0, $FE,$3F
                DB      $01,$E0, $FE,$1F
                DB      $01,$B0, $FE,$4F
                DB      $00,$88, $FF,$77
                DB      $07,$B0, $F8,$4F
                DB      $04,$38, $FB,$C7
                DB      $02,$00, $FD,$FF
PlayerFrameR5:
                DB      2, 16
                DB      $00,$00, $FF,$FF
                DB      $01,$C0, $FE,$3F
                DB      $03,$30, $FC,$CF
                DB      $02,$08, $FD,$F7
                DB      $02,$CC, $FD,$33
                DB      $03,$46, $FC,$B9
                DB      $03,$62, $FC,$9D
                DB      $01,$72, $FE,$8D
                DB      $01,$4E, $FE,$B1
                DB      $01,$C0, $FE,$3F
                DB      $01,$F8, $FE,$07
                DB      $01,$88, $FE,$77
                DB      $0D,$98, $F2,$67
                DB      $1E,$1C, $E1,$E3
                DB      $14,$00, $EB,$FF
                DB      $00,$00, $FF,$FF
PlayerFrameR6:
                DB      2, 16
                DB      $00,$00, $FF,$FF
                DB      $01,$C0, $FE,$3F
                DB      $02,$20, $FD,$DF
                DB      $02,$D8, $FD,$27
                DB      $03,$44, $FC,$BB
                DB      $03,$62, $FC,$9D
                DB      $01,$72, $FE,$8D
                DB      $01,$4E, $FE,$B1
                DB      $01,$C0, $FE,$3F
                DB      $01,$E0, $FE,$1F
                DB      $0D,$98, $F2,$67
                DB      $1F,$0D, $E0,$F2
                DB      $12,$06, $ED,$F9
                DB      $00,$04, $FF,$FB
                DB      $00,$00, $FF,$FF
                DB      $00,$00, $FF,$FF

; ---- player: 7 left-facing walk frames (mirrored) -----------------------
PlayerFramesL:
                DW      PlayerFrameL0
                DW      PlayerFrameL1
                DW      PlayerFrameL2
                DW      PlayerFrameL3
                DW      PlayerFrameL4
                DW      PlayerFrameL5
                DW      PlayerFrameL6

PlayerFrameL0:
                DB      2, 16
                DB      $00,$00, $FF,$FF
                DB      $03,$80, $FC,$7F
                DB      $0C,$C0, $F3,$3F
                DB      $10,$40, $EF,$BF
                DB      $33,$40, $CC,$BF
                DB      $62,$C0, $9D,$3F
                DB      $46,$C0, $B9,$3F
                DB      $4E,$80, $B1,$7F
                DB      $72,$80, $8D,$7F
                DB      $03,$80, $FC,$7F
                DB      $07,$80, $F8,$7F
                DB      $0C,$C0, $F3,$3F
                DB      $08,$78, $F7,$87
                DB      $08,$0C, $F7,$F3
                DB      $0C,$04, $F3,$FB
                DB      $1C,$00, $E3,$FF
PlayerFrameL1:
                DB      2, 16
                DB      $00,$00, $FF,$FF
                DB      $03,$80, $FC,$7F
                DB      $0C,$C0, $F3,$3F
                DB      $10,$40, $EF,$BF
                DB      $33,$40, $CC,$BF
                DB      $62,$C0, $9D,$3F
                DB      $46,$C0, $B9,$3F
                DB      $4E,$80, $B1,$7F
                DB      $72,$80, $8D,$7F
                DB      $03,$80, $FC,$7F
                DB      $03,$80, $FC,$7F
                DB      $02,$80, $FD,$7F
                DB      $02,$B0, $FD,$4F
                DB      $02,$D0, $FD,$2F
                DB      $02,$10, $FD,$EF
                DB      $06,$00, $F9,$FF
PlayerFrameL2:
                DB      2, 16
                DB      $00,$00, $FF,$FF
                DB      $03,$80, $FC,$7F
                DB      $0C,$C0, $F3,$3F
                DB      $10,$40, $EF,$BF
                DB      $33,$40, $CC,$BF
                DB      $62,$C0, $9D,$3F
                DB      $46,$C0, $B9,$3F
                DB      $4E,$80, $B1,$7F
                DB      $72,$80, $8D,$7F
                DB      $03,$80, $FC,$7F
                DB      $03,$80, $FC,$7F
                DB      $03,$80, $FC,$7F
                DB      $03,$00, $FC,$FF
                DB      $03,$E0, $FC,$1F
                DB      $00,$60, $FF,$9F
                DB      $03,$00, $FC,$FF
PlayerFrameL3:
                DB      2, 16
                DB      $00,$00, $FF,$FF
                DB      $03,$80, $FC,$7F
                DB      $0C,$C0, $F3,$3F
                DB      $10,$40, $EF,$BF
                DB      $33,$40, $CC,$BF
                DB      $62,$C0, $9D,$3F
                DB      $46,$C0, $B9,$3F
                DB      $4E,$80, $B1,$7F
                DB      $72,$80, $8D,$7F
                DB      $03,$80, $FC,$7F
                DB      $07,$80, $F8,$7F
                DB      $0D,$80, $F2,$7F
                DB      $06,$00, $F9,$FF
                DB      $01,$00, $FE,$FF
                DB      $07,$40, $F8,$BF
                DB      $00,$80, $FF,$7F
PlayerFrameL4:
                DB      2, 16
                DB      $00,$00, $FF,$FF
                DB      $03,$80, $FC,$7F
                DB      $0C,$C0, $F3,$3F
                DB      $10,$40, $EF,$BF
                DB      $33,$40, $CC,$BF
                DB      $62,$C0, $9D,$3F
                DB      $46,$C0, $B9,$3F
                DB      $4E,$80, $B1,$7F
                DB      $72,$80, $8D,$7F
                DB      $03,$80, $FC,$7F
                DB      $07,$80, $F8,$7F
                DB      $0D,$80, $F2,$7F
                DB      $11,$00, $EE,$FF
                DB      $0D,$E0, $F2,$1F
                DB      $1C,$20, $E3,$DF
                DB      $00,$40, $FF,$BF
PlayerFrameL5:
                DB      2, 16
                DB      $00,$00, $FF,$FF
                DB      $03,$80, $FC,$7F
                DB      $0C,$C0, $F3,$3F
                DB      $10,$40, $EF,$BF
                DB      $33,$40, $CC,$BF
                DB      $62,$C0, $9D,$3F
                DB      $46,$C0, $B9,$3F
                DB      $4E,$80, $B1,$7F
                DB      $72,$80, $8D,$7F
                DB      $03,$80, $FC,$7F
                DB      $1F,$80, $E0,$7F
                DB      $11,$80, $EE,$7F
                DB      $19,$B0, $E6,$4F
                DB      $38,$78, $C7,$87
                DB      $00,$28, $FF,$D7
                DB      $00,$00, $FF,$FF
PlayerFrameL6:
                DB      2, 16
                DB      $00,$00, $FF,$FF
                DB      $03,$80, $FC,$7F
                DB      $04,$40, $FB,$BF
                DB      $1B,$40, $E4,$BF
                DB      $22,$C0, $DD,$3F
                DB      $46,$C0, $B9,$3F
                DB      $4E,$80, $B1,$7F
                DB      $72,$80, $8D,$7F
                DB      $03,$80, $FC,$7F
                DB      $07,$80, $F8,$7F
                DB      $19,$B0, $E6,$4F
                DB      $B0,$F8, $4F,$07
                DB      $60,$48, $9F,$B7
                DB      $20,$00, $DF,$FF
                DB      $00,$00, $FF,$FF
                DB      $00,$00, $FF,$FF

; ---- movers: ball (centred), scissors, pot ------------------------------
BallSprite:
                DB      2, 16
                DB      $00,$00, $FF,$FF
                DB      $00,$00, $FF,$FF
                DB      $00,$00, $FF,$FF
                DB      $00,$00, $FF,$FF
                DB      $00,$00, $FF,$FF
                DB      $03,$80, $FC,$7F
                DB      $07,$C0, $F8,$3F
                DB      $07,$C0, $F8,$3F
                DB      $07,$C0, $F8,$3F
                DB      $03,$80, $FC,$7F
                DB      $00,$00, $FF,$FF
                DB      $00,$00, $FF,$FF
                DB      $00,$00, $FF,$FF
                DB      $00,$00, $FF,$FF
                DB      $00,$00, $FF,$FF
                DB      $00,$00, $FF,$FF

ScissorsSprite:
                DB      2, 16
                DB      $00,$00, $FF,$FF
                DB      $00,$00, $FF,$FF
                DB      $00,$01, $FF,$FE
                DB      $00,$02, $FF,$FD
                DB      $00,$04, $FF,$FB
                DB      $60,$08, $9F,$F7
                DB      $90,$10, $6F,$EF
                DB      $9F,$A0, $60,$5F
                DB      $60,$5F, $9F,$A0
                DB      $00,$80, $FF,$7F
                DB      $01,$00, $FE,$FF
                DB      $0F,$00, $F0,$FF
                DB      $10,$80, $EF,$7F
                DB      $10,$80, $EF,$7F
                DB      $11,$00, $EE,$FF
                DB      $0E,$00, $F1,$FF

PotSprite:
                DB      2, 16
                DB      $00,$00, $FF,$FF
                DB      $04,$90, $FB,$6F
                DB      $0A,$A8, $F5,$57
                DB      $0A,$A8, $F5,$57
                DB      $05,$D0, $FA,$2F
                DB      $02,$A0, $FD,$5F
                DB      $01,$C0, $FE,$3F
                DB      $00,$80, $FF,$7F
                DB      $0F,$F0, $F0,$0F
                DB      $08,$10, $F7,$EF
                DB      $04,$20, $FB,$DF
                DB      $04,$20, $FB,$DF
                DB      $02,$40, $FD,$BF
                DB      $02,$40, $FD,$BF
                DB      $01,$80, $FE,$7F
                DB      $00,$00, $FF,$FF

