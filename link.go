package zenzx

import (
	"errors"
	"fmt"
	"net"
	"sync"
)

// The link: a byte channel between a machine and the world outside it.
//
// A program on the machine sees two I/O ports. A program outside sees a TCP
// socket. Bytes written to the socket are handed to the machine one at a
// time through a four-step handshake that a BASIC program can keep up with;
// bytes the machine writes to its data port go out on the socket as they
// are.
//
//	data port     IN: the byte being offered     OUT: send a byte to the host
//	control port  IN: 0 idle, 1 ready, 2 taken   OUT: bit 1 is the acknowledge
//
// Receiving a byte, as the machine sees it:
//
//  1. the control port reads 1: a byte is on the data port
//  2. the machine reads the data port, then writes 3 to the control port
//  3. the control port reads 2: the host has seen the acknowledge
//  4. the machine writes 0 to the control port, which reads 0 again until
//     the next byte is ready
//
// The two ports are matched on all sixteen address bits and answered before
// anything else in the machine, the ULA and the 128K paging latch included.
// That matters with the default numbers, 16384 and 16385: on the hardware
// an even port is the ULA's and 16385 falls on the 128K's paging port. A
// machine with no link attached is not affected in any way.
const (
	LinkDataPort    = 16384
	LinkControlPort = 16385
)

// linkOutLimit is how many bytes from the machine are kept while no host is
// connected; they are sent to the first one that connects. Past it the
// oldest are dropped.
const linkOutLimit = 64 << 10

type linkState uint8

const (
	linkIdle  linkState = iota // nothing offered: control reads 0
	linkReady                  // a byte is on the data port: control reads 1
	linkTaken                  // the machine acknowledged it: control reads 2
)

// Link is one machine's channel to the outside. Make one with NewLink,
// attach it with ZenZX.AttachLink, and either serve it on a socket with
// Listen or drive it directly with Send and Receive. It is safe to use from
// several goroutines.
type Link struct {
	mu       sync.Mutex
	dataPort uint16
	ctlPort  uint16

	in    []byte    // from the host, not yet offered to the machine
	cur   uint8     // the byte on the data port
	state linkState // where the handshake is
	out   []byte    // from the machine, not yet taken by a host

	ln    net.Listener
	peers map[net.Conn]chan []byte
	done  bool

	received, sent uint64 // bytes the machine has taken, and has written
}

// NewLink returns a link on the default ports, not yet listening.
func NewLink() *Link {
	return &Link{dataPort: LinkDataPort, ctlPort: LinkControlPort, peers: map[net.Conn]chan []byte{}}
}

// SetPorts changes the machine's two I/O ports. They must differ.
func (l *Link) SetPorts(data, control uint16) error {
	if data == control {
		return errors.New("link: the data and control ports must differ")
	}
	l.mu.Lock()
	l.dataPort, l.ctlPort = data, control
	l.mu.Unlock()
	return nil
}

// Ports reports the machine's data and control ports.
func (l *Link) Ports() (data, control uint16) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.dataPort, l.ctlPort
}

// Send queues bytes for the machine, as a host writing to the socket does.
func (l *Link) Send(p []byte) {
	l.mu.Lock()
	l.in = append(l.in, p...)
	l.mu.Unlock()
}

// Receive takes the bytes the machine has written that no host has had.
// With a host connected there are none: they went to the socket.
func (l *Link) Receive() []byte {
	l.mu.Lock()
	defer l.mu.Unlock()
	p := l.out
	l.out = nil
	return p
}

// Pending is how many bytes from the host the machine has not taken yet,
// the one being offered included.
func (l *Link) Pending() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := len(l.in)
	if l.state == linkReady {
		n++
	}
	return n
}

// Counts reports how many bytes the machine has taken from the host and how
// many it has written.
func (l *Link) Counts() (received, sent uint64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.received, l.sent
}

// Reset returns the handshake to idle, as a reset of the machine does. The
// bytes still queued stay queued; a byte that was being offered and had not
// been acknowledged is offered again.
func (l *Link) Reset() {
	l.mu.Lock()
	if l.state == linkReady {
		l.in = append([]byte{l.cur}, l.in...)
	}
	l.state = linkIdle
	l.mu.Unlock()
}

// readPort answers a read of one of the link's ports; ok is false for any
// other port.
func (l *Link) readPort(port uint16) (v uint8, ok bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	switch port {
	case l.dataPort:
		return l.cur, true
	case l.ctlPort:
		if l.state == linkIdle && len(l.in) > 0 {
			l.cur = l.in[0]
			l.in = l.in[1:]
			l.state = linkReady
		}
		return uint8(l.state), true
	}
	return 0, false
}

// writePort takes a write to one of the link's ports; ok is false for any
// other port.
func (l *Link) writePort(port uint16, v uint8) (ok bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	switch port {
	case l.dataPort:
		l.sent++
		l.emit(v)
		return true
	case l.ctlPort:
		ack := v&2 != 0
		switch {
		case ack && l.state == linkReady:
			l.state = linkTaken
			l.received++
		case !ack && l.state == linkTaken:
			l.state = linkIdle
		}
		return true
	}
	return false
}

// emit passes a byte from the machine to every connected host, or keeps it
// for the first to connect. The caller holds the lock. It never waits: a
// host that has stopped reading is dropped rather than allowed to stall
// the machine.
func (l *Link) emit(v uint8) {
	if len(l.peers) == 0 {
		l.out = append(l.out, v)
		if len(l.out) > linkOutLimit {
			l.out = l.out[len(l.out)-linkOutLimit:]
		}
		return
	}
	for conn, ch := range l.peers {
		select {
		case ch <- []byte{v}:
		default:
			delete(l.peers, conn)
			close(ch)
			conn.Close()
		}
	}
}

// Listen serves the link on a TCP address such as "127.0.0.1:16384". A port
// of 0 takes any free one; Addr says which. Several hosts may connect at
// once: what any of them writes goes to the machine in the order it
// arrives, and what the machine writes goes to all of them.
func (l *Link) Listen(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("link: %w", err)
	}
	l.mu.Lock()
	if l.ln != nil || l.done {
		l.mu.Unlock()
		ln.Close()
		return errors.New("link: already listening or closed")
	}
	l.ln = ln
	l.mu.Unlock()
	go l.accept(ln)
	return nil
}

// Addr is the address the link listens on, or "" if it does not.
func (l *Link) Addr() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.ln == nil {
		return ""
	}
	return l.ln.Addr().String()
}

func (l *Link) accept(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return // the listener was closed
		}
		ch := make(chan []byte, 4096)
		l.mu.Lock()
		if l.done {
			l.mu.Unlock()
			conn.Close()
			return
		}
		if len(l.out) > 0 {
			ch <- l.out // what the machine wrote before anyone was there
			l.out = nil
		}
		l.peers[conn] = ch
		l.mu.Unlock()
		go l.writeTo(conn, ch)
		go l.readFrom(conn)
	}
}

// writeTo sends the machine's bytes to one host until its queue is closed.
func (l *Link) writeTo(conn net.Conn, ch chan []byte) {
	for p := range ch {
		if _, err := conn.Write(p); err != nil {
			break
		}
	}
	l.drop(conn)
}

// readFrom queues one host's bytes for the machine until it disconnects.
func (l *Link) readFrom(conn net.Conn) {
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			l.Send(buf[:n])
		}
		if err != nil {
			break
		}
	}
	l.drop(conn)
}

func (l *Link) drop(conn net.Conn) {
	l.mu.Lock()
	if ch, ok := l.peers[conn]; ok {
		delete(l.peers, conn)
		close(ch)
	}
	l.mu.Unlock()
	conn.Close()
}

// Close stops listening and disconnects every host. The link can still be
// driven with Send and Receive afterwards.
func (l *Link) Close() {
	l.mu.Lock()
	l.done = true
	ln := l.ln
	l.ln = nil
	peers := l.peers
	l.peers = map[net.Conn]chan []byte{}
	l.mu.Unlock()
	if ln != nil {
		ln.Close()
	}
	for conn, ch := range peers {
		close(ch)
		conn.Close()
	}
}

// AttachLink connects a link to the machine's I/O ports, or with nil takes
// the machine's link away.
func (zx *ZenZX) AttachLink(l *Link) {
	zx.io.link = l
}

// Link is the machine's link, or nil.
func (zx *ZenZX) Link() *Link {
	return zx.io.link
}
