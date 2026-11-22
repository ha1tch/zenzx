; ============================================================================
;  world.asm -- a connected world across several ZX Spectrums
;
;  Each machine draws one screen of a larger world: a 16x12 grid of 16x16
;  tiles that fills the display exactly. A player character walks around with
;  the keys; a ball drifts and bounces. When either leaves the edge of the
;  screen it is handed to the neighbouring machine over the link and appears
;  on the far side of that machine's screen. The machines do not know the
;  shape of the world: a host program on the other end of each link holds the
;  3x3 map and routes an exit on one machine to an entry on its neighbour.
;
;  The link is ZenZX's own: I/O port 16384 is the data byte, 16385 the
;  control/handshake. Reading a byte is the four-step handshake; writing one
;  is a bare OUT. Every machine uses the same two ports -- each is its own
;  Z80 address space -- and the host reaches each on a different TCP port.
;
;  Sprite drawing, screen addressing and save/restore-under are the verified
;  Oakhollow engine routines, included verbatim. The tile and sprite bitmaps
;  are in assets.asm.
;
;  Copyright (c) 2026 haitch
;  SPDX-License-Identifier: Apache-2.0
; ============================================================================

                .ORG    $8000

SCREEN:         EQU     $4000
ATTR:           EQU     $5800
DATA_PORT:      EQU     16384
CTRL_PORT:      EQU     16385

; Grid of tiles that fills the screen: 16 wide x 12 tall, each 16x16 px.
COLS:           EQU     16
ROWS:           EQU     12
CELL:           EQU     16

; Pixel limits for a 16x16 sprite's top-left corner.
MAXX:           EQU     256-CELL        ; 240
MAXY:           EQU     192-CELL        ; 176

; The wall borders have an opening two cells wide in the middle of each edge.
; An entity may only cross through the opening; against the solid wall it is
; blocked (the hero) or it bounces (a mover). The opening spans columns 7..8
; (x 112..143) on the top and bottom, and rows 5..6 (y 80..111) on the left
; and right. We test the 16px sprite's CENTRE (position + 8) against these.
OPEN_H_LO:      EQU     112             ; horizontal opening (top/bottom cross)
OPEN_H_HI:      EQU     143
OPEN_V_LO:      EQU     80              ; vertical opening (left/right cross)
OPEN_V_HI:      EQU     111

; The centre of a room, where the horizontal corridor (y = CORRIDOR_Y) and the
; vertical corridor (x = CORRIDOR_X) cross. A rook mover travels along one of
; these lines, and at the centre it picks a new direction -- any of the three
; that are not a reversal. The corridors run clear to all four openings, so
; from the centre it can head out any edge. A 16px sprite sits on the corridor
; with these top-left coordinates (centre falls in the opening band).
CORRIDOR_X:     EQU     120             ; vertical corridor (x), centre col 7..8
CORRIDOR_Y:     EQU     88              ; horizontal corridor (y), centre row 5..6

; Link message bytes. An "enter" packet is four bytes:
;   SYNC, kind, edge, coord
; kind: 0 = player, 1 = ball. edge: the edge it comes IN through on this
; machine (0 left, 1 right, 2 top, 3 bottom). coord: the perpendicular
; pixel position (Y for left/right, X for top/bottom). For the ball a fifth
; and sixth byte carry its signed velocity; the player has none.
SYNC:           EQU     $AA
; Entity kinds. The player (0) is keyboard-driven; the movers (1..3) drift and
; bounce on their own and carry a velocity. A mover's kind doubles as its
; slot index in the mover table, so K_BALL=1 is mover 0, etc. (kind-1 = slot).
K_PLAYER:       EQU     0
K_BALL:         EQU     1
K_SCISSORS:     EQU     2
K_POT:          EQU     3
NMOVERS:        EQU     3                   ; ball, scissors, pot

EDGE_L:         EQU     0
EDGE_R:         EQU     1
EDGE_T:         EQU     2
EDGE_B:         EQU     3

; Which room each entity starts on (its place in the 3x3 grid, 0..8). One of
; each exists in the whole world; crossings hand ownership to the neighbour.
; The player starts in the centre; each mover in a different room.
START_PLAYER_ROOM: EQU   4
START_BALL_ROOM:   EQU   0
START_SCISSORS_ROOM: EQU 2
START_POT_ROOM:    EQU   6

; A fixed byte the host program (the GUI deck) can read to see which machine
; holds the player: 1 while the player is on this machine, 0 otherwise. The
; deck polls it and moves the keyboard focus to follow the player, so the
; controls always drive the machine the player is on. Kept at a fixed address
; so the deck needs no symbol table.
PLAYER_BEACON:     EQU   $BFFE

; ============================================================================
;  Entry
; ============================================================================
Main:
                DI
                LD      SP,$7F00
                IM      1
                EI                          ; 50Hz frame interrupt, for HALT
                CALL    WaitRoom            ; link tells us which room we are
                CALL    SeedRng             ; per-machine RNG seed (room + R)
                CALL    LoadRoom            ; copy that room's 192 cells in
                CALL    ClearAttr           ; paper black, ink per band
                CALL    DrawTilemap         ; paint the whole background once

                ; One player and one of each mover (ball, scissors, pot) exist
                ; in the whole world, each on a single machine at a time. The
                ; player starts in the centre; each mover in its own room. Every
                ; other machine starts without them, and only draws/moves an
                ; entity once it holds it (the present flags).
                CALL    ClearMovers         ; all slots absent, beacon clear
                XOR     A
                CALL    SetPlayerPresent

                LD      A,(room)
                CP      START_PLAYER_ROOM
                JR      NZ,init_noplayer
                LD      A,112
                LD      (px),A
                LD      A,80
                LD      (py),A
                LD      A,1
                CALL    SetPlayerPresent
                CALL    SaveDrawPlayer
init_noplayer:
                CALL    InitMovers          ; spawn any mover whose room is ours

MainLoop:
                HALT                        ; wait for the frame interrupt, so
                                            ; sprite redraws land in the blank
                                            ; period and do not tear/flicker
                CALL    PollLink            ; take any entering entities
                CALL    MovePlayer
                CALL    MoveMovers          ; ball, scissors, pot
                JR      MainLoop

; ============================================================================
;  WaitRoom -- block until the host sends our room id (0..8), store it.
;  The very first byte the host sends after connecting is the room id.
; ============================================================================
WaitRoom:
wr_wait:        CALL    LinkInByte          ; A = byte, or Cy=0 if none yet
                JR      NC,wr_wait
                LD      (room),A
                RET

; ============================================================================
;  LinkInByte -- non-blocking read. Carry set + A = byte if one arrived,
;  Carry clear if the link had nothing. Four-step handshake.
; ============================================================================
LinkInByte:
                LD      BC,CTRL_PORT
                IN      A,(C)
                CP      1
                JR      Z,lib_have
                OR      A                   ; clear carry
                RET
lib_have:
                LD      BC,DATA_PORT
                IN      A,(C)
                LD      E,A                 ; keep the byte
                LD      BC,CTRL_PORT
                LD      A,3                 ; acknowledge (bit 1 set)
                OUT     (C),A
lib_wait2:      IN      A,(C)               ; wait for state 2 (host saw ack)
                CP      2
                JR      NZ,lib_wait2
                XOR     A
                OUT     (C),A               ; back to idle
                LD      A,E
                SCF                         ; carry set = we have a byte
                RET

; ============================================================================
;  LinkOutByte -- send A to the host. A bare OUT; the host reads the socket.
; ============================================================================
LinkOutByte:
                PUSH    BC
                LD      BC,DATA_PORT
                OUT     (C),A
                POP     BC
                RET

; ============================================================================
;  PollLink -- read all waiting bytes; assemble enter-packets and act on them.
;  Packet: SYNC, kind, edge, coord [, vx, vy for a ball].
; ============================================================================
PollLink:
pl_next:        CALL    LinkInByte
                RET     NC                  ; nothing left
                CP      SYNC
                JR      NZ,pl_next          ; resync on the sync byte
                ; kind
pl_kind:        CALL    LinkInByteBlock
                LD      (pk_kind),A
pl_edge:        CALL    LinkInByteBlock
                LD      (pk_edge),A
pl_coord:       CALL    LinkInByteBlock
                LD      (pk_coord),A
                LD      A,(pk_kind)
                CP      K_PLAYER
                JR      Z,pl_place          ; the player carries no velocity
                ; any mover carries two velocity bytes
                CALL    LinkInByteBlock
                LD      (pk_vx),A
                CALL    LinkInByteBlock
                LD      (pk_vy),A
pl_place:
                CALL    PlaceEntering
                JR      pl_next

; LinkInByteBlock -- like LinkInByte but waits until a byte really arrives.
; Used once a packet has started, to pull its remaining bytes.
LinkInByteBlock:
libb_l:         CALL    LinkInByte
                JR      NC,libb_l
                RET

; ============================================================================
;  PlaceEntering -- put the entering entity on the far edge from pk_edge, at
;  the stored perpendicular coordinate.
; ============================================================================
PlaceEntering:
                LD      A,(pk_kind)
                CP      K_PLAYER
                JR      NZ,pe_mover
                ; --- player ---
                ; If one is already here, erase it first (restore its under);
                ; otherwise there is nothing saved to restore.
                LD      A,(ppresent)
                OR      A
                CALL    NZ,ErasePlayer
                LD      A,(pk_edge)
                CALL    EdgeToXY            ; returns D=x, E=y for a 16px entity
                LD      A,D
                LD      (px),A
                LD      A,E
                LD      (py),A
                LD      A,1
                CALL    SetPlayerPresent
                CALL    SaveDrawPlayer
                RET
pe_mover:
                ; a mover: its slot is kind-1. Load it, erase an old copy if
                ; present, place at the far edge with the carried velocity.
                LD      A,(pk_kind)
                DEC     A
                CALL    LoadMover
                LD      A,(bpresent)
                OR      A
                CALL    NZ,EraseMover       ; erase only if one is already here
                LD      A,(pk_edge)
                CALL    EdgeToXY
                LD      A,D
                LD      (bx),A
                LD      A,E
                LD      (by),A
                LD      A,(pk_vx)
                LD      (bvx),A
                LD      A,(pk_vy)
                LD      (bvy),A
                LD      A,1
                LD      (bpresent),A
                CALL    SaveDrawMover
                CALL    StoreMover
                RET

; InVOpening -- A = a 16px sprite's top-left y. Carry set if its centre (y+8)
; lies within the vertical edge opening, i.e. it may cross a left/right edge.
InVOpening:
                ADD     A,8                 ; centre
                CP      OPEN_V_LO
                JR      C,io_no             ; below the opening
                CP      OPEN_V_HI+1
                JR      NC,io_no            ; above it
                SCF
                RET
io_no:          OR      A                   ; clear carry
                RET

; InHOpening -- A = a 16px sprite's top-left x. Carry set if its centre (x+8)
; lies within the horizontal edge opening (a top/bottom crossing is allowed).
InHOpening:
                ADD     A,8
                CP      OPEN_H_LO
                JR      C,ih_no
                CP      OPEN_H_HI+1
                JR      NC,ih_no
                SCF
                RET
ih_no:          OR      A
                RET

; EdgeToXY -- given the edge a thing enters through (A), and pk_coord, work
; out the top-left (x,y) for a 16px sprite just inside that edge.
;   left  -> x = 0,      y = coord
;   right -> x = MAXX,   y = coord
;   top   -> x = coord,  y = 0
;   bottom-> x = coord,  y = MAXY
; Returns D = x, E = y.
EdgeToXY:
                LD      HL,pk_coord
                LD      C,(HL)              ; C = coord
                CP      EDGE_L
                JR      NZ,e2_r
                LD      D,0
                LD      E,C
                RET
e2_r:           CP      EDGE_R
                JR      NZ,e2_t
                LD      D,MAXX
                LD      E,C
                RET
e2_t:           CP      EDGE_T
                JR      NZ,e2_b
                LD      D,C
                LD      E,0
                RET
e2_b:           LD      D,C
                LD      E,MAXY
                RET

; ============================================================================
;  MovePlayer -- read keys, try to move, bounce off walls, cross edges.
; ============================================================================
MovePlayer:
                LD      A,(ppresent)
                OR      A
                RET     Z                   ; no player on this machine
                ; copy current position as the trial position
                LD      A,(px)
                LD      (tx),A
                LD      A,(py)
                LD      (ty),A
                LD      B,0                 ; B = "moved?" flag

                ; Q = up (A..)  -- read key rows
                ; We use: Q up, A down, O left, P right (classic).
                CALL    ReadKeysClear       ; clears then sets kbits
                LD      A,(kbits)
                LD      C,A

                BIT     0,C                 ; up
                JR      Z,mp_nup
                LD      A,(ty)
                SUB     2
                JR      C,mp_cross_top
                LD      (ty),A
                LD      B,1
mp_nup:
                BIT     1,C                 ; down
                JR      Z,mp_ndn
                LD      A,(ty)
                ADD     A,2
                CP      MAXY+1
                JR      NC,mp_cross_bot
                LD      (ty),A
                LD      B,1
mp_ndn:
                BIT     2,C                 ; left
                JR      Z,mp_nlf
                LD      A,1
                LD      (facing),A          ; face left
                LD      A,(tx)
                SUB     2
                JR      C,mp_cross_left
                LD      (tx),A
                LD      B,1
mp_nlf:
                BIT     3,C                 ; right
                JR      Z,mp_nrt
                XOR     A
                LD      (facing),A          ; face right
                LD      A,(tx)
                ADD     A,2
                CP      MAXX+1
                JR      NC,mp_cross_right
                LD      (tx),A
                LD      B,1
mp_nrt:
                LD      A,B
                OR      A
                RET     Z                   ; nothing pressed

                ; wall test at the trial position
                CALL    PlayerHitsWall
                RET     C                   ; blocked: leave player where it was

                ; commit: erase old, advance anim frame, draw new
                CALL    ErasePlayer
                LD      A,(tx)
                LD      (px),A
                LD      A,(ty)
                LD      (py),A
                CALL    NextPlayerFrame
                CALL    SaveDrawPlayer
                RET

; --- player edge crossings: only through the opening, else stay put ---
; Each tests whether the hero is at the opening for that edge; if not, it is
; against the solid wall, so it does not cross and does not move (RET leaves
; px/py untouched -- the move this frame is simply dropped).
mp_cross_top:
                LD      A,(px)
                CALL    InHOpening
                RET     NC                  ; against the wall: no crossing
                LD      D,EDGE_B            ; it will ENTER the neighbour's bottom
                LD      A,(px)
                LD      E,A
                LD      C,K_PLAYER
                CALL    SendCross
                CALL    HidePlayer
                RET
mp_cross_bot:
                LD      A,(px)
                CALL    InHOpening
                RET     NC
                LD      D,EDGE_T
                LD      A,(px)
                LD      E,A
                LD      C,K_PLAYER
                CALL    SendCross
                CALL    HidePlayer
                RET
mp_cross_left:
                LD      A,(py)
                CALL    InVOpening
                RET     NC
                LD      D,EDGE_R
                LD      A,(py)
                LD      E,A
                LD      C,K_PLAYER
                CALL    SendCross
                CALL    HidePlayer
                RET
mp_cross_right:
                LD      A,(py)
                CALL    InVOpening
                RET     NC
                LD      D,EDGE_L
                LD      A,(py)
                LD      E,A
                LD      C,K_PLAYER
                CALL    SendCross
                CALL    HidePlayer
                RET

; HidePlayer -- erase the player from this screen and mark it absent, so the
; machine stops drawing and moving it once it has crossed to the neighbour.
HidePlayer:
                CALL    ErasePlayer
                XOR     A
                CALL    SetPlayerPresent
                RET

; SetPlayerPresent -- A (0/1) into ppresent and the deck beacon, together.
SetPlayerPresent:
                LD      (ppresent),A
                LD      (PLAYER_BEACON),A
                RET

; HideMover -- erase the current mover and mark its slot absent, then store.
HideMover:
                CALL    EraseMover
                XOR     A
                LD      (bpresent),A
                CALL    StoreMover
                RET

; SendCross -- send an enter-packet for the current machine's exit.
;   C = kind, D = edge it enters the neighbour through, E = perpendicular coord
;   Any mover (kind != player) also sends bvx, bvy; the player sends neither.
SendCross:
                LD      A,SYNC
                CALL    LinkOutByte
                LD      A,C
                CALL    LinkOutByte
                LD      A,D
                CALL    LinkOutByte
                LD      A,E
                CALL    LinkOutByte
                LD      A,C
                CP      K_PLAYER
                RET     Z                   ; the player carries no velocity
                LD      A,(bvx)
                CALL    LinkOutByte
                LD      A,(bvy)
                CALL    LinkOutByte
                RET

; ============================================================================
;  Movers: ball, scissors, pot. All three share the same drift/bounce/cross
;  code (MoveMover); a slot is loaded into the working-set, moved, stored back.
;
;  Mover table, NMOVERS slots of 5 bytes: present, x, y, vx, vy.
;  Parallel tables give each slot its sprite, its start room, and its own
;  save-under buffer so erasing one does not disturb another.
; ============================================================================
MoverState:    DS      NMOVERS*5, 0
MoverSprites:  DW      BallSprite
               DW      ScissorsSprite
               DW      PotSprite
MoverRooms:    DB      START_BALL_ROOM
               DB      START_SCISSORS_ROOM
               DB      START_POT_ROOM
; Starting positions (x,y) per slot -- chosen so they do not sit on a wall.
; Starting top-left (x,y) per slot. A rook mover must sit on the line that
; passes through an opening or it could never leave its room: the horizontal
; scissors starts at a y inside the vertical opening (rows 5..6), the vertical
; pot at an x inside the horizontal opening (cols 7..8).
MoverStartXY:  DB      48,40                ; ball (diagonal): anywhere clear
               DB      48,88                ; scissors (horizontal): y in opening
               DB      120,40               ; pot (vertical): x in opening
; Starting velocity per slot (two signed bytes), $FF = -1. The movers travel
; differently: the ball goes diagonally (a bishop), the scissors straight
; across (a rook, horizontal), the pot straight up and down (a rook,
; vertical). A rook mover keeps one axis zero, so it only ever bounces back
; and forth along its line.
MoverStartVel: DB      1,1                  ; ball: diagonal
               DB      1,0                  ; scissors: horizontal rook
               DB      0,1                  ; pot: vertical rook
; Speed mask per slot: 0 = move every frame, 1 = every other frame (half
; speed). The pot is the heavy one, so it drifts at half the scissors' pace.
MoverSpeed:    DB      0                    ; ball: full speed
               DB      0                    ; scissors: full speed
               DB      1                    ; pot: half speed
MoverUnders:   DS      NMOVERS*48, 0        ; 48 = 3 bytes x 16 rows per slot

; ClearMovers -- every slot absent.
ClearMovers:
                LD      HL,MoverState
                LD      B,NMOVERS
cm_slot:
                LD      (HL),0               ; present = 0
                PUSH    BC
                LD      BC,5
                ADD     HL,BC
                POP     BC
                DJNZ    cm_slot
                RET

; InitMovers -- for each slot whose start room is ours, spawn it (present=1,
; start position, velocity 1,1), draw it, and store it.
InitMovers:
                LD      B,NMOVERS
                LD      C,0                  ; C = slot index
im_slot:
                PUSH    BC
                LD      A,C
                LD      E,A
                LD      D,0
                LD      HL,MoverRooms
                ADD     HL,DE
                LD      A,(HL)               ; this slot's start room
                LD      HL,room
                CP      (HL)
                JR      NZ,im_next
                ; spawn here
                LD      A,C
                CALL    LoadMover            ; set curslot/kind/sprite/under
                LD      HL,MoverStartXY
                LD      A,C
                ADD     A,A                  ; *2 (x,y pairs)
                LD      E,A
                LD      D,0
                ADD     HL,DE
                LD      A,(HL)
                LD      (bx),A
                INC     HL
                LD      A,(HL)
                LD      (by),A
                ; velocity from MoverStartVel[slot] (two signed bytes)
                LD      HL,MoverStartVel
                LD      A,C
                ADD     A,A
                LD      E,A
                LD      D,0
                ADD     HL,DE
                LD      A,(HL)
                LD      (bvx),A
                INC     HL
                LD      A,(HL)
                LD      (bvy),A
                LD      A,1
                LD      (bpresent),A
                CALL    SaveDrawMover
                CALL    StoreMover
im_next:
                POP     BC
                INC     C
                DJNZ    im_slot
                RET

; MoveMovers -- load, move, (store happens inside move paths) each slot. Each
; slot has a speed mask in MoverSpeed: 0 moves every frame, 1 every other
; frame (half speed), so a heavier mover like the pot drifts more slowly.
MoveMovers:
                LD      A,(framecnt)
                INC     A
                LD      (framecnt),A
                LD      B,NMOVERS
                LD      C,0
mm_slot:
                PUSH    BC
                LD      A,C
                CALL    LoadMover
                LD      A,(bpresent)
                OR      A
                JR      Z,mm_next            ; absent: nothing to move or store
                ; speed gate: skip this frame if (framecnt AND mask) != 0
                LD      HL,MoverSpeed
                LD      A,C
                LD      E,A
                LD      D,0
                ADD     HL,DE
                LD      A,(HL)               ; this slot's speed mask
                LD      E,A
                LD      A,(framecnt)
                AND     E
                JR      NZ,mm_next           ; not this slot's frame: hold still
                CALL    MoveMover
                CALL    StoreMover           ; MoveMover left the working-set current
mm_next:
                POP     BC
                INC     C
                DJNZ    mm_slot
                RET

; LoadMover -- A = slot (0..NMOVERS-1). Copies that slot's state into the
; working-set (bpresent,bx,by,bvx,bvy) and sets curslot, curkind, curSprite,
; curUnder.
LoadMover:
                LD      (curslot),A
                ; curkind = slot + 1  (K_BALL=1 is slot 0)
                INC     A
                LD      (curkind),A
                DEC     A
                ; source = MoverState + slot*5
                LD      L,A
                LD      H,0
                LD      D,H
                LD      E,L
                ADD     HL,HL                ; *2
                ADD     HL,HL                ; *4
                ADD     HL,DE                ; *5
                LD      DE,MoverState
                ADD     HL,DE                ; HL -> slot's 5 bytes
                LD      A,(HL)
                LD      (bpresent),A
                INC     HL
                LD      A,(HL)
                LD      (bx),A
                INC     HL
                LD      A,(HL)
                LD      (by),A
                INC     HL
                LD      A,(HL)
                LD      (bvx),A
                INC     HL
                LD      A,(HL)
                LD      (bvy),A
                ; curSprite = MoverSprites[slot]
                LD      A,(curslot)
                ADD     A,A                  ; *2 (word)
                LD      L,A
                LD      H,0
                LD      DE,MoverSprites
                ADD     HL,DE
                LD      A,(HL)
                LD      (curSprite),A
                INC     HL
                LD      A,(HL)
                LD      (curSprite+1),A
                ; curUnder = MoverUnders + slot*48
                LD      A,(curslot)
                LD      L,A
                LD      H,0
                ; *48 = *32 + *16
                ADD     HL,HL                ; *2
                ADD     HL,HL                ; *4
                ADD     HL,HL                ; *8
                ADD     HL,HL                ; *16
                LD      D,H
                LD      E,L                  ; DE = slot*16
                ADD     HL,HL                ; *32
                ADD     HL,DE                ; *48
                LD      DE,MoverUnders
                ADD     HL,DE
                LD      (curUnder),HL
                RET

; StoreMover -- copy the working-set back into the current slot.
StoreMover:
                LD      A,(curslot)
                LD      L,A
                LD      H,0
                LD      D,H
                LD      E,L
                ADD     HL,HL
                ADD     HL,HL
                ADD     HL,DE                ; slot*5
                LD      DE,MoverState
                ADD     HL,DE
                LD      A,(bpresent)
                LD      (HL),A
                INC     HL
                LD      A,(bx)
                LD      (HL),A
                INC     HL
                LD      A,(by)
                LD      (HL),A
                INC     HL
                LD      A,(bvx)
                LD      (HL),A
                INC     HL
                LD      A,(bvy)
                LD      (HL),A
                RET

; ============================================================================
;  MoveMover -- drift by (bvx,bvy); bounce off walls; cross edges. Works on
;  whatever slot LoadMover last loaded.
; ============================================================================
MoveMover:
                ; operates on the current working-set (bx..bpresent, curkind, curslot)
                LD      A,(bpresent)
                OR      A
                RET     Z                   ; this mover not on this machine
                ; trial x
                LD      A,(bx)
                LD      B,A
                LD      A,(bvx)
                ADD     A,B                 ; new x = bx + bvx (signed step +-1)
                LD      (tbx),A
                ; trial y
                LD      A,(by)
                LD      B,A
                LD      A,(bvy)
                ADD     A,B
                LD      (tby),A

                ; left/right edge? Only cross if the mover is at the vertical
                ; opening; against the solid wall it bounces instead.
                LD      A,(tbx)
                CP      MAXX+1
                JR      NC,mb_xoff          ; wrapped past 0 or past MAXX
                JR      mb_xok
mb_xoff:
                LD      A,(by)
                CALL    InVOpening
                JR      NC,mb_xwall         ; not at the opening: bounce
                ; at the opening: cross to the left/right neighbour
                LD      A,(bvx)
                BIT     7,A
                JR      NZ,mb_left_edge     ; moving left -> went below 0
                LD      D,EDGE_L            ; moving right -> enters neighbour left
                LD      A,(by)
                LD      E,A
                LD      A,(curkind)
                LD      C,A
                CALL    SendCross
                CALL    HideMover
                RET
mb_left_edge:
                LD      D,EDGE_R
                LD      A,(by)
                LD      E,A
                LD      A,(curkind)
                LD      C,A
                CALL    SendCross
                CALL    HideMover
                RET
mb_xwall:
                ; bounce off the border wall: reverse bvx, keep x, go on to y.
                LD      A,(bvx)
                NEG
                LD      (bvx),A
                LD      A,(bx)
                LD      (tbx),A
mb_xok:
                ; top/bottom edge? Same gate for the horizontal opening.
                LD      A,(tby)
                CP      MAXY+1
                JR      NC,mb_yoff
                JR      mb_yok
mb_yoff:
                LD      A,(bx)
                CALL    InHOpening
                JR      NC,mb_ywall         ; not at the opening: bounce
                LD      A,(bvy)
                BIT     7,A
                JR      NZ,mb_top_edge
                LD      D,EDGE_T
                LD      A,(bx)
                LD      E,A
                LD      A,(curkind)
                LD      C,A
                CALL    SendCross
                CALL    HideMover
                RET
mb_top_edge:
                LD      D,EDGE_B
                LD      A,(bx)
                LD      E,A
                LD      A,(curkind)
                LD      C,A
                CALL    SendCross
                CALL    HideMover
                RET
mb_ywall:
                ; bounce off the border wall: reverse bvy, keep y.
                LD      A,(bvy)
                NEG
                LD      (bvy),A
                LD      A,(by)
                LD      (tby),A
mb_yok:
                ; wall bounce: if the trial cell is a wall, flip that velocity
                LD      A,(tbx)
                LD      (probe_x),A
                LD      A,(by)
                LD      (probe_y),A
                CALL    CellIsWall
                JR      NC,mb_xclear
                ; hit: invert bvx, keep x
                LD      A,(bvx)
                NEG
                LD      (bvx),A
                LD      A,(bx)
                LD      (tbx),A
mb_xclear:
                LD      A,(bx)
                LD      (probe_x),A
                LD      A,(tby)
                LD      (probe_y),A
                CALL    CellIsWall
                JR      NC,mb_yclear
                LD      A,(bvy)
                NEG
                LD      (bvy),A
                LD      A,(by)
                LD      (tby),A
mb_yclear:
                CALL    EraseMover
                LD      A,(tbx)
                LD      (bx),A
                LD      A,(tby)
                LD      (by),A
                CALL    MoverDecide         ; rook movers may turn at the centre
                CALL    SaveDrawMover
                RET

; ============================================================================
;  MoverDecide -- a rook mover that has just landed on the room centre picks a
;  new direction: one of the three that are not a reversal of its current one.
;  The ball (diagonal) does not take part. Non-rooks and movers not at the
;  centre return unchanged.
; ============================================================================
MoverDecide:
                ; only rook movers decide; the ball (K_BALL) keeps its diagonal
                LD      A,(curkind)
                CP      K_BALL
                RET     Z
                ; at the centre? x == CORRIDOR_X and y == CORRIDOR_Y
                LD      A,(bx)
                CP      CORRIDOR_X
                RET     NZ
                LD      A,(by)
                CP      CORRIDOR_Y
                RET     NZ
                ; pick one of three: 0 = carry straight on, 1 and 2 = the two
                ; perpendicular turns. A reversal is never offered.
                CALL    Rnd3                ; A = 0..2
                OR      A
                RET     Z                   ; 0: keep going the same way
                ; a turn: swap to the perpendicular axis. If we were moving
                ; horizontally (bvy == 0) we now move vertically, and vice versa.
                DEC     A                   ; A = 0 or 1 -> which of the two ways
                LD      B,A                 ; B = direction selector
                LD      A,(bvy)
                OR      A
                JR      NZ,md_vert          ; was moving vertically -> go horizontal
                ; was horizontal: new vy = +1 (B=0) or -1 (B=1), vx = 0
                XOR     A
                LD      (bvx),A
                LD      A,1
                BIT     0,B
                JR      Z,md_seth
                LD      A,$FF               ; -1
md_seth:        LD      (bvy),A
                RET
md_vert:        ; was vertical: new vx = +1 (B=0) or -1 (B=1), vy = 0
                XOR     A
                LD      (bvy),A             ; vy = 0
                LD      A,1
                BIT     0,B
                JR      Z,md_setv
                LD      A,$FF
md_setv:        LD      (bvx),A             ; vx = +1 or -1
                RET

; Rnd3 -- returns A = 0, 1 or 2. A 16-bit xorshift-style LFSR (taps giving a
; long period), mixed with the refresh register R so machines that start in
; step still diverge. The low bits are reduced to 0..2 (3 maps back to 0).
; SeedRng -- give this machine its own RNG start, so the nine do not all make
; the same turns. Mixes the room id, the refresh register, and a non-zero
; constant (an all-zero LFSR state would stick).
SeedRng:
                LD      A,(room)
                LD      H,A
                LD      A,R
                LD      L,A
                LD      A,H
                OR      L
                JR      NZ,sr_ok
                LD      HL,$1234            ; never seed all-zero
sr_ok:          LD      (rngstate),HL
                RET

Rnd3:
                LD      HL,(rngstate)
                ; HL ^= HL << 7  (approximate xorshift step with byte ops)
                LD      A,L
                ADD     A,A
                ADD     A,A
                ADD     A,A
                ADD     A,A
                ADD     A,A
                ADD     A,A
                ADD     A,A                 ; A = L << 7 (low byte)
                XOR     H
                LD      H,A
                ; mix the refresh register into the low byte
                LD      A,R
                XOR     L
                ADD     A,H
                LD      L,A
                LD      (rngstate),HL
                AND     3                   ; 0..3
                CP      3
                RET     NZ
                XOR     A                   ; map 3 -> 0, giving 0..2
                RET

; ============================================================================
;  Collision: does a 16x16 entity at (probe_x, probe_y) overlap a wall cell?
;  We test the four corners' cells. Carry set = wall.
; ============================================================================
CellIsWall:
                ; top-left
                LD      A,(probe_x)
                LD      D,A
                LD      A,(probe_y)
                LD      E,A
                CALL    CellAt
                CP      1
                JR      Z,cw_yes
                ; top-right
                LD      A,(probe_x)
                ADD     A,CELL-1
                LD      D,A
                LD      A,(probe_y)
                LD      E,A
                CALL    CellAt
                CP      1
                JR      Z,cw_yes
                ; bottom-left
                LD      A,(probe_x)
                LD      D,A
                LD      A,(probe_y)
                ADD     A,CELL-1
                LD      E,A
                CALL    CellAt
                CP      1
                JR      Z,cw_yes
                ; bottom-right
                LD      A,(probe_x)
                ADD     A,CELL-1
                LD      D,A
                LD      A,(probe_y)
                ADD     A,CELL-1
                LD      E,A
                CALL    CellAt
                CP      1
                JR      Z,cw_yes
                OR      A                   ; clear carry
                RET
cw_yes:
                SCF
                RET

; PlayerHitsWall -- 16x16 player at (tx,ty). Carry set = blocked.
PlayerHitsWall:
                LD      A,(tx)
                LD      (probe_x),A
                LD      A,(ty)
                LD      (probe_y),A
                CALL    CellIsWall
                RET

; CellAt -- cell index at pixel (D=x, E=y). Returns tile id in A.
;   col = x/16, row = y/16, index = row*16 + col, read room[index].
CellAt:
                LD      A,D
                SRL     A
                SRL     A
                SRL     A
                SRL     A                   ; col = x>>4
                LD      C,A
                LD      A,E
                SRL     A
                SRL     A
                SRL     A
                SRL     A                   ; row = y>>4
                ; index = row*16 + col
                LD      L,A
                LD      H,0
                ADD     HL,HL
                ADD     HL,HL
                ADD     HL,HL
                ADD     HL,HL               ; HL = row*16
                LD      A,L
                ADD     A,C
                LD      L,A
                JR      NC,ca_nocarry
                INC     H
ca_nocarry:
                LD      DE,roomcells
                ADD     HL,DE
                LD      A,(HL)
                RET

; ============================================================================
;  Keyboard: Q up, A down, O left, P right. Sets kbits bit0..3.
; ============================================================================
ReadKeys:
                LD      C,0
                ; Q is in row B'FBFE' bit 0
                LD      BC,$FBFE
                IN      A,(C)
                BIT     0,A                 ; Q
                JR      NZ,rk_nup
                LD      A,(kbits)
                OR      1
                LD      (kbits),A
rk_nup:
                ; A is in row B'FDFE' bit 0
                LD      BC,$FDFE
                IN      A,(C)
                BIT     0,A                 ; A
                JR      NZ,rk_ndn
                LD      A,(kbits)
                OR      2
                LD      (kbits),A
rk_ndn:
                ; O is in row B'DFFE' bit 1
                LD      BC,$DFFE
                IN      A,(C)
                BIT     1,A                 ; O
                JR      NZ,rk_nlf
                LD      A,(kbits)
                OR      4
                LD      (kbits),A
rk_nlf:
                ; P is in row B'DFFE' bit 0
                LD      BC,$DFFE
                IN      A,(C)
                BIT     0,A                 ; P
                JR      NZ,rk_nrt
                LD      A,(kbits)
                OR      8
                LD      (kbits),A
rk_nrt:
                RET

; ReadKeys must start from a cleared kbits each frame.
ReadKeysClear:
                XOR     A
                LD      (kbits),A
                JP      ReadKeys

; ============================================================================
;  Player drawing: SaveUnder then draw current frame; erase = RestoreUnder.
; ============================================================================
SaveDrawPlayer:
                LD      A,(px)
                LD      C,A
                LD      A,(py)
                LD      B,A
                LD      A,16
                LD      (UnderRows),A
                CALL    SaveUnder
                ; draw current player frame
                LD      A,(px)
                LD      C,A
                LD      A,(py)
                LD      B,A
                CALL    CurPlayerFrame      ; DE -> frame data
                CALL    Render_Sprite_RT
                RET

ErasePlayer:
                LD      A,(px)
                LD      C,A
                LD      A,(py)
                LD      B,A
                LD      A,16
                LD      (UnderRows),A
                CALL    RestoreUnder
                RET

; Ball uses its own save buffer; to keep the single UnderBuf correct we save
; the ball under immediately before drawing and restore before moving. Since
; player and ball never share a frame step, one buffer is reused safely by
; always pairing SaveDrawBall with EraseBall.
SaveDrawMover:
                LD      A,(bx)
                LD      C,A
                LD      A,(by)
                LD      B,A
                LD      A,16
                LD      (UnderRows),A
                CALL    SaveUnderB
                LD      A,(bx)
                LD      C,A
                LD      A,(by)
                LD      B,A
                LD      DE,(curSprite)      ; the current mover's sprite
                CALL    Render_Sprite_RT
                RET

EraseMover:
                LD      A,(bx)
                LD      C,A
                LD      A,(by)
                LD      B,A
                LD      A,16
                LD      (UnderRows),A
                CALL    RestoreUnderB
                RET

; ============================================================================
;  Player animation frames
; ============================================================================
CurPlayerFrame:
                ; pick the table for the way the character faces
                LD      DE,PlayerFramesR
                LD      A,(facing)
                OR      A
                JR      Z,cpf_have
                LD      DE,PlayerFramesL
cpf_have:
                LD      A,(pframe)
                ADD     A,A                 ; *2 (word table)
                LD      L,A
                LD      H,0
                ADD     HL,DE
                LD      E,(HL)
                INC     HL
                LD      D,(HL)              ; DE = frame address
                RET

; NextPlayerFrame -- advance the walk cycle, but only every ANIM_EVERY moves
; so the legs do not blur at the frame rate.
ANIM_EVERY:     EQU     4
NextPlayerFrame:
                LD      A,(animtick)
                INC     A
                CP      ANIM_EVERY
                JR      C,npf_keep
                XOR     A                   ; tick wrapped: advance the frame
                LD      (animtick),A
                LD      A,(pframe)
                INC     A
                CP      PLAYER_FRAMES
                JR      C,npf_ok
                XOR     A
npf_ok:
                LD      (pframe),A
                RET
npf_keep:
                LD      (animtick),A
                RET

; ============================================================================
;  Room loading: 9 compiled rooms, 192 cells each, selected by (room).
; ============================================================================
LoadRoom:
                ; HL = room * 64
                LD      A,(room)
                LD      L,A
                LD      H,0
                ADD     HL,HL               ; *2
                ADD     HL,HL               ; *4
                ADD     HL,HL               ; *8
                ADD     HL,HL               ; *16
                ADD     HL,HL               ; *32
                ADD     HL,HL               ; *64
                ; DE = HL (room*64); HL = room*128; HL += DE -> room*192
                LD      D,H
                LD      E,L                 ; DE = room*64
                ADD     HL,HL               ; HL = room*128
                ADD     HL,DE               ; HL = room*192
                LD      DE,RoomTable
                ADD     HL,DE               ; HL -> this room's 192 bytes
                LD      DE,roomcells
                LD      BC,192
                LDIR
                RET

; DrawTilemap -- paint all 192 cells from roomcells.
DrawTilemap:
                LD      HL,roomcells
                LD      (dt_ptr),HL
                LD      D,0                 ; row
dt_row:
                LD      E,0                 ; col
dt_col:
                ; tile id
                LD      HL,(dt_ptr)
                LD      A,(HL)
                INC     HL
                LD      (dt_ptr),HL
                ; draw tile A at cell (E col, D row) -> pixel (E*16, D*16)
                PUSH    DE
                CALL    DrawTileCell
                POP     DE
                INC     E
                LD      A,E
                CP      COLS
                JR      NZ,dt_col
                INC     D
                LD      A,D
                CP      ROWS
                JR      NZ,dt_row
                RET

; DrawTileCell -- tile id in A, cell col E, row D. Blits 16x16 from TileData.
DrawTileCell:
                ; frame address = TileData + id*32
                LD      L,A
                LD      H,0
                ADD     HL,HL
                ADD     HL,HL
                ADD     HL,HL
                ADD     HL,HL
                ADD     HL,HL               ; *32
                LD      BC,TileData
                ADD     HL,BC
                LD      (tile_src),HL
                ; pixel y = D*16, x = E*16
                LD      A,D
                ADD     A,A
                ADD     A,A
                ADD     A,A
                ADD     A,A                 ; D*16
                LD      (tc_y),A
                LD      A,E
                ADD     A,A
                ADD     A,A
                ADD     A,A
                ADD     A,A                 ; E*16
                LD      (tc_x),A
                ; draw 16 rows, 2 bytes each, opaque
                LD      A,(tc_y)
                LD      B,A
                LD      A,(tc_x)
                LD      C,A
                CALL    Get_Pixel_Address   ; HL = screen addr (x on cell bndry)
                LD      (tc_scr),HL
                LD      HL,(tile_src)
                LD      (tile_src),HL
                LD      B,16
dtc_row:
                PUSH    BC
                LD      HL,(tile_src)
                LD      A,(HL)
                INC     HL
                LD      E,(HL)              ; E = second byte
                INC     HL
                LD      (tile_src),HL
                LD      D,A                 ; D = first byte
                LD      HL,(tc_scr)
                LD      (HL),D
                INC     L
                LD      (HL),E
                DEC     L
                ; next screen line
                CALL    Next_Screen_Line
                LD      (tc_scr),HL
                POP     BC
                DJNZ    dtc_row
                RET

; ============================================================================
;  Attributes: each room gets a plain scheme -- black paper, white ink, with
;  a bright border row/column feel left to the tiles. Simple: all cells
;  ink 7 paper 0. The host could vary this; kept uniform for clarity.
; ============================================================================
ClearAttr:
                ; ink colour per room, so the nine screens read as nine places.
                LD      A,(room)
                LD      E,A
                LD      D,0
                LD      HL,RoomInk
                ADD     HL,DE
                LD      A,(HL)              ; A = attribute byte for this room
                LD      HL,ATTR
                LD      DE,ATTR+1
                LD      BC,768-1
                LD      (HL),A
                LDIR
                RET

; Attribute (ink+bright, paper 0) per room id 0..8.
RoomInk:        DB      %01000110           ; 0 yellow bright
                DB      %01000100           ; 1 green bright
                DB      %01000101           ; 2 cyan bright
                DB      %01000010           ; 3 red bright
                DB      %01000111           ; 4 white bright
                DB      %01000011           ; 5 magenta bright
                DB      %00000110           ; 6 yellow
                DB      %01000100           ; 7 green bright
                DB      %01000101           ; 8 cyan bright

; ============================================================================
;  Delay -- a coarse frame wait so motion is watchable.
; ============================================================================
Delay:
                LD      BC,$0A00
dl_l:           DEC     BC
                LD      A,B
                OR      C
                JR      NZ,dl_l
                RET

; ============================================================================
;  Included library: Oakhollow engine routines (verified), verbatim.
;  Render_Sprite_RT, Next_Screen_Line, Get_Pixel_Address, SaveUnder,
;  RestoreUnder -- and a second save buffer for the ball.
; ============================================================================

Render_Sprite_RT:
                LD      A,(DE)              ; width (chars)
                INC     DE
                CP      2
                RET     NZ                  ; only 16px wide supported here
                LD      A,(DE)              ; height (pixels)
                INC     DE
                LD      (rsrt_rowcount+1),A
                LD      A,C
                AND     7
                LD      (rsrt_shiftcount+1),A
                PUSH    DE
                POP     IX
                CALL    Get_Pixel_Address
rsrt_rowcount:  LD      B,16
rsrt_rowloop:
                PUSH    BC
                PUSH    HL
                EXX
                LD      C,(IX+0)
                LD      D,(IX+1)
                LD      E,0
rsrt_shiftcount:LD      A,0
                OR      A
                JR      Z,rsrt_gfxdone
rsrt_gfxshift:  SRL     C
                RR      D
                RR      E
                DEC     A
                JR      NZ,rsrt_gfxshift
rsrt_gfxdone:
                EXX
                LD      C,(IX+2)
                LD      D,(IX+3)
                LD      E,$FF
                LD      A,(rsrt_shiftcount+1)
                OR      A
                JR      Z,rsrt_maskdone
rsrt_mskshift:  SCF
                RR      C
                RR      D
                RR      E
                DEC     A
                JR      NZ,rsrt_mskshift
rsrt_maskdone:
                LD      A,(HL)
                AND     C
                EXX
                OR      C
                EXX
                LD      (HL),A
                INC     L
                LD      A,(HL)
                AND     D
                EXX
                OR      D
                EXX
                LD      (HL),A
                INC     L
                LD      A,(HL)
                AND     E
                EXX
                OR      E
                EXX
                LD      (HL),A
                POP     HL
                CALL    Next_Screen_Line
                LD      BC,4
                ADD     IX,BC
                POP     BC
                DJNZ    rsrt_rowloop
                RET

Next_Screen_Line:
                INC     H
                LD      A,H
                AND     7
                RET     NZ
                LD      A,L
                ADD     A,32
                LD      L,A
                RET     C
                LD      A,H
                SUB     8
                LD      H,A
                RET

Get_Pixel_Address:
                LD      A,B
                AND     %00000111
                OR      %01000000
                LD      H,A
                LD      A,B
                RLA
                RLA
                AND     %11100000
                LD      L,A
                LD      A,B
                AND     %11000000
                RRA
                RRA
                RRA
                OR      H
                LD      H,A
                LD      A,C
                RRA
                RRA
                RRA
                AND     %00011111
                OR      L
                LD      L,A
                LD      A,C
                AND     7
                RET

UnderRows:      DB      16
UnderBuf:       DS      3*32, 0

SaveUnder:
                CALL    Get_Pixel_Address
                LD      IX,UnderBuf
                LD      A,(UnderRows)
                LD      B,A
su_row:
                LD      A,(HL)
                LD      (IX+0),A
                INC     L
                LD      A,(HL)
                LD      (IX+1),A
                INC     L
                LD      A,(HL)
                LD      (IX+2),A
                DEC     L
                DEC     L
                PUSH    BC
                CALL    Next_Screen_Line
                POP     BC
                LD      DE,3
                ADD     IX,DE
                DJNZ    su_row
                RET

RestoreUnder:
                CALL    Get_Pixel_Address
                LD      IX,UnderBuf
                LD      A,(UnderRows)
                LD      B,A
ru_row:
                LD      A,(IX+0)
                LD      (HL),A
                INC     L
                LD      A,(IX+1)
                LD      (HL),A
                INC     L
                LD      A,(IX+2)
                LD      (HL),A
                DEC     L
                DEC     L
                PUSH    BC
                CALL    Next_Screen_Line
                POP     BC
                LD      DE,3
                ADD     IX,DE
                DJNZ    ru_row
                RET

; Second save buffer for the ball, so player and ball unders don't clash.
UnderBufB:      DS      3*32, 0

SaveUnderB:
                CALL    Get_Pixel_Address
                LD      IX,(curUnder)
                LD      A,(UnderRows)
                LD      B,A
sub_row:
                LD      A,(HL)
                LD      (IX+0),A
                INC     L
                LD      A,(HL)
                LD      (IX+1),A
                INC     L
                LD      A,(HL)
                LD      (IX+2),A
                DEC     L
                DEC     L
                PUSH    BC
                CALL    Next_Screen_Line
                POP     BC
                LD      DE,3
                ADD     IX,DE
                DJNZ    sub_row
                RET

RestoreUnderB:
                CALL    Get_Pixel_Address
                LD      IX,(curUnder)
                LD      A,(UnderRows)
                LD      B,A
rub_row:
                LD      A,(IX+0)
                LD      (HL),A
                INC     L
                LD      A,(IX+1)
                LD      (HL),A
                INC     L
                LD      A,(IX+2)
                LD      (HL),A
                DEC     L
                DEC     L
                PUSH    BC
                CALL    Next_Screen_Line
                POP     BC
                LD      DE,3
                ADD     IX,DE
                DJNZ    rub_row
                RET

; ============================================================================
;  State
; ============================================================================
room:           DB      0
px:             DB      0
py:             DB      0
; The mover working-set: LoadMover copies a slot's state in here, the move
; code works on it, StoreMover copies it back. bpresent doubles as the
; current mover's present flag.
bx:             DB      0
by:             DB      0
bvx:            DB      1
bvy:            DB      1
bpresent:       DB      0                   ; current mover present on this machine
curslot:        DB      0                   ; which mover slot is loaded
curkind:        DB      0                   ; its kind (slot+1)
curSprite:      DW      0                   ; its sprite data pointer
curUnder:       DW      0                   ; its save-under buffer
rngstate:       DW      $A5C3               ; LFSR state for Rnd3 (reseeded)
framecnt:       DB      0                   ; frame counter for mover speed gates
tx:             DB      0
ty:             DB      0
tbx:            DB      0
tby:            DB      0
pframe:         DB      0
animtick:       DB      0                   ; counts moves between frame advances
facing:         DB      0                   ; 0 = right, 1 = left
ppresent:       DB      0                   ; 1 if the player is on this machine
kbits:          DB      0
probe_x:        DB      0
probe_y:        DB      0
pk_kind:        DB      0
pk_edge:        DB      0
pk_coord:       DB      0
pk_vx:          DB      0
pk_vy:          DB      0
dt_ptr:         DW      0
tile_src:       DW      0
tc_scr:         DW      0
tc_x:           DB      0
tc_y:           DB      0

roomcells:      DS      192, 0

                INCLUDE "assets.asm"
                INCLUDE "rooms.asm"

                END     Main
