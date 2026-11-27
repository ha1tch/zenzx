//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

// shotSay tells the person what became of a screenshot: a toast in the window, or the console
// for a machine nobody has given a window (see notice).
func (zx *ZenZX) shotSay(format string, a ...any) {
	zx.notice(format, a...)
}
