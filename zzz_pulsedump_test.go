package main

import (
	"bufio"
	"fmt"
	"os"
	"testing"
)

// Dumps the generated pulse stream (one duration per line; stop points
// marked) for external auditing. Opt-in via ZPD_OUT.
func TestPulseDump(t *testing.T) {
	outPath := os.Getenv("ZPD_OUT")
	if outPath == "" {
		t.Skip("ZPD_OUT not set")
	}
	tape := os.Getenv("ZPD_TAPE")
	if tape == "" {
		tape = "/tmp/newdiv_corpus/Batman/Batman - Release 3.tzx"
	}
	zx := NewZenZX(AudioBackendOto)
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatal(err)
	}
	zx.tape.SetMode(TapeAccurate)
	if err := zx.tape.LoadFile(tape); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()
	for i, p := range zx.tape.st.Pulses {
		stop := ""
		if zx.tape.st.StopPoints[i] {
			stop = " S"
		}
		fmt.Fprintf(w, "%d%s\n", p, stop)
	}
	t.Logf("dumped %d pulses", len(zx.tape.st.Pulses))
}
