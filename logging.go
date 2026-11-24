package zenzx

import (
	"fmt"
	"io"
)

// What the machine has to say while it works (a disc inserted, a snapshot
// saved, a tape's state) goes to logOut. As a library it says nothing: the
// writer discards until a host sets one. The two programs set standard
// output, where these lines have always gone. The writer is the package's,
// not a machine's: several machines in one program share it.
var logOut io.Writer = io.Discard

// SetLogOutput sets where the machine's messages go; nil discards them.
// It is meant to be called once, before any machine runs.
func SetLogOutput(w io.Writer) {
	if w == nil {
		w = io.Discard
	}
	logOut = w
}

// sayf and sayln are fmt.Printf and fmt.Println to logOut.
func sayf(format string, a ...any) { fmt.Fprintf(logOut, format, a...) }
func sayln(a ...any)               { fmt.Fprintln(logOut, a...) }

// The opt-in traces (the SD card's, DivMMC's and the NextREG log's, each
// switched on by its own Debug field) and a script's warnings go to traceOut, kept apart from logOut so a host can show one and
// drop the other. The two programs set standard error.
var traceOut io.Writer = io.Discard

// SetTraceOutput sets where traces and warnings go; nil discards them.
// Like SetLogOutput it is meant to be called once, before any machine runs.
func SetTraceOutput(w io.Writer) {
	if w == nil {
		w = io.Discard
	}
	traceOut = w
}

// tracef is fmt.Fprintf to traceOut.
func tracef(format string, a ...any) { fmt.Fprintf(traceOut, format, a...) }
