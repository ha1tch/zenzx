// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ha1tch/zenzx/pkg/nextcache"
	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// What -next-firmware says and does about the boot caches (docs/proposals/NEXT_BOOT_CACHE.md):
// check reports whether the caches in the folder are good for the firmware that a Next
// would start from, and which part of the key rules one out when it is not; cleanup and
// setup remove the caches that no firmware on this computer can use any longer.

// nextCacheProbeKey is the key a Next started from the files in dir would have. The layout
// of the machine's state is a property of the program, not of the machine that is running,
// so the key is made from a machine that has not been started.
func nextCacheProbeKey(files nextcard.Files) (nextcache.Key, error) {
	return NewZenZX(AudioBackendOto).nextCacheKeyFor(files)
}

// reportNextCaches says, for the folder a Next starts from, what each stage's cache is.
func reportNextCaches(w io.Writer, filesDir string) {
	dir, err := nextBootCache.directory()
	if err != nil {
		fmt.Fprintf(w, "Boot caches: no folder to keep them in: %v\n", err)
		return
	}
	key, err := nextCacheProbeKey(nextcard.In(filesDir))
	if err != nil {
		fmt.Fprintf(w, "Boot caches in %s: cannot say for the files in %s: %v\n", dir, filesDir, err)
		return
	}
	infos := nextcache.List(dir)
	if len(infos) == 0 {
		fmt.Fprintf(w, "Boot caches in %s: none yet; the first start from the firmware makes them (-next-boot-cache)\n", dir)
		return
	}
	fmt.Fprintf(w, "Boot caches in %s, for the firmware on %s (digest %.12s):\n", dir, filepath.Join(filesDir, nextcard.CardName), key.Firmware.Digest)
	other := 0
	for _, stage := range nextcache.Stages {
		var found *nextcache.Info
		for i := range infos {
			if infos[i].Err == nil && infos[i].Stage == stage && infos[i].Key.Firmware.Digest == key.Firmware.Digest {
				found = &infos[i]
			}
		}
		switch {
		case found == nil:
			fmt.Fprintf(w, "  %-9s none\n", stage)
		case len(found.Key.DiffFor(key, stage)) == 0:
			fmt.Fprintf(w, "  %-9s valid: %d frames of boot skipped, %.1f MB, made %s\n", stage, found.Frames,
				float64(found.Size)/(1<<20), found.Created.Format("2006-01-02 15:04"))
		default:
			fmt.Fprintf(w, "  %-9s out of date, differs in: %s\n", stage, strings.Join(found.Key.DiffFor(key, stage), ", "))
		}
	}
	for _, in := range infos {
		if in.Err != nil || in.Key.Firmware.Digest != key.Firmware.Digest {
			other++
		}
	}
	if other > 0 {
		fmt.Fprintf(w, "  %d more for other firmware or damaged (-next-firmware cleanup removes them)\n", other)
	}
}

// pruneNextCaches removes the caches that nothing can use: those that do not read back, those
// of a firmware that no folder a Next starts from holds now, and those made by another layout
// of the machine's state. It says what it removed.
func pruneNextCaches(flagDir string, w io.Writer) error {
	dir, err := nextBootCache.directory()
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	var keys []nextcache.Key
	seen := map[string]bool{}
	for _, d := range nextFilesDirs(flagDir) {
		if seen[d] {
			continue
		}
		seen[d] = true
		if f := nextcard.In(d); f.Present() && nextFilesUsable(d) == nil {
			if k, err := nextCacheProbeKey(f); err == nil {
				keys = append(keys, k)
			}
		}
	}
	n, err := nextcache.Prune(dir, func(in nextcache.Info) bool {
		for _, k := range keys {
			if in.Key.Firmware.Digest == k.Firmware.Digest && in.Key.Layout == k.Layout && in.Key.Version == k.Version {
				return true
			}
		}
		return false
	})
	if err != nil {
		return err
	}
	if n > 0 {
		fmt.Fprintf(w, "Deleted %d boot cache file(s) in %s that no firmware here can use\n", n, dir)
	}
	return nil
}

// nextBootCacheHelp is the help of -next-boot-cache, for both front ends.
const nextBootCacheHelp = "Next: start from the state of a Next that has booted instead of booting. auto uses a cache made from the same firmware and makes one when there is none (the first start boots as ever, the later ones skip the boot); off boots from the firmware every time; refresh boots from the firmware and makes the caches again. They are kept in the per-user folder, or in ZENZX_NEXT_CACHE, and -next-firmware check says what is in them"

// applyNextBootCacheFlag sets what the front end chose from the value of -next-boot-cache.
func applyNextBootCacheFlag(value string) error {
	mode, err := parseNextCacheMode(strings.TrimSpace(value))
	if err != nil {
		return err
	}
	nextBootCache = nextBootCacheSettings{mode: mode, logf: func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}}
	return nil
}
