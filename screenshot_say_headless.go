//go:build headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import "fmt"

// shotSay tells the person what became of a screenshot. There is no window to show a toast in,
// so it goes to the console.
func (zx *ZenZX) shotSay(format string, a ...any) {
	fmt.Println(fmt.Sprintf(format, a...))
}
