//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenui/zenuiraylib"
)

// The debugger HUD takes its keyboard and pointer as a zenui.Input, as every other zenui
// surface does; this is the one place that makes that Input for it from the window. It is
// zenuiraylib.Input() (typed characters, Enter, Backspace, the pointer) with the two things the
// HUD needs that it does not do:
//
//   - A frame in which Alt is held has no characters and no keys: Alt+D opened the HUD, and
//     other Alt combinations are system controls, not debugger text. The character queue is
//     drained all the same (zenuiraylib.Input does that), so nothing spills into a later frame.
//   - Where the window's character queue gave nothing printable this frame, the keys pressed
//     are mapped to characters here, honouring Shift (pressedChars). On some platforms and
//     builds the queue is not populated, and typing in the HUD must work regardless.
//
// Both belong in zenuiraylib itself, which is not this program's to change; until they are
// there, they are here, and not in the HUD.
func hudInput() zenui.Input {
	in := zenuiraylib.Input()
	if rl.IsKeyDown(rl.KeyLeftAlt) || rl.IsKeyDown(rl.KeyRightAlt) {
		in.Chars, in.Keys = nil, nil
		return in
	}
	printable := in.Chars[:0]
	for _, r := range in.Chars {
		if r >= 0x20 && r != 0x7F {
			printable = append(printable, r)
		}
	}
	in.Chars = printable
	if len(in.Chars) == 0 {
		shift := rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift)
		in.Chars = pressedChars(shift)
	}
	return in
}

// pressedChars maps the raylib keys pressed this frame to characters, for the
// fallback input path. It covers what the debugger grammar needs: letters,
// digits, space, and the few symbols addresses and commands use ($ # . - ( )
// .. etc.). Shift selects the upper symbol where there is one.
func pressedChars(shift bool) []rune {
	var out []rune
	// Letters: always lower-case for commands (the grammar is case-insensitive
	// on verbs and hex is accepted either case).
	for k := int32(rl.KeyA); k <= int32(rl.KeyZ); k++ {
		if rl.IsKeyPressed(k) {
			out = append(out, rune('a'+(k-int32(rl.KeyA))))
		}
	}
	// Digits and their shifted symbols.
	digitShift := map[int32]rune{
		rl.KeyOne: '!', rl.KeyThree: '#', rl.KeyFour: '$', rl.KeyFive: '%',
		rl.KeyNine: '(', rl.KeyZero: ')',
	}
	for k := int32(rl.KeyZero); k <= int32(rl.KeyNine); k++ {
		if rl.IsKeyPressed(k) {
			if shift {
				if sym, ok := digitShift[k]; ok {
					out = append(out, sym)
					continue
				}
			}
			out = append(out, rune('0'+(k-int32(rl.KeyZero))))
		}
	}
	// Space and a few punctuation keys the grammar uses.
	if rl.IsKeyPressed(rl.KeySpace) {
		out = append(out, ' ')
	}
	if rl.IsKeyPressed(rl.KeyPeriod) {
		out = append(out, '.')
	}
	if rl.IsKeyPressed(rl.KeyMinus) {
		if shift {
			out = append(out, '_')
		} else {
			out = append(out, '-')
		}
	}
	return out
}
