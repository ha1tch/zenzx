// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package fetch

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// MaxUnpacked is the most an archive may hold once it is unpacked. It is far more than any
// archive ZenZX fetches, and stops a small file that expands without limit.
const MaxUnpacked = 8 << 30

// UnpackOptions say how Unpack takes an archive apart.
type UnpackOptions struct {
	// Strip is how many leading folders of every name are dropped, as tar's --strip-components:
	// with 1, "python/bin/python3" becomes "bin/python3", and an entry that is only "python/"
	// is not made.
	Strip int
	// Progress is told how far on it is; total is 0 when that is not known.
	Progress func(done, total int64)
}

// Unpack takes the zip or .tar.gz at archive apart into the folder dest, which is made if it
// is not there. The kind is told by the first bytes of the file, not by its name.
//
// It is built to take apart a file from the network: a name that is absolute, has a drive
// letter, or climbs out with ".." is refused; so is a link that points outside dest, and
// anything that would be written through a link; a file inside an archive keeps its owner's
// permissions (so a program stays a program) but nothing wider is set; and an archive that
// unpacks to more than MaxUnpacked is stopped. On an error, what was already written stays:
// Install, which unpacks into a folder of its own and moves it into place only when whole, is
// what callers use to avoid that.
func Unpack(ctx context.Context, archive, dest string, o UnpackOptions) error {
	if o.Progress == nil {
		o.Progress = func(int64, int64) {}
	}
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	var magic [4]byte
	n, _ := io.ReadFull(f, magic[:])
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	switch {
	case n >= 4 && bytes.Equal(magic[:4], []byte("PK\x03\x04")), n >= 4 && bytes.Equal(magic[:4], []byte("PK\x05\x06")):
		st, err := f.Stat()
		if err != nil {
			return err
		}
		return unpackZip(ctx, f, st.Size(), dest, o)
	case n >= 2 && magic[0] == 0x1f && magic[1] == 0x8b:
		st, err := f.Stat()
		if err != nil {
			return err
		}
		return unpackTarGz(ctx, f, st.Size(), dest, o)
	}
	return fmt.Errorf("%s is neither a zip nor a .tar.gz file", filepath.Base(archive))
}

// entryPath turns the name of an archive entry into a path below dest, or says why it is
// refused. skip is true for an entry that is only a folder that Strip drops.
func entryPath(dest, name string, strip int) (full, rel string, skip bool, err error) {
	name = strings.ReplaceAll(name, "\\", "/")
	if strings.HasPrefix(name, "/") || (len(name) >= 2 && name[1] == ':') {
		return "", "", false, fmt.Errorf("the archive holds a name that is not inside the folder: %q", name)
	}
	var parts []string
	for _, p := range strings.Split(name, "/") {
		switch p {
		case "", ".":
		case "..":
			return "", "", false, fmt.Errorf("the archive holds a name that climbs out of the folder: %q", name)
		default:
			parts = append(parts, p)
		}
	}
	if len(parts) <= strip {
		return "", "", true, nil
	}
	parts = parts[strip:]
	rel = path.Join(parts...)
	return filepath.Join(dest, filepath.FromSlash(rel)), rel, false, nil
}

// noLinksAbove reports an error when a folder on the way from root to path's parent is a link:
// nothing is written through one.
func noLinksAbove(root, p string) error {
	rel, err := filepath.Rel(root, filepath.Dir(p))
	if err != nil || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("%s is not inside %s", p, root)
	}
	cur := root
	if rel == "." {
		return nil
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		cur = filepath.Join(cur, part)
		st, err := os.Lstat(cur)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("the archive would write through a link (%s)", cur)
		}
	}
	return nil
}

// linkStaysInside reports whether a link at rel (a slash path below dest) that points to
// target stays below dest.
func linkStaysInside(rel, target string) error {
	t := strings.ReplaceAll(target, "\\", "/")
	if t == "" || strings.HasPrefix(t, "/") || (len(t) >= 2 && t[1] == ':') {
		return fmt.Errorf("the archive holds a link that points outside the folder: %s -> %s", rel, target)
	}
	joined := path.Join(path.Dir(rel), t)
	if joined == ".." || strings.HasPrefix(joined, "../") {
		return fmt.Errorf("the archive holds a link that points outside the folder: %s -> %s", rel, target)
	}
	return nil
}

func fileMode(m os.FileMode) os.FileMode { return (m.Perm() & 0o755) | 0o600 }

// writeFile copies r into a new file at full, at most max bytes.
func writeFile(root, full string, mode os.FileMode, r io.Reader, max int64) (int64, error) {
	if err := noLinksAbove(root, full); err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return 0, err
	}
	os.Remove(full) // a file that is already there is replaced, never written through
	out, err := os.OpenFile(full, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(out, io.LimitReader(r, max+1))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > max {
		err = fmt.Errorf("%s is larger than the archive says", filepath.Base(full))
	}
	return n, err
}

// linksOK says whether the system can make symbolic links. Windows cannot without a privilege
// an ordinary user lacks, and Windows 7 not at all in the way an archive means them, so there a
// link is made as a copy of the file it points to (see linker.finish).
var linksOK = runtime.GOOS != "windows"

// linker makes the links of one unpacking.
type linker struct {
	root  string
	later []pendingLink // links to be made as copies, once the files they point to are there
}

type pendingLink struct{ full, rel, target string }

func (l *linker) make(full, rel, target string) error {
	if err := linkStaysInside(rel, target); err != nil {
		return err
	}
	if err := noLinksAbove(l.root, full); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	os.Remove(full)
	if !linksOK {
		l.later = append(l.later, pendingLink{full, rel, target})
		return nil
	}
	return os.Symlink(filepath.FromSlash(target), full)
}

// finish makes the links that are copies. A link to a link is made after the link it points to;
// a link to a folder cannot be a copy, and is refused with the reason.
func (l *linker) finish() error {
	for len(l.later) > 0 {
		var left []pendingLink
		for _, p := range l.later {
			from := filepath.Join(l.root, filepath.FromSlash(path.Join(path.Dir(p.rel), strings.ReplaceAll(p.target, "\\", "/"))))
			st, err := os.Stat(from)
			switch {
			case err != nil:
				left = append(left, p)
			case st.IsDir():
				return fmt.Errorf("the archive holds a link to a folder (%s), which this system cannot make", p.rel)
			default:
				if err := copyInside(l.root, from, p.full); err != nil {
					return err
				}
			}
		}
		if len(left) == len(l.later) {
			return fmt.Errorf("the archive holds a link to a file it does not hold: %s -> %s", left[0].rel, left[0].target)
		}
		l.later = left
	}
	return nil
}

type countingReader struct {
	r    io.Reader
	n    int64
	tick func(int64)
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	c.tick(c.n)
	return n, err
}

func unpackTarGz(ctx context.Context, f *os.File, size int64, dest string, o UnpackOptions) error {
	cr := &countingReader{r: f, tick: func(n int64) { o.Progress(n, size) }}
	gz, err := gzip.NewReader(cr)
	if err != nil {
		return fmt.Errorf("the archive is damaged: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	budget := int64(MaxUnpacked)
	lk := &linker{root: dest}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		h, err := tr.Next()
		if err == io.EOF {
			return lk.finish()
		}
		if err != nil {
			return fmt.Errorf("the archive is damaged: %w", err)
		}
		full, rel, skip, err := entryPath(dest, h.Name, o.Strip)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := noLinksAbove(dest, full); err != nil {
				return err
			}
			if err := os.MkdirAll(full, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if h.Size > budget {
				return fmt.Errorf("the archive holds more than %d bytes", int64(MaxUnpacked))
			}
			n, err := writeFile(dest, full, fileMode(os.FileMode(h.Mode)), tr, h.Size)
			if err != nil {
				return err
			}
			budget -= n
		case tar.TypeSymlink:
			if err := lk.make(full, rel, h.Linkname); err != nil {
				return err
			}
		case tar.TypeLink:
			_, trel, tskip, err := entryPath(dest, h.Linkname, o.Strip)
			if err != nil {
				return err
			}
			if tskip {
				return fmt.Errorf("the archive holds a link to a file it does not hold: %s", h.Linkname)
			}
			if err := copyInside(dest, filepath.Join(dest, filepath.FromSlash(trel)), full); err != nil {
				return err
			}
		default:
			// devices, FIFOs and the like are not made
		}
	}
}

// copyInside makes a hard link's file as a copy of the one it names, which is already unpacked.
func copyInside(root, from, to string) error {
	if err := noLinksAbove(root, from); err != nil {
		return err
	}
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer src.Close()
	st, err := src.Stat()
	if err != nil {
		return err
	}
	_, err = writeFile(root, to, fileMode(st.Mode()), src, st.Size())
	return err
}

func unpackZip(ctx context.Context, f *os.File, size int64, dest string, o UnpackOptions) error {
	zr, err := zip.NewReader(f, size)
	if err != nil {
		return fmt.Errorf("the archive is damaged: %w", err)
	}
	var total uint64
	for _, e := range zr.File {
		total += e.UncompressedSize64
	}
	if total > MaxUnpacked {
		return fmt.Errorf("the archive holds more than %d bytes", int64(MaxUnpacked))
	}
	var done int64
	lk := &linker{root: dest}
	for _, e := range zr.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		full, rel, skip, err := entryPath(dest, e.Name, o.Strip)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		mode := e.Mode()
		switch {
		case e.FileInfo().IsDir() || strings.HasSuffix(e.Name, "/"):
			if err := noLinksAbove(dest, full); err != nil {
				return err
			}
			if err := os.MkdirAll(full, 0o755); err != nil {
				return err
			}
		case mode&os.ModeSymlink != 0:
			rc, err := e.Open()
			if err != nil {
				return err
			}
			b, err := io.ReadAll(io.LimitReader(rc, 4096))
			rc.Close()
			if err != nil {
				return err
			}
			if err := lk.make(full, rel, string(b)); err != nil {
				return err
			}
		default:
			rc, err := e.Open()
			if err != nil {
				return err
			}
			_, err = writeFile(dest, full, fileMode(mode), rc, int64(e.UncompressedSize64))
			rc.Close()
			if err != nil {
				return err
			}
		}
		done += int64(e.UncompressedSize64)
		o.Progress(done, int64(total))
	}
	return lk.finish()
}
