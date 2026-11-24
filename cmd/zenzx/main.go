//go:build !headless

// Command zenzx is the ZenZX emulator with its window.
package main

import (
	"os"

	"github.com/ha1tch/zenzx"
)

func main() {
	zenzx.RunGUI(os.Args[1:])
}
