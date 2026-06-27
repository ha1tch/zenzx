package main

// ============================================================================
// AY-3-8912 Sound Chip Emulation (Corrected Version)
// ============================================================================

// AY-3-8912 uses logarithmic volume levels
var ayVolumeTable = []float32{
	0.000000, 0.007813, 0.011049, 0.015625,
	0.022097, 0.031250, 0.044194, 0.062500,
	0.088388, 0.125000, 0.176777, 0.250000,
	0.353553, 0.500000, 0.707107, 1.000000,
}

type AYChip struct {
	// Registers (16 total)
	registers [16]uint8

	// Channel states
	toneCounters    [3]int
	toneOutputs     [3]bool
	noiseCounter    int
	noiseOutput     bool
	noiseShift      uint32
	envelopeCounter int
	envelopeOutput  uint8
	envelopePhase   int
	envelopeDir     int // Direction: 1 = up, -1 = down

	// Clock frequency
	clockFreq int

	// Sample generation
	cycleAccumulator float64
}

func NewAYChip(clockFreq int) *AYChip {
	ay := &AYChip{
		clockFreq:   clockFreq,
		noiseShift:  1, // Initialize noise generator
		envelopeDir: 1, // Start going up
	}

	// Initialize registers to sensible defaults
	ay.registers[7] = 0xFF // All channels disabled by default

	return ay
}

// WriteRegister writes a value to an AY register
func (ay *AYChip) WriteRegister(reg, value uint8) {
	if reg > 15 {
		return
	}

	// Mask values based on register
	switch reg {
	case 0, 2, 4: // Tone period fine
		ay.registers[reg] = value
	case 1, 3, 5: // Tone period coarse
		ay.registers[reg] = value & 0x0F
	case 6: // Noise period
		ay.registers[reg] = value & 0x1F
	case 7: // Enable
		ay.registers[reg] = value
	case 8, 9, 10: // Channel volumes
		ay.registers[reg] = value & 0x1F
	case 11, 12: // Envelope period
		ay.registers[reg] = value
	case 13: // Envelope shape
		ay.registers[reg] = value & 0x0F
		// Reset envelope when shape changes
		ay.envelopeCounter = 0
		ay.envelopePhase = 0
		// Set initial direction based on shape
		if value&0x04 != 0 {
			// Attack - start from 0 going up
			ay.envelopeOutput = 0
			ay.envelopeDir = 1
		} else {
			// Decay - start from 15 going down
			ay.envelopeOutput = 15
			ay.envelopeDir = -1
		}
	case 14, 15: // I/O ports (not used on Spectrum)
		ay.registers[reg] = value
	}
}

// ReadRegister reads a value from an AY register
func (ay *AYChip) ReadRegister(reg uint8) uint8 {
	if reg > 15 {
		return 0xFF
	}
	return ay.registers[reg]
}

// GenerateSamples generates audio samples for the given number of CPU cycles
func (ay *AYChip) GenerateSamples(cycles int, sampleRate int, cpuFreq int) []float32 {
	// Calculate samples to generate
	samplesNeeded := int(float64(cycles) * float64(sampleRate) / float64(cpuFreq))
	samples := make([]float32, samplesNeeded)

	// AY clock is CPU clock / 2
	ayClockPerSample := float64(cpuFreq) / 2.0 / float64(sampleRate)

	for i := 0; i < samplesNeeded; i++ {
		// Accumulate AY clocks
		ay.cycleAccumulator += ayClockPerSample

		// Process whole AY clocks
		for ay.cycleAccumulator >= 1.0 {
			ay.cycleAccumulator -= 1.0

			// Update tone generators with divide-by-16
			for ch := 0; ch < 3; ch++ {
				period := int(ay.registers[ch*2]) | (int(ay.registers[ch*2+1]) << 8)
				if period == 0 {
					period = 1
				}
				period *= 16 // AY internally divides by 16

				ay.toneCounters[ch]++
				if ay.toneCounters[ch] >= period {
					ay.toneCounters[ch] = 0
					ay.toneOutputs[ch] = !ay.toneOutputs[ch]
				}
			}

			// Update noise generator with proper 17-bit LFSR
			noisePeriod := int(ay.registers[6] & 0x1F)
			if noisePeriod == 0 {
				noisePeriod = 1
			}
			noisePeriod *= 16 // Also needs divide-by-16

			ay.noiseCounter++
			if ay.noiseCounter >= noisePeriod {
				ay.noiseCounter = 0
				// Proper 17-bit LFSR with taps at bits 17 and 14
				if (ay.noiseShift & 1) != 0 {
					ay.noiseShift = (ay.noiseShift >> 1) ^ 0x12000
				} else {
					ay.noiseShift >>= 1
				}
				ay.noiseOutput = (ay.noiseShift & 1) != 0
			}

			// Update envelope generator
			envPeriod := int(ay.registers[11]) | (int(ay.registers[12]) << 8)
			if envPeriod == 0 {
				envPeriod = 1
			}
			envPeriod *= 16 // Envelope also uses divide-by-16

			ay.envelopeCounter++
			if ay.envelopeCounter >= envPeriod {
				ay.envelopeCounter = 0
				ay.updateEnvelope()
			}
		}

		// Mix channels with proper AND logic
		output := float32(0)
		enable := ay.registers[7]

		for ch := 0; ch < 3; ch++ {
			// Check if tone and noise are disabled (inverted logic in register)
			toneDisable := (enable & (1 << ch)) != 0
			noiseDisable := (enable & (8 << ch)) != 0

			// Channel output using AND logic:
			// Output is ON only if all enabled sources are ON
			channelOn := true

			// If tone is enabled and tone output is low, channel is off
			if !toneDisable && !ay.toneOutputs[ch] {
				channelOn = false
			}

			// If noise is enabled and noise output is low, channel is off
			if !noiseDisable && !ay.noiseOutput {
				channelOn = false
			}

			if channelOn {
				// Get volume
				vol := ay.registers[8+ch]
				if vol&0x10 != 0 {
					// Use envelope
					vol = ay.envelopeOutput
				} else {
					vol = vol & 0x0F
				}

				// Use logarithmic volume table
				output += ayVolumeTable[vol] / 3.0 // Divide by 3 for mixing
			}
		}

		samples[i] = output
	}

	return samples
}

// updateEnvelope updates the envelope generator with corrected patterns
func (ay *AYChip) updateEnvelope() {
	shape := ay.registers[13]

	// The envelope shapes are defined by 4 bits:
	// Bit 0: Hold
	// Bit 1: Alternate
	// Bit 2: Attack
	// Bit 3: Continue

	hold := shape&0x01 != 0
	alternate := shape&0x02 != 0
	cont := shape&0x08 != 0

	// Update envelope value based on current direction
	if ay.envelopeDir > 0 {
		// Going up
		if ay.envelopeOutput < 15 {
			ay.envelopeOutput++
		} else {
			// Reached top
			ay.envelopePhase++

			if cont {
				if hold {
					if alternate {
						// Will alternate, so reverse
						ay.envelopeDir = -1
					}
					// else stay at 15
				} else {
					// Not holding
					if alternate {
						// Reverse direction
						ay.envelopeDir = -1
					} else {
						// Reset to bottom
						ay.envelopeOutput = 0
					}
				}
			} else {
				// No continue - stay at current value or reset
				if !hold {
					ay.envelopeOutput = 0
				}
				ay.envelopeDir = 0 // Stop
			}
		}
	} else if ay.envelopeDir < 0 {
		// Going down
		if ay.envelopeOutput > 0 {
			ay.envelopeOutput--
		} else {
			// Reached bottom
			ay.envelopePhase++

			if cont {
				if hold {
					if alternate {
						// Will alternate, so reverse
						ay.envelopeDir = 1
					}
					// else stay at 0
				} else {
					// Not holding
					if alternate {
						// Reverse direction
						ay.envelopeDir = 1
					} else {
						// Reset to top
						ay.envelopeOutput = 15
					}
				}
			} else {
				// No continue - stay at current value or reset
				if !hold {
					ay.envelopeOutput = 15
				}
				ay.envelopeDir = 0 // Stop
			}
		}
	}
	// If envelopeDir == 0, we're stopped and do nothing
}

// Alternative simplified envelope implementation using the traditional patterns
func (ay *AYChip) updateEnvelopeSimple() {
	shape := ay.registers[13]

	// This is a simplified but functional implementation of the 16 envelope shapes
	switch shape {
	case 0, 1, 2, 3, 9: // \___
		if ay.envelopePhase == 0 {
			if ay.envelopeOutput > 0 {
				ay.envelopeOutput--
			} else {
				ay.envelopePhase = 1
				ay.envelopeOutput = 0
			}
		}

	case 4, 5, 6, 7, 15: // /___
		if ay.envelopePhase == 0 {
			if ay.envelopeOutput < 15 {
				ay.envelopeOutput++
			} else {
				ay.envelopePhase = 1
				ay.envelopeOutput = 0
			}
		}

	case 8: // \\\\
		if ay.envelopeOutput > 0 {
			ay.envelopeOutput--
		} else {
			ay.envelopeOutput = 15
		}

	case 10: // \/\/
		if ay.envelopePhase == 0 {
			// Down phase
			if ay.envelopeOutput > 0 {
				ay.envelopeOutput--
			} else {
				ay.envelopePhase = 1
			}
		} else {
			// Up phase
			if ay.envelopeOutput < 15 {
				ay.envelopeOutput++
			} else {
				ay.envelopePhase = 0
			}
		}

	case 11: // \---
		if ay.envelopePhase == 0 {
			if ay.envelopeOutput > 0 {
				ay.envelopeOutput--
			} else {
				ay.envelopePhase = 1
				ay.envelopeOutput = 15
			}
		}

	case 12: // ////
		if ay.envelopeOutput < 15 {
			ay.envelopeOutput++
		} else {
			ay.envelopeOutput = 0
		}

	case 13: // /---
		if ay.envelopePhase == 0 {
			if ay.envelopeOutput < 15 {
				ay.envelopeOutput++
			} else {
				ay.envelopePhase = 1
			}
		}

	case 14: // /\/\
		if ay.envelopePhase == 0 {
			// Up phase
			if ay.envelopeOutput < 15 {
				ay.envelopeOutput++
			} else {
				ay.envelopePhase = 1
			}
		} else {
			// Down phase
			if ay.envelopeOutput > 0 {
				ay.envelopeOutput--
			} else {
				ay.envelopePhase = 0
			}
		}
	}
}
