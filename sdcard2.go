package zenzx

import (
	"errors"
	"os"
)

// A second SD card.
//
// The Next has two card slots on one SPI bus. Port 0xE7 selects between them, from the
// Next's ports.txt: bit 0 = 0 selects card 0, bit 1 = 0 selects card 1. NextZXOS starts
// from card 0 and gives the volumes of card 1 drive letters of their own, so a second card
// is a way to bring programs to a Next without touching the card it boots from.
//
// The DivMMC holds both. The data port goes to card 1 while it alone is selected, and to
// card 0 at every other time, which includes the clocks sent with nothing selected when a
// card is started. With no second card, selecting it reads 0xFF: an empty slot.

// WriteCS is a write to port 0xE7.
func (d *DivMMC) WriteCS(value uint8) {
	d.sd.WriteCS(value)
	d.second = value&0x03 == 0x01 // card 1 selected and card 0 not
	if d.sd2 != nil {
		d.sd2.WriteCS(value >> 1)
	}
}

// WriteData and ReadData are port 0xEB.
func (d *DivMMC) WriteData(value uint8) {
	switch {
	case !d.second:
		d.sd.WriteData(value)
	case d.sd2 != nil:
		d.sd2.WriteData(value)
	}
}

func (d *DivMMC) ReadData() uint8 {
	switch {
	case !d.second:
		return d.sd.ReadData()
	case d.sd2 != nil:
		return d.sd2.ReadData()
	}
	return 0xFF
}

// SD2 returns the second card, nil when the slot is empty.
func (d *DivMMC) SD2() *SDCard { return d.sd2 }

// secondCard is a second card's file, as the machine holds it.
type secondCard struct {
	path      string
	file      *os.File
	protected bool
}

// InsertSecondCard puts the card image at path in the second slot, taking out the one that
// was there. The machine must have a DivMMC (a Next that boots NextZXOS has). NextZXOS
// looks at the slots when it starts and when its Browser is told to remount (M, then Y), so
// a card put in while it runs is seen after one of those.
func (zx *ZenZX) InsertSecondCard(path string, readOnly bool) (protected bool, err error) {
	if !zx.io.hasDivMMC || zx.io.divMMC == nil {
		return false, errors.New("this machine has no SD card slots (a ZX Spectrum Next booting NextZXOS has)")
	}
	file, protected, err := openSDCardImage(path, readOnly)
	if err != nil {
		return false, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return false, err
	}
	zx.RemoveSecondCard()
	sd := NewSDCard(file, info.Size())
	sd.wr.WriteProtected = protected
	sd.Debug = zx.io.divMMC.Debug
	zx.io.divMMC.sd2 = sd
	zx.card2 = &secondCard{path: path, file: file, protected: protected}
	return protected, nil
}

// RemoveSecondCard empties the second slot.
func (zx *ZenZX) RemoveSecondCard() {
	if zx.io.divMMC != nil {
		zx.io.divMMC.sd2 = nil
	}
	if zx.card2 != nil {
		zx.card2.file.Close()
		zx.card2 = nil
	}
}

// SecondCard returns the path of the card in the second slot, "" when it is empty.
func (zx *ZenZX) SecondCard() string {
	if zx.card2 == nil {
		return ""
	}
	return zx.card2.path
}
