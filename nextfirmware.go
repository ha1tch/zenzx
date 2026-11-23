// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// The Next's boot files (the boot ROM, the DivMMC ROM and the SD card, nextcard.Files) are
// not shipped with ZenZX. Getting them is the same work wherever it is asked for: the GUI's
// first-start window and the -next-firmware switch, in both builds, do it through the
// pieces in this file and in pkg/nextcard.
//
//	pkg/nextcard      what the work is: Setup (download, build the card, verify), Verify,
//	                  Usable, Cleanup. Nothing in it knows about a window or a command line.
//	nextchain.go      where the files are looked for and where a download puts them.
//	firmwareJob       one run of Setup with its progress kept where another goroutine can
//	                  read it: the window polls it every frame, the command line waits on it.
//	runNextFirmware   the switch: setup, check or cleanup, with the exit code.
//
// What belongs to the window alone is in nextsetup_gui.go: the layout, the two address
// fields, the buttons and the wording of what is asked.

// The actions of -next-firmware.
const (
	firmwareSetup   = "setup"
	firmwareCheck   = "check"
	firmwareCleanup = "cleanup"
)

// nextFirmwareHelp is the help of -next-firmware, for both front ends.
const nextFirmwareHelp = "Next: do something about the boot files and exit. setup downloads them once (the emulator distribution, about 70 MB, and the boot ROM), builds the SD card and verifies it, keeping what is already there; check says where they are and whether a Next can start from them; cleanup deletes what an unfinished download left behind, never the boot files themselves. They go to the -next-files folder, or the per-user one"

const (
	nextDistributionHelp = "Next: address of the emulator distribution zip for -next-firmware setup (default: the 26.11 distribution at specnext.com)"
	nextBootROMURLHelp   = "Next: address of the boot ROM for -next-firmware setup"
	nextSetupHelp        = "Next: the older spelling of -next-firmware setup"
)

// nextFirmwareAction is the action asked for by -next-firmware, or by its older spelling
// -next-setup (legacySetup): "" when neither was given.
func nextFirmwareAction(firmware string, legacySetup bool) (string, error) {
	action := strings.ToLower(strings.TrimSpace(firmware))
	switch action {
	case "":
		if legacySetup {
			return firmwareSetup, nil
		}
		return "", nil
	case firmwareSetup, firmwareCheck, firmwareCleanup:
		if legacySetup && action != firmwareSetup {
			return "", fmt.Errorf("-next-setup is -next-firmware setup; it cannot be given with -next-firmware %s", action)
		}
		return action, nil
	}
	return "", fmt.Errorf("-next-firmware %q: it is one of %s, %s or %s", firmware, firmwareSetup, firmwareCheck, firmwareCleanup)
}

// firmwareState is how far a firmwareJob has got.
type firmwareState struct {
	Stage    nextcard.Stage
	Done     int64
	Total    int64 // 0 when the size of the stage is not known
	Finished bool
	Err      error // set with Finished
}

// firmwareRunFunc is the work of a firmwareJob; a test replaces nextcard.Setup with its own.
type firmwareRunFunc func(ctx context.Context, opts nextcard.Options, progress nextcard.Progress) error

// firmwareJob is one run of the download, with its progress kept under a lock: Run does it
// on the caller's goroutine, Start on one of its own, and State can be read from any.
type firmwareJob struct {
	opts nextcard.Options
	run  firmwareRunFunc

	mu     sync.Mutex
	state  firmwareState
	cancel context.CancelFunc
}

// newFirmwareJob makes a job for dir. An address that is empty, once the spaces round it
// are taken off, is the default one; run nil is nextcard.Setup.
func newFirmwareJob(dir, distributionURL, bootROMURL string, run firmwareRunFunc) *firmwareJob {
	if run == nil {
		run = nextcard.Setup
	}
	return &firmwareJob{
		opts:  nextcard.Options{Dir: dir, DistributionURL: strings.TrimSpace(distributionURL), BootROMURL: strings.TrimSpace(bootROMURL)},
		run:   run,
		state: firmwareState{Stage: nextcard.StageBootROM},
	}
}

// Run does the work and returns its result. onProgress, when not nil, is told of every step
// after State has been brought up to date.
func (j *firmwareJob) Run(ctx context.Context, onProgress nextcard.Progress) error {
	err := j.run(ctx, j.opts, func(stage nextcard.Stage, done, total int64) {
		j.mu.Lock()
		j.state.Stage, j.state.Done, j.state.Total = stage, done, total
		j.mu.Unlock()
		if onProgress != nil {
			onProgress(stage, done, total)
		}
	})
	j.mu.Lock()
	j.state.Finished, j.state.Err = true, err
	j.mu.Unlock()
	return err
}

// Start does the work on a goroutine of its own.
func (j *firmwareJob) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	j.mu.Lock()
	j.cancel = cancel
	j.mu.Unlock()
	go j.Run(ctx, nil)
}

// Cancel stops a job that was started; the work ends on its own, leaving only ".part"
// files. It does nothing for one that was not started, and can be called again.
func (j *firmwareJob) Cancel() {
	j.mu.Lock()
	cancel := j.cancel
	j.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// State is how far the job has got.
func (j *firmwareJob) State() firmwareState {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.state
}

// nextFirmwareVerify is the check a download ends with; a test replaces it.
var nextFirmwareVerify = nextcard.Verify

// nextSetupRun is the work of -next-firmware setup: nil is nextcard.Setup; a test replaces it.
var nextSetupRun firmwareRunFunc

// runNextFirmware is -next-firmware: the action asked for, with its report on stdout (check)
// or stderr (the others, which are progress), and the exit code of the program: 0 when it
// did what was asked, 1 when it could not, and for check when a Next cannot start from the
// files. dist and boot are the addresses of -next-distribution and -next-bootrom-url.
func runNextFirmware(action, flagDir, dist, boot string, stdout, stderr io.Writer) int {
	var err error
	switch action {
	case firmwareSetup:
		var dir string
		if dir, err = nextDownloadDir(flagDir); err == nil {
			err = runNextSetup(dir, dist, boot, stderr)
		}
		if err == nil {
			err = pruneNextCaches(flagDir, stderr)
		}
	case firmwareCheck:
		err = checkNextFirmware(flagDir, stdout)
	case firmwareCleanup:
		if err = cleanupNextFirmware(flagDir, stderr); err == nil {
			err = pruneNextCaches(flagDir, stderr)
		}
	default:
		err = fmt.Errorf("-next-firmware %q is not an action", action)
	}
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

// firmwareNames are the three files in the order the reports give them.
var firmwareNames = []string{nextcard.BootROMName, nextcard.DivMMCROMName, nextcard.CardName}

// missingFirmware names those of the three files that dir does not hold.
func missingFirmware(dir string) []string {
	var missing []string
	for _, n := range firmwareNames {
		if st, err := os.Stat(filepath.Join(dir, n)); err != nil || st.IsDir() {
			missing = append(missing, n)
		}
	}
	return missing
}

// checkNextFirmware says, folder by folder in the order they are searched, what is there and
// whether a Next can start from it, and then what a download would do. It is an error when
// there is no folder to start from.
func checkNextFirmware(flagDir string, w io.Writer) error {
	fmt.Fprintf(w, "Next boot files: %s, %s and %s\n", firmwareNames[0], firmwareNames[1], firmwareNames[2])
	var usable string
	seen := map[string]bool{}
	for _, d := range nextFilesDirs(flagDir) {
		if seen[d] {
			continue
		}
		seen[d] = true
		missing := missingFirmware(d)
		switch {
		case len(missing) == len(firmwareNames):
			fmt.Fprintf(w, "  %s: none of them\n", d)
		case len(missing) > 0:
			fmt.Fprintf(w, "  %s: missing %s\n", d, strings.Join(missing, ", "))
		default:
			if err := nextFilesUsable(d); err != nil {
				fmt.Fprintf(w, "  %s: all three, not usable: %v\n", d, err)
				continue
			}
			fmt.Fprintf(w, "  %s: all three, usable\n", d)
			if usable == "" {
				usable = d
			}
		}
	}
	dir, err := nextDownloadDir(flagDir)
	if err != nil {
		return err
	}
	if left := nextcard.Leftovers(dir); len(left) > 0 {
		var size int64
		for _, l := range left {
			size += l.Size
		}
		fmt.Fprintf(w, "Left by a download that did not finish, in %s: %d file(s), %.1f MB (-next-firmware cleanup deletes them)\n", dir, len(left), float64(size)/(1<<20))
	}
	if usable == "" {
		fmt.Fprintf(w, "A download would put them in %s (-next-firmware setup)\n", dir)
		return fmt.Errorf("no folder holds Next boot files that a Next can start from")
	}
	fmt.Fprintf(w, "A Next starts from %s\n", usable)
	reportNextCaches(w, usable)
	if err := nextFirmwareVerify(usable); err != nil {
		fmt.Fprintf(w, "  not as a download leaves them: %v (a card that has been in use is accepted all the same)\n", err)
	} else {
		fmt.Fprintf(w, "  as a download leaves them: verified\n")
	}
	return nil
}

// cleanupNextFirmware deletes what an unfinished download left in the folder a download
// goes to and in the one a Next starts from, and says what it deleted.
func cleanupNextFirmware(flagDir string, w io.Writer) error {
	dir, err := nextDownloadDir(flagDir)
	if err != nil {
		return err
	}
	dirs := []string{dir}
	if f, ok, _ := locateNextFiles(flagDir); ok && f.Dir != dir {
		dirs = append(dirs, f.Dir)
	}
	var total int64
	count := 0
	for _, d := range dirs {
		removed, err := nextcard.Cleanup(d)
		for _, r := range removed {
			fmt.Fprintf(w, "Deleted %s (%.1f MB)\n", r.Path, float64(r.Size)/(1<<20))
			total += r.Size
			count++
		}
		if err != nil {
			return err
		}
	}
	if count == 0 {
		fmt.Fprintf(w, "Nothing to clean up in %s\n", strings.Join(dirs, " or "))
		return nil
	}
	fmt.Fprintf(w, "%d file(s), %.1f MB freed. The boot ROM, the DivMMC ROM and the card are not touched: the card may hold files that were saved on it\n", count, float64(total)/(1<<20))
	return nil
}
