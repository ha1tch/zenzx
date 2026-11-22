package nextcard

import (
	"context"
	"errors"
	"fmt"
	"os"
)

// BootFiles are the files a Next must have on its card to start NextZXOS and
// reach its main menu; with any one of them missing it does not get there.
//
//	TBBLUE.FW                    the firmware the boot ROM loads
//	machines/next/config.ini     the machine's settings (timing=0 for ZenZX)
//	machines/next/menu.def       the firmware's list of machine personalities
//	machines/next/enNextZX.rom   NextZXOS's ROMs
//	machines/next/enAltZX.rom    the alternate ROM
//	machines/next/enNxtmmc.rom   the DivMMC ROM
//	machines/next/keymap.bin     the keyboard map
//	nextzxos/enSystem.sys        NextZXOS itself
//
// The list was found with the 26.11 distribution's card: first by watching
// which of its 1,393 files are read on the way to the menu, which is
// thirteen with the Browser's, then by leaving each of those out in turn.
// Eight are needed and five are not (BootExtras). A card holding only these
// eight is booted by TestNextZXOSBootsFromAMinimalCard.
var BootFiles = []string{
	"TBBLUE.FW",
	"machines/next/config.ini",
	"machines/next/menu.def",
	"machines/next/enNextZX.rom",
	"machines/next/enAltZX.rom",
	"machines/next/enNxtmmc.rom",
	"machines/next/keymap.bin",
	"nextzxos/enSystem.sys",
}

// BootExtras are the files a Next also reads from the distribution's card as
// it starts, and starts without: the Multiface's ROM, a ROM for one of the
// firmware's other personalities, the clock driver, the welcome screen, and
// the Browser's settings. MakeBootable does not copy them.
var BootExtras = []string{
	"machines/next/enNextMf.rom",
	"machines/next/gw03v33.rom",
	"nextzxos/RTC.SYS",
	"nextzxos/autoexec.1st",
	"nextzxos/browser.cfg",
}

// MakeBootable copies BootFiles from the card at source, which must be one
// a Next boots from, onto the card at cardPath, in place of any files of
// the same names there. Everything else on that card is kept. It returns
// how many bytes of files it put there.
//
// The card is rewritten as AddFiles rewrites one: a failure leaves it as it
// was, and it needs the card's size again in free disk space while it works.
func MakeBootable(ctx context.Context, source, cardPath string, progress func(done, total int64)) (int64, error) {
	if same, err := sameFile(source, cardPath); err != nil {
		return 0, err
	} else if same {
		return 0, errors.New("that is the card the files would be copied from")
	}
	src, err := os.Open(source)
	if err != nil {
		return 0, err
	}
	defer src.Close()
	if err := VerifyCard(src); err != nil {
		return 0, fmt.Errorf("the card to copy the system from, %s, is not usable: %w", source, err)
	}
	vol, err := Open(src)
	if err != nil {
		return 0, err
	}
	var files []File
	var total int64
	for _, path := range BootFiles {
		n := vol.Root.Find(path)
		if n == nil || n.IsDir() {
			return 0, fmt.Errorf("the card to copy the system from has no %s", path)
		}
		data, err := vol.ReadFile(n)
		if err != nil {
			return 0, fmt.Errorf("%s: %w", path, err)
		}
		files = append(files, File{Name: path, Data: data})
		total += int64(len(data))
	}
	if err := AddTree(ctx, cardPath, files, progress); err != nil {
		return 0, err
	}
	dst, err := os.Open(cardPath)
	if err != nil {
		return 0, err
	}
	defer dst.Close()
	if err := VerifyCard(dst); err != nil {
		return 0, fmt.Errorf("the files were copied, but the card is still not one a Next boots from: %w", err)
	}
	return total, nil
}

// sameFile reports whether two names are the same file.
func sameFile(a, b string) (bool, error) {
	sa, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	sb, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	return os.SameFile(sa, sb), nil
}
