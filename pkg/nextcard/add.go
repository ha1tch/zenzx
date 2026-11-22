package nextcard

import (
	"context"
	"errors"
	"os"
	"strings"
)

// File is a file to put on a card.
type File struct {
	Name string
	Data []byte
}

// AddFiles puts files into a folder of the card at cardPath, making the folder (and the
// folders above it) when it is not there and replacing a file of the same name. Everything
// else on the card is kept as it is, what the user has saved included.
//
// A card is written as a whole (Build), so this writes a new one beside the old and puts it
// in its place when it is complete: it needs the card's size again in free disk space while
// it works, and a failure leaves the card untouched.
func AddFiles(ctx context.Context, cardPath, folder string, files []File, progress func(done, total int64)) error {
	return rewriteCard(ctx, cardPath, progress, func(root *Node) error {
		dir, err := folderOf(root, folder)
		if err != nil {
			return err
		}
		for _, f := range files {
			if err := putFile(dir, f.Name, f.Data); err != nil {
				return err
			}
		}
		return nil
	})
}

// AddTree is AddFiles for files that go to different folders: each file's Name is its
// path on the card, folders and all, "machines/next/config.ini". It is one rewrite of the
// card however many folders the files are in.
func AddTree(ctx context.Context, cardPath string, files []File, progress func(done, total int64)) error {
	return rewriteCard(ctx, cardPath, progress, func(root *Node) error {
		for _, f := range files {
			path := strings.Trim(f.Name, "/")
			folder, name := "", path
			if i := strings.LastIndex(path, "/"); i >= 0 {
				folder, name = path[:i], path[i+1:]
			}
			dir, err := folderOf(root, folder)
			if err != nil {
				return err
			}
			if err := putFile(dir, name, f.Data); err != nil {
				return err
			}
		}
		return nil
	})
}

// folderOf returns the folder at path under root, making it and the folders above it when
// they are not there. A name that is there already is taken whatever its case.
func folderOf(root *Node, path string) (*Node, error) {
	dir := root
	for _, part := range strings.Split(strings.Trim(path, "/"), "/") {
		if part == "" {
			continue
		}
		next := dir.Find(part)
		if next == nil {
			next = dir.AddDir(part)
		} else if !next.IsDir() {
			return nil, errors.New("the card has a file called " + part + " where the folder would go")
		}
		dir = next
	}
	return dir, nil
}

// putFile puts a file in a folder, in place of one of the same name if there is one.
func putFile(dir *Node, name string, data []byte) error {
	if name == "" || strings.ContainsAny(name, `/\:*?"<>|`) {
		return errors.New("not a name a file on the card can have: " + name)
	}
	if old := dir.Find(name); old != nil {
		if old.IsDir() {
			return errors.New("the card has a folder called " + name + " where the file would go")
		}
		old.SetData(data)
		return nil
	}
	dir.Add(name, data)
	return nil
}

// rewriteCard reads the card at cardPath, lets change alter its tree, and writes the
// result in the card's place.
func rewriteCard(ctx context.Context, cardPath string, progress func(done, total int64), change func(root *Node) error) error {
	src, err := os.Open(cardPath)
	if err != nil {
		return err
	}
	defer src.Close()
	st, err := src.Stat()
	if err != nil {
		return err
	}
	mbr, start, sectors, err := partition(src)
	if err != nil {
		return err
	}
	vol, err := Open(src)
	if err != nil {
		return err
	}
	if err := change(vol.Root); err != nil {
		return err
	}
	// Short names are made again from the long ones, as Rebuild does.
	vol.Root.Walk(func(_ string, n *Node) {
		n.Short, n.NTRes = [11]byte{}, 0
	})
	part := cardPath + ".part"
	out, err := os.Create(part)
	if err != nil {
		return err
	}
	if err = out.Truncate(st.Size()); err == nil {
		_, err = Build(out, mbr, start, sectors, vol.Root, func(n *Node) ([]byte, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return vol.ReadFile(n)
		}, progress)
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	// Files that are new to the card are not read through the callback, so
	// a run that was stopped is looked for once more before the new card
	// takes the old one's place.
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		os.Remove(part)
		return err
	}
	src.Close()
	return os.Rename(part, cardPath)
}
