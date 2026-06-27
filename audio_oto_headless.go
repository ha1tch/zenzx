//go:build headless

package main

import "sync"

// ============================================================================
// Headless audio (silent)
//
// The real Oto-backed AudioManagerOto (audio_oto.go) opens a platform audio
// device via cgo/purego, which is unavailable in a headless environment.
// This stub provides an AudioManagerOto with the exact field and method
// surface that audio_selector.go's AudioWrapper depends on, but performs no
// audio output. The AY chip is still instantiated so register reads/writes
// from the emulation core behave normally; only sound playback is dropped.
// ============================================================================

type AudioManagerOto struct {
	ay      *AYChip
	ayMutex sync.RWMutex

	masterVolume float32
	beeperVolume float32
	ayVolume     float32

	enabled bool
}

func NewAudioManagerOto() *AudioManagerOto {
	return &AudioManagerOto{
		ay:           NewAYChip(1773400),
		masterVolume: 1.0,
		beeperVolume: 1.0,
		ayVolume:     1.0,
		enabled:      false,
	}
}

func (am *AudioManagerOto) Initialize() error { return nil }
func (am *AudioManagerOto) Close()            {}

func (am *AudioManagerOto) UpdateSpeaker(speaker bool, cycles uint64) {}
func (am *AudioManagerOto) UpdateCPUCycle(cycles uint64)              {}

func (am *AudioManagerOto) GetBufferStatus() (level float32, samples int, requested int) {
	return 0, 0, 0
}
func (am *AudioManagerOto) GetDebugInfo() (speakerChanges int, cpuCycle uint64) { return 0, 0 }

func (am *AudioManagerOto) SetMasterVolume(volume float32) { am.masterVolume = volume }
func (am *AudioManagerOto) SetBeeperVolume(volume float32) { am.beeperVolume = volume }
func (am *AudioManagerOto) SetAYVolume(volume float32)     { am.ayVolume = volume }
func (am *AudioManagerOto) SetEnabled(enabled bool)        { am.enabled = enabled }
func (am *AudioManagerOto) IsEnabled() bool                { return am.enabled }
func (am *AudioManagerOto) GetAY() *AYChip                 { return am.ay }
