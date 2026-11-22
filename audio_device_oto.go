//go:build !headless

package zenzx

import (
	"fmt"
	"io"

	"github.com/ebitengine/oto/v3"
)

// The GUI program's AudioDevice: one engine played through its own player on
// the process's oto context (audio_oto.go).

// otoDevice plays one engine through its own player on the shared context.
type otoDevice struct {
	player *oto.Player
}

// NewOtoAudioDevice returns a device that plays through the sound card.
func NewOtoAudioDevice() AudioDevice {
	return &otoDevice{}
}

func (d *otoDevice) Open(src io.Reader) error {
	otoShared.mu.Lock()
	defer otoShared.mu.Unlock()

	if d.player != nil {
		return nil
	}
	ctx, err := otoContext()
	if err != nil {
		sayf("Failed to create Oto context: %v\n", err)
		return err
	}
	if ctx == nil {
		return fmt.Errorf("no audio context")
	}
	if otoShared.suspended {
		if err := ctx.Resume(); err != nil {
			return err
		}
		otoShared.suspended = false
	}
	d.player = ctx.NewPlayer(src)
	d.player.SetBufferSize(2048)
	d.player.Play()
	otoShared.playing++
	return nil
}

func (d *otoDevice) Close() {
	otoShared.mu.Lock()
	defer otoShared.mu.Unlock()

	if d.player == nil {
		return
	}
	d.player.Pause()
	d.player.Close()
	d.player = nil
	otoShared.playing--
	if otoShared.playing == 0 && otoShared.ctx != nil {
		otoShared.ctx.Suspend()
		otoShared.suspended = true
	}
}
