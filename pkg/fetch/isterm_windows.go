// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

//go:build windows

package fetch

import (
	"os"

	"golang.org/x/sys/windows"
)

// IsTerminal reports whether f is a console: somebody is there to be asked. The Windows
// build of ZenZX is a windowed program, which has no console input of its own, so for it this
// is false and a launcher says what it wants with -fetch-how instead.
func IsTerminal(f *os.File) bool {
	var mode uint32
	return windows.GetConsoleMode(windows.Handle(f.Fd()), &mode) == nil
}
