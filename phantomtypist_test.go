// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"testing"

	"github.com/ha1tch/zentools/pkg/tap"
)

// The phantom typist starts a tape by typing what loads it, on each model's own ROM.

// typistMachine is a machine of the model with its real ROMs and a tape deck, running nothing yet.
func typistMachine(t *testing.T, model string) *ZenZX {
	t.Helper()
	zx := NewZenZX(AudioBackendOto)
	zx.cpu.Reset()
	rom := func(name string) []byte {
		t.Helper()
		data, err := resolveROMBytes(name, "./rom")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return data
	}
	plus3 := func(prefix string) [4][]byte {
		t.Helper()
		d, err := loadPlus3Bytes("./rom", prefix+"-0.rom", prefix+"-1.rom", prefix+"-2.rom", prefix+"-3.rom")
		if err != nil {
			t.Fatalf("%s ROMs: %v", prefix, err)
		}
		return d
	}
	var err error
	switch model {
	case "48k":
		err = zx.LoadROMBytes(rom("48.rom"))
	case "spanish48k":
		err = zx.LoadROMBytes(rom("48-spanish.rom"))
	case "128k":
		err = zx.Load128KROMBytes(rom("128-0.rom"), rom("128-1.rom"))
	case "plus2":
		err = zx.Load128KROMBytes(rom("plus2-0.rom"), rom("plus2-1.rom"))
	case "spanish128k":
		err = zx.Load128KROMBytes(rom("128-spanish-0.rom"), rom("128-spanish-1.rom"))
	case "spanishplus2":
		err = zx.Load128KROMBytes(rom("plus2-spanish-0.rom"), rom("plus2-spanish-1.rom"))
	case "plus2a", "plus3":
		d := plus3("plus3")
		err = zx.LoadPlus3ROMBytes(d[0], d[1], d[2], d[3])
		if model == "plus3" {
			zx.io.EnableFDC()
		}
	case "spanishplus3":
		d := plus3("plus3-spanish")
		err = zx.LoadPlus3ROMBytes(d[0], d[1], d[2], d[3])
		zx.io.EnableFDC()
	case "ts2068":
		err = zx.LoadTS2068ROMBytes(rom("ts2068-0.rom"), rom("ts2068-1.rom"))
	default:
		t.Fatalf("no such model %q", model)
	}
	if err != nil {
		t.Fatalf("%s: %v", model, err)
	}
	zx.tape = NewTape(zx)
	zx.tape.AttachFastLoader(&FastLoader{Enabled: true})
	return zx
}

// insertTape puts a tape image in the deck, rewound and stopped, as a file opened does.
func insertTape(t *testing.T, zx *ZenZX, image []byte) {
	t.Helper()
	if err := zx.tape.loadTAP(image); err != nil {
		t.Fatal(err)
	}
	zx.tape.st.Loaded, zx.tape.st.Filename, zx.tape.st.Position, zx.tape.st.Playing = true, "test.tap", 0, false
}

// typistMark is the byte the test programs POKE into 32768 when they run: the screen of a 128K
// machine is not read by ReadScreen, and a program that has run leaves this behind on any model.
const typistMark = 171

// ranProgram runs frames until the test program has run, and says how many it took (-1: never).
func ranProgram(zx *ZenZX, limit int) int {
	for f := 0; f < limit; f++ {
		zx.RunFrame()
		if zx.memory.Read(0x8000) == typistMark {
			return f
		}
	}
	return -1
}

// A BASIC program that pokes a byte, started by the typist on each model.
func TestTypistLoadsABasicProgramOnEachModel(t *testing.T) {
	image, err := BasicTape("hello", "10 POKE 32768,171\n")
	if err != nil {
		t.Fatal(err)
	}
	for _, model := range []string{"48k", "spanish48k", "ts2068", "128k", "plus2", "spanish128k", "spanishplus2", "plus2a", "plus3", "spanishplus3"} {
		t.Run(model, func(t *testing.T) {
			zx := typistMachine(t, model)
			for i := 0; i < 200; i++ { // a machine that has been running for a while
				zx.RunFrame()
			}
			insertTape(t, zx, image)
			if !zx.TapeAutoloads() || !zx.AutoloadTape() {
				t.Fatal("the machine would not autoload")
			}
			if !zx.TypistActive() {
				t.Fatal("the typist is not active after AutoloadTape")
			}
			frames := ranProgram(zx, 5000)
			if frames < 0 {
				t.Fatalf("the program never ran:\n%s", screenText(zx))
			}
			t.Logf("%s: the program ran after %d frames", model, frames)
			if got := zx.io.typistKeys; got != [8]uint8{} && !zx.TypistActive() {
				t.Errorf("the typist left keys down: %v", got)
			}
		})
	}
}

// A tape that starts with a CODE header is loaded with LOAD ""CODE, which LOAD "" does not do.
func TestTypistLoadsCodeOnEachModel(t *testing.T) {
	image := tap.EncodeCode("code", []byte{typistMark}, 0x8000)
	for _, model := range []string{"48k", "spanish48k", "ts2068", "128k", "plus2", "spanish128k", "spanishplus2", "plus2a", "plus3", "spanishplus3"} {
		t.Run(model, func(t *testing.T) {
			zx := typistMachine(t, model)
			for i := 0; i < 200; i++ {
				zx.RunFrame()
			}
			insertTape(t, zx, image)
			if !zx.tapeStartsWithCode() {
				t.Fatal("a tape that begins with a CODE header was not taken for one")
			}
			if !zx.AutoloadTape() {
				t.Fatal("the machine would not autoload")
			}
			frames := ranProgram(zx, 6000)
			if frames < 0 {
				t.Fatalf("the code never loaded:\n%s", screenText(zx))
			}
			t.Logf("%s: the code was in after %d frames", model, frames)
		})
	}
}

// A tape that starts with a program is not taken for code, nor an empty deck for either.
func TestTapeStartsWithCode(t *testing.T) {
	zx := typistMachine(t, "48k")
	if zx.tapeStartsWithCode() {
		t.Error("an empty deck starts with code")
	}
	prog, _ := BasicTape("p", "10 REM\n")
	insertTape(t, zx, prog)
	if zx.tapeStartsWithCode() {
		t.Error("a program was taken for code")
	}
	insertTape(t, zx, tap.EncodeCode("c", []byte{1}, 0x8000))
	if !zx.tapeStartsWithCode() {
		t.Error("code was not taken for code")
	}
}
