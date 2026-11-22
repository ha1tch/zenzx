package zenzx

// ============================================================================
// The machine's display
//
// A machine draws through a Display. The GUI program's is DisplayManager
// (display.go), which uploads the active VideoRenderer's output to a texture
// and draws it scaled in a window. A machine starts with nullDisplay, which
// draws nothing: whoever wants a picture without a window calls
// zx.DecodeDisplay() and gets the pixels (see writeScreenPNG in scheduler.go).
// SetDisplay (zenzx.go) gives a machine another one.
// ============================================================================

// Display is what the machine asks of whatever shows its picture.
type Display interface {
	// Render draws the current frame.
	Render(paused bool, mem *SpectrumMemory, screen *SpectrumScreen, io *SpectrumIO)
	// SetBorderChanges hands over the finished frame's border history.
	SetBorderChanges(changes []BorderChange, frameOrigin uint64, cyclesPerFrame int)
	// UpdateWindowSize is called once a frame, before Render.
	UpdateWindowSize()
	// SetAudioManager, SetSpectrumIO and SetVideoRenderer tell the display
	// about the machine it is showing; SetDisplay calls all three.
	SetAudioManager(audio *AudioWrapper)
	SetSpectrumIO(io *SpectrumIO)
	SetVideoRenderer(r VideoRenderer)
}

// nullDisplay is the display of a machine nobody is watching.

type nullDisplay struct {
	screen         *SpectrumScreen
	borderColor    uint8
	borderChanges  []BorderChange
	frameOrigin    uint64
	cyclesPerFrame int
}

func newNullDisplay(screen *SpectrumScreen) *nullDisplay {
	return &nullDisplay{
		screen:        screen,
		borderChanges: make([]BorderChange, 0),
	}
}

// Render draws nothing. Screenshot capture is driven explicitly by the
// headless main and scheduler via zx.DecodeDisplay().
func (dm *nullDisplay) Render(paused bool, mem *SpectrumMemory, screen *SpectrumScreen, io *SpectrumIO) {
}

// SetBorderChanges records the frame's border history, as the GUI manager
// does; nothing is drawn from it.
func (dm *nullDisplay) SetBorderChanges(changes []BorderChange, frameOrigin uint64, cyclesPerFrame int) {
	dm.borderChanges = changes
	dm.frameOrigin = frameOrigin
	dm.cyclesPerFrame = cyclesPerFrame
	if n := len(changes); n > 0 {
		dm.borderColor = changes[n-1].Color
	}
}

// UpdateWindowSize does nothing: there is no window.
func (dm *nullDisplay) UpdateWindowSize() {}

// SetAudioManager does nothing. The GUI manager uses the audio reference
// only for its debug overlay.
func (dm *nullDisplay) SetAudioManager(audio *AudioWrapper) {}

// SetVideoRenderer does nothing: there is no window or texture to resize. zx.DecodeDisplay() already calls the active renderer
// directly regardless of what DisplayManager knows.
func (dm *nullDisplay) SetVideoRenderer(r VideoRenderer) {}

// SetSpectrumIO does nothing: the GUI manager's copy exists only to gate
// the 640x256 Layer 2 mode's minimum window zoom, which has no meaning
// without a window.
func (dm *nullDisplay) SetSpectrumIO(io *SpectrumIO) {}
