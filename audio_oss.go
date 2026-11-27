//go:build !headless

package zenzx

import (
	"encoding/binary"
	"math"
)

// What the OSS sound device (audio_oss_freebsd.go) needs that does not depend
// on the operating system, kept apart so a test on any system can check it.

// An OSS ioctl request is _IOWR('P', n, int): read and write (0xC0000000), a
// four-byte argument (4<<16), the group letter 'P' and the number n.
const ossIOWR = 0xC0000000 | 4<<16 | 'P'<<8

const (
	ossSetFmt      = ossIOWR | 5  // SNDCTL_DSP_SETFMT
	ossChannels    = ossIOWR | 6  // SNDCTL_DSP_CHANNELS
	ossSpeed       = ossIOWR | 2  // SNDCTL_DSP_SPEED
	ossSetFragment = ossIOWR | 10 // SNDCTL_DSP_SETFRAGMENT
	ossFmtS16LE    = 0x00000010   // AFMT_S16_LE
	ossChunk       = 512          // samples per write
)

// floatToS16 turns float32 little-endian samples into 16-bit little-endian
// ones, clipped to the range, and returns the number of bytes written to out.
// out must hold half as many bytes as in.
func floatToS16(in []byte, out []byte) int {
	n := len(in) / 4
	for i := 0; i < n; i++ {
		f := math.Float32frombits(binary.LittleEndian.Uint32(in[i*4:]))
		if f > 1 {
			f = 1
		} else if f < -1 {
			f = -1
		}
		binary.LittleEndian.PutUint16(out[i*2:], uint16(int16(f*32767)))
	}
	return n * 2
}
