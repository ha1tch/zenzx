//go:build headless

// Command zenzx-headless is the ZenZX emulator without a window: it runs a
// number of frames and writes screenshots. Build it with -tags headless.
package main

import (
	"os"

	"github.com/ha1tch/zenzx"
)

func main() {
	zenzx.RunHeadless(os.Args[1:])
}
