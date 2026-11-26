// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

//go:build windows

package fetch

import (
	"runtime"

	"golang.org/x/sys/windows"
)

// ThisHost is the computer the program runs on. On Windows it carries the version of the
// system: the manifest-free way, RtlGetVersion, says what the system is, where GetVersionEx
// would say Windows 8 for every later one.
func ThisHost() Host {
	h := Host{OS: runtime.GOOS, Arch: runtime.GOARCH}
	if v := windows.RtlGetVersion(); v != nil {
		h.WinMajor, h.WinMinor = int(v.MajorVersion), int(v.MinorVersion)
	}
	return h
}
