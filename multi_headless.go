//go:build headless

package zenzx

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const paceHelp = "How fast to run: max (as fast as the computer allows) or real (50 frames a second). " +
	"The default is max for a run of a set number of frames and real for one that runs until interrupted"

const framesHelp = "Number of frames to run; 0 runs until interrupted (Ctrl+C), which is the default with -listen. " +
	"A screenshot of every machine is written when the run ends either way"

// flagGiven reports whether a flag was on the command line.
func flagGiven(fs *flag.FlagSet, name string) bool {
	given := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			given = true
		}
	})
	return given
}

// realPace decides whether a run keeps to 50 frames a second.
func realPace(pace string, frames int) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(pace)) {
	case "":
		return frames == 0, nil
	case "real":
		return true, nil
	case "max":
		return false, nil
	}
	return false, fmt.Errorf("-pace %q: want max or real", pace)
}

// frameClock holds a run to 50 frames a second: wait is called after each
// frame and sleeps until that frame's time has come. A run that has fallen
// behind (the computer was busy, or was asleep) does not rush to catch up.
type frameClock struct {
	next time.Time
}

const frameTime = 20 * time.Millisecond

func (c *frameClock) wait() {
	now := time.Now()
	if c.next.IsZero() || now.Sub(c.next) > 5*frameTime {
		c.next = now
	}
	c.next = c.next.Add(frameTime)
	if d := c.next.Sub(now); d > 0 {
		time.Sleep(d)
	}
}

// interrupted returns a function that reports whether the program has been
// asked to stop (Ctrl+C, or the terminate signal).
func interrupted() func() bool {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	stopped := false
	return func() bool {
		select {
		case <-ch:
			stopped = true
		default:
		}
		return stopped
	}
}

// shotName is the screenshot file of machine i (from 0) of n at a frame:
// the name it always had for a machine on its own, and with the machine's
// number in it when there are several.
func shotName(prefix string, i, n, frame int) string {
	if n == 1 {
		return fmt.Sprintf("%s-frame%06d.png", prefix, frame)
	}
	return fmt.Sprintf("%s-m%d-frame%06d.png", prefix, i+1, frame)
}

// captureMachines writes a screenshot of every machine and returns how many
// were written.
func captureMachines(machines []*ZenZX, dir, prefix string, frame int, quiet bool) int {
	written := 0
	for i, zx := range machines {
		path := filepath.Join(dir, shotName(prefix, i, len(machines), frame))
		if err := writeScreenPNG(path, zx, 1); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", path, err)
			continue
		}
		written++
		logf(quiet, "  frame %d -> %s", frame, path)
	}
	return written
}
