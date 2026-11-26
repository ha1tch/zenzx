#!/usr/bin/env python3
"""Send text to a machine's link and print what it sends back.

Start a machine that listens and runs one of the programs beside this file:

    zenzx -listen 16384 -autorun examples/link/echo.bas

then, from another terminal:

    python3 examples/link/send.py "hello speccy"

A BASIC program takes a few bytes a second at the machine's own speed, so
this waits for the reply rather than expecting it at once.
"""
import socket
import sys

text = sys.argv[1] if len(sys.argv) > 1 else "hello speccy"
port = int(sys.argv[2]) if len(sys.argv) > 2 else 16384
wait = float(sys.argv[3]) if len(sys.argv) > 3 else 10.0

with socket.create_connection(("localhost", port)) as sock:
    sock.sendall(text.encode("ascii"))
    sock.settimeout(wait)
    reply = b""
    try:
        while len(reply) < len(text):
            chunk = sock.recv(4096)
            if not chunk:
                break
            reply += chunk
    except socket.timeout:
        pass
    print(reply.decode("ascii", "replace"))
