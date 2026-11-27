// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ha1tch/plus3/pkg/diskimg"
)

// addFilesToDisk writes host files into the +3DOS filesystem of the .dsk image at
// dskPath, using plus3's diskimg (the +3DOS/CP-M directory and allocation layer).
// It loads the image, adds each file under its 8.3 name, and saves the image back
// to the same path. It returns the names as written to the disk, for the toast.
//
// The files are added raw (no +3DOS header is synthesised): a dropped file is
// copied onto the disk as-is, the way a .nex or .tap would be, so the bytes on
// the disk are the bytes of the file. A file whose name does not fit +3DOS 8.3 is
// truncated to fit by plus3's own name handling.
func addFilesToDisk(dskPath string, files []string) ([]string, error) {
	di, err := diskimg.LoadFromFile(dskPath)
	if err != nil {
		return nil, fmt.Errorf("reading the disk image: %w", err)
	}
	written := make([]string, 0, len(files))
	for _, host := range files {
		name := diskName(host)
		if err := di.ImportFile(host, name, &diskimg.ImportOptions{AddHeader: false}); err != nil {
			return nil, fmt.Errorf("adding %s: %w", filepath.Base(host), err)
		}
		written = append(written, name)
	}
	if err := di.SaveToFile(dskPath); err != nil {
		return nil, fmt.Errorf("saving the disk image: %w", err)
	}
	return written, nil
}

// diskName turns a host filename into a +3DOS 8.3 name: the base name, upper-cased,
// with the stem clipped to 8 and the extension to 3. plus3 also normalises the
// name, but clipping here keeps the name reported in the toast the same as the one
// written to the disk.
func diskName(host string) string {
	base := filepath.Base(host)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	ext = strings.TrimPrefix(ext, ".")
	if len(stem) > 8 {
		stem = stem[:8]
	}
	if len(ext) > 3 {
		ext = ext[:3]
	}
	stem = strings.ToUpper(stem)
	ext = strings.ToUpper(ext)
	if ext == "" {
		return stem
	}
	return stem + "." + ext
}
