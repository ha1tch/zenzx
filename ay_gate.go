package main

// hasAY reports whether the machine being emulated has an AY-3-8912 on the
// 128K-style ports 0xFFFD (register select, data read) and 0xBFFD (data
// write). The 128K family (128K, +2, +2A, +3) and Next do; the 48K does not
// (beeper only); the TS2068 has its own AY on ports F5H/F6H (ts2068.go),
// which this gate does not affect. A SpectrumIO with no memory attached
// carries no model information, so it keeps the historical behaviour and
// answers true.
func (io *SpectrumIO) hasAY() bool {
	if io.memory == nil {
		return true
	}
	return io.memory.is128K || io.memory.isNext
}
