// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

//go:build !windows

package fetch

import "runtime"

// ThisHost is the computer the program runs on.
func ThisHost() Host { return Host{OS: runtime.GOOS, Arch: runtime.GOARCH} }
