//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"embed"
	"image"
	"io/fs"
	"math"
	"sort"
	"strconv"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// The figure that waits in a window with no machine in it: it stands on the background, still for
// a long while between gestures: a blink, three taps of a foot, a look to its right, two blinks,
// over and over, as if showing impatience. It goes when a machine is started.
//
// The pictures are the frame*.png files in sprites/wait and what it does with them is sequence.txt
// there (README.txt says how to change them); each picture is enlarged by a whole number, without
// smoothing.

//go:embed sprites/wait
var waitFiles embed.FS

// waitStep is one step of the figure's sequence: a picture, and the seconds it is shown.
type waitStep struct {
	frame int
	secs  float64
}

// waitScript is what the figure does: the steps of an introduction, played once when it appears,
// and then the steps that go round and round.
type waitScript struct {
	intro, loop []waitStep
}

// waitFrameEvery is how long each picture is shown when the folder has no sequence of its own.
const waitFrameEvery = 0.2

// defaultWaitScript shows each of n pictures in turn, for ever.
func defaultWaitScript(n int) waitScript {
	s := waitScript{}
	for i := 0; i < n; i++ {
		s.loop = append(s.loop, waitStep{i, waitFrameEvery})
	}
	return s
}

// at is the picture shown t seconds after the figure appeared.
func (s waitScript) at(t float64) int {
	if len(s.loop) == 0 {
		return 0
	}
	if t < 0 {
		t = 0
	}
	for _, st := range s.intro {
		if t < st.secs {
			return st.frame
		}
		t -= st.secs
	}
	total := 0.0
	for _, st := range s.loop {
		total += st.secs
	}
	t = math.Mod(t, total)
	for _, st := range s.loop {
		if t < st.secs {
			return st.frame
		}
		t -= st.secs
	}
	return s.loop[len(s.loop)-1].frame
}

// parseWaitScript reads sequence.txt for n pictures: a line is a picture's number and the seconds
// it is shown, "#" starts a comment, and a line "loop" divides what is played once from what goes
// round (with no such line, everything goes round). It says no if anything is not as it should be.
func parseWaitScript(text string, n int) (waitScript, bool) {
	var before, after []waitStep
	cur, looped := &before, false
	for _, line := range strings.Split(text, "\n") {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		f := strings.Fields(line)
		switch {
		case len(f) == 0:
		case len(f) == 1 && f[0] == "loop":
			if looped {
				return waitScript{}, false
			}
			looped, cur = true, &after
		case len(f) == 2:
			frame, err1 := strconv.Atoi(f[0])
			secs, err2 := strconv.ParseFloat(f[1], 64)
			if err1 != nil || err2 != nil || frame < 0 || frame >= n || !(secs > 0 && secs <= 60) {
				return waitScript{}, false
			}
			*cur = append(*cur, waitStep{frame, secs})
		default:
			return waitScript{}, false
		}
	}
	s := waitScript{intro: before, loop: after}
	if !looped {
		s = waitScript{loop: before}
	}
	return s, len(s.loop) > 0
}

// waitImages are the pictures, before they are textures, and what the figure does with them.
type waitImages struct {
	frames []*image.RGBA
	script waitScript
}

// loadWaitImages reads every frame*.png of fsys (the embedded folder, or a test's) in the order of
// their names, and its sequence.txt if it has one that is right for them. With one of the
// pictures not a picture it takes none: an animation drawn half by one hand and half by another
// is no animation.
func loadWaitImages(fsys fs.FS) waitImages {
	names, err := fs.Glob(fsys, "frame*.png")
	if err != nil || len(names) == 0 {
		return waitImages{}
	}
	sort.Strings(names)
	frames := make([]*image.RGBA, 0, len(names))
	for _, name := range names {
		img := readSpritePNG(fsys, name)
		if img == nil {
			return waitImages{}
		}
		frames = append(frames, img)
	}
	script := defaultWaitScript(len(frames))
	if data, err := fs.ReadFile(fsys, "sequence.txt"); err == nil {
		if s, ok := parseWaitScript(string(data), len(frames)); ok {
			script = s
		}
	}
	return waitImages{frames: frames, script: script}
}

// embeddedWaitImages are the pictures this build has.
func embeddedWaitImages() waitImages {
	sub, err := fs.Sub(waitFiles, "sprites/wait")
	if err != nil {
		return loadWaitImages(emptyFS{})
	}
	return loadWaitImages(sub)
}

// waitTextures are the pictures on the graphics card, made the first time a window with no
// machine in it is drawn.
type waitTextures struct {
	frames []rl.Texture2D
	script waitScript

	// since is when the figure appeared, and last when it was last drawn: a gap between the two
	// draws is a time with a machine in the window, and the figure starts again after it.
	since, last float64
	drawn       bool
}

func (dm *DisplayManager) waitingTextures() *waitTextures {
	if dm.wait == nil {
		imgs := embeddedWaitImages()
		t := &waitTextures{script: imgs.script}
		for _, img := range imgs.frames {
			tex := rl.LoadTextureFromImage(rl.NewImageFromImage(img))
			rl.SetTextureFilter(tex, rl.FilterPoint)
			t.frames = append(t.frames, tex)
		}
		dm.wait = t
	}
	return dm.wait
}

// frameAt is the picture to draw at time now (seconds, as the window counts them). The figure
// starts from the beginning when it has not been drawn for a moment: it was gone, with a machine
// in the window.
func (w *waitTextures) frameAt(now float64) int {
	if !w.drawn || now-w.last > 0.25 {
		w.since = now
	}
	w.drawn, w.last = true, now
	return w.script.at(now - w.since)
}

// unloadWaiting gives the figure's textures back, before the window closes.
func (dm *DisplayManager) unloadWaiting() {
	if dm.wait != nil {
		for _, tex := range dm.wait.frames {
			rl.UnloadTexture(tex)
		}
		dm.wait = nil
	}
}

// drawWaiting draws the figure in the middle of the window below the bar's band, a whole number
// of times its size, about a third of the window's height. All the pictures stand on the same line.
func (dm *DisplayManager) drawWaiting(top, winW, winH int) {
	if winH <= top {
		return
	}
	tex := dm.waitingTextures()
	if len(tex.frames) == 0 {
		return
	}
	area := winH - top
	maxH := int32(0)
	for _, f := range tex.frames {
		maxH = max(maxH, f.Height)
	}
	s := int32(max(2, min(8, area/3/int(maxH))))
	t := tex.frames[tex.frameAt(rl.GetTime())]
	w, h := t.Width*s, t.Height*s
	x := int32(winW)/2 - w/2
	y := int32(top) + int32(area)/2 + maxH*s/2 - h
	rl.DrawTexturePro(t, rl.Rectangle{Width: float32(t.Width), Height: float32(t.Height)},
		rl.Rectangle{X: float32(x), Y: float32(y), Width: float32(w), Height: float32(h)}, rl.Vector2{}, 0, rl.White)
}
