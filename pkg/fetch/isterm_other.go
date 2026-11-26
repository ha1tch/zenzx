// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

//go:build !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly && !windows

package fetch

import "os"

// IsTerminal reports whether f looks like a terminal: a character device.
func IsTerminal(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}
