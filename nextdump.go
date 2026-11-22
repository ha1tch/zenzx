package zenzx

import (
	"fmt"
	"io"
)

// DumpNextState writes the state of the Next's hardware that decides what is mapped and
// what is drawn: the registers that are not zero, the memory mapping, and the start of the
// palettes. It is the headless build's -next-dump, for working out why a program does not
// display.
func (zx *ZenZX) DumpNextState(w io.Writer) {
	m, sio := zx.memory, zx.io
	if !m.isNext {
		fmt.Fprintln(w, "not a Next")
		return
	}
	fmt.Fprintf(w, "PC=%04X SP=%04X IM=%d IFF1=%v\n", zx.cpu.PC, zx.cpu.SP, zx.cpu.IM, zx.cpu.IFF1)
	fmt.Fprintf(w, "MMU slots %v  ROM %d (ports: 7FFD=%02X 1FFD=%02X DFFD=%02X)  screen bank %d  config mode %v  boot ROM %v\n",
		m.nextSlots, m.nextROMIndex(), m.port7FFD, m.port1FFD, m.nextLegacyPortDFFD, m.screenBank, m.nextCfg.active, m.bootROMEnabled)
	fmt.Fprintf(w, "port 123B=%02X  Layer 2 on %v  tilemap on %v (clip %v)\n",
		sio.layer2Access.raw, sio.layer2Enabled(), sio.tilemapEnabled(), sio.tilemap.clip.values)
	if sio.divMMC != nil {
		d := sio.divMMC
		fmt.Fprintf(w, "DivMMC control=%02X mapped %v automap %v entry points %02X %02X %02X %02X\n",
			d.ctrl, d.active(), d.automapEnabled, d.entry0, d.valid0, d.timing0, d.entry1)
	}
	fmt.Fprint(w, "NextREG (not zero):")
	n := 0
	for r := 0; r < 256; r++ {
		if v := sio.GetNextRegister(uint8(r)); v != 0 {
			if n%12 == 0 {
				fmt.Fprint(w, "\n ")
			}
			fmt.Fprintf(w, " %02X=%02X", r, v)
			n++
		}
	}
	fmt.Fprintln(w)
	for _, p := range []struct {
		name   string
		target nextPaletteTarget
		count  int
	}{
		{"ULA 1", nextPaletteULA1st, 32}, {"ULA 2", nextPaletteULA2nd, 32},
		{"tilemap 1", nextPaletteTilemap1st, 32}, {"tilemap 2", nextPaletteTilemap2nd, 32},
		{"Layer 2 1", nextPaletteLayer2_1st, 16}, {"sprites 1", nextPaletteSprites1st, 16},
	} {
		fmt.Fprintf(w, "palette %-9s", p.name)
		for i := 0; i < p.count; i++ {
			c := sio.nextPalette.palettes[p.target][i] // nine bits, RRRGGGBBB, as three hex digits
			fmt.Fprintf(w, " %03X", uint16(c.R)<<6|uint16(c.G)<<3|uint16(c.B))
		}
		fmt.Fprintln(w)
	}
}
