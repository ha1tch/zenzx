// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"path/filepath"
	"sort"
	"time"
)

// The animated GIF: the next two seconds of the machine, 25 frames a second, which is 50
// frames, one for every other frame the machine draws (it draws 50 a second). The frames are
// counted in the machine's own frames and not in the wall clock's, so a machine that is paused
// records nothing until it plays, and one that runs fast or slow plays back at the speed it
// has on the screen at 50 a second.
//
// The recording runs inside the machine's frame loop (shotFrame, called as each frame ends) and
// the encoding, which is the slow part, runs on its own goroutine. What it made comes back through
// TakeShotResults, which the window calls once a frame, so a toast is made on the window's thread.

const (
	// GIFFrames is how many frames a recording holds: two seconds at 25 a second.
	GIFFrames = 50
	// gifEvery is how many of the machine's frames go for each one of the recording's.
	gifEvery = 2
	// gifDelay is the time of a frame in the GIF, in hundredths of a second: 25 a second.
	gifDelay = 4
)

// ShotResult is what a recording that has finished made.
type ShotResult struct {
	Format ShotFormat
	Path   string
	Err    error
}

// gifRecorder is a recording under way.
type gifRecorder struct {
	border  bool
	machine string
	started time.Time
	frames  []*image.RGBA
	tick    int // the machine's frames since it began
}

// shotState is a machine's screenshots under way: the one recording it can have at a time, and
// the results of those that finished.
type shotState struct {
	rec     *gifRecorder
	results chan ShotResult
}

// StartGIF begins recording the next two seconds, as a screenshot of machine started at now.
// It returns false when a recording is under way already.
func (zx *ZenZX) StartGIF(border bool, now time.Time) bool {
	if zx.shots.rec != nil {
		return false
	}
	if zx.shots.results == nil {
		zx.shots.results = make(chan ShotResult, 8)
	}
	zx.shots.rec = &gifRecorder{border: border, machine: zx.ShotMachineName(), started: now}
	return true
}

// GIFRecording says how many of the recording's frames are in, when there is one.
func (zx *ZenZX) GIFRecording() (have int, recording bool) {
	if r := zx.shots.rec; r != nil {
		return len(r.frames), true
	}
	return 0, false
}

// CancelGIF drops a recording under way.
func (zx *ZenZX) CancelGIF() { zx.shots.rec = nil }

// TakeShotResults is what the recordings that have finished since the last call made.
func (zx *ZenZX) TakeShotResults() []ShotResult {
	var out []ShotResult
	for {
		select {
		case r := <-zx.shots.results:
			out = append(out, r)
		default:
			return out
		}
	}
}

// announceShots says, as a notice, what the recordings that finished since it was last called
// made. The window calls it every frame, paused or not, and shotFrame calls it as well, so that a
// machine without a window says it too.
func (zx *ZenZX) announceShots() {
	for _, r := range zx.TakeShotResults() {
		if r.Err != nil {
			zx.shotSay("The GIF could not be saved: %v", r.Err)
		} else {
			zx.shotSay("Saved %s", filepath.Base(r.Path))
		}
	}
}

// shotFrame is called as each of the machine's frames ends: it takes the frame for the recording
// when it is one of the frames that count, and hands the recording to the encoder when it is full.
func (zx *ZenZX) shotFrame() {
	zx.announceShots()
	r := zx.shots.rec
	if r == nil {
		return
	}
	if r.tick%gifEvery == 0 {
		if img := zx.ShotImage(r.border); img != nil {
			r.frames = append(r.frames, fitFrame(img, img.Bounds().Dx(), img.Bounds().Dy())) // a copy: the decoder may reuse its image
		}
	}
	r.tick++
	if len(r.frames) < GIFFrames {
		return
	}
	zx.shots.rec = nil
	results := zx.shots.results
	go func() {
		res := ShotResult{Format: ShotGIF}
		data, err := encodeGIF(r.frames, gifDelay)
		if err == nil {
			res.Path, err = writeScreenshotFile(r.machine, r.started, "gif", data)
		}
		res.Err = err
		select {
		case results <- res:
		default: // nobody is collecting: the file is written, which is what matters
		}
	}()
}

// encodeGIF makes an animated GIF of frames, each shown for delay hundredths of a second. The
// frames are not all the same size when the machine changed mode while it was recorded (a Next
// goes from 320 pixels across to 640): each is made the size of the biggest, by whole
// multiples of its pixels when it goes in, and otherwise in the middle of black.
func encodeGIF(frames []*image.RGBA, delay int) ([]byte, error) {
	if len(frames) == 0 {
		return nil, errors.New("there are no frames to make a GIF of")
	}
	w, h := 0, 0
	for _, f := range frames {
		w, h = max(w, f.Bounds().Dx()), max(h, f.Bounds().Dy())
	}
	fitted := make([]*image.RGBA, len(frames))
	for i, f := range frames {
		fitted[i] = fitFrame(f, w, h)
	}
	pal, index := gifPalette(fitted)
	g := &gif.GIF{
		LoopCount: 0, // for ever
		Config:    image.Config{ColorModel: pal, Width: w, Height: h},
	}
	for _, f := range fitted {
		p := image.NewPaletted(f.Bounds(), pal)
		for i, o := 0, 0; i < len(f.Pix); i, o = i+4, o+1 {
			p.Pix[o] = index[uint32(f.Pix[i])<<16|uint32(f.Pix[i+1])<<8|uint32(f.Pix[i+2])]
		}
		g.Image = append(g.Image, p)
		g.Delay = append(g.Delay, delay)
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// fitFrame is a copy of img, opaque, w by h: img made bigger by whole multiples of its pixels
// when w and h are such multiples of its size, and otherwise put in the middle of black.
func fitFrame(img *image.RGBA, w, h int) *image.RGBA {
	b := img.Bounds()
	dx, dy := b.Dx(), b.Dy()
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 3; i < len(out.Pix); i += 4 {
		out.Pix[i] = 0xFF
	}
	if dx == 0 || dy == 0 {
		return out
	}
	fx, fy := w/dx, h/dy
	if fx < 1 || fy < 1 || fx*dx != w || fy*dy != h {
		fx, fy = 1, 1
	}
	ox, oy := (w-fx*dx)/2, (h-fy*dy)/2
	for y := 0; y < fy*dy && y+oy < h; y++ {
		for x := 0; x < fx*dx && x+ox < w; x++ {
			s := (y/fy)*img.Stride + (x/fx)*4
			d := (y+oy)*out.Stride + (x+ox)*4
			out.Pix[d], out.Pix[d+1], out.Pix[d+2] = img.Pix[s], img.Pix[s+1], img.Pix[s+2]
		}
	}
	return out
}

// colourCount is a colour, packed 0xRRGGBB, and how many pixels of the frames have it.
type colourCount struct {
	rgb uint32
	n   int
}

// gifPalette is the palette of the frames' colours, and which of its entries each colour has.
// A picture of up to 256 colours, which is every classic machine's and most of a Next's, is
// kept exactly; one with more is cut down to 256 by the median cut.
func gifPalette(frames []*image.RGBA) (color.Palette, map[uint32]uint8) {
	counts := map[uint32]int{}
	for _, f := range frames {
		for i := 0; i+3 < len(f.Pix); i += 4 {
			counts[uint32(f.Pix[i])<<16|uint32(f.Pix[i+1])<<8|uint32(f.Pix[i+2])]++
		}
	}
	list := make([]colourCount, 0, len(counts))
	for rgb, n := range counts {
		list = append(list, colourCount{rgb, n})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].rgb < list[j].rgb })
	index := make(map[uint32]uint8, len(list))
	var pal color.Palette
	if len(list) <= 256 {
		for i, c := range list {
			pal = append(pal, rgbColour(c.rgb))
			index[c.rgb] = uint8(i)
		}
		return pal, index
	}
	for i, box := range medianCut(list, 256) {
		var r, g, b, total int
		for _, c := range box {
			r += int(c.rgb>>16&0xFF) * c.n
			g += int(c.rgb>>8&0xFF) * c.n
			b += int(c.rgb&0xFF) * c.n
			total += c.n
		}
		pal = append(pal, color.RGBA{R: uint8(r / total), G: uint8(g / total), B: uint8(b / total), A: 0xFF})
		for _, c := range box {
			index[c.rgb] = uint8(i)
		}
	}
	return pal, index
}

func rgbColour(rgb uint32) color.RGBA {
	return color.RGBA{R: uint8(rgb >> 16), G: uint8(rgb >> 8), B: uint8(rgb), A: 0xFF}
}

// channelOf is one channel (0 red, 1 green, 2 blue) of a packed colour.
func channelOf(rgb uint32, ch int) int { return int(rgb >> uint(16-8*ch) & 0xFF) }

// medianCut splits the colours into at most n boxes: the box with the most colours is cut in two
// across its widest channel, where half of its pixels are on each side.
func medianCut(cols []colourCount, n int) [][]colourCount {
	boxes := [][]colourCount{cols}
	for len(boxes) < n {
		best := -1
		for i, b := range boxes {
			if len(b) > 1 && (best < 0 || len(b) > len(boxes[best])) {
				best = i
			}
		}
		if best < 0 {
			break
		}
		b := boxes[best]
		var lo, hi [3]int
		for ch := range lo {
			lo[ch], hi[ch] = 255, 0
			for _, c := range b {
				v := channelOf(c.rgb, ch)
				lo[ch], hi[ch] = min(lo[ch], v), max(hi[ch], v)
			}
		}
		ch := 0
		for i := 1; i < 3; i++ {
			if hi[i]-lo[i] > hi[ch]-lo[ch] {
				ch = i
			}
		}
		sort.Slice(b, func(i, j int) bool {
			vi, vj := channelOf(b[i].rgb, ch), channelOf(b[j].rgb, ch)
			if vi != vj {
				return vi < vj
			}
			return b[i].rgb < b[j].rgb
		})
		total := 0
		for _, c := range b {
			total += c.n
		}
		split, acc := len(b)/2, 0
		for i, c := range b {
			if acc += c.n; acc*2 >= total {
				split = i + 1
				break
			}
		}
		split = min(max(split, 1), len(b)-1)
		boxes[best] = b[:split]
		boxes = append(boxes, b[split:])
	}
	return boxes
}
