package main

// ============================================================================
// Audio Constants
// ============================================================================

const (
	// Core audio parameters
	AudioSampleRate = 44100 // 44.1kHz sample rate
	AudioChannels   = 1     // Mono
	AudioBufferSize = 512   // For raylib compatibility

	// Ring buffer size - smaller now for low latency
	// Must be power of 2 for efficient masking
	RingBufferSize   = 16384              // ~370ms at 44100Hz
	RingBufferMask   = RingBufferSize - 1 // 16383
	RingBufferTarget = RingBufferSize / 2 // Target fill level

)

// ============================================================================
// Speaker Change Tracking
// ============================================================================

// SpeakerChange records when the speaker state changed
type SpeakerChange struct {
	Cycle uint64
	State bool
}
