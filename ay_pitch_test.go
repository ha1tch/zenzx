package zenzx

import (
	"math"
	"testing"
)

// The AY's tone is fclk / (16 * TP) (the datasheet's figure; 1.7734 MHz on a 128K), and
// these tests hold the emulation to it, to a twentieth of a semitone, both from the chip
// and from the engine's own stream. The tone used to flip every 16*TP clocks, which is
// 32*TP to a whole wave and a pitch an octave low.

const ayTestClock = 1773400

func ayWantHz(period int) float64 { return float64(ayTestClock) / (16 * float64(period)) }

// peakHz finds the strongest frequency in one second of samples, within 3% of near, by
// scanning it in steps of 0.05 Hz.
func peakHz(samples []float32, near float64) float64 {
	n := AudioSampleRate
	if len(samples) < n {
		n = len(samples)
	}
	best, bestP := 0.0, -1.0
	for f := near * 0.97; f <= near*1.03; f += 0.05 {
		var re, im float64
		w := 2 * math.Pi * f / float64(AudioSampleRate)
		for i := 0; i < n; i++ {
			re += float64(samples[i]) * math.Cos(w*float64(i))
			im += float64(samples[i]) * math.Sin(w*float64(i))
		}
		if p := re*re + im*im; p > bestP {
			best, bestP = f, p
		}
	}
	return best
}

// cents is how far a is above b, in hundredths of a semitone.
func cents(a, b float64) float64 { return 1200 * math.Log2(a/b) }

func setAYTone(ay *AYChip, period int) {
	ay.WriteRegister(0, uint8(period&0xff))
	ay.WriteRegister(1, uint8(period>>8))
	ay.WriteRegister(7, 0b00111110) // tone on channel A only
	ay.WriteRegister(8, 15)
}

func TestAYToneIsClockOverSixteenTimesPeriod(t *testing.T) {
	for _, period := range []int{100, 254, 500, 1000} {
		ay := NewAYChip(ayTestClock)
		setAYTone(ay, period)
		samples := ay.GenerateN(2*AudioSampleRate, AudioSampleRate)
		if len(samples) != 2*AudioSampleRate {
			t.Fatalf("GenerateN gave %d samples, want %d", len(samples), 2*AudioSampleRate)
		}
		got := peakHz(samples[AudioSampleRate/2:], ayWantHz(period))
		if c := cents(got, ayWantHz(period)); math.Abs(c) > 5 {
			t.Errorf("period %d: %.2f Hz, want %.2f Hz (%.1f cents off)", period, got, ayWantHz(period), c)
		}
	}
}

// A 440 Hz A is a period of 252 on a 128K: the pitch a player of the AY expects.
func TestAYConcertAIsAPeriodOf252(t *testing.T) {
	ay := NewAYChip(ayTestClock)
	setAYTone(ay, 252)
	got := peakHz(ay.GenerateN(AudioSampleRate, AudioSampleRate), 440)
	if c := cents(got, 440); math.Abs(c) > 10 {
		t.Errorf("period 252 plays %.2f Hz, want 440 Hz (%.1f cents off)", got, c)
	}
}

// However the engine slices time into chunks, a chunk is as many samples as it was asked
// for and the stream's pitch is the chip's. A chunk used to come up one sample short now
// and then, and the chip lost a sample's worth of its time to each.
func TestAYPitchThroughTheEngineWhateverTheChunking(t *testing.T) {
	const period = 254
	for _, slice := range []int{CPUFrequency / 1000, CPUFrequency / 200, CPUFrequency / 50, CPUFrequency / 10} {
		am := NewAudioEngine()
		setAYTone(am.ay, period)
		var out []float32
		cyc := uint64(0)
		for cyc < uint64(3*CPUFrequency) {
			cyc += uint64(slice)
			am.cycleMutex.Lock()
			am.currentCPUCycle = cyc
			am.cycleMutex.Unlock()
			for i := 0; i < 8; i++ { // a slice may need more than one fill
				am.fillRingBuffer()
			}
			for am.ringReadPos != am.ringWritePos {
				out = append(out, am.ringBuffer[am.ringReadPos])
				am.ringReadPos = (am.ringReadPos + 1) & RingBufferMask
			}
		}
		if len(out) < 2*AudioSampleRate {
			t.Fatalf("slice %d: only %d samples came out", slice, len(out))
		}
		got := peakHz(out[AudioSampleRate:], ayWantHz(period))
		if c := cents(got, ayWantHz(period)); math.Abs(c) > 5 {
			t.Errorf("slice of %d cycles: engine stream %.2f Hz, want %.2f Hz (%.1f cents off)", slice, got, ayWantHz(period), c)
		}
	}
}

// Noise and the envelope were right and stay so: noise shifts every 16*NP clocks, and the
// envelope takes 16*EP clocks to a step, 256*EP to a ramp of its sixteen.
func TestAYEnvelopeRampTakesTwoHundredFiftySixTimesItsPeriod(t *testing.T) {
	ay := NewAYChip(ayTestClock)
	ay.WriteRegister(7, 0xFF)
	ay.WriteRegister(8, 0x10) // the envelope drives channel A's volume
	ay.WriteRegister(11, 4)   // EP = 4
	ay.WriteRegister(12, 0)
	ay.WriteRegister(13, 0x0C) // saw up, repeating: 0,1..15,0,1..
	clocks := 0
	steps := 0
	last := ay.envelopeOutput
	for steps < 32 && clocks < 1_000_000 {
		ay.GenerateN(1, int(ay.clockFreq)) // one AY clock of time
		clocks++
		if ay.envelopeOutput != last {
			steps++
			last = ay.envelopeOutput
		}
	}
	if want := 2 * 16 * 16 * 4; math.Abs(float64(clocks-want)) > 40 {
		t.Errorf("32 envelope steps took %d clocks, want about %d (16*EP each)", clocks, want)
	}
}
