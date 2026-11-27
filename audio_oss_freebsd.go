//go:build !headless && freebsd && !cgo

package zenzx

import (
	"fmt"
	"io"
	"sync"
	"syscall"
	"unsafe"
)

// FreeBSD's sound card without cgo: the OSS device, /dev/dsp, which sound(4)
// provides on every FreeBSD, written to as 16-bit mono at AudioSampleRate.
// oto's Unix backend is ALSA through cgo and cannot be built here, so the
// cgo-free FreeBSD program plays through this; a FreeBSD program built with
// cgo uses oto as every other system does (audio_device_oto.go). The numbers
// and the sample conversion are in audio_oss.go.

func ossIoctl(fd int, req uintptr, arg int32) (int32, error) {
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), req, uintptr(unsafe.Pointer(&arg))); e != 0 {
		return 0, e
	}
	return arg, nil
}

type ossDevice struct {
	mu   sync.Mutex
	fd   int
	stop chan struct{}
	done chan struct{}
}

// NewSystemAudioDevice returns a device that plays through the sound card.
func NewSystemAudioDevice() AudioDevice { return &ossDevice{fd: -1} }

func (d *ossDevice) Open(src io.Reader) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.fd >= 0 {
		return nil
	}
	fd, err := syscall.Open("/dev/dsp", syscall.O_WRONLY, 0)
	if err != nil {
		sayf("Failed to open /dev/dsp: %v\n", err)
		return err
	}
	// Four fragments of 1024 bytes: about 46 ms in the card. A refusal is
	// not fatal, the card then keeps its own size.
	ossIoctl(fd, ossSetFragment, 4<<16|10)
	if got, err := ossIoctl(fd, ossSetFmt, ossFmtS16LE); err != nil || got != ossFmtS16LE {
		syscall.Close(fd)
		return fmt.Errorf("/dev/dsp does not take 16-bit samples")
	}
	if got, err := ossIoctl(fd, ossChannels, AudioChannels); err != nil || got != AudioChannels {
		syscall.Close(fd)
		return fmt.Errorf("/dev/dsp does not take %d channel(s)", AudioChannels)
	}
	if _, err := ossIoctl(fd, ossSpeed, AudioSampleRate); err != nil {
		syscall.Close(fd)
		return err
	}
	d.fd, d.stop, d.done = fd, make(chan struct{}), make(chan struct{})
	go d.pump(fd, src, d.stop, d.done)
	return nil
}

// pump reads the engine and writes the card until stopped. The write blocks
// while the card's buffer is full, which is what paces it.
func (d *ossDevice) pump(fd int, src io.Reader, stop, done chan struct{}) {
	defer close(done)
	in := make([]byte, ossChunk*4)
	out := make([]byte, ossChunk*2)
	for {
		select {
		case <-stop:
			return
		default:
		}
		if _, err := io.ReadFull(src, in); err != nil {
			return
		}
		n := floatToS16(in, out)
		for off := 0; off < n; {
			w, err := syscall.Write(fd, out[off:n])
			if err != nil {
				if err == syscall.EINTR {
					continue
				}
				return
			}
			off += w
		}
	}
}

func (d *ossDevice) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.fd < 0 {
		return
	}
	close(d.stop)
	<-d.done
	syscall.Close(d.fd)
	d.fd = -1
}
