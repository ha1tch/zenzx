package zenzx

import (
	"io"
	"math"
	"os"
	"sync"
	"time"
)

// audioDebug turns on the audio path's diagnostic messages (speaker history truncated, CPU
// cycle counter jumping or going backwards). Set ZENZX_AUDIO_DEBUG to anything to get them.
var audioDebug = os.Getenv("ZENZX_AUDIO_DEBUG") != ""

// ============================================================================
// Audio engine
//
// The engine turns the beeper's and the AY chip's history into float32 samples
// (little-endian, mono, AudioSampleRate) and hands them to whoever reads from
// it. It knows nothing of sound cards: an AudioDevice does the playing, and
// with none set the engine is silent and does no work.
// ============================================================================

// AudioDevice plays what an AudioEngine produces. Open is given the engine as
// a reader of float32 little-endian mono samples at AudioSampleRate and starts
// pulling from it; Close stops. A device may be opened again after Close.
type AudioDevice interface {
	Open(src io.Reader) error
	Close()
}

type AudioEngine struct {
	// Where the sound goes; nil for none (SetDevice)
	device AudioDevice

	// Ring buffer - using simple mutex like Version O for stability
	ringBuffer   [RingBufferSize]float32
	ringWritePos int
	ringReadPos  int
	ringMutex    sync.Mutex

	// Beeper state with history tracking
	speakerState     bool
	speakerHistory   []SpeakerChange
	historyMutex     sync.RWMutex
	lastProcessCycle uint64
	currentCPUCycle  uint64
	cycleMutex       sync.RWMutex

	// AY chip
	ay      *AYChip
	ayMutex sync.RWMutex

	// Volume controls
	masterVolume float32
	beeperVolume float32
	ayVolume     float32

	// State
	enabled  bool
	running  bool
	stopChan chan bool

	// Buffer monitoring
	lastBufferLevel  float32
	samplesGenerated int
	samplesRequested int

	// Output conditioning: a DC blocker removes the offset of the unipolar
	// beeper output, and a lowpass biquad reconstructs the duty-cycle stream,
	// removing the residual high-frequency aliasing that the boxcar
	// (per-sample duty cycle) averaging leaves behind -- the "jagged" edge of
	// the square wave.
	dcBlocker *DCBlocker
	outputLP  *BiquadFilter
	filterOn  bool

	// History cleanup
	lastCleanupCycle uint64
}

// NewAudioEngine makes an engine with sound on and no device.
func NewAudioEngine() *AudioEngine {
	am := &AudioEngine{
		masterVolume:     0.8,
		beeperVolume:     0.5,
		ayVolume:         0.5,
		enabled:          true,
		speakerState:     false,
		speakerHistory:   make([]SpeakerChange, 0, 10000), // Much larger history
		lastProcessCycle: 0,
		currentCPUCycle:  0,
		stopChan:         make(chan bool, 2),
		ringWritePos:     0,
		ringReadPos:      0,
		lastCleanupCycle: 0,
	}

	// Initialize AY chip
	am.ay = NewAYChip(1773400)

	// Output conditioning filters. The lowpass cutoff (14 kHz) sits above the
	// beeper's musically useful range but well below Nyquist (22.05 kHz),
	// rolling off the aliased harmonics that make the square wave sound jagged
	// while leaving its brightness intact. Q = 0.707 is Butterworth (maximally
	// flat passband, no resonant peak). The DC blocker removes the offset of
	// the unipolar 0..volume output so it does not eat headroom or click.
	am.dcBlocker = NewDCBlocker(20.0, float32(AudioSampleRate))
	am.outputLP = NewLowpassBiquad(float32(AudioSampleRate), 14000.0, 0.707)
	am.filterOn = true

	return am
}

// SetDevice sets where the engine's sound goes. It is meant to be called
// before Initialize; nil leaves the engine silent.
func (am *AudioEngine) SetDevice(d AudioDevice) {
	am.device = d
}

// silent reports that nothing would hear the engine, so it need not work.
func (am *AudioEngine) silent() bool {
	return !am.enabled || am.device == nil
}

func (am *AudioEngine) Initialize() error {
	if am.silent() {
		return nil
	}

	if err := am.device.Open(am); err != nil {
		am.enabled = false
		return err
	}

	// Start audio generation goroutines
	am.running = true
	go am.audioGenerationLoop()
	go am.historyCleanupLoop()

	sayf("Oto Audio initialized: rate=%d, buffer=%d, channels=%d, volume=%.2f\n",
		AudioSampleRate, AudioBufferSize, AudioChannels, am.masterVolume)

	return nil
}

// Read implements io.Reader for the AudioDevice
// Keep this SIMPLE and STABLE
func (am *AudioEngine) Read(buf []byte) (int, error) {
	if !am.enabled || !am.running {
		// Fill with silence
		for i := range buf {
			buf[i] = 0
		}
		return len(buf), nil
	}

	// The device asks for bytes, but we work with float32 samples
	samplesNeeded := len(buf) / 4 // 4 bytes per float32
	am.samplesRequested = samplesNeeded

	am.ringMutex.Lock()
	defer am.ringMutex.Unlock()

	// Calculate samples available in ring buffer
	available := (am.ringWritePos - am.ringReadPos) & RingBufferMask

	// Update buffer level for monitoring
	am.lastBufferLevel = float32(available) / float32(RingBufferTarget) * 100.0
	if am.lastBufferLevel > 100.0 {
		am.lastBufferLevel = 100.0
	}

	// Prepare samples
	samples := make([]float32, samplesNeeded)

	if available >= samplesNeeded {
		// We have enough samples
		for i := 0; i < samplesNeeded; i++ {
			samples[i] = am.ringBuffer[am.ringReadPos] * am.masterVolume
			am.ringReadPos = (am.ringReadPos + 1) & RingBufferMask
		}
	} else if available > 0 {
		// Not enough samples - use what we have and pad with zeros (silence)
		for i := 0; i < available; i++ {
			samples[i] = am.ringBuffer[am.ringReadPos] * am.masterVolume
			am.ringReadPos = (am.ringReadPos + 1) & RingBufferMask
		}
		// Rest remains zeros (silence)
	}
	// else: no samples available, samples array is already zeros (silence)

	// Convert float32 samples to bytes (little-endian)
	for i, sample := range samples {
		// Clamp to [-1, 1]
		if sample > 1.0 {
			sample = 1.0
		} else if sample < -1.0 {
			sample = -1.0
		}

		// Convert to bytes using math.Float32bits (same as Version O)
		bits := math.Float32bits(sample)
		buf[i*4] = byte(bits)
		buf[i*4+1] = byte(bits >> 8)
		buf[i*4+2] = byte(bits >> 16)
		buf[i*4+3] = byte(bits >> 24)
	}

	return len(buf), nil
}

func (am *AudioEngine) Close() {
	if !am.enabled {
		return
	}

	// Stop audio generation goroutines
	if am.running {
		am.running = false
		am.stopChan <- true
		am.stopChan <- true
		time.Sleep(20 * time.Millisecond)
	}

	if am.device != nil {
		am.device.Close()
	}
}

// UpdateSpeaker records a speaker state change with cycle-accurate timing
func (am *AudioEngine) UpdateSpeaker(speaker bool, cycles uint64) {
	if am.silent() {
		return
	}

	am.historyMutex.Lock()
	defer am.historyMutex.Unlock()

	if speaker != am.speakerState {
		am.speakerState = speaker
		am.speakerHistory = append(am.speakerHistory, SpeakerChange{
			Cycle: cycles,
			State: speaker,
		})

		if len(am.speakerHistory) > 10000 {
			if audioDebug {
				sayf("WARNING: Truncating speaker history at cycle %d\n", cycles)
			}
			copy(am.speakerHistory, am.speakerHistory[5000:])
			am.speakerHistory = am.speakerHistory[:len(am.speakerHistory)-5000]
		}
	}
}

// audioGenerationLoop continuously generates audio into the ring buffer
func (am *AudioEngine) audioGenerationLoop() {
	ticker := time.NewTicker(500 * time.Microsecond) // 0.5ms - same as Version O
	defer ticker.Stop()

	sayln("Oto Audio: Generation goroutine started")

	tickCount := 0
	lastReportTime := time.Now()

	for am.running {
		select {
		case <-ticker.C:
			tickCount++
			// Report generation rate every second
			if time.Since(lastReportTime) >= time.Second {
				//sayf("Audio generation: %d ticks/sec\n", tickCount)
				tickCount = 0
				lastReportTime = time.Now()
			}
			am.fillRingBuffer()

		case <-am.stopChan:
			sayln("Oto Audio: Generation goroutine stopping")
			return
		}
	}
}

// fillRingBuffer generates audio samples into the ring buffer
func (am *AudioEngine) fillRingBuffer() {
	am.cycleMutex.RLock()
	currentCycle := am.currentCPUCycle
	am.cycleMutex.RUnlock()

	if am.lastProcessCycle == 0 && currentCycle > 0 {
		if currentCycle > uint64(CPUFrequency/10) {
			am.lastProcessCycle = currentCycle - uint64(CPUFrequency/10)
		} else {
			am.lastProcessCycle = 0
		}
	}

	if currentCycle == 0 || currentCycle <= am.lastProcessCycle {
		return
	}

	cyclesPassed := currentCycle - am.lastProcessCycle

	// Sanity check
	maxCyclesBehind := uint64(CPUFrequency)
	if cyclesPassed > maxCyclesBehind {
		am.lastProcessCycle = currentCycle - maxCyclesBehind
		cyclesPassed = maxCyclesBehind
	}

	samplesAvailable := int(float64(cyclesPassed) * float64(AudioSampleRate) / float64(CPUFrequency))

	if samplesAvailable <= 0 {
		return
	}

	am.ringMutex.Lock()
	defer am.ringMutex.Unlock()

	currentFill := (am.ringWritePos - am.ringReadPos) & RingBufferMask
	available := RingBufferSize - currentFill - 1

	if available <= 0 {
		return // Buffer full
	}

	samplesToGenerate := samplesAvailable
	if samplesToGenerate > available {
		samplesToGenerate = available
	}
	if samplesToGenerate > 4096 {
		samplesToGenerate = 4096
	}

	if samplesToGenerate <= 0 {
		return
	}

	cyclesConsumed := uint64(float64(samplesToGenerate) * float64(CPUFrequency) / float64(AudioSampleRate))
	if cyclesConsumed > cyclesPassed {
		cyclesConsumed = cyclesPassed
	}

	// Generate samples
	samples := make([]float32, samplesToGenerate)
	am.generateSamplesFromHistory(samples, am.lastProcessCycle, cyclesConsumed)

	// Output conditioning: block DC, then lowpass to remove the residual
	// high-frequency aliasing left by the per-sample duty-cycle averaging.
	// Both are stateful IIR filters processing the continuous stream in order.
	if am.filterOn {
		for i := 0; i < samplesToGenerate; i++ {
			s := am.dcBlocker.Process(samples[i])
			samples[i] = am.outputLP.Process(s)
		}
	}

	// Write to ring buffer
	for i := 0; i < samplesToGenerate; i++ {
		am.ringBuffer[am.ringWritePos] = samples[i]
		am.ringWritePos = (am.ringWritePos + 1) & RingBufferMask
	}

	am.lastProcessCycle += cyclesConsumed
	am.samplesGenerated = samplesToGenerate
}

// generateSamplesFromHistory generates samples based on speaker history
// Using the EXACT approach from Version O - no filtering, no tricks
func (am *AudioEngine) generateSamplesFromHistory(samples []float32, startCycle uint64, cycleCount uint64) {
	am.historyMutex.RLock()
	historySnapshot := make([]SpeakerChange, len(am.speakerHistory))
	copy(historySnapshot, am.speakerHistory)
	currentSpeakerState := am.speakerState
	am.historyMutex.RUnlock()

	samplesNeeded := len(samples)
	cyclesPerSample := float64(CPUFrequency) / float64(AudioSampleRate)

	for i := 0; i < samplesNeeded; i++ {
		sampleStartCycle := startCycle + uint64(float64(i)*cyclesPerSample)
		sampleEndCycle := startCycle + uint64(float64(i+1)*cyclesPerSample)

		cyclesOn := uint64(0)
		cyclesTotal := sampleEndCycle - sampleStartCycle

		lastState := false
		lastCycle := sampleStartCycle

		// Find initial state from history
		for j := len(historySnapshot) - 1; j >= 0; j-- {
			if historySnapshot[j].Cycle <= sampleStartCycle {
				lastState = historySnapshot[j].State
				break
			}
		}

		if len(historySnapshot) == 0 {
			lastState = currentSpeakerState
		}

		// Process changes within this sample's range
		for _, change := range historySnapshot {
			if change.Cycle > sampleStartCycle && change.Cycle <= sampleEndCycle {
				if lastState {
					cyclesOn += change.Cycle - lastCycle
				}
				lastState = change.State
				lastCycle = change.Cycle
			}
		}

		if lastState {
			cyclesOn += sampleEndCycle - lastCycle
		}

		// Calculate duty cycle
		dutyCycle := float32(0.0)
		if cyclesTotal > 0 {
			dutyCycle = float32(cyclesOn) / float32(cyclesTotal)
		}

		// Generate sample based on duty cycle - EXACTLY as Version O
		// Unipolar output (0 to beeperVolume), no filtering
		samples[i] = dutyCycle * am.beeperVolume
	}

	// Mix in AY samples if present - EXACTLY as Version O
	if am.ay != nil && cycleCount > 0 {
		am.ayMutex.RLock()
		aySamples := am.ay.GenerateN(samplesNeeded, AudioSampleRate)
		am.ayMutex.RUnlock()

		for i := 0; i < samplesNeeded && i < len(aySamples); i++ {
			samples[i] += aySamples[i] * am.ayVolume
		}
	}
}

// historyCleanupLoop periodically cleans up old speaker history
func (am *AudioEngine) historyCleanupLoop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	sayln("Oto Audio: History cleanup goroutine started")

	for am.running {
		select {
		case <-ticker.C:
			am.cycleMutex.RLock()
			currentCycle := am.currentCPUCycle
			am.cycleMutex.RUnlock()

			if currentCycle > uint64(CPUFrequency) {
				am.historyMutex.Lock()
				cutoffCycle := currentCycle - uint64(CPUFrequency)
				newStart := 0
				for i, change := range am.speakerHistory {
					if change.Cycle >= cutoffCycle {
						newStart = i
						break
					}
				}
				if newStart > 0 {
					am.speakerHistory = am.speakerHistory[newStart:]
				}
				am.historyMutex.Unlock()
			}

		case <-am.stopChan:
			sayln("Oto Audio: History cleanup goroutine stopping")
			return
		}
	}
}

// UpdateCPUCycle updates the current CPU cycle count
func (am *AudioEngine) UpdateCPUCycle(cycles uint64) {
	if am.silent() {
		return
	}

	am.cycleMutex.Lock()
	oldCycle := am.currentCPUCycle
	am.currentCPUCycle = cycles
	if cycles == 0 {
		am.lastProcessCycle = 0
	}
	// Cycle jumps and resets, reported only when asked for: a machine that resets itself,
	// as the Next does when its firmware hands over, would otherwise print here while the
	// audio lock is held.
	if audioDebug && oldCycle > 0 && cycles < oldCycle {
		sayf("WARNING: CPU cycles went backwards! %d -> %d\n", oldCycle, cycles)
	}
	if audioDebug && oldCycle > 0 && cycles > oldCycle+CPUFrequency {
		sayf("WARNING: Large CPU cycle jump! %d -> %d (delta=%d)\n",
			oldCycle, cycles, cycles-oldCycle)
	}
	am.cycleMutex.Unlock()
}

// GetBufferStatus returns the current buffer level and health
func (am *AudioEngine) GetBufferStatus() (level float32, samples int, requested int) {
	if !am.enabled {
		return 0, 0, 0
	}

	am.ringMutex.Lock()
	available := (am.ringWritePos - am.ringReadPos) & RingBufferMask
	am.ringMutex.Unlock()

	bufferLevel := float32(available) / float32(RingBufferTarget) * 100.0
	if bufferLevel > 100.0 {
		bufferLevel = 100.0
	}

	return bufferLevel, am.samplesGenerated, am.samplesRequested
}

// GetDebugInfo returns additional debug information
func (am *AudioEngine) GetDebugInfo() (speakerChanges int, cpuCycle uint64) {
	am.historyMutex.RLock()
	changes := len(am.speakerHistory)
	am.historyMutex.RUnlock()

	am.cycleMutex.RLock()
	cycle := am.currentCPUCycle
	am.cycleMutex.RUnlock()

	return changes, cycle
}

// SetMasterVolume sets the master volume (0.0 to 1.0)
func (am *AudioEngine) SetMasterVolume(volume float32) {
	if volume < 0 {
		volume = 0
	} else if volume > 1 {
		volume = 1
	}
	am.masterVolume = volume
}

// SetAudioFilter enables or disables the output anti-alias filtering (DC
// blocker + lowpass). On by default; disabling yields the raw duty-cycle
// output for comparison or for a deliberately harsher sound.
func (am *AudioEngine) SetAudioFilter(on bool) {
	am.filterOn = on
	if !on && am.dcBlocker != nil {
		am.dcBlocker.Reset()
		am.outputLP.Reset()
	}
}

// SetBeeperVolume sets the beeper volume (0.0 to 1.0)
func (am *AudioEngine) SetBeeperVolume(volume float32) {
	if volume < 0 {
		volume = 0
	} else if volume > 1 {
		volume = 1
	}
	am.beeperVolume = volume
}

// SetAYVolume sets the AY chip volume (0.0 to 1.0)
func (am *AudioEngine) SetAYVolume(volume float32) {
	if volume < 0 {
		volume = 0
	} else if volume > 1 {
		volume = 1
	}
	am.ayVolume = volume
}

// SetEnabled enables or disables audio
func (am *AudioEngine) SetEnabled(enabled bool) {
	if am.enabled == enabled {
		return
	}

	am.enabled = enabled
	if enabled {
		am.Initialize()
	} else {
		am.Close()
	}
}

// IsEnabled returns whether audio is enabled
func (am *AudioEngine) IsEnabled() bool {
	return am.enabled
}

// GetAY returns the AY chip instance
func (am *AudioEngine) GetAY() *AYChip {
	return am.ay
}
