package zenzx

import (
	"context"
	"fmt"
	"io"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// Making a card bootable: the files a Next needs to start NextZXOS, copied
// from the standard card onto another that lacks them (nextcard.BootFiles,
// nextcard.MakeBootable). The SD Cards window has it as a button, and the
// command line as -next-bootable.

const nextBootableHelp = "Next: make this SD card image one a Next can boot from, by copying onto it the few files NextZXOS needs to start (about 800 KB, from the standard card in the -next-files folder), and exit. " +
	"What is on the card already is kept. The name is looked for as -multi looks for a card's"

// runNextBootable is -next-bootable: it makes the card named bootable and
// says what it did.
func runNextBootable(filesFlag, name string, w io.Writer) error {
	f, ok, note := locateNextFiles(filesFlag)
	if !ok {
		if note == "" {
			note = "none were found"
		}
		return fmt.Errorf("the Next's boot files are not in place (%s); start a Next on its own once, or run with -next-firmware setup, and they are fetched", note)
	}
	card, err := findCardFile(name, f.Dir)
	if err != nil {
		return err
	}
	n, err := nextcard.MakeBootable(context.Background(), f.Card, card, nil)
	if err != nil {
		return fmt.Errorf("%s: %w", card, err)
	}
	fmt.Fprintf(w, "%s can boot a Next now: %d files, %d KB, copied from %s\n", card, len(nextcard.BootFiles), (n+1023)/1024, f.Card)
	return nil
}
