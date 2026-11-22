package zenzx

import "testing"

func BenchmarkComposeNextFrame(b *testing.B) {
	for _, c := range []struct {
		name string
		regs map[uint8]uint8
	}{
		{"layer2-256", map[uint8]uint8{0x69: 0x80}},
		{"layer2-320", map[uint8]uint8{0x69: 0x80, 0x70: 0x10}},
		{"layer2-640", map[uint8]uint8{0x69: 0x80, 0x70: 0x20}},
		{"tilemap-80-over-hires", map[uint8]uint8{0x6B: 0xC1, 0x69: 0x06}},
		{"all-three", map[uint8]uint8{0x69: 0x80, 0x6B: 0x81, 0x15: 0x03}},
	} {
		b.Run(c.name, func(b *testing.B) {
			mem, screen := newTestMemoryAndScreen()
			mem.Enable128K()
			mem.EnablePlus3()
			mem.EnableNext()
			io := NewSpectrumIO(mem, nil)
			for reg, v := range c.regs {
				io.nextRegWriteDirect(reg, v)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				io.ComposeNextFrame(nil, screen, 1)
			}
		})
	}
}
