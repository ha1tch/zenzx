// Package nextcard obtains and prepares the files a ZX Spectrum Next needs to boot
// NextZXOS: the SD card image, the DivMMC ROM and the boot ROM.
//
// The card comes from the official emulator distribution (sn-emulator-NN.NN.zip), which
// holds a 1 GB FAT32 image and the ROMs that match it. That image cannot be used as it is:
//
//  1. It has 32K clusters, which gives it about 32,750 of them. FAT32 needs at least
//     65,525, and the Next's firmware checks. The card is therefore built again with 4K
//     clusters, same size and same partition table, and every file copied across.
//  2. Its /machines/next/config.ini says timing=8, which makes the firmware run its
//     video-mode test and wait for a key. The copy says timing=0 (VGA, 50 Hz).
//  3. If /machines/next/zx81.rom is the 16K file of older distributions it is doubled to
//     the 32K the firmware's menu loads.
//  4. Its folders have short names with numeric tails ("MACHIN~1" for "machines"), as the
//     tool that made it writes them for any name with lower case letters. The firmware
//     reads short names only and asks for /machines/next/menu.def, so it does not find its
//     own files. Every short name is made again: a name that fits 8.3 apart from the case
//     of its letters gets that name in capitals, with the long name kept beside it.
//  5. The free space record (the FSInfo sector) is written from the finished FAT.
//
// This file is the FAT32 part: a reader that walks an existing volume and a builder that
// writes a new one. Both were written from Microsoft's FAT32 specification (fatgen103) for
// this one job, and are not a general FAT library: the builder only creates a volume from
// scratch, with every file and folder in one contiguous run of clusters.
package nextcard

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf16"
)

const (
	sectorSize = 512

	attrReadOnly  = 0x01
	attrHidden    = 0x02
	attrSystem    = 0x04
	attrVolumeID  = 0x08
	attrDirectory = 0x10
	attrArchive   = 0x20
	attrLongName  = 0x0F

	fatEOC  = 0x0FFFFFFF
	fatMask = 0x0FFFFFFF

	// MinFAT32Clusters is the smallest number of clusters a FAT32 volume may have.
	MinFAT32Clusters = 65525
)

// Node is one file or folder of a volume.
type Node struct {
	Name     string   // the long name, or the short name as it is displayed when there is none
	Short    [11]byte // the 8.3 name as stored
	NTRes    byte     // the case bits of the short name
	Attr     byte
	Times    [11]byte // creation tenth, time and date, access date; then write time and date
	Size     uint32
	Children []*Node // folders only

	hasLong bool
	src     uint32 // first cluster in the volume it was read from
	first   uint32 // first cluster in the volume being built
	count   uint32 // clusters it occupies in the volume being built
	// data, when not nil, replaces the file's content in the volume being built.
	data []byte
}

// IsDir reports whether the node is a folder.
func (n *Node) IsDir() bool { return n.Attr&attrDirectory != 0 }

// Find returns the node at path, with "/" between names, ignoring case; nil when there is
// none. Find("") is the node itself.
func (n *Node) Find(path string) *Node {
	cur := n
	for _, part := range strings.Split(strings.Trim(path, "/"), "/") {
		if part == "" {
			continue
		}
		var next *Node
		for _, c := range cur.Children {
			if strings.EqualFold(c.Name, part) || strings.EqualFold(shortDisplay(c.Short, 0), part) {
				next = c
				break
			}
		}
		if next == nil {
			return nil
		}
		cur = next
	}
	return cur
}

// Walk calls fn for the node and everything under it, parents first.
func (n *Node) Walk(fn func(path string, n *Node)) { n.walk("", fn) }

func (n *Node) walk(path string, fn func(string, *Node)) {
	fn(path, n)
	for _, c := range n.Children {
		c.walk(path+"/"+c.Name, fn)
	}
}

// FileAt returns the path of the file whose content starts at byte offset
// of the image the volume was read from, for a host that has seen a block
// read and wants to know whose it is.
func (v *Volume) FileAt(offset int64) (path string, ok bool) {
	v.Root.Walk(func(p string, n *Node) {
		if ok || n.IsDir() || n.src < 2 || n.Size == 0 {
			return
		}
		if v.geo.clusterOffset(n.src) == offset {
			path, ok = p, true
		}
	})
	return path, ok
}

// geometry is what a FAT32 boot sector says about its volume.
type geometry struct {
	start        int64 // first sector of the partition in the image
	sectors      uint32
	perCluster   uint32
	reserved     uint32
	fats         uint32
	fatSectors   uint32
	rootCluster  uint32
	fsInfo       uint32
	backupBoot   uint32
	clusterCount uint32
}

func (g geometry) clusterBytes() int64 { return int64(g.perCluster) * sectorSize }

func (g geometry) clusterOffset(cluster uint32) int64 {
	first := int64(g.reserved) + int64(g.fats)*int64(g.fatSectors)
	return (g.start + first + int64(cluster-2)*int64(g.perCluster)) * sectorSize
}

func (g geometry) fatOffset(copy uint32) int64 {
	return (g.start + int64(g.reserved) + int64(copy)*int64(g.fatSectors)) * sectorSize
}

// partition reads the MBR and returns the first partition's start and length in sectors.
// The MBR itself is returned too, for the builder to copy.
func partition(img io.ReaderAt) (mbr []byte, start, sectors uint32, err error) {
	mbr = make([]byte, sectorSize)
	if _, err = img.ReadAt(mbr, 0); err != nil {
		return nil, 0, 0, fmt.Errorf("reading the partition table: %w", err)
	}
	if mbr[510] != 0x55 || mbr[511] != 0xAA {
		return nil, 0, 0, errors.New("the image has no partition table")
	}
	if t := mbr[446+4]; t != 0x0B && t != 0x0C {
		return nil, 0, 0, fmt.Errorf("the first partition has type 0x%02X, not FAT32", t)
	}
	start = binary.LittleEndian.Uint32(mbr[446+8:])
	sectors = binary.LittleEndian.Uint32(mbr[446+12:])
	if start == 0 || sectors == 0 {
		return nil, 0, 0, errors.New("the first partition is empty")
	}
	return mbr, start, sectors, nil
}

func readGeometry(img io.ReaderAt, start uint32) (geometry, error) {
	b := make([]byte, sectorSize)
	if _, err := img.ReadAt(b, int64(start)*sectorSize); err != nil {
		return geometry{}, fmt.Errorf("reading the boot sector: %w", err)
	}
	g := geometry{
		start:       int64(start),
		perCluster:  uint32(b[13]),
		reserved:    uint32(binary.LittleEndian.Uint16(b[14:])),
		fats:        uint32(b[16]),
		sectors:     binary.LittleEndian.Uint32(b[32:]),
		fatSectors:  binary.LittleEndian.Uint32(b[36:]),
		rootCluster: binary.LittleEndian.Uint32(b[44:]),
		fsInfo:      uint32(binary.LittleEndian.Uint16(b[48:])),
		backupBoot:  uint32(binary.LittleEndian.Uint16(b[50:])),
	}
	switch {
	case b[510] != 0x55 || b[511] != 0xAA:
		return g, errors.New("the partition has no boot sector")
	case binary.LittleEndian.Uint16(b[11:]) != sectorSize:
		return g, errors.New("the volume does not have 512-byte sectors")
	case g.perCluster == 0 || g.fats == 0 || g.fatSectors == 0 || g.reserved == 0:
		return g, errors.New("the boot sector does not describe a FAT32 volume")
	case binary.LittleEndian.Uint16(b[17:]) != 0 || binary.LittleEndian.Uint16(b[22:]) != 0:
		return g, errors.New("the volume is FAT12 or FAT16, not FAT32")
	}
	data := int64(g.sectors) - int64(g.reserved) - int64(g.fats)*int64(g.fatSectors)
	if data <= 0 {
		return g, errors.New("the boot sector leaves no room for data")
	}
	g.clusterCount = uint32(data / int64(g.perCluster))
	return g, nil
}

// Volume is a FAT32 volume opened for reading.
type Volume struct {
	img  io.ReaderAt
	geo  geometry
	fat  []uint32
	Root *Node
}

// ClusterCount is the number of data clusters the volume has.
func (v *Volume) ClusterCount() uint32 { return v.geo.clusterCount }

// SectorsPerCluster is the cluster size in sectors.
func (v *Volume) SectorsPerCluster() uint32 { return v.geo.perCluster }

// Open reads the first partition of img: its FAT and its whole folder tree.
func Open(img io.ReaderAt) (*Volume, error) {
	_, start, _, err := partition(img)
	if err != nil {
		return nil, err
	}
	geo, err := readGeometry(img, start)
	if err != nil {
		return nil, err
	}
	v := &Volume{img: img, geo: geo}
	raw := make([]byte, (int(geo.clusterCount)+2)*4)
	if _, err := img.ReadAt(raw, geo.fatOffset(0)); err != nil {
		return nil, fmt.Errorf("reading the FAT: %w", err)
	}
	v.fat = make([]uint32, geo.clusterCount+2)
	for i := range v.fat {
		v.fat[i] = binary.LittleEndian.Uint32(raw[i*4:]) & fatMask
	}
	v.Root = &Node{Attr: attrDirectory, src: geo.rootCluster}
	if err := v.readDir(v.Root, 0); err != nil {
		return nil, err
	}
	return v, nil
}

// chain returns the clusters of a file or folder, in order.
func (v *Volume) chain(first uint32) ([]uint32, error) {
	var out []uint32
	for c := first; c >= 2 && c < 0x0FFFFFF8; c = v.fat[c] {
		if int(c) >= len(v.fat) || len(out) > len(v.fat) {
			return nil, errors.New("a cluster chain leaves the volume or loops")
		}
		out = append(out, c)
	}
	return out, nil
}

func (v *Volume) readChain(first uint32) ([]byte, error) {
	clusters, err := v.chain(first)
	if err != nil {
		return nil, err
	}
	size := v.geo.clusterBytes()
	out := make([]byte, int64(len(clusters))*size)
	for i, c := range clusters {
		if _, err := v.img.ReadAt(out[int64(i)*size:int64(i+1)*size], v.geo.clusterOffset(c)); err != nil {
			return nil, fmt.Errorf("reading cluster %d: %w", c, err)
		}
	}
	return out, nil
}

// ReadFile returns the content of a file node of this volume.
func (v *Volume) ReadFile(n *Node) ([]byte, error) {
	if n.data != nil {
		return n.data, nil
	}
	if n.Size == 0 {
		return nil, nil
	}
	data, err := v.readChain(n.src)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) < int64(n.Size) {
		return nil, fmt.Errorf("%s: the chain is shorter than the file's length", n.Name)
	}
	return data[:n.Size], nil
}

func (v *Volume) readDir(dir *Node, depth int) error {
	if depth > 64 {
		return errors.New("the folders are nested too deeply")
	}
	raw, err := v.readChain(dir.src)
	if err != nil {
		return err
	}
	var long []uint16
	longSum, longNext := byte(0), 0
	for off := 0; off+32 <= len(raw); off += 32 {
		e := raw[off : off+32]
		if e[0] == 0x00 {
			break
		}
		if e[0] == 0xE5 {
			long = nil
			continue
		}
		if e[11]&0x3F == attrLongName {
			seq := int(e[0] & 0x1F)
			if e[0]&0x40 != 0 {
				long = make([]uint16, seq*13)
				longSum, longNext = e[13], seq
			}
			if long == nil || seq != longNext || seq == 0 || e[13] != longSum {
				long = nil
				continue
			}
			longNext--
			for i, at := range [13]int{1, 3, 5, 7, 9, 14, 16, 18, 20, 22, 24, 28, 30} {
				long[(seq-1)*13+i] = binary.LittleEndian.Uint16(e[at:])
			}
			continue
		}
		if e[11]&attrVolumeID != 0 {
			long = nil
			continue
		}
		n := &Node{Attr: e[11], NTRes: e[12], Size: binary.LittleEndian.Uint32(e[28:])}
		copy(n.Short[:], e[0:11])
		copy(n.Times[0:7], e[13:20])
		copy(n.Times[7:11], e[22:26])
		n.src = uint32(binary.LittleEndian.Uint16(e[20:]))<<16 | uint32(binary.LittleEndian.Uint16(e[26:]))
		if long != nil && longNext == 0 && shortChecksum(n.Short) == longSum {
			end := len(long)
			for i, u := range long {
				if u == 0 {
					end = i
					break
				}
			}
			n.Name, n.hasLong = string(utf16.Decode(long[:end])), true
		} else {
			n.Name = shortDisplay(n.Short, n.NTRes)
		}
		long = nil
		if n.Name == "." || n.Name == ".." {
			continue
		}
		dir.Children = append(dir.Children, n)
		if n.IsDir() {
			if err := v.readDir(n, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

// shortDisplay turns a stored 8.3 name into the name it is shown as.
func shortDisplay(short [11]byte, ntres byte) string {
	base := strings.TrimRight(string(short[0:8]), " ")
	ext := strings.TrimRight(string(short[8:11]), " ")
	if len(base) > 0 && base[0] == 0x05 {
		base = "\xE5" + base[1:]
	}
	if ntres&0x08 != 0 {
		base = strings.ToLower(base)
	}
	if ntres&0x10 != 0 {
		ext = strings.ToLower(ext)
	}
	if ext == "" {
		return base
	}
	return base + "." + ext
}

// shortChecksum is the checksum a long name's entries carry of the short name they belong to.
func shortChecksum(short [11]byte) byte {
	var sum byte
	for _, c := range short {
		sum = (sum>>1 | sum<<7) + c
	}
	return sum
}
