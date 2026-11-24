package nextcard

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf16"
)

// The geometry of a card that Build writes: 4K clusters, the usual 32 reserved sectors with
// the backup boot sector at 6, two FATs.
const (
	buildPerCluster = 8
	buildReserved   = 32
	buildFATs       = 2
)

// nowFunc is the clock for new-file timestamps; a package var so tests can pin
// it. A file or folder added to a card is stamped with the current time rather
// than a zero (invalid) FAT date, which some FAT drivers -- NextZXOS's Browser
// among them -- will not refresh or display correctly.
var nowFunc = time.Now

// fatTimes encodes a time into the 11-byte Times layout of a directory entry:
// creation tenth, creation time and date, last-access date (bytes 0..6), then
// write time and date (bytes 7..10). A zero time (the year is below 1980) is
// left as all zeros, matching a FAT "no date" entry.
func fatTimes(t time.Time) [11]byte {
	var b [11]byte
	if t.Year() < 1980 {
		return b
	}
	date := uint16((t.Year()-1980)<<9) | uint16(t.Month())<<5 | uint16(t.Day())
	tm := uint16(t.Hour())<<11 | uint16(t.Minute())<<5 | uint16(t.Second()/2)
	// CrtTimeTenth counts hundredths of a second past CrtTime's 2-second
	// granularity, 0..199: the odd second contributes 100.
	b[0] = byte((t.Second()%2)*100 + t.Nanosecond()/1e7)
	binary.LittleEndian.PutUint16(b[1:3], tm)    // CrtTime
	binary.LittleEndian.PutUint16(b[3:5], date)  // CrtDate
	binary.LittleEndian.PutUint16(b[5:7], date)  // LstAccDate
	binary.LittleEndian.PutUint16(b[7:9], tm)    // WrtTime
	binary.LittleEndian.PutUint16(b[9:11], date) // WrtDate
	return b
}

// touchWrite advances only the write time and date (bytes 7..10) of an existing
// entry to now, leaving its creation time as it was. Used to bump a directory's
// own timestamp when its contents change, so a FAT driver sees the change.
func touchWrite(times [11]byte, t time.Time) [11]byte {
	if t.Year() < 1980 {
		return times
	}
	date := uint16((t.Year()-1980)<<9) | uint16(t.Month())<<5 | uint16(t.Day())
	tm := uint16(t.Hour())<<11 | uint16(t.Minute())<<5 | uint16(t.Second()/2)
	binary.LittleEndian.PutUint16(times[7:9], tm)
	binary.LittleEndian.PutUint16(times[9:11], date)
	return times
}

// NewDir returns an empty folder node for a tree that Build will write.
func NewDir(name string) *Node {
	return &Node{Name: name, Attr: attrDirectory, Times: fatTimes(nowFunc())}
}

// Add puts a file with the given content into a folder and returns its node. Short names
// are worked out when the tree is built. The new file is stamped with the current time, and
// the folder's own write time is bumped, so a FAT driver sees the folder as changed.
func (n *Node) Add(name string, data []byte) *Node {
	now := nowFunc()
	c := &Node{Name: name, Attr: attrArchive, Size: uint32(len(data)), data: append([]byte{}, data...), Times: fatTimes(now)}
	n.Children = append(n.Children, c)
	n.Times = touchWrite(n.Times, now)
	return c
}

// AddDir puts an empty folder into a folder and returns its node. The parent's write time is
// bumped so a FAT driver sees it as changed.
func (n *Node) AddDir(name string) *Node {
	c := NewDir(name)
	n.Children = append(n.Children, c)
	n.Times = touchWrite(n.Times, nowFunc())
	return c
}

// SetData replaces a file's content in the volume that Build will write, advancing the file's
// own write time so a FAT driver sees the change.
func (n *Node) SetData(data []byte) {
	n.data = append([]byte{}, data...)
	n.Size = uint32(len(data))
	n.Times = touchWrite(n.Times, nowFunc())
}

// Layout is where Build put things; Prepare reports it and the tests check it.
type Layout struct {
	Clusters     uint32 // data clusters in the volume
	Used         uint32 // clusters holding files and folders
	Files, Dirs  int
	Bytes        int64 // total length of the files
	FirstFree    uint32
	FreeClusters uint32
}

// Build writes a new FAT32 volume to dst: the partition table mbr, then a partition of the
// given start and length in sectors holding the tree under root. content returns a file's
// bytes; it is not called for a node whose content was given with Add or SetData. progress,
// when not nil, is called with the file bytes written so far and their total.
//
// dst must read as zeros wherever Build does not write: a new file extended with Truncate.
func Build(dst io.WriterAt, mbr []byte, start, sectors uint32, root *Node,
	content func(n *Node) ([]byte, error), progress func(done, total int64)) (Layout, error) {
	geo, err := buildGeometry(start, sectors)
	if err != nil {
		return Layout{}, err
	}
	if len(mbr) != sectorSize {
		return Layout{}, errors.New("the partition table must be one sector")
	}
	if err := assignShortNames(root); err != nil {
		return Layout{}, err
	}

	// Give everything its run of clusters: a folder, then its files, then its folders.
	lay := Layout{Clusters: geo.clusterCount}
	next := uint32(2)
	cb := geo.clusterBytes()
	var place func(dir *Node, isRoot bool)
	place = func(dir *Node, isRoot bool) {
		dir.first, dir.count = next, uint32((int64(dirEntryCount(dir, isRoot))*32+cb-1)/cb)
		if dir.count == 0 {
			dir.count = 1
		}
		next += dir.count
		lay.Dirs++
		for _, c := range dir.Children {
			if c.IsDir() {
				continue
			}
			c.first, c.count = 0, uint32((int64(c.Size)+cb-1)/cb)
			if c.count > 0 {
				c.first = next
				next += c.count
			}
			lay.Files++
			lay.Bytes += int64(c.Size)
		}
		for _, c := range dir.Children {
			if c.IsDir() {
				place(c, false)
			}
		}
	}
	place(root, true)
	lay.Dirs-- // the root is not counted
	lay.Used = next - 2
	if lay.Used > geo.clusterCount {
		return lay, fmt.Errorf("the files need %d clusters and the card has %d", lay.Used, geo.clusterCount)
	}
	lay.FreeClusters = geo.clusterCount - lay.Used
	lay.FirstFree = next
	if lay.FreeClusters == 0 {
		lay.FirstFree = 0xFFFFFFFF
	}

	// The reserved sectors.
	write := func(sector int64, b []byte) error {
		_, err := dst.WriteAt(b, sector*sectorSize)
		return err
	}
	if err := write(0, mbr); err != nil {
		return lay, err
	}
	boot, info, third := bootSector(geo), fsInfoSector(lay.FreeClusters, lay.FirstFree), make([]byte, sectorSize)
	third[510], third[511] = 0x55, 0xAA
	for _, base := range []int64{0, int64(geo.backupBoot)} {
		for i, b := range [][]byte{boot, info, third} {
			if err := write(geo.start+base+int64(i), b); err != nil {
				return lay, err
			}
		}
	}

	// The FAT: every run is contiguous, so each entry points at the next one.
	fat := make([]byte, int64(geo.fatSectors)*sectorSize)
	binary.LittleEndian.PutUint32(fat[0:], 0x0FFFFFF8)
	binary.LittleEndian.PutUint32(fat[4:], fatEOC)
	root.Walk(func(_ string, n *Node) {
		for i := uint32(0); i < n.count; i++ {
			link := n.first + i + 1
			if i == n.count-1 {
				link = fatEOC
			}
			binary.LittleEndian.PutUint32(fat[(n.first+i)*4:], link)
		}
	})
	for c := uint32(0); c < geo.fats; c++ {
		if _, err := dst.WriteAt(fat, geo.fatOffset(c)); err != nil {
			return lay, err
		}
	}

	// Folders and files.
	var done int64
	var emit func(dir, parent *Node) error
	emit = func(dir, parent *Node) error {
		table := make([]byte, int64(dir.count)*cb)
		encodeDir(table, dir, parent, parent == nil)
		if _, err := dst.WriteAt(table, geo.clusterOffset(dir.first)); err != nil {
			return err
		}
		for _, c := range dir.Children {
			if c.IsDir() {
				if err := emit(c, dir); err != nil {
					return err
				}
				continue
			}
			if c.Size == 0 {
				continue
			}
			data := c.data
			if data == nil {
				if data, err = content(c); err != nil {
					return fmt.Errorf("%s: %w", c.Name, err)
				}
			}
			if uint32(len(data)) != c.Size {
				return fmt.Errorf("%s: %d bytes of content for a file of %d", c.Name, len(data), c.Size)
			}
			if _, err := dst.WriteAt(data, geo.clusterOffset(c.first)); err != nil {
				return err
			}
			done += int64(len(data))
			if progress != nil {
				progress(done, lay.Bytes)
			}
		}
		return nil
	}
	if err := emit(root, nil); err != nil {
		return lay, err
	}
	return lay, nil
}

func buildGeometry(start, sectors uint32) (geometry, error) {
	g := geometry{start: int64(start), sectors: sectors, perCluster: buildPerCluster, reserved: buildReserved,
		fats: buildFATs, rootCluster: 2, fsInfo: 1, backupBoot: 6}
	if sectors <= buildReserved {
		return g, errors.New("the partition is too small")
	}
	// The FAT has to hold an entry for every cluster of what is left after the FAT itself.
	for g.fatSectors = 1; ; g.fatSectors++ {
		data := int64(sectors) - buildReserved - buildFATs*int64(g.fatSectors)
		if data <= 0 {
			return g, errors.New("the partition is too small")
		}
		g.clusterCount = uint32(data / buildPerCluster)
		if int64(g.fatSectors)*sectorSize >= (int64(g.clusterCount)+2)*4 {
			break
		}
	}
	if g.clusterCount < MinFAT32Clusters {
		return g, fmt.Errorf("the partition gives %d clusters; FAT32 needs %d", g.clusterCount, MinFAT32Clusters)
	}
	return g, nil
}

func bootSector(g geometry) []byte {
	b := make([]byte, sectorSize)
	copy(b[0:], []byte{0xEB, 0x58, 0x90})
	copy(b[3:], "ZENZX   ")
	binary.LittleEndian.PutUint16(b[11:], sectorSize)
	b[13] = byte(g.perCluster)
	binary.LittleEndian.PutUint16(b[14:], uint16(g.reserved))
	b[16] = byte(g.fats)
	b[21] = 0xF8
	binary.LittleEndian.PutUint16(b[24:], 63)
	binary.LittleEndian.PutUint16(b[26:], 255)
	binary.LittleEndian.PutUint32(b[28:], uint32(g.start))
	binary.LittleEndian.PutUint32(b[32:], g.sectors)
	binary.LittleEndian.PutUint32(b[36:], g.fatSectors)
	binary.LittleEndian.PutUint32(b[44:], g.rootCluster)
	binary.LittleEndian.PutUint16(b[48:], uint16(g.fsInfo))
	binary.LittleEndian.PutUint16(b[50:], uint16(g.backupBoot))
	b[64] = 0x80
	b[66] = 0x29
	binary.LittleEndian.PutUint32(b[67:], 0x5A454E58)
	copy(b[71:], "NEXT       ")
	copy(b[82:], "FAT32   ")
	b[510], b[511] = 0x55, 0xAA
	return b
}

func fsInfoSector(free, next uint32) []byte {
	b := make([]byte, sectorSize)
	copy(b[0:], "RRaA")
	copy(b[484:], "rrAa")
	binary.LittleEndian.PutUint32(b[488:], free)
	binary.LittleEndian.PutUint32(b[492:], next)
	b[510], b[511] = 0x55, 0xAA
	return b
}

// longSlots is the number of long-name entries a node needs before its short entry.
func longSlots(n *Node) int {
	if !n.hasLong {
		return 0
	}
	return (len(utf16.Encode([]rune(n.Name))) + 12) / 13
}

func dirEntryCount(dir *Node, isRoot bool) int {
	count := 0
	if !isRoot {
		count = 2
	}
	for _, c := range dir.Children {
		count += longSlots(c) + 1
	}
	return count
}

func encodeEntry(e []byte, n *Node, short [11]byte, first uint32) {
	copy(e[0:11], short[:])
	e[11], e[12] = n.Attr, n.NTRes
	copy(e[13:20], n.Times[0:7])
	binary.LittleEndian.PutUint16(e[20:], uint16(first>>16))
	copy(e[22:26], n.Times[7:11])
	binary.LittleEndian.PutUint16(e[26:], uint16(first))
	if !n.IsDir() {
		binary.LittleEndian.PutUint32(e[28:], n.Size)
	}
}

func encodeDir(table []byte, dir, parent *Node, isRoot bool) {
	off := 0
	if !isRoot {
		dot := [11]byte{'.', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' '}
		dotdot := dot
		dotdot[1] = '.'
		up := parent.first
		if parent.first == 2 {
			up = 0 // the root is written as cluster 0 in a ".." entry
		}
		plain := &Node{Attr: attrDirectory, Times: dir.Times}
		encodeEntry(table[0:], plain, dot, dir.first)
		encodeEntry(table[32:], plain, dotdot, up)
		off = 64
	}
	for _, c := range dir.Children {
		if slots := longSlots(c); slots > 0 {
			units := utf16.Encode([]rune(c.Name))
			padded := make([]uint16, slots*13)
			for i := range padded {
				switch {
				case i < len(units):
					padded[i] = units[i]
				case i == len(units):
					padded[i] = 0
				default:
					padded[i] = 0xFFFF
				}
			}
			sum := shortChecksum(c.Short)
			for seq := slots; seq >= 1; seq-- {
				e := table[off : off+32]
				e[0] = byte(seq)
				if seq == slots {
					e[0] |= 0x40
				}
				e[11], e[13] = attrLongName, sum
				for i, at := range [13]int{1, 3, 5, 7, 9, 14, 16, 18, 20, 22, 24, 28, 30} {
					binary.LittleEndian.PutUint16(e[at:], padded[(seq-1)*13+i])
				}
				off += 32
			}
		}
		encodeEntry(table[off:], c, c.Short, c.first)
		off += 32
	}
}

// assignShortNames gives an 8.3 name to every node that has none: the nodes made with Add
// and AddDir. Nodes read from a volume keep the short names they had.
func assignShortNames(dir *Node) error {
	used := map[[11]byte]bool{}
	for _, c := range dir.Children {
		if c.Short != ([11]byte{}) {
			used[c.Short] = true
		}
	}
	for _, c := range dir.Children {
		if c.Short == ([11]byte{}) {
			short, exact := makeShort(c.Name)
			if !exact || used[short] {
				found := false
				for i := 1; i < 1000000 && !found; i++ {
					tail := fmt.Sprintf("~%d", i)
					cand := short
					base := strings.TrimRight(string(short[0:8]), " ")
					if len(base) > 8-len(tail) {
						base = base[:8-len(tail)]
					}
					copy(cand[0:8], fmt.Sprintf("%-8s", base+tail))
					if !used[cand] {
						short, found = cand, true
					}
				}
				if !found {
					return fmt.Errorf("no short name left for %s", c.Name)
				}
			}
			c.Short, c.hasLong = short, !exact || c.Name != shortDisplay(short, 0)
			used[short] = true
		}
		if c.IsDir() {
			if err := assignShortNames(c); err != nil {
				return err
			}
		}
	}
	return nil
}

// makeShort makes the 8.3 form of a name. exact is true when nothing had to be dropped or
// changed apart from the case of letters.
func makeShort(name string) (short [11]byte, exact bool) {
	clean := func(s string) (string, bool) {
		ok := true
		var out []byte
		for _, r := range strings.ToUpper(s) {
			switch {
			case r >= 'A' && r <= 'Z', r >= '0' && r <= '9', strings.ContainsRune("_-~!#$%&'()@^`{}", r):
				out = append(out, byte(r))
			case r == ' ' || r == '.':
				ok = false
			default:
				out = append(out, '_')
				ok = false
			}
		}
		return string(out), ok
	}
	base, ext := name, ""
	if i := strings.LastIndex(name, "."); i > 0 {
		base, ext = name[:i], name[i+1:]
	}
	b, okB := clean(base)
	e, okE := clean(ext)
	exact = okB && okE && len(b) >= 1 && len(b) <= 8 && len(e) <= 3
	if len(b) > 8 {
		b = b[:8]
	}
	if len(e) > 3 {
		e = e[:3]
	}
	if b == "" {
		b = "_"
	}
	copy(short[:], fmt.Sprintf("%-8s%-3s", b, e))
	return short, exact
}
