#!/usr/bin/env python3
"""The world host: it ties a grid of Spectrums into one connected world.

Each machine draws one screen and knows nothing of the others. When the
player or the ball walks off an edge, the machine sends a short packet on its
link; this host holds the grid, works out which machine is on the far side of
that edge, and hands the packet to it so the entity reappears there. At the
world's outer border there is no neighbour, so the host turns the ball back
(a bounce) and keeps the player on-screen.

Topology is a rectangular grid, rows x cols, given on the command line:

    python3 host.py --rows 3 --cols 3  16384 16385 16386 16387 16388 \\
                                        16389 16390 16391 16392

The ports are listed row by row, left to right, top to bottom: the first is
grid cell (0,0), the next (0,1), and so on. The host also sends each machine
its room id (its index in that list) as the first byte, which the engine
waits for before it starts.

Packet, both directions (host<->machine):
    SYNC(0xAA), kind, edge, coord [, vx, vy if kind==ball]
kind: 0 player, 1 ball. edge: the edge the entity ENTERS through on the
destination (0 left, 1 right, 2 top, 3 bottom). coord: the perpendicular
pixel position. A ball also carries its two signed velocity bytes.

A machine reports an exit as the entry it should become on the neighbour:
the engine already converts "left my right edge" into "enters a left edge",
so the host only has to pick the neighbour and, at a border, reflect.
"""
import argparse
import socket
import sys
import threading

SYNC = 0xAA
# Entity kinds. The player is keyboard-driven; the others drift on their own
# and carry a velocity. Any kind other than the player is treated the same
# way by the host -- it just carries two velocity bytes.
K_PLAYER, K_BALL, K_SCISSORS, K_POT = 0, 1, 2, 3
NAMES = {K_PLAYER: "player", K_BALL: "ball",
         K_SCISSORS: "scissors", K_POT: "pot"}
EDGE_L, EDGE_R, EDGE_T, EDGE_B = 0, 1, 2, 3
OPPOSITE = {EDGE_L: EDGE_R, EDGE_R: EDGE_L, EDGE_T: EDGE_B, EDGE_B: EDGE_T}


class Machine:
    def __init__(self, index, port, row, col):
        self.index = index
        self.port = port
        self.row = row
        self.col = col
        self.buf = bytearray()
        self.sock = socket.create_connection(("localhost", port))

    def send(self, data):
        try:
            self.sock.sendall(bytes(data))
        except OSError:
            pass


def neighbour(grid, rows, cols, m, enter_edge):
    """The machine on the far side of m's edge. enter_edge is the edge the
    entity wants to ENTER through on the neighbour, i.e. the opposite of the
    edge it left m by. The world is a torus: walking off one edge enters the
    room on the opposite edge, wrapping around at the border, so there is
    always a neighbour."""
    # The edge it LEFT m by is the opposite of the edge it will enter.
    left_edge = OPPOSITE[enter_edge]
    dr, dc = {EDGE_L: (0, -1), EDGE_R: (0, 1),
              EDGE_T: (-1, 0), EDGE_B: (1, 0)}[left_edge]
    nr = (m.row + dr) % rows
    nc = (m.col + dc) % cols
    return grid[(nr, nc)]


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--rows", type=int, required=True)
    ap.add_argument("--cols", type=int, required=True)
    ap.add_argument("ports", type=int, nargs="+")
    args = ap.parse_args()

    if len(args.ports) != args.rows * args.cols:
        sys.exit(f"need {args.rows*args.cols} ports, got {len(args.ports)}")

    machines = []
    grid = {}
    for i, port in enumerate(args.ports):
        r, c = divmod(i, args.cols)
        try:
            m = Machine(i, port, r, c)
        except OSError as e:
            sys.exit(f"port {port}: {e}")
        machines.append(m)
        grid[(r, c)] = m

    # Tell each machine its room id (its index), which the engine waits for.
    for m in machines:
        m.send([m.index])
    print(f"world: {args.rows}x{args.cols}, {len(machines)} machines on "
          f"ports {', '.join(str(m.port) for m in machines)}")

    lock = threading.Lock()

    def handle_packet(m, kind, edge, coord, vx, vy):
        # The world wraps, so every edge has a neighbour (the opposite room).
        dest = neighbour(grid, args.rows, args.cols, m, edge)
        with lock:
            print(f"[{m.port}] {NAMES.get(kind, '?')} -> [{dest.port}] "
                  f"(enters edge {edge})")
        if kind == K_PLAYER:
            dest.send([SYNC, kind, edge, coord])
        else:
            dest.send([SYNC, kind, edge, coord, vx, vy])

    def relay(m):
        """Read a machine's link, parse packets, route them."""
        state = 0
        kind = edge = coord = vx = vy = 0
        while True:
            try:
                chunk = m.sock.recv(4096)
            except OSError:
                return
            if not chunk:
                return
            for b in chunk:
                if state == 0:
                    if b == SYNC:
                        state = 1
                elif state == 1:
                    kind = b; state = 2
                elif state == 2:
                    edge = b; state = 3
                elif state == 3:
                    coord = b
                    if kind == K_PLAYER:
                        handle_packet(m, kind, edge, coord, 0, 0)
                        state = 0
                    else:
                        state = 4  # a mover: two velocity bytes follow
                elif state == 4:
                    vx = b; state = 5
                elif state == 5:
                    vy = b
                    handle_packet(m, kind, edge, coord, vx, vy)
                    state = 0

    for m in machines:
        threading.Thread(target=relay, args=(m,), daemon=True).start()

    try:
        threading.Event().wait()
    except (EOFError, KeyboardInterrupt):
        pass


if __name__ == "__main__":
    main()
