package main

import (
	"math"
)

// ============================================================================
// Windowed Sinc Interpolation for High-Quality Audio Resampling
// ============================================================================

// SincFilter provides high-quality audio resampling using windowed sinc interpolation
type SincFilter struct {
	taps       int       // Number of filter taps (must be even)
	kernel     []float32 // Pre-computed sinc kernel
	buffer     []float32 // Circular buffer for input samples
	bufferPos  int       // Current position in buffer
	cutoff     float32   // Normalized cutoff frequency (0-1)
}

// NewSincFilter creates a new windowed sinc filter
// taps: number of filter taps (8, 16, 32, 64 for quality/performance tradeoff)
// cutoff: normalized cutoff frequency (typically 0.45-0.495 to prevent aliasing)
func NewSincFilter(taps int, cutoff float32) *SincFilter {
	if taps%2 != 0 {
		taps++ // Ensure even number of taps
	}
	
	sf := &SincFilter{
		taps:   taps,
		buffer: make([]float32, taps),
		cutoff: cutoff,
	}
	
	// Generate windowed sinc kernel
	sf.kernel = sf.generateKernel()
	
	return sf
}

// generateKernel creates a windowed sinc filter kernel
func (sf *SincFilter) generateKernel() []float32 {
	kernel := make([]float32, sf.taps)
	center := float32(sf.taps-1) / 2.0
	sum := float32(0.0)
	
	for i := 0; i < sf.taps; i++ {
		x := float32(i) - center
		
		// Sinc function
		var sincVal float32
		if x == 0 {
			sincVal = 2.0 * sf.cutoff
		} else {
			sincVal = float32(math.Sin(2.0*math.Pi*float64(sf.cutoff*x)) / (math.Pi * float64(x)))
		}
		
		// Apply Blackman window for better stopband attenuation
		window := blackmanWindow(i, sf.taps)
		
		kernel[i] = sincVal * window
		sum += kernel[i]
	}
	
	// Normalize kernel
	for i := range kernel {
		kernel[i] /= sum
	}
	
	return kernel
}

// blackmanWindow applies Blackman window function
func blackmanWindow(n, N int) float32 {
	a0 := float64(0.42)
	a1 := float64(0.5)
	a2 := float64(0.08)
	
	term1 := a0
	term2 := a1 * math.Cos(2.0*math.Pi*float64(n)/float64(N-1))
	term3 := a2 * math.Cos(4.0*math.Pi*float64(n)/float64(N-1))
	
	return float32(term1 - term2 + term3)
}

// Process applies the sinc filter to a single input sample
func (sf *SincFilter) Process(input float32) float32 {
	// Add sample to circular buffer
	sf.buffer[sf.bufferPos] = input
	sf.bufferPos = (sf.bufferPos + 1) % sf.taps
	
	// Convolve with kernel
	output := float32(0.0)
	for i := 0; i < sf.taps; i++ {
		bufIdx := (sf.bufferPos + i) % sf.taps
		output += sf.buffer[bufIdx] * sf.kernel[i]
	}
	
	return output
}

// ProcessBatch processes multiple samples at once (more efficient)
func (sf *SincFilter) ProcessBatch(input []float32) []float32 {
	output := make([]float32, len(input))
	
	for i, sample := range input {
		output[i] = sf.Process(sample)
	}
	
	return output
}

// Reset clears the filter buffer
func (sf *SincFilter) Reset() {
	for i := range sf.buffer {
		sf.buffer[i] = 0
	}
	sf.bufferPos = 0
}

// ============================================================================
// Band-Limited Interpolation for Arbitrary Sample Rate Conversion
// ============================================================================

// BandLimitedInterpolator provides high-quality arbitrary rate conversion
type BandLimitedInterpolator struct {
	oversampleFactor int       // Oversampling factor for interpolation
	sincTaps         int       // Number of sinc taps
	lutSize          int       // Size of lookup table
	lut              []float32 // Sinc lookup table
}

// NewBandLimitedInterpolator creates a new interpolator
// oversampleFactor: internal oversampling (higher = better quality, more CPU)
// sincTaps: number of sinc taps (8-64 typical)
func NewBandLimitedInterpolator(oversampleFactor, sincTaps int) *BandLimitedInterpolator {
	bli := &BandLimitedInterpolator{
		oversampleFactor: oversampleFactor,
		sincTaps:         sincTaps,
		lutSize:          oversampleFactor * sincTaps,
	}
	
	// Generate lookup table
	bli.lut = bli.generateLUT()
	
	return bli
}

// generateLUT creates a sinc interpolation lookup table
func (bli *BandLimitedInterpolator) generateLUT() []float32 {
	lut := make([]float32, bli.lutSize)
	
	for i := 0; i < bli.lutSize; i++ {
		// Position in the sinc function
		x := (float64(i)/float64(bli.oversampleFactor) - float64(bli.sincTaps)/2.0)
		
		// Windowed sinc
		var val float64
		if x == 0 {
			val = 1.0
		} else {
			val = math.Sin(math.Pi*x) / (math.Pi * x)
		}
		
		// Apply Kaiser window
		windowPos := float64(i) / float64(bli.lutSize-1)
		window := kaiserWindow(windowPos, 8.0) // Beta = 8.0 for good stopband
		
		lut[i] = float32(val * window)
	}
	
	return lut
}

// kaiserWindow applies Kaiser window function
func kaiserWindow(x, beta float64) float64 {
	// Simplified Kaiser window (normally uses Bessel function)
	// This is an approximation for performance
	term := 1.0 - (2.0*x - 1.0)*(2.0*x - 1.0)
	if term < 0 {
		term = 0
	}
	return math.Pow(term, beta/2.0)
}

// Interpolate performs band-limited interpolation at fractional position
// samples: input samples around the interpolation point
// fraction: fractional position (0-1) between samples
func (bli *BandLimitedInterpolator) Interpolate(samples []float32, fraction float32) float32 {
	if len(samples) < bli.sincTaps {
		// Not enough samples, fall back to linear interpolation
		if len(samples) >= 2 {
			return samples[0]*(1-fraction) + samples[1]*fraction
		}
		return 0
	}
	
	// Find position in LUT
	lutOffset := int(fraction * float32(bli.oversampleFactor))
	
	// Convolve with sinc kernel from LUT
	output := float32(0.0)
	for i := 0; i < bli.sincTaps; i++ {
		lutIdx := lutOffset + i*bli.oversampleFactor
		if lutIdx < bli.lutSize {
			output += samples[i] * bli.lut[lutIdx]
		}
	}
	
	return output
}

// ============================================================================
// Simple Biquad Filter for Additional Processing
// ============================================================================

// BiquadFilter implements a second-order IIR filter
type BiquadFilter struct {
	b0, b1, b2 float32 // Feedforward coefficients
	a1, a2     float32 // Feedback coefficients
	x1, x2     float32 // Input delay line
	y1, y2     float32 // Output delay line
}

// NewLowpassBiquad creates a lowpass biquad filter
// sampleRate: sample rate in Hz
// cutoff: cutoff frequency in Hz
// q: quality factor (0.707 for Butterworth)
func NewLowpassBiquad(sampleRate, cutoff, q float32) *BiquadFilter {
	omega := 2.0 * math.Pi * float64(cutoff/sampleRate)
	sinOmega := math.Sin(omega)
	cosOmega := math.Cos(omega)
	alpha := sinOmega / (2.0 * float64(q))
	
	b0 := (1.0 - cosOmega) / 2.0
	b1 := 1.0 - cosOmega
	b2 := (1.0 - cosOmega) / 2.0
	a0 := 1.0 + alpha
	a1 := -2.0 * cosOmega
	a2 := 1.0 - alpha
	
	// Normalize coefficients
	return &BiquadFilter{
		b0: float32(b0 / a0),
		b1: float32(b1 / a0),
		b2: float32(b2 / a0),
		a1: float32(a1 / a0),
		a2: float32(a2 / a0),
	}
}

// Process applies the biquad filter to a sample
func (bf *BiquadFilter) Process(input float32) float32 {
	// Direct Form II
	output := bf.b0*input + bf.b1*bf.x1 + bf.b2*bf.x2 - bf.a1*bf.y1 - bf.a2*bf.y2
	
	// Update delay lines
	bf.x2 = bf.x1
	bf.x1 = input
	bf.y2 = bf.y1
	bf.y1 = output
	
	return output
}

// Reset clears the filter state
func (bf *BiquadFilter) Reset() {
	bf.x1, bf.x2 = 0, 0
	bf.y1, bf.y2 = 0, 0
}

// ============================================================================
// DC Blocker for Removing DC Offset
// ============================================================================

// DCBlocker removes DC offset from audio signal
type DCBlocker struct {
	alpha float32
	prev  float32
	acc   float32
}

// NewDCBlocker creates a new DC blocking filter
// cutoff: highpass cutoff frequency (typically 20-40 Hz)
// sampleRate: sample rate in Hz
func NewDCBlocker(cutoff, sampleRate float32) *DCBlocker {
	// Calculate filter coefficient
	rc := 1.0 / (2.0 * math.Pi * float64(cutoff))
	dt := 1.0 / float64(sampleRate)
	alpha := float32(rc / (rc + dt))
	
	return &DCBlocker{
		alpha: alpha,
	}
}

// Process removes DC from input sample
func (dc *DCBlocker) Process(input float32) float32 {
	output := input - dc.prev + dc.alpha*dc.acc
	dc.prev = input
	dc.acc = output
	return output
}

// Reset clears the filter state
func (dc *DCBlocker) Reset() {
	dc.prev = 0
	dc.acc = 0
}