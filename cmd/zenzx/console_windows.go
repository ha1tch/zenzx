//go:build windows && !headless

package main

import (
	"log"
	"os"
	"syscall"
)

// The Windows GUI program is linked for the GUI subsystem (-ldflags -H=windowsgui), so that
// starting it from Explorer opens its window and no console window behind it. A program of
// that kind has no standard handles of its own, and what it prints (-help, a bad option, the
// messages of -version) would go nowhere. So that it can still be used from a console, it
// attaches to the console that started it, if there is one, and prints there. A handle that
// was given to it (output redirected to a file or a pipe) is left alone.

const attachParentProcess = ^uintptr(0)

func init() {
	attach := syscall.NewLazyDLL("kernel32.dll").NewProc("AttachConsole")
	if r, _, _ := attach.Call(attachParentProcess); r == 0 {
		return // no console: it was started from Explorer, or a shortcut
	}
	if !haveStdHandle(syscall.STD_OUTPUT_HANDLE) {
		if f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
			os.Stdout = f
		}
	}
	if !haveStdHandle(syscall.STD_ERROR_HANDLE) {
		if f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
			os.Stderr = f
		}
	}
	if !haveStdHandle(syscall.STD_INPUT_HANDLE) {
		if f, err := os.OpenFile("CONIN$", os.O_RDONLY, 0); err == nil {
			os.Stdin = f
		}
	}
	log.SetOutput(os.Stderr)
}

// haveStdHandle reports whether the process was given the standard handle.
func haveStdHandle(which int) bool {
	h, err := syscall.GetStdHandle(which)
	return err == nil && h != 0 && h != syscall.InvalidHandle
}
