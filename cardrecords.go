// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// The cards of a snapshot of a Next.
//
// A Next that boots NextZXOS has the card it boots from in the first slot and may have another
// in the second, and the state a snapshot holds is of a machine that was running over them: where
// NextZXOS was on the card, which drive letters it gave the volumes. A snapshot is no use over
// other cards. It does not hold them (they are files of a gigabyte and change as the machine
// runs), so it holds what tells them apart (nextcard.Identity): when a snapshot is made, each
// card in a slot is given a signature if it has none, and the signature, the size and the name
// the file had go in a chunk of the snapshot, ChunkCards, beside the machine's state. Loading
// the snapshot looks for the cards by that (cardsearch.go), and asks the person for them when
// they are not to be found (cardsask_gui.go).
//
// The name is the file's name and not its path: a snapshot is a file that gets sent to other people,
// and the folders of one computer mean nothing on another.

// ChunkCards is the chunk that holds the cards a Next had: "NC".
const ChunkCards = 0x4E43

// The most a list of cards can come to: two records of a few hundred bytes. A snapshot that claims a
// bigger one is not read, and the name of a card is cut to what a line can show.
const (
	maxCardChunk = 64 << 10
	maxCardName  = 80
)

// cardRecord is one card a snapshot was made over.
type cardRecord struct {
	Slot  int    `json:"slot"`  // 1 is the card the Next boots from, 2 the second card
	Sig   uint32 `json:"sig"`   // the disk signature, 0 when the card could not be given one
	Size  int64  `json:"size"`  // the image's size in bytes
	Table string `json:"table"` // the partition table, in hex
	Name  string `json:"name"`  // the file's name when the snapshot was made
}

// recordOfCard makes the record of the card in a slot.
func recordOfCard(slot int, path string, id nextcard.Identity) cardRecord {
	return cardRecord{Slot: slot, Sig: id.Sig, Size: id.Size, Table: hex.EncodeToString(id.Table[:]), Name: filepath.Base(path)}
}

// identity is the identity the record holds.
func (r cardRecord) identity() nextcard.Identity {
	id := nextcard.Identity{Sig: r.Sig, Size: r.Size}
	if b, err := hex.DecodeString(r.Table); err == nil && len(b) == len(id.Table) {
		copy(id.Table[:], b)
	} else {
		id.Size = -1 // a record that does not say its layout is no card's
	}
	return id
}

// slotWords is what a slot is called to the person.
func slotWords(slot int) string {
	if slot == 2 {
		return "second card"
	}
	return "card"
}

// describe is the card as it is said in a message: its file name, in quotes so that it is seen for
// what it is, and its size.
func (r cardRecord) describe() string {
	if r.Name == "" {
		return fmt.Sprintf("(a card of %s, whose name was not kept)", cardSizeWords(r.Size))
	}
	return fmt.Sprintf("\"%s\" (%s)", r.Name, cardSizeWords(r.Size))
}

// cardSizeWords is a card's size in words.
func cardSizeWords(size int64) string {
	if size >= 1<<30 {
		return fmt.Sprintf("%.1f GB", float64(size)/(1<<30))
	}
	return fmt.Sprintf("%d MB", size>>20)
}

// encodeCardRecords is the chunk's data for records.
func encodeCardRecords(recs []cardRecord) []byte {
	b, err := json.Marshal(recs)
	if err != nil {
		panic(err) // plain values
	}
	return b
}

// maxCardSize is the biggest card there is to believe: 16 TB.
const maxCardSize = 1 << 44

// decodeCardRecords reads a chunk's data. A snapshot is a file that comes from anywhere, so what
// it says is looked at before anything is done with it: a Next has two slots, a card has a size and
// a partition table, and a name is text that can be shown (cleanCardName). A list that says
// otherwise is damaged, and is refused.
func decodeCardRecords(data []byte) ([]cardRecord, error) {
	damaged := func(format string, args ...any) error {
		return cardsUnusablef("the snapshot's list of cards is damaged: "+format, args...)
	}
	var recs []cardRecord
	if err := json.Unmarshal(data, &recs); err != nil {
		return nil, damaged("%v", err)
	}
	if len(recs) > 2 {
		return nil, damaged("it lists %d cards, and a Next has two slots", len(recs))
	}
	seen := map[int]bool{}
	for i := range recs {
		r := &recs[i]
		switch {
		case r.Slot != 1 && r.Slot != 2:
			return nil, damaged("a card is in slot %d", r.Slot)
		case seen[r.Slot]:
			return nil, damaged("two cards are in slot %d", r.Slot)
		case r.Size <= 0 || r.Size > maxCardSize:
			return nil, damaged("a card of %d bytes", r.Size)
		}
		if b, err := hex.DecodeString(r.Table); err != nil || len(b) != len(nextcard.Identity{}.Table) {
			return nil, damaged("a card has no partition table")
		}
		seen[r.Slot] = true
		r.Name = cleanCardName(r.Name)
	}
	return recs, nil
}

// cleanCardName makes the name a snapshot gives a card into a line of text that can be shown: it has
// no control characters (a line break would start a line of the dialogue that the snapshot wrote),
// none of the invisible ones that make text read backwards or look like another name, and is not
// longer than maxCardName characters.
func cleanCardName(s string) string {
	var b strings.Builder
	n := 0
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r) {
			continue
		}
		if n++; n > maxCardName {
			out := []rune(strings.TrimSpace(b.String()))
			return string(out[:len(out)-1]) + "…"
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

// mountedCard is a card in a slot of a machine.
type mountedCard struct {
	slot      int
	path      string
	file      *os.File
	protected bool
	served    bool // the machine has read from it: it is mounted, and not only inserted
}

// mountedCards lists the cards in the machine's slots. The first is the one the Next boots from, so
// it counts as served; the second only when the firmware has read from it.
func (zx *ZenZX) mountedCards() []mountedCard {
	var cards []mountedCard
	dm := zx.io.divMMC
	if zx.next != nil && zx.next.card != nil {
		cards = append(cards, mountedCard{1, zx.next.files.Card, zx.next.card, zx.next.protected, true})
	}
	if zx.card2 != nil && zx.card2.file != nil {
		served := dm != nil && dm.sd2 != nil && dm.sd2.served
		cards = append(cards, mountedCard{2, zx.card2.path, zx.card2.file, zx.card2.protected, served})
	}
	return cards
}

// identityOf is the identity of a card in a slot.
func (c mountedCard) identity() (nextcard.Identity, error) {
	st, err := c.file.Stat()
	if err != nil {
		return nextcard.Identity{}, err
	}
	return nextcard.ReadIdentity(c.file, st.Size())
}

// nextCardRecords makes the records of the cards in the machine's slots for a snapshot, giving a
// card that has no signature one first. A card that cannot be written is recorded as it is, and
// says so in notes: a snapshot of it is taken for the card of that size and layout.
func (zx *ZenZX) nextCardRecords() (recs []cardRecord, notes []string) {
	for _, c := range zx.mountedCards() {
		if !c.served {
			continue // inserted and never mounted: nothing in the machine depends on it
		}
		st, err := c.file.Stat()
		if err != nil {
			continue
		}
		id, stamped, err := nextcard.Stamp(c.file, st.Size())
		switch {
		case err != nil && !id.Keyed() && id.Size == 0:
			continue // not a card with a partition table: nothing tells it from another
		case err != nil:
			notes = append(notes, fmt.Sprintf("%s could not be given an identity (%v): the snapshot will take any card of its size and layout", filepath.Base(c.path), cardErrorText(err)))
		case stamped:
			sayf("Card %s given the identity %08X\n", filepath.Base(c.path), id.Sig)
		}
		recs = append(recs, recordOfCard(c.slot, c.path, id))
	}
	return recs, notes
}

// cardChunk is the chunk a snapshot of this Next carries, nil when it has no cards.
func (zx *ZenZX) cardChunk() (data []byte, notes []string) {
	recs, notes := zx.nextCardRecords()
	if len(recs) == 0 {
		return nil, notes
	}
	return encodeCardRecords(recs), notes
}

// readSnapshotCards reads the cards a snapshot at path was made over: none for a snapshot that is
// not of a Next, or is of one that had no cards.
func readSnapshotCards(path, format string) ([]cardRecord, error) {
	if format == "" {
		format = DetectSnapshotFormat(path)
	}
	if format != "zxs" {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readCardRecords(f)
}

// readCardRecords reads the cards a snapshot was made over from the snapshot itself, which r reads.
func readCardRecords(f io.ReadSeeker) ([]cardRecord, error) {
	var h SnapshotHeader
	if err := binary.Read(f, binary.LittleEndian, &h); err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}
	if string(h.Magic[:]) != SnapshotMagic {
		return nil, fmt.Errorf("invalid snapshot magic: %s", h.Magic)
	}
	if h.Flags&FlagNext == 0 {
		return nil, nil
	}
	for {
		var ch ChunkHeader
		if err := binary.Read(f, binary.LittleEndian, &ch); err != nil {
			if err == io.EOF {
				return nil, nil
			}
			return nil, fmt.Errorf("reading chunk header: %w", err)
		}
		if ch.ID == ChunkEnd {
			return nil, nil
		}
		if ch.ID != ChunkCards {
			if _, err := f.Seek(int64(ch.Size), io.SeekCurrent); err != nil {
				return nil, err
			}
			continue
		}
		if ch.Size > maxCardChunk {
			// a snapshot is not to say how much room is to be made for it before it has shown it
			return nil, cardsUnusablef("the snapshot's list of cards is damaged: it claims %d bytes", ch.Size)
		}
		data := make([]byte, ch.Size)
		if _, err := io.ReadFull(f, data); err != nil {
			return nil, fmt.Errorf("reading the cards: %w", err)
		}
		return decodeCardRecords(data)
	}
}

// cardsUnusable is the refusal of a snapshot for what is wrong with its list of cards, or with the
// files they are to be loaded over when they are looked at again: said as it is.
type cardsUnusable struct{ msg string }

func (e *cardsUnusable) Error() string { return e.msg }

func cardsUnusablef(format string, args ...any) error {
	return &cardsUnusable{fmt.Sprintf(format, args...)}
}

// cardMismatch is the refusal of a snapshot over cards that are not the ones it was made over.
type cardMismatch struct {
	Want cardRecord
	Have string // what is in the slot: "" for nothing
}

func (e *cardMismatch) Error() string {
	if e.Have == "" {
		return fmt.Sprintf("the snapshot was made with %s in the %s slot, and there is no card in it", e.Want.describe(), slotWords(e.Want.Slot))
	}
	return fmt.Sprintf("the snapshot was made with %s in the %s slot, and %s is not that card", e.Want.describe(), slotWords(e.Want.Slot), e.Have)
}

// checkSnapshotCards is the loader's last look at the cards before a Next's state goes over the
// machine: the cards in the slots have to be the ones the snapshot was made over. The window
// makes sure of that before it loads (cardsearch.go) and says that the cards are approved when
// the person chose to go on with others, or with none in the second slot (cardsApproved); this
// is what stops the other ways of loading one, a script's snapshot verb or the command line,
// over cards that are not its own. A slot the snapshot had no card in is not looked at.
func (zx *ZenZX) checkSnapshotCards(data []byte) error {
	if zx.cardsApproved {
		return nil
	}
	recs, err := decodeCardRecords(data)
	if err != nil {
		return err
	}
	have := map[int]mountedCard{}
	for _, c := range zx.mountedCards() {
		have[c.slot] = c
	}
	for _, rec := range recs {
		c, ok := have[rec.Slot]
		if !ok {
			return &cardMismatch{Want: rec}
		}
		id, err := c.identity()
		if err != nil {
			return &cardMismatch{Want: rec, Have: filepath.Base(c.path)}
		}
		if rec.identity().Compare(id) == nextcard.Different {
			return &cardMismatch{Want: rec, Have: filepath.Base(c.path)}
		}
	}
	return nil
}

// errCardsAsked is what opening a snapshot returns when the window has put a question about the
// cards to the person: nothing is wrong, the snapshot is loaded when it is answered.
var errCardsAsked = errors.New("the cards of the snapshot are being asked for")

// cardErrorText is an error without the path that the message already names.
func cardErrorText(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	return err.Error()
}
