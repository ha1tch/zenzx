//go:build !headless

package zenzx

import (
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
)

// The sound card, through oto: the context every oto device in the process
// shares. The device is in audio_device_oto.go, the engine it plays in
// audio_engine.go.

const (
	AudioFormat   = oto.FormatFloat32LE
	OtoBufferSize = 512 // Samples per buffer (low latency)
)

// oto allows one context in a process, so every device shares it: it is made
// by the first Open and suspended while no device is playing.
var otoShared struct {
	mu        sync.Mutex
	ctx       *oto.Context
	err       error
	tried     bool
	playing   int
	suspended bool
}

// otoContext returns the process's context, making it on first use.
// The caller holds otoShared.mu.
func otoContext() (*oto.Context, error) {
	if otoShared.tried {
		return otoShared.ctx, otoShared.err
	}
	otoShared.tried = true
	op := &oto.NewContextOptions{
		SampleRate:   AudioSampleRate,
		ChannelCount: AudioChannels,
		Format:       AudioFormat,
		BufferSize:   time.Duration(OtoBufferSize) * time.Second / AudioSampleRate,
	}
	ctx, ready, err := oto.NewContext(op)
	if err != nil {
		otoShared.err = err
		return nil, err
	}
	<-ready
	otoShared.ctx = ctx
	return ctx, nil
}
