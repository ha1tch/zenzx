// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

//go:build linux

package fetch

import (
	"os"

	"golang.org/x/sys/unix"
)

// IsTerminal reports whether f is a terminal: somebody is there to be asked. A character
// device is not enough (/dev/null is one).
func IsTerminal(f *os.File) bool {
	_, err := unix.IoctlGetTermios(int(f.Fd()), unix.TCGETS)
	return err == nil
}
