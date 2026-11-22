package zenzx

import (
	"fmt"
	"strconv"
	"strings"
)

// A log of writes to chosen NextREG registers, for working out what a program asks of the
// video hardware: -nextreg-log=40,41,43,44 prints every write to those four to stderr.
// "all" logs every register. It is off unless asked for; when off the cost is one test of
// a flag per register write.

// nextRegLog is the set of registers whose writes are printed.
type nextRegLog struct {
	on    bool
	watch [256]bool
}

// setNextRegLog reads the -nextreg-log value: register numbers in hex, separated by commas,
// or "all". An empty value turns the log off.
func (io *SpectrumIO) setNextRegLog(list string) error {
	io.nextRegLog = nextRegLog{}
	for _, part := range strings.Split(list, ",") {
		part = strings.TrimSpace(part)
		switch {
		case part == "":
			continue
		case strings.EqualFold(part, "all"):
			for i := range io.nextRegLog.watch {
				io.nextRegLog.watch[i] = true
			}
		default:
			n, err := strconv.ParseUint(strings.TrimPrefix(strings.ToLower(part), "0x"), 16, 8)
			if err != nil {
				return fmt.Errorf("-nextreg-log: %q is not a register number in hex", part)
			}
			io.nextRegLog.watch[n] = true
		}
		io.nextRegLog.on = true
	}
	return nil
}

func (io *SpectrumIO) logNextRegWrite(reg, value uint8) {
	tracef("[nextreg] %02X <- %02X\n", reg, value)
}
