// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ha1tch/zenzx/pkg/nextcache"
	"github.com/ha1tch/zenzx/pkg/nextcard"
	zxversion "github.com/ha1tch/zenzx/pkg/version"
)

// A Next that boots once (docs/proposals/NEXT_BOOT_CACHE.md).
//
// The firmware loads the ROMs and NextZXOS starts: about 430 frames to the firmware's
// hand-over and 550 to a NextZXOS at rest, 9 s and 11 s at the real speed. The first time
// that happens the state of the machine is kept (nextstate.go), and a later start puts it
// back instead of booting. It is used only for the firmware it was made from, which is
// identified by one digest over the firmware files on the card (pkg/nextcache).

// nextCacheMode is the -next-boot-cache setting.
type nextCacheMode int

const (
	nextCacheOff     nextCacheMode = iota // always boot from the firmware, and keep nothing
	nextCacheAuto                         // start from a cache when there is a good one, and make one when there is not
	nextCacheRefresh                      // boot from the firmware and make a new cache, whatever there is
)

func (m nextCacheMode) String() string {
	return [...]string{"off", "auto", "refresh"}[m]
}

// parseNextCacheMode reads a -next-boot-cache value.
func parseNextCacheMode(s string) (nextCacheMode, error) {
	switch s {
	case "off":
		return nextCacheOff, nil
	case "auto":
		return nextCacheAuto, nil
	case "refresh":
		return nextCacheRefresh, nil
	}
	return nextCacheOff, fmt.Errorf("-next-boot-cache %q: it is auto, off or refresh", s)
}

// nextBootCacheSettings is what the front end chose; the zero value is off, which is what
// every test and every machine that is not started by a front end gets.
type nextBootCacheSettings struct {
	mode nextCacheMode
	dir  string // empty: nextCacheDir()
	logf func(format string, args ...any)
}

// nextBootCache is set once, at start-up, by the front end.
var nextBootCache nextBootCacheSettings

// nextCacheDirEnv names a folder for the caches, for a test or a portable install.
const nextCacheDirEnv = "ZENZX_NEXT_CACHE"

// nextCacheDir is where the caches are kept: the per-user folder, never the card and never
// the tree, because they hold the firmware's and NextZXOS's own contents.
func nextCacheDir() (string, error) {
	if d := os.Getenv(nextCacheDirEnv); d != "" {
		return d, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "zenzx", "next-cache"), nil
}

func (s nextBootCacheSettings) directory() (string, error) {
	if s.dir != "" {
		return s.dir, nil
	}
	return nextCacheDir()
}

func (s nextBootCacheSettings) say(format string, args ...any) {
	if s.logf != nil {
		s.logf(format, args...)
	}
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// nextCacheKeyFor is the key of a boot chain: what a cache made from it depends on.
func (zx *ZenZX) nextCacheKeyFor(f nextcard.Files) (nextcache.Key, error) {
	boot, err := os.ReadFile(f.BootROM)
	if err != nil {
		return nextcache.Key{}, err
	}
	div, err := os.ReadFile(f.DivMMCROM)
	if err != nil {
		return nextcache.Key{}, err
	}
	card, err := os.Open(f.Card)
	if err != nil {
		return nextcache.Key{}, err
	}
	defer card.Close()
	fw, err := nextcache.ReadFirmware(card, nextcard.BootFiles)
	if err != nil {
		return nextcache.Key{}, err
	}
	start, err := nextcache.ReadFiles(card, nextcard.BootFiles, nextcard.BootExtras)
	if err != nil {
		return nextcache.Key{}, err
	}
	issue := zx.memory.nextPhys.issue
	if issue == 0 {
		issue = 5
	}
	fp := nextStatePolicy().Fingerprint(zx.nextStateRoots()...)
	return nextcache.Key{
		BootROM:   sha256Hex(boot),
		DivMMCROM: sha256Hex(div),
		Firmware:  fw,
		Startup:   start,
		Issue:     strconv.Itoa(issue),
		Patches:   zx.romPatchKeys(),
		Version:   zxversion.Version,
		Layout:    hex.EncodeToString(fp[:]),
	}, nil
}

// armNextBootCache is called by attachNextBootChain: the start of the cache is done at the
// next frame boundary, once whoever attached the chain has finished resetting the machine.
func (zx *ZenZX) armNextBootCache() {
	if zx.next != nil && nextBootCache.mode != nextCacheOff {
		zx.next.cachePending = true
	}
}

// startNextBootCache puts a cache over the machine, or arranges for one to be made. It runs
// at the first frame boundary after a boot chain is attached.
func (zx *ZenZX) startNextBootCache() {
	s := nextBootCache
	zx.next.cachePending = false
	dir, err := s.directory()
	if err != nil {
		s.say("Next boot cache: no folder to keep it in: %v", err)
		return
	}
	key, err := zx.nextCacheKeyFor(zx.next.files)
	if err != nil {
		s.say("Next boot cache is not used: %v", err)
		return
	}
	scratch := zx.next.scratchDir != "" // a card made to run one program: nothing of it is cached
	if s.mode == nextCacheAuto {
		for _, stage := range nextcache.Stages {
			if scratch && stage == nextcache.Settled {
				continue
			}
			e, err := nextcache.Read(dir, key, stage)
			switch {
			case err == nil:
				if rerr := zx.RestoreNextState(bytes.NewReader(e.State)); rerr != nil {
					// Whatever was half put back is not to be trusted: start again from the firmware.
					s.say("Next boot cache (%s) could not be put back: %v; starting from the firmware", stage, rerr)
					os.Remove(nextcache.Path(dir, key.Firmware, stage))
					zx.Reset()
					zx.io.nextResetRequest = nextResetHard
					zx.armNextCacheMaking(dir, key, scratch, true, true)
					return
				}
				s.say("Next started from the %s cache: %d frames of boot skipped", stage, e.Frames)
				if stage == nextcache.HandOver && !scratch {
					// The settled one was not there: make it, from here.
					zx.armNextCacheMaking(dir, key, scratch, false, true)
				}
				return
			case errors.Is(err, nextcache.ErrAbsent):
			default:
				var stale *nextcache.StaleError
				if errors.As(err, &stale) {
					s.say("Next boot cache (%s) is out of date: %v", stage, stale.Differs)
				} else {
					s.say("Next boot cache (%s) is not used: %v", stage, err)
				}
			}
		}
	}
	zx.armNextCacheMaking(dir, key, scratch, true, true)
}

// armNextCacheMaking arranges for the caches to be taken as the machine boots: the
// hand-over one (when handOver is set) as the firmware finishes, the settled one (when
// settled is set, and the chain is not a scratch card) when NextZXOS is at rest.
func (zx *ZenZX) armNextCacheMaking(dir string, key nextcache.Key, scratch, handOver, settled bool) {
	s := nextBootCache
	write := func(stage nextcache.Stage, frames int) {
		b, err := zx.NextStateBytes()
		if err != nil {
			s.say("Next boot cache (%s) was not made: %v", stage, err)
			return
		}
		if err := nextcache.Write(dir, nextcache.Entry{Key: key, Stage: stage, Frames: frames, State: b}); err != nil {
			s.say("Next boot cache (%s) was not written: %v", stage, err)
			return
		}
		s.say("Next boot cache (%s) made at frame %d: %d bytes", stage, frames, len(b))
	}
	watch := func() {
		if scratch || !settled {
			return
		}
		zx.watchNextSettle(func(frames int) { write(nextcache.Settled, frames) },
			func() { s.say("Next boot cache: NextZXOS did not come to rest, so no settled cache was made") })
	}
	if !handOver {
		watch()
		return
	}
	zx.onNextHandOver = func() {
		write(nextcache.HandOver, zx.frameCount)
		watch()
	}
}
