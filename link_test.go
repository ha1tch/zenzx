package zenzx

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"
)

// takeByte does what the receiving program does for one byte and reports
// it, or ok false if none is ready.
func takeByte(t *testing.T, l *Link) (b uint8, ok bool) {
	t.Helper()
	if c, _ := l.readPort(LinkControlPort); c != 1 {
		return 0, false
	}
	b, _ = l.readPort(LinkDataPort)
	l.writePort(LinkControlPort, 3)
	if c, _ := l.readPort(LinkControlPort); c != 2 {
		t.Fatalf("after the acknowledge the control port reads %d, want 2", c)
	}
	l.writePort(LinkControlPort, 0)
	return b, true
}

func TestLinkHandshakeDeliversEachByteOnce(t *testing.T) {
	l := NewLink()
	if c, ok := l.readPort(LinkControlPort); !ok || c != 0 {
		t.Fatalf("an idle link's control port reads %d (answered %v), want 0", c, ok)
	}
	l.Send([]byte("hello speccy"))
	if n := l.Pending(); n != 12 {
		t.Errorf("%d bytes pending, want 12", n)
	}
	var got []byte
	for {
		b, ok := takeByte(t, l)
		if !ok {
			break
		}
		got = append(got, b)
	}
	if string(got) != "hello speccy" {
		t.Errorf("the machine received %q", got)
	}
	if n := l.Pending(); n != 0 {
		t.Errorf("%d bytes pending after all were taken", n)
	}
	if r, _ := l.Counts(); r != 12 {
		t.Errorf("the link counts %d bytes received, want 12", r)
	}
}

func TestLinkHoldsEachStepUntilTheMachineMoves(t *testing.T) {
	l := NewLink()
	l.Send([]byte{0x41, 0x42})
	for i := 0; i < 3; i++ {
		if c, _ := l.readPort(LinkControlPort); c != 1 {
			t.Fatalf("read %d of the control port gave %d, want 1 until acknowledged", i, c)
		}
		if d, _ := l.readPort(LinkDataPort); d != 0x41 {
			t.Fatalf("the data port reads %#x, want 0x41 until acknowledged", d)
		}
	}
	l.writePort(LinkControlPort, 0) // not an acknowledge: nothing moves
	if c, _ := l.readPort(LinkControlPort); c != 1 {
		t.Errorf("a write without the acknowledge bit moved the handshake to %d", c)
	}
	l.writePort(LinkControlPort, 3)
	l.writePort(LinkControlPort, 3) // twice is once
	for i := 0; i < 3; i++ {
		if c, _ := l.readPort(LinkControlPort); c != 2 {
			t.Fatalf("the control port reads %d, want 2 until the acknowledge is cleared", c)
		}
	}
	l.writePort(LinkControlPort, 0)
	if c, _ := l.readPort(LinkControlPort); c != 1 {
		t.Errorf("the second byte is not offered: control reads %d", c)
	}
	if d, _ := l.readPort(LinkDataPort); d != 0x42 {
		t.Errorf("the second byte reads %#x, want 0x42", d)
	}
}

func TestLinkAnswersOnlyItsOwnPorts(t *testing.T) {
	l := NewLink()
	for _, p := range []uint16{0x00FE, 0x7FFD, 0x4002, 0x4101, 0xC000, 0x0000} {
		if _, ok := l.readPort(p); ok {
			t.Errorf("the link answered a read of port %#04x", p)
		}
		if l.writePort(p, 0xFF) {
			t.Errorf("the link took a write to port %#04x", p)
		}
	}
	if err := l.SetPorts(0x00E7, 0x00E7); err == nil {
		t.Error("the same port for data and control was accepted")
	}
	if err := l.SetPorts(0xFF3F, 0xFE3F); err != nil {
		t.Fatal(err)
	}
	if _, ok := l.readPort(LinkDataPort); ok {
		t.Error("the link still answers its old data port")
	}
	if _, ok := l.readPort(0xFF3F); !ok {
		t.Error("the link does not answer its new data port")
	}
}

func TestLinkResetOffersAnUnacknowledgedByteAgain(t *testing.T) {
	l := NewLink()
	l.Send([]byte("ab"))
	l.readPort(LinkControlPort) // 'a' is offered, not acknowledged
	l.Reset()
	if b, ok := takeByte(t, l); !ok || b != 'a' {
		t.Errorf("after a reset the first byte taken is %q (ok %v), want 'a'", b, ok)
	}
	// Reset between the acknowledge and its release: the byte was taken.
	l.readPort(LinkControlPort)
	l.writePort(LinkControlPort, 3)
	l.Reset()
	if _, ok := takeByte(t, l); ok {
		t.Error("an acknowledged byte was offered again after a reset")
	}
}

func TestLinkKeepsOutputUntilAHostConnects(t *testing.T) {
	l := NewLink()
	for _, b := range []byte("ready") {
		l.writePort(LinkDataPort, b)
	}
	if got := l.Receive(); string(got) != "ready" {
		t.Errorf("Receive gave %q", got)
	}
	if got := l.Receive(); len(got) != 0 {
		t.Errorf("a second Receive gave %q", got)
	}
	for i := 0; i < linkOutLimit+10; i++ {
		l.writePort(LinkDataPort, uint8(i))
	}
	if got := l.Receive(); len(got) != linkOutLimit || got[len(got)-1] != uint8((linkOutLimit+9)&0xFF) {
		t.Errorf("kept %d bytes ending %#x, want the newest %d", len(got), got[len(got)-1], linkOutLimit)
	}
}

func TestLinkOverTCP(t *testing.T) {
	l := NewLink()
	l.writePort(LinkDataPort, '>') // written before anyone is listening
	if err := l.Listen("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if err := l.Listen("127.0.0.1:0"); err == nil {
		t.Error("a second Listen on one link was accepted")
	}
	conn, err := net.Dial("tcp", l.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("hello speccy")); err != nil {
		t.Fatal(err)
	}

	// The machine's side, polling as a program would.
	var got []byte
	deadline := time.Now().Add(5 * time.Second)
	for len(got) < 12 && time.Now().Before(deadline) {
		if b, ok := takeByte(t, l); ok {
			got = append(got, b)
			l.writePort(LinkDataPort, b-32*boolByte(b >= 'a' && b <= 'z')) // echo in capitals
		} else {
			time.Sleep(time.Millisecond)
		}
	}
	if string(got) != "hello speccy" {
		t.Fatalf("the machine received %q", got)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	back := make([]byte, 13)
	if _, err := io.ReadFull(conn, back); err != nil {
		t.Fatalf("reading the machine's bytes: %v (got %q)", err, back)
	}
	if !bytes.Equal(back, []byte(">HELLO SPECCY")) {
		t.Errorf("the host received %q", back)
	}
}

func boolByte(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}

func TestLinkCloseDisconnectsHosts(t *testing.T) {
	l := NewLink()
	if err := l.Listen("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	addr := l.Addr()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.Write([]byte("x"))
	deadline := time.Now().Add(5 * time.Second)
	for l.Pending() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	l.Close()
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Read(make([]byte, 1)); err == nil {
		t.Error("the host's connection is still open after Close")
	}
	if l.Addr() != "" {
		t.Error("a closed link still reports an address")
	}
	if _, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
		t.Error("a closed link still accepts connections")
	}
	l.Send([]byte("y")) // still usable directly
	if l.Pending() != 2 {
		t.Errorf("%d bytes pending after Close, want 2", l.Pending())
	}
}
