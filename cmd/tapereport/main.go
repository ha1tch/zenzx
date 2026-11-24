// Command tapereport generates an HTML compatibility report for zenzx's
// TAP/TZX decoding and playback wiring, measured against a real corpus of
// tape files rather than synthetic test cases.
//
// It does not ship or assume any particular corpus. The one this report was
// first built against is the "ZX Spectrum Top 100" collection curated by
// akeley -- the same games behind the newdiv distribution -- available at
// https://archive.org/details/zxspectrum-top-100 (ZXSpectrumTop100-noDoc.zip
// is the smallest download that still has every game's .tap/.tzx files).
// Any directory tree containing .tap/.tzx files works.
//
// Usage:
//
//	go run ./cmd/tapereport -corpus /path/to/games -out report.html
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	corpus := flag.String("corpus", "", "directory to walk for .tap/.tzx files (required)")
	out := flag.String("out", "tape-compatibility-report.html", "output HTML file path")
	flag.Parse()

	if *corpus == "" {
		fmt.Fprintln(os.Stderr, "tapereport: -corpus is required")
		flag.Usage()
		os.Exit(1)
	}

	stats, err := computeStats(*corpus)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tapereport: %v\n", err)
		os.Exit(1)
	}

	f, err := os.Create(*out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tapereport: creating %s: %v\n", *out, err)
		os.Exit(1)
	}
	defer f.Close()

	if err := renderReport(f, stats); err != nil {
		fmt.Fprintf(os.Stderr, "tapereport: rendering report: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("wrote %s\n", *out)
}
