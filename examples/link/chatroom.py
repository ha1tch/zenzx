#!/usr/bin/env python3
"""A chat room for several Spectrums over the link.

Start the machines, each running chat.bas (or one of its variants) on its
own port:

    zenzx -multi 4:48k,48k,128k,plus3 \\
          -listen 16384,16385,16386,16387 \\
          -autorun examples/link/chat.bas,examples/link/chat-ada.bas,\\
examples/link/chat-clive.bas,examples/link/chat-grace.bas

then run this host, which connects to all four:

    python3 examples/link/chatroom.py 16384 16385 16386 16387

A line one Spectrum sends (its program sends "name: message" ending in a
carriage return) is relayed to every other Spectrum and printed here in a
colour of its own. A line typed here goes to all of them, as "host: ...".

Each machine shows what it is sent and sends a line of its own a moment
after it starts; the host ties them together. Nothing here is specific to
four machines -- give as many ports as you started.
"""
import socket
import sys
import threading

# A colour per machine, the host in bold white. The first few match the
# border colours of the chat*.bas clients (cyan, magenta, green, yellow),
# so a machine's line reads the same here as on its own screen.
COLOURS = [36, 35, 32, 33, 34, 31, 91, 92]
RESET = "\033[0m"
HOST_COLOUR = "\033[1;97m"


def colour(i):
    return f"\033[{COLOURS[i % len(COLOURS)]}m"


class Peer:
    """One Spectrum: its socket, and the line it is building up."""

    def __init__(self, index, port):
        self.index = index
        self.port = port
        self.buf = bytearray()
        self.sock = socket.create_connection(("localhost", port))

    def send(self, text):
        # The Spectrum reads bytes one at a time; a carriage return (13)
        # ends a line, as its program expects.
        try:
            self.sock.sendall(text.encode("ascii", "replace") + b"\r")
        except OSError:
            pass


def main():
    ports = [int(a) for a in sys.argv[1:]] or [16384]
    peers = []
    for i, port in enumerate(ports):
        try:
            peers.append(Peer(i, port))
        except OSError as e:
            print(f"port {port}: {e}")
    if not peers:
        sys.exit("no machine to connect to")

    print(f"{HOST_COLOUR}chat room: {len(peers)} machines on ports "
          f"{', '.join(str(p.port) for p in peers)}{RESET}")
    print(f"{HOST_COLOUR}type a line to send to all of them, "
          f"or Ctrl-D to leave{RESET}\n")

    lock = threading.Lock()

    def relay(frm):
        """Read bytes from one machine; on each complete line, print it
        here and pass it to all the others."""
        while True:
            try:
                chunk = frm.sock.recv(4096)
            except OSError:
                return
            if not chunk:
                return
            for b in chunk:
                if b in (13, 10):
                    line = frm.buf.decode("ascii", "replace").strip()
                    frm.buf.clear()
                    if not line:
                        continue
                    with lock:
                        print(f"{colour(frm.index)}[{frm.port}] "
                              f"{line}{RESET}")
                    # Relay to the others. The sender already printed its
                    # own line on its screen as it typed it, so it is not
                    # sent back to itself.
                    for other in peers:
                        if other is not frm:
                            other.send(line)
                else:
                    frm.buf.append(b)

    for p in peers:
        threading.Thread(target=relay, args=(p,), daemon=True).start()

    # The host's own line, read from the terminal, goes to all of them.
    try:
        for line in sys.stdin:
            line = line.rstrip("\n")
            if not line:
                continue
            msg = f"host: {line}"
            with lock:
                print(f"{HOST_COLOUR}{msg}{RESET}")
            for p in peers:
                p.send(msg)
    except (EOFError, KeyboardInterrupt):
        pass
    print(f"\n{HOST_COLOUR}leaving the chat room{RESET}")


if __name__ == "__main__":
    main()
