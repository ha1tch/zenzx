//go:build headless

package zenzx

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

// The connected-world example (examples/world) must keep a single-ownership
// model: exactly one machine holds the player at a time. The classic bug is
// every machine spawning its own hero, so this drives the REAL path -- nine
// machines, links on TCP ports, the actual host.py routing -- and checks that
// precisely one machine's player beacon (0xBFFE) is set. It guards against a
// regression to "every screen owns the hero".
func TestWorldExampleSingleOwner(t *testing.T) {
	tap, err := os.ReadFile("examples/world/world.tap")
	if err != nil {
		t.Skip("examples/world/world.tap not built; skipping")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available; skipping the host-driven check")
	}

	const n = 9
	const base = 16500
	machines := make([]*ZenZX, n)
	for i := 0; i < n; i++ {
		m := newSideMachine(t, "48k", 0, "")
		machines[i] = m.zx
		l := NewLink()
		machines[i].AttachLink(l)
		addr := fmt.Sprintf("127.0.0.1:%d", base+i)
		if err := l.Listen(addr); err != nil {
			t.Skipf("cannot listen on %s (%v); skipping", addr, err)
		}
		if err := machines[i].AutorunTAP("world", tap, "48k"); err != nil {
			t.Fatal(err)
		}
	}

	// Pump all nine machines, as the GUI's runBehind does.
	stop := make(chan struct{})
	go func() {
		tick := time.NewTicker(20 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tick.C:
				for _, zx := range machines {
					zx.RunFrame()
				}
			}
		}
	}()
	defer close(stop)

	time.Sleep(3 * time.Second) // boot + load

	ports := make([]string, n)
	for i := range ports {
		ports[i] = fmt.Sprintf("%d", base+i)
	}
	args := append([]string{"examples/world/host.py", "--rows", "3", "--cols", "3"}, ports...)
	host := exec.Command("python3", args...)
	if err := host.Start(); err != nil {
		t.Fatalf("start host.py: %v", err)
	}
	defer func() { _ = host.Process.Kill(); _ = host.Wait() }()

	time.Sleep(2 * time.Second) // host sends room ids; engines consume them

	owners := 0
	for i := 0; i < n; i++ {
		if machines[i].memory.Read(0xBFFE) != 0 {
			owners++
		}
	}
	if owners != 1 {
		t.Errorf("%d machines own the player, want exactly 1 (every screen spawning its own hero is the regression this guards)", owners)
	}
}

// A mover must cross only through the wall opening and bounce off solid wall
// sections -- not ping-pong at the edge. A wedged mover floods the link with
// crossing packets, which clogged the room and stopped the hero passing. This
// runs a single machine holding one mover for many frames and checks it emits
// at most a few packets, not hundreds.
func TestWorldExampleMoverDoesNotFloodTheLink(t *testing.T) {
	tap, err := os.ReadFile("examples/world/world.tap")
	if err != nil {
		t.Skip("examples/world/world.tap not built; skipping")
	}
	// Rooms 0 (ball), 2 (scissors), 6 (pot) each hold one mover at start.
	for _, room := range []byte{0, 2, 6} {
		m := newSideMachine(t, "48k", 0, "")
		zx := m.zx
		l := NewLink()
		zx.AttachLink(l)
		if err := zx.AutorunTAP("world", tap, "48k"); err != nil {
			t.Fatal(err)
		}
		runUntil(zx, 3000, func() bool { return !zx.AutorunPending() })
		l.Send([]byte{room})
		runUntil(zx, 200, func() bool { return false })
		bytes := 0
		for i := 0; i < 2000; i++ {
			zx.RunFrame()
			bytes += len(l.Receive())
		}
		// One clean crossing is 6 bytes; a flood would be hundreds. Allow a
		// handful of legitimate crossings as the mover tours the room.
		if bytes > 60 {
			t.Errorf("room %d: mover emitted %d bytes over 2000 frames -- looks like an edge ping-pong flood", room, bytes)
		}
	}
}

// A mover that enters a room (via a host packet) must keep moving with the
// velocity the packet carried. The regression this guards: PollLink only read
// the velocity bytes for the ball, so scissors and pot entered with zero
// velocity and sat still, clogging the room. A moving scissors crosses the
// room and leaves the far edge, emitting a packet; a stuck one never does. We
// send a scissors enter-packet (moving right) and check it eventually crosses.
func TestWorldExampleEnteringMoverKeepsMoving(t *testing.T) {
	tap, err := os.ReadFile("examples/world/world.tap")
	if err != nil {
		t.Skip("examples/world/world.tap not built; skipping")
	}
	m := newSideMachine(t, "48k", 0, "")
	zx := m.zx
	l := NewLink()
	zx.AttachLink(l)
	if err := zx.AutorunTAP("world", tap, "48k"); err != nil {
		t.Fatal(err)
	}
	runUntil(zx, 3000, func() bool { return !zx.AutorunPending() })
	l.Send([]byte{4}) // room 4: no scissors at start
	runUntil(zx, 200, func() bool { return false })
	_ = l.Receive() // discard the room-setup traffic

	// SYNC, kind=2 (scissors), edge=0 (enters left, x=0), coord=88, vx=1, vy=0.
	// Moving right across the ~240px room, it should reach and cross the right
	// edge within a few hundred frames and emit a 6-byte packet.
	l.Send([]byte{0xAA, 2, 0, 88, 1, 0})
	crossed := false
	for i := 0; i < 500 && !crossed; i++ {
		zx.RunFrame()
		if len(l.Receive()) > 0 {
			crossed = true
		}
	}
	if !crossed {
		t.Error("the scissors never crossed after entering -- it entered with no velocity and sat still (the clog regression)")
	}
}

// A rook mover turns at the room centre, so over time it leaves by more than
// one axis -- a scissors that started horizontal should eventually exit top or
// bottom, not only left/right. We keep it in play by wrapping its crossings
// back (opposite edge) and collect which edges it leaves by; both a
// horizontal edge (L/R) and a vertical edge (T/B) must appear.
func TestWorldExampleRookTurnsAtCentre(t *testing.T) {
	tap, err := os.ReadFile("examples/world/world.tap")
	if err != nil {
		t.Skip("examples/world/world.tap not built; skipping")
	}
	m := newSideMachine(t, "48k", 0, "")
	zx := m.zx
	l := NewLink()
	zx.AttachLink(l)
	if err := zx.AutorunTAP("world", tap, "48k"); err != nil {
		t.Fatal(err)
	}
	runUntil(zx, 3000, func() bool { return !zx.AutorunPending() })
	l.Send([]byte{2}) // room 2 holds the scissors
	runUntil(zx, 200, func() bool { return false })
	_ = l.Receive()

	var buf []byte
	horiz, vert := false, false
	for i := 0; i < 8000 && !(horiz && vert); i++ {
		zx.RunFrame()
		buf = append(buf, l.Receive()...)
		for len(buf) >= 6 && buf[0] == 0xAA {
			// The engine's packet already names the edge the mover enters the
			// neighbour through (on a torus that is the opposite side). Feed
			// the packet back unchanged so it re-enters and keeps touring,
			// crossing the room again toward the centre.
			edge := buf[2]
			switch edge {
			case 0, 1:
				horiz = true
			case 2, 3:
				vert = true
			}
			pkt := append([]byte(nil), buf[0:6]...)
			buf = buf[6:]
			l.Send(pkt)
		}
		if len(buf) > 0 && buf[0] != 0xAA {
			buf = buf[1:]
		}
	}
	if !(horiz && vert) {
		t.Errorf("rook left only by horizontal=%v vertical=%v edges -- it is not turning at the centre", horiz, vert)
	}
}
