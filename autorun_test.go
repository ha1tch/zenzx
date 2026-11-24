package zenzx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/zentools/pkg/tap"
)

func TestBasicTape(t *testing.T) {
	image, err := BasicTape("a-very-long-name", "  20 PRINT \"HI\"\n 30 GO TO 20\n")
	if err != nil {
		t.Fatal(err)
	}
	blocks, err := tap.Decode(image)
	if err != nil || len(blocks) != 2 {
		t.Fatalf("the tape decodes to %d blocks (%v), want a header and a data block", len(blocks), err)
	}
	if _, err := BasicTape("x", "PRINT 1\n"); err == nil {
		t.Error("a line without a number was accepted")
	}
	if _, err := BasicTape("x", "\n\n"); err == nil {
		t.Error("an empty program was accepted")
	}
}

func TestAutorunRefusals(t *testing.T) {
	m := newSideMachine(t, "48k", 0, "")
	if err := m.zx.AutorunBASIC("x", "10 PRINT 1\n", "next"); err == nil {
		t.Error("autorun on a Next was accepted")
	}
	if err := m.zx.AutorunFile("no-such-file.bas", "48k"); err == nil {
		t.Error("a missing file was accepted")
	}
	if m.zx.AutorunPending() {
		t.Error("a refused autorun is pending")
	}
}

// runUntil runs frames until done says so or the limit is reached.
func runUntil(zx *ZenZX, limit int, done func() bool) int {
	for f := 0; f < limit; f++ {
		if done() {
			return f
		}
		zx.RunFrame()
	}
	return limit
}

func screenText(zx *ZenZX) string { return strings.Join(ReadScreen(zx), "\n") }

// The receiver program from examples/link, autorun on a 48K and on a 128K,
// is sent a line of text through the link and prints it.
func TestAutorunReceiverPrintsWhatTheLinkSends(t *testing.T) {
	src, err := os.ReadFile("examples/link/receiver.bas")
	if err != nil {
		t.Fatal(err)
	}
	for _, model := range []string{"48k", "128k"} {
		m := newSideMachine(t, model, 0, "")
		zx := m.zx
		link := NewLink()
		zx.AttachLink(link)
		if err := zx.AutorunBASIC("receiver", string(src), model); err != nil {
			t.Fatalf("%s: %v", model, err)
		}
		link.Send([]byte("hello speccy"))
		frames := runUntil(zx, 4000, func() bool { return link.Pending() == 0 && !zx.AutorunPending() })
		runUntil(zx, 25, func() bool { return false }) // let the last PRINT land
		if link.Pending() != 0 {
			t.Fatalf("%s: %d bytes not taken after %d frames\n%s", model, link.Pending(), frames, screenText(zx))
		}
		if text := screenText(zx); !strings.Contains(text, "hello speccy") {
			t.Errorf("%s: the screen does not show the text after %d frames:\n%s", model, frames, text)
		}
		if got := zx.FirstProgram(); got != "receiver" {
			t.Errorf("%s: the machine's first program is %q, want receiver", model, got)
		}
		t.Logf("%s: 12 bytes printed by frame %d", model, frames)
	}
}

// The echo program sends back what it is sent, in capitals.
func TestAutorunEchoAnswersThroughTheLink(t *testing.T) {
	src, err := os.ReadFile("examples/link/echo.bas")
	if err != nil {
		t.Fatal(err)
	}
	m := newSideMachine(t, "48k", 0, "")
	zx := m.zx
	link := NewLink()
	zx.AttachLink(link)
	if err := zx.AutorunBASIC("echo", string(src), "48k"); err != nil {
		t.Fatal(err)
	}
	link.Send([]byte("zx 48k"))
	var back []byte
	runUntil(zx, 4000, func() bool {
		back = append(back, link.Receive()...)
		return len(back) >= 6
	})
	if string(back) != "ZX 48K" {
		t.Errorf("the machine sent back %q, want ZX 48K\n%s", back, screenText(zx))
	}
}

// A 128K's paging port shares the control port's address bits: with a link
// attached, a write to 16385 must not page memory, and without one it must.
func TestLinkPortsTakePriorityOnA128K(t *testing.T) {
	m := newSideMachine(t, "128k", 0, "")
	zx := m.zx
	before := zx.memory.port7FFD
	zx.AttachLink(NewLink())
	zx.io.WritePort(LinkControlPort, 3)
	if zx.memory.port7FFD != before {
		t.Errorf("with a link attached a write to the control port paged memory: 7FFD %#x -> %#x", before, zx.memory.port7FFD)
	}
	zx.AttachLink(nil)
	zx.io.WritePort(LinkControlPort, 3)
	if zx.memory.port7FFD&7 != 3 {
		t.Errorf("without a link a write to port 16385 did not reach the paging latch: 7FFD = %#x", zx.memory.port7FFD)
	}
}

// A prebuilt .tap file autoruns through the same LOAD "" path as BASIC source:
// AutorunFile sees the .tap extension and loads the image straight into the
// deck instead of tokenising it. We build a tiny tape with BasicTape (which
// produces a .tap image), hand it to AutorunFile as a .tap, and check the
// program loads and runs.
func TestAutorunFileLoadsAPrebuiltTAP(t *testing.T) {
	image, err := BasicTape("taptest", "10 PRINT \"from the tape\"\n20 GO TO 20\n")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "taptest.tap")
	if err := os.WriteFile(path, image, 0o644); err != nil {
		t.Fatal(err)
	}
	m := newSideMachine(t, "48k", 0, "")
	zx := m.zx
	if err := zx.AutorunFile(path, "48k"); err != nil {
		t.Fatalf("autorun of a .tap: %v", err)
	}
	frames := runUntil(zx, 4000, func() bool {
		return !zx.AutorunPending() && strings.Contains(screenText(zx), "from the tape")
	})
	if text := screenText(zx); !strings.Contains(text, "from the tape") {
		t.Errorf("the .tap did not run after %d frames:\n%s", frames, text)
	}
	if got := zx.FirstProgram(); got != "taptest" {
		t.Errorf("the machine's first program is %q, want taptest", got)
	}
	t.Logf("prebuilt .tap ran by frame %d", frames)
}
