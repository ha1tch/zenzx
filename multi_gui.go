//go:build !headless

package zenzx

// startMachines adds the machines that -multi asked for to a deck that has
// only the primary one, and returns them all in the order they were listed.
// models is that list and primary the place in it of the machine the deck
// already holds.
//
// The deck itself is left with the primary machine first and in front,
// because the program's start-up goes on to finish setting that machine up
// through the display it holds. settleMachines puts the deck in the listed
// order once that is done.
func (d *machineDeck) startMachines(models []string, primary int) []*ZenZX {
	for i, m := range models {
		if i != primary {
			d.add(m)
		}
	}
	all := make([]*ZenZX, 0, len(d.machines))
	for i := range models {
		switch {
		case i == primary:
			all = append(all, d.machines[0].zx)
		case i < primary:
			all = append(all, d.machines[i+1].zx)
		default:
			all = append(all, d.machines[i].zx)
		}
	}
	return all
}

// settleMachines moves the primary machine from the first place in the deck
// to its place in the list, and brings machine 1 to the front the way
// choosing it in the dock does.
func (d *machineDeck) settleMachines(primary int) {
	if primary <= 0 || primary >= len(d.machines) {
		return
	}
	p := d.machines[0]
	copy(d.machines, d.machines[1:primary+1])
	d.machines[primary] = p
	d.front, d.sel = primary, primary
	d.flip(0)
	d.sel = 0
}
