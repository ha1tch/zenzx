//go:build !headless

package zenzx

import (
	"encoding/binary"
	"math"
	"testing"
)

// The OSS request numbers are _IOWR('P', n, int); these are the values the
// FreeBSD kernel accepted when the cgo-free program was run on FreeBSD 14.5.
func TestOSSRequestNumbers(t *testing.T) {
	for _, c := range []struct {
		name      string
		got, want uint32
	}{
		{"SNDCTL_DSP_SETFMT", ossSetFmt, 0xC0045005},
		{"SNDCTL_DSP_CHANNELS", ossChannels, 0xC0045006},
		{"SNDCTL_DSP_SPEED", ossSpeed, 0xC0045002},
		{"SNDCTL_DSP_SETFRAGMENT", ossSetFragment, 0xC004500A},
	} {
		if c.got != c.want {
			t.Errorf("%s = %#x, want %#x", c.name, c.got, c.want)
		}
	}
}

func TestFloatToS16(t *testing.T) {
	in := make([]byte, 0, 7*4)
	for _, f := range []float32{0, 1, -1, 0.5, -0.5, 3, -3} {
		in = binary.LittleEndian.AppendUint32(in, math.Float32bits(f))
	}
	out := make([]byte, len(in)/2)
	if n := floatToS16(in, out); n != 14 {
		t.Fatalf("wrote %d bytes, want 14", n)
	}
	want := []int16{0, 32767, -32767, 16383, -16383, 32767, -32767}
	for i, w := range want {
		if got := int16(binary.LittleEndian.Uint16(out[i*2:])); got != w {
			t.Errorf("sample %d = %d, want %d", i, got, w)
		}
	}
}
