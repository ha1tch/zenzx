package zenzx

import (
	"context"
	"fmt"
	"io"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// runNextSetup is -next-setup: the download, the card and the verification that the GUI's
// first-start dialog does, with its progress written as lines to w. Asking for it on the
// command line is the confirmation, so nothing is asked here.
func runNextSetup(dir, distributionURL, bootROMURL string, w io.Writer) error {
	opts := nextcard.Options{Dir: dir, DistributionURL: distributionURL, BootROMURL: bootROMURL}
	if opts.DistributionURL == "" {
		opts.DistributionURL = nextcard.DefaultDistributionURL
	}
	if opts.BootROMURL == "" {
		opts.BootROMURL = nextcard.DefaultBootROMURL
	}
	fmt.Fprintf(w, "Next boot files: %s, %s and %s, kept in %s\n", nextcard.BootROMName, nextcard.DivMMCROMName, nextcard.CardName, dir)
	fmt.Fprintf(w, "  distribution (about 70 MB): %s\n  boot ROM: %s\n", opts.DistributionURL, opts.BootROMURL)
	fmt.Fprintf(w, "  what is already there is kept; about 2.2 GB of disk is used while the card is built, 1.1 GB afterwards\n")
	stage, tenth := nextcard.Stage(-1), int64(-1)
	err := nextcard.Setup(context.Background(), opts, func(s nextcard.Stage, done, total int64) {
		if s != stage {
			stage, tenth = s, -1
			fmt.Fprintf(w, "%s...\n", s)
		}
		if total > 0 {
			if t := done * 10 / total; t != tenth {
				tenth = t
				fmt.Fprintf(w, "  %3d%%  %.1f of %.1f MB\n", done*100/total, float64(done)/(1<<20), float64(total)/(1<<20))
			}
		}
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "Verified. -model=next now boots NextZXOS from %s\n", dir)
	return nil
}
