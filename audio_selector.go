package main

// AudioBackend represents which audio system to use
type AudioBackend int

const (
	AudioBackendOto AudioBackend = iota
)

// AudioInterface defines the common interface for audio managers
type AudioInterface interface {
	Initialize() error
	Close()
	UpdateSpeaker(speaker bool, cycles uint64)
	UpdateCPUCycle(cycles uint64)
	GetBufferStatus() (level float32, samples int, requested int)
	GetDebugInfo() (speakerChanges int, cpuCycle uint64)
	SetMasterVolume(volume float32)
	SetBeeperVolume(volume float32)
	SetAYVolume(volume float32)
	SetEnabled(enabled bool)
	IsEnabled() bool
	GetAY() *AYChip
}

// AudioWrapper wraps either raylib or Oto audio implementation
type AudioWrapper struct {
	backend AudioBackend
	oto     *AudioManagerOto // New Oto implementation
}

// NewAudioWrapper creates a new audio wrapper with the specified backend
func NewAudioWrapper(backend AudioBackend) *AudioWrapper {
	wrapper := &AudioWrapper{
		backend: backend,
	}

	switch backend {

	case AudioBackendOto:
		wrapper.oto = NewAudioManagerOto()
	}

	return wrapper
}

// Initialize initializes the audio system
func (aw *AudioWrapper) Initialize() error {
	switch aw.backend {

	case AudioBackendOto:
		if aw.oto != nil {
			return aw.oto.Initialize()
		}
	}
	return nil
}

// Close closes the audio system
func (aw *AudioWrapper) Close() {
	switch aw.backend {

	case AudioBackendOto:
		if aw.oto != nil {
			aw.oto.Close()
		}
	}
}

// UpdateSpeaker updates speaker state
func (aw *AudioWrapper) UpdateSpeaker(speaker bool, cycles uint64) {
	switch aw.backend {

	case AudioBackendOto:
		if aw.oto != nil {
			aw.oto.UpdateSpeaker(speaker, cycles)
		}
	}
}

// UpdateCPUCycle updates CPU cycle count
func (aw *AudioWrapper) UpdateCPUCycle(cycles uint64) {
	switch aw.backend {

	case AudioBackendOto:
		if aw.oto != nil {
			aw.oto.UpdateCPUCycle(cycles)
		}
	}
}

// GetBufferStatus returns buffer status
func (aw *AudioWrapper) GetBufferStatus() (level float32, samples int, requested int) {
	switch aw.backend {

	case AudioBackendOto:
		if aw.oto != nil {
			return aw.oto.GetBufferStatus()
		}
	}
	return 0, 0, 0
}

// GetDebugInfo returns debug info
func (aw *AudioWrapper) GetDebugInfo() (speakerChanges int, cpuCycle uint64) {
	switch aw.backend {

	case AudioBackendOto:
		if aw.oto != nil {
			return aw.oto.GetDebugInfo()
		}
	}
	return 0, 0
}

// SetMasterVolume sets master volume
func (aw *AudioWrapper) SetMasterVolume(volume float32) {
	switch aw.backend {

	case AudioBackendOto:
		if aw.oto != nil {
			aw.oto.SetMasterVolume(volume)
		}
	}
}

// SetBeeperVolume sets beeper volume
func (aw *AudioWrapper) SetBeeperVolume(volume float32) {
	switch aw.backend {

	case AudioBackendOto:
		if aw.oto != nil {
			aw.oto.SetBeeperVolume(volume)
		}
	}
}

// SetAYVolume sets AY chip volume
func (aw *AudioWrapper) SetAYVolume(volume float32) {
	switch aw.backend {

	case AudioBackendOto:
		if aw.oto != nil {
			aw.oto.SetAYVolume(volume)
		}
	}
}

// SetEnabled enables/disables audio
func (aw *AudioWrapper) SetEnabled(enabled bool) {
	switch aw.backend {

	case AudioBackendOto:
		if aw.oto != nil {
			aw.oto.SetEnabled(enabled)
		}
	}
}

// IsEnabled returns if audio is enabled
func (aw *AudioWrapper) IsEnabled() bool {
	switch aw.backend {

	case AudioBackendOto:
		if aw.oto != nil {
			return aw.oto.enabled
		}
	}
	return false
}

// GetAY returns the AY chip
func (aw *AudioWrapper) GetAY() *AYChip {
	switch aw.backend {

	case AudioBackendOto:
		if aw.oto != nil {
			return aw.oto.ay
		}
	}
	return nil
}

// WriteAYRegister writes to AY register with proper locking
func (aw *AudioWrapper) WriteAYRegister(reg, value uint8) {
	switch aw.backend {

	case AudioBackendOto:
		if aw.oto != nil && aw.oto.ay != nil {
			aw.oto.ayMutex.Lock()
			aw.oto.ay.WriteRegister(reg, value)
			aw.oto.ayMutex.Unlock()
		}
	}
}

// ReadAYRegister reads from AY register with proper locking
func (aw *AudioWrapper) ReadAYRegister(reg uint8) uint8 {
	switch aw.backend {

	case AudioBackendOto:
		if aw.oto != nil && aw.oto.ay != nil {
			aw.oto.ayMutex.RLock()
			defer aw.oto.ayMutex.RUnlock()
			return aw.oto.ay.ReadRegister(reg)
		}
	}
	return 0xFF
}

// Direct property access helpers for compatibility
func (aw *AudioWrapper) GetMasterVolume() float32 {
	switch aw.backend {

	case AudioBackendOto:
		if aw.oto != nil {
			return aw.oto.masterVolume
		}
	}
	return 0
}

func (aw *AudioWrapper) GetBeeperVolume() float32 {
	switch aw.backend {

	case AudioBackendOto:
		if aw.oto != nil {
			return aw.oto.beeperVolume
		}
	}
	return 0
}

func (aw *AudioWrapper) GetAYVolume() float32 {
	switch aw.backend {

	case AudioBackendOto:
		if aw.oto != nil {
			return aw.oto.ayVolume
		}
	}
	return 0
}
