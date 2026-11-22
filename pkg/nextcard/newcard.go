package nextcard

import (
	"encoding/binary"
	"errors"
	"os"
)

// SmallestCard is the size of the smallest card NewCard makes: FAT32 needs 65525 clusters
// and Build's are 4K each. The file is mostly holes, so where the file system has sparse
// files (Linux's and macOS's own do) it takes little more disk than what is put on it.
const SmallestCard = 280 << 20

// NewCard writes a new card image to path with the tree under root on it: a partition
// table with one FAT32 partition, as a card bought and formatted has. size is the card's
// size in bytes, SmallestCard or more. The file must not exist.
func NewCard(path string, size int64, root *Node) (Layout, error) {
	if size < SmallestCard {
		return Layout{}, errors.New("a card has to be 280 MB or more")
	}
	const start = 2048 // where a partition usually starts: at 1 MB
	sectors := uint32(size/sectorSize) - start
	mbr := make([]byte, sectorSize)
	entry := mbr[446:]
	entry[4] = 0x0C // FAT32, LBA
	binary.LittleEndian.PutUint32(entry[8:], start)
	binary.LittleEndian.PutUint32(entry[12:], sectors)
	mbr[510], mbr[511] = 0x55, 0xAA

	out, err := os.OpenFile(path+".part", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return Layout{}, err
	}
	if _, err := os.Stat(path); err == nil {
		out.Close()
		os.Remove(path + ".part")
		return Layout{}, errors.New(path + " is already there")
	}
	var lay Layout
	if err = out.Truncate(size); err == nil {
		lay, err = Build(out, mbr, start, sectors, root, nil, nil)
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(path + ".part")
		return Layout{}, err
	}
	return lay, os.Rename(path+".part", path)
}
