//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenui/zenuiraylib"
)

// ============================================================================
// Asking for the cards of a snapshot
//
// A snapshot of a Next that boots NextZXOS was made over the cards the Next had (cardrecords.go),
// and is loaded over them: the window looks for them by their identity (cardsearch.go) and mounts
// them, and says nothing when it finds them all. When one is not to be found (the file was moved
// or deleted, or another machine has it) this dialogue says which, and the person chooses the
// file. The file is checked against the card the snapshot was made over: a file that is not it is
// said not to be, and may be gone on with all the same, by pressing Load a second time after the
// warning, since a card that has been made again or lost its identity is still the one the person
// means. A second card that is not found may be left out: the snapshot is then loaded with the
// second slot empty. The first card is needed.
//
// One dialogue serves a snapshot and a multisnapshot, which has the cards of every Next in it.
// ============================================================================

// cardSlots is the cards a Next is given for a snapshot: the files, and whether the loader is to
// take them though they are not the ones the snapshot was made over, or the second slot empty
// (Zenzx.cardsApproved).
type cardSlots struct {
	one, two string
	approved bool
}

// cardPlan is the cards of each machine of a multisnapshot, by the machine's place in it.
type cardPlan map[int]cardSlots

// cardNeed is the cards of one machine of a snapshot, and how far they are found.
type cardNeed struct {
	machine int    // its place in a multisnapshot; 0 for the machine of a snapshot
	label   string // what it is called to the person: "" for a snapshot of one machine
	recs    []cardRecord
	found   []cardFound
}

// slotsOf is the cards found for a machine: what the search settled on, and whether any of it is
// to be gone on with without being sure (a card that is not the one, or left out).
func slotsOf(found []cardFound, confirmed bool) (cardSlots, bool) {
	var s cardSlots
	for _, f := range found {
		use := f.Path != "" && (f.acceptable() || confirmed)
		if !use {
			if f.Rec.Slot == 1 {
				return s, false // the first card is needed
			}
			s.approved = true // the second slot is left empty
			continue
		}
		if !f.acceptable() || f.Layout {
			s.approved = true // not the card by its signature: the loader is to take it all the same
		}
		if f.Rec.Slot == 1 {
			s.one = f.Path
		} else {
			s.two = f.Path
		}
	}
	return s, true
}

// allFound is whether every card of every need is settled.
func allFound(needs []cardNeed) bool {
	for _, n := range needs {
		for _, f := range n.found {
			if !f.acceptable() {
				return false
			}
		}
	}
	return true
}

// cardRow is one card image the dialogue lists for the card that is to be chosen.
type cardRow struct {
	path   string
	size   int64
	tag    string // what is said of it beside its name
	usable bool   // it can be chosen: it is not in use by a machine, nor chosen for another card, and is a card
	same   bool   // it is the card, by its signature
	layout bool   // it has the card's size and layout and another signature: a copy, which is taken for it
}

// cardAskColumns is how many characters a line of what the dialogue says may have, and cardAskRows how
// many cards it shows at once.
const (
	cardAskColumns = 66
	cardAskRows    = 6
)

// cardsAsk is the dialogue: what is needed, a list of the card images there are to choose one from
// (those in use by a machine are there and cannot be chosen), and the buttons.
type cardsAsk struct {
	title   string // what it is about: the snapshot's name
	multi   bool   // a multisnapshot
	needs   []cardNeed
	search  *cardSearch
	confirm bool   // Load was pressed on cards that are not the ones, and has said so
	why     string // what the last answer was refused for
	buttons []string
	done    func(plan cardPlan) // the cards are settled: load
	cancel  func()
	said    []string // the lines it says, for a test to read
	whyAt   []int
	lastDir string   // the folder of the last file chosen, where the file browser opens next
	extra   []string // the files chosen with Browse..., which are listed with the card images of the folders
	rows    []cardRow
	list    *zenui.List
	bar     *zenui.ButtonBar

	panel, listArea, barArea zenui.Rect
	scale, lineH             int
	theme                    zenui.Theme // as last drawn with, for the cards' pictures
}

// newCardsAsk makes the dialogue for needs, which the search that found them is kept to look
// again with what the person chooses.
func newCardsAsk(title string, multi bool, needs []cardNeed, search *cardSearch, done func(cardPlan), cancel func()) *cardsAsk {
	a := &cardsAsk{title: title, multi: multi, needs: needs, search: search, done: done, cancel: cancel,
		list: zenui.NewList(), bar: zenui.NewButtonBar()}
	a.list.NoToolbar = true
	a.build()
	return a
}

// refind looks again, with the files the person has chosen.
func (a *cardsAsk) refind() {
	a.search.used = nil
	for i := range a.needs {
		a.needs[i].found = a.search.find(a.needs[i].machine, a.needs[i].recs)
	}
}

// target is the card the person is to choose a file for next: the first that has no file, and when every
// card has one, the first that is not settled (a file that is not the card is chosen over again).
func (a *cardsAsk) target() (machine, slot int, ok bool) {
	for _, n := range a.needs {
		for _, f := range n.found {
			if f.Path == "" {
				return n.machine, f.Rec.Slot, true
			}
		}
	}
	for _, n := range a.needs {
		for _, f := range n.found {
			if !f.acceptable() {
				return n.machine, f.Rec.Slot, true
			}
		}
	}
	return 0, 0, false
}

// cardLine says how far a card is found: which card it is (its slot, the name it had and its size)
// and where it stands, in a few plain words.
func (n cardNeed) cardLine(f cardFound) string {
	who := ""
	if n.label != "" {
		who = n.label + ": "
	}
	line := fmt.Sprintf("%sCard %d: %s", who, f.Rec.Slot, f.Rec.describe())
	switch {
	case f.Layout:
		return line + ", you chose " + filepath.Base(f.Path) + ", which has the same size and layout."
	case f.acceptable() && f.Chosen:
		return line + ", you chose " + filepath.Base(f.Path) + "."
	case f.acceptable():
		return line + ", found as " + filepath.Base(f.Path) + "."
	case f.Path != "" && f.Chosen:
		return line + ", you chose " + filepath.Base(f.Path) + ", which is not that card."
	case len(f.Several) > 0:
		return line + ", could be any of " + joinNames(f.Several) + "."
	case f.Held != 0:
		return line + fmt.Sprintf(", in use by machine %d.", f.Held)
	case f.Said != "":
		return line + ", " + f.Said + "."
	}
	return line + ", not found."
}

// needsChoice is whether some card has no file at all, so that a file is to be chosen for it from the
// list. When every card has a file, whether or not it is the card, there is nothing to choose.
func (a *cardsAsk) needsChoice() bool {
	for _, n := range a.needs {
		for _, f := range n.found {
			if f.Path == "" {
				return true
			}
		}
	}
	return false
}

// targetCard is the need and the card of it that target says.
func (a *cardsAsk) targetCard() (cardNeed, cardFound, bool) {
	machine, slot, ok := a.target()
	if !ok {
		return cardNeed{}, cardFound{}, false
	}
	for _, n := range a.needs {
		if n.machine != machine {
			continue
		}
		for _, f := range n.found {
			if f.Rec.Slot == slot {
				return n, f, true
			}
		}
	}
	return cardNeed{}, cardFound{}, false
}

// needText says which card a need is of: "the card "next-card.img" (1.0 GB)", and for whom when
// there are several machines.
func (n cardNeed) needText(f cardFound) string {
	text := "the " + slotWords(f.Rec.Slot) + " " + f.Rec.describe()
	if n.label != "" {
		text += " for " + n.label
	}
	return text
}

// reason says why the card is to be chosen, and that it is: the card is in use by a machine, or
// could be one of several, or the file that was chosen cannot be used, or it is not to be found.
func (f cardFound) reason() string {
	var s string
	switch {
	case f.Held != 0 && f.Chosen:
		s = fmt.Sprintf(". You chose %s, which is in use by machine %d. Choose another card below.", f.HeldName, f.Held)
	case f.Held != 0:
		s = fmt.Sprintf(", which is in use by machine %d. Choose another card below.", f.Held)
	case len(f.Several) > 0:
		s = ". More than one card could be it: " + joinNames(f.Several) + ". Choose the one you want below."
	case f.Said != "":
		s = ". " + f.Said + ". Choose another card below."
	default:
		s = ", which was not found. Choose a card in its place below."
	}
	if f.Rec.Slot == 2 {
		s += " Or press Load to leave it out: the second slot is then left empty."
	}
	return s
}

// head says what the person is to know and do: which card is wanted and why a file is to be chosen
// for it, or which file was chosen that is not the card, or that the cards are settled.
func (a *cardsAsk) head() (lines []string, tail string) {
	what := "The snapshot " + a.title
	if a.multi {
		what = "The multisnapshot " + a.title
	}
	tn, tf, hasTarget := a.targetCard()
	switch {
	case hasTarget && tf.Path == "":
		lines = append(lines, what+" needs "+tn.needText(tf)+tf.reason())
	case a.chosenButNot():
		for _, n := range a.needs {
			for _, f := range n.found {
				if f.Path != "" && !f.acceptable() {
					who := ""
					if n.label != "" {
						who = n.label + ": "
					}
					lines = append(lines, fmt.Sprintf("%sYou chose %s for %s, but %s.", who, filepath.Base(f.Path), n.needText(f), f.Said))
				}
			}
		}
		tail = "Load goes on with the file you chose. Choose another lets you pick a different card."
	default:
		lines = append(lines, what+" will be loaded with these cards:")
		for _, n := range a.needs {
			for _, f := range n.found {
				lines = append(lines, n.cardLine(f))
			}
		}
		tail = "Press Load."
	}
	total := 0
	for _, n := range a.needs {
		total += len(n.found)
	}
	if total > 1 && hasTarget && tf.Path == "" {
		for _, n := range a.needs {
			for _, f := range n.found {
				if n.machine == tn.machine && f.Rec.Slot == tf.Rec.Slot {
					continue
				}
				lines = append(lines, n.cardLine(f))
			}
		}
	}
	return lines, tail
}

// build makes the dialogue as it stands: what it says, the list of cards, and the buttons.
func (a *cardsAsk) build() {
	a.said, a.whyAt = a.said[:0], a.whyAt[:0]
	say := func(s string) {
		a.said = append(a.said, zenui.WrapText(s, cardAskColumns, "")...)
	}
	lines, tail := a.head()
	for _, l := range lines {
		say(l)
	}
	if a.why != "" {
		first := len(a.said)
		say(a.why)
		for i := first; i < len(a.said); i++ {
			a.whyAt = append(a.whyAt, i)
		}
	}
	if tail != "" {
		say(tail)
	}
	a.buildRows()
	a.setButtons()
}

// heading is what the dialogue is headed.
func (a *cardsAsk) heading() string {
	if a.multi {
		return "THE CARDS OF THE MULTISNAPSHOT"
	}
	return "THE CARDS OF THE SNAPSHOT"
}

// listed is whether the list of cards is shown: while there is a card to choose a file for.
func (a *cardsAsk) listed() bool { return a.needsChoice() }

// takenBy is what is said of a file that is the card of another of the cards that are settled.
func (a *cardsAsk) takenBy() map[string]string {
	taken := map[string]string{}
	for _, n := range a.needs {
		for _, f := range n.found {
			if f.Path == "" {
				continue
			}
			who := fmt.Sprintf("CARD %d", f.Rec.Slot)
			if n.label != "" {
				who = strings.ToUpper(strings.SplitN(n.label, " (", 2)[0]) + " " + who
			}
			taken[canonical(f.Path)] = "TAKEN BY " + who
		}
	}
	return taken
}

// buildRows lists the card images there are to choose from for the card that is to be chosen: those of
// the folders looked through and the files that were chosen with Browse.... A card that is in use by a
// machine, or is another card's, or is no card, is listed and cannot be chosen; the ones that are the
// card come first, and then those that are another card, which may be gone on with.
func (a *cardsAsk) buildRows() {
	keep := ""
	if a.list.Selected >= 0 && a.list.Selected < len(a.rows) {
		keep = canonical(a.rows[a.list.Selected].path)
	}
	a.rows = a.rows[:0]
	a.list.Items, a.list.Selected = a.list.Items[:0], -1
	_, tf, ok := a.targetCard()
	if !ok || tf.Path != "" {
		return
	}
	taken := a.takenBy()
	paths := a.search.candidates()
	seen := map[string]bool{}
	for _, p := range paths {
		seen[canonical(p)] = true
	}
	for _, p := range a.extra {
		key := canonical(p)
		if st, err := os.Stat(p); seen[key] || err != nil || !st.Mode().IsRegular() {
			continue
		}
		seen[key] = true
		paths = append(paths, p)
	}
	for _, p := range paths {
		row := cardRow{path: p}
		if st, err := os.Stat(p); err == nil {
			row.size = st.Size()
		}
		held := 0
		if a.search.held != nil {
			held = a.search.held(p)
		}
		switch {
		case held != 0:
			row.tag = fmt.Sprintf("IN USE, machine %d", held)
		case taken[canonical(p)] != "":
			row.tag = taken[canonical(p)]
		default:
			id, err := a.search.identityOf(p)
			switch {
			case err != nil:
				row.tag = "NOT A CARD IMAGE"
			case (cardFound{Rec: tf.Rec, Path: p, Match: tf.Rec.identity().Compare(id)}).acceptable():
				row.usable, row.same, row.tag = true, true, "the same card"
			case id.Size == tf.Rec.identity().Size && id.Table == tf.Rec.identity().Table:
				row.usable, row.layout, row.tag = true, true, "same layout"
			default:
				row.usable, row.tag = true, "very different"
			}
		}
		a.rows = append(a.rows, row)
	}
	rank := func(r cardRow) int {
		switch {
		case r.same:
			return 0
		case r.layout:
			return 1
		case r.usable:
			return 2
		}
		return 3
	}
	sort.SliceStable(a.rows, func(i, j int) bool { return rank(a.rows[i]) < rank(a.rows[j]) })
	for _, r := range a.rows {
		usable := r.usable
		a.list.Items = append(a.list.Items, zenui.ListItem{Name: a.rowText(r), Thumb: a.cardPicture(usable)})
	}
	for i, r := range a.rows {
		if r.usable && (keep == "" || canonical(r.path) == keep) {
			a.list.Selected = i
			break
		}
	}
	if a.list.Selected < 0 {
		for i, r := range a.rows {
			if r.usable {
				a.list.Selected = i
				break
			}
		}
	}
}

// rowText is a card's line in the list: its name, its size, and what is to be said of it.
func (a *cardsAsk) rowText(r cardRow) string {
	name := filepath.Base(r.path)
	if len(name) > 22 {
		name = name[:21] + "~"
	}
	return fmt.Sprintf("%-22s %5d MB  %s", name, r.size>>20, r.tag)
}

// cardPicture draws a small SD card beside a row, as the SD Cards window does: filled for a card that can
// be chosen, in outline for one that cannot.
func (a *cardsAsk) cardPicture(usable bool) func(r zenui.Renderer, area zenui.Rect) {
	return func(r zenui.Renderer, area zenui.Rect) {
		h := area.H - 4
		if h < 6 {
			return
		}
		cw := h * 3 / 4
		c := zenui.Rect{X: area.X + (area.W-cw)/2, Y: area.Y + 2, W: cw, H: h}
		notch := cw / 3
		body := zenui.Rect{X: c.X, Y: c.Y + notch, W: c.W, H: c.H - notch}
		top := zenui.Rect{X: c.X, Y: c.Y, W: c.W - notch, H: notch}
		if usable {
			r.FillRect(body, a.theme.Text)
			r.FillRect(top, a.theme.Text)
			return
		}
		r.StrokeRect(body, a.theme.DimText, 1)
		r.StrokeRect(top, a.theme.DimText, 1)
	}
}

// selectedRow is the place of the card chosen in the list when it can be chosen, and -1 when it is not.
func (a *cardsAsk) selectedRow() int {
	if i := a.list.Selected; a.listed() && i >= 0 && i < len(a.rows) && a.rows[i].usable {
		return i
	}
	return -1
}

// step moves the choice in the list by key, over the cards that cannot be chosen.
func (a *cardsAsk) step(k zenui.Key) {
	n := len(a.rows)
	if n == 0 {
		return
	}
	next := func(from, d int) int {
		for i := from + d; i >= 0 && i < n; i += d {
			if a.rows[i].usable {
				return i
			}
		}
		return -1
	}
	to := -1
	switch k {
	case zenui.KeyUp:
		to = next(a.list.Selected, -1)
	case zenui.KeyDown:
		to = next(a.list.Selected, 1)
	case zenui.KeyHome, zenui.KeyPageUp:
		to = next(-1, 1)
	case zenui.KeyEnd, zenui.KeyPageDown:
		to = next(n, -1)
	}
	if to >= 0 {
		a.list.Selected = to
	}
}

// canLoad is whether Load could go ahead: every card has a file, or is a second card that may be left out.
func (a *cardsAsk) canLoad() bool {
	for _, n := range a.needs {
		if _, ok := slotsOf(n.found, true); !ok {
			return false
		}
	}
	return true
}

// setButtons makes the buttons for the present state. Choose is there while a card is to be chosen from
// the list, and can be pressed when one that can be chosen is selected; Choose another is there when the
// file chosen is not the card, to pick a different one; Load can be pressed when there is a card for
// every slot that is needed.
func (a *cardsAsk) setButtons() {
	var bs []zenui.BarButton
	add := func(label string, disabled bool) {
		bs = append(bs, zenui.BarButton{ID: label, Label: label, Disabled: disabled})
	}
	switch {
	case a.listed():
		add("Cancel", false)
		add("Browse...", false)
		add("Choose", a.selectedRow() < 0)
		add("Load", !a.canLoad())
	case a.chosenButNot():
		add("Cancel", false)
		add("Choose another", false)
		add("Load", false)
	default:
		add("Cancel", false)
		add("Load", false)
	}
	a.bar.Buttons = bs
	a.buttons = a.buttons[:0]
	for _, b := range bs {
		a.buttons = append(a.buttons, b.Label)
	}
}

// enabled is whether the button called name is there and can be pressed.
func (a *cardsAsk) enabled(name string) bool {
	for _, b := range a.bar.Buttons {
		if b.Label == name {
			return !b.Disabled
		}
	}
	return false
}

// layout places the parts of the dialogue as the SD Cards window does its own.
func (a *cardsAsk) layout(r zenui.Renderer, screenW, screenH int) {
	a.scale = zenui.TextBodyScale(r, screenW)
	a.lineH = r.LineHeight(a.scale)
	if a.lineH < 1 {
		a.lineH = 1
	}
	pad := r.LineHeight(1)
	pw := zenui.TextPanelWidth(r, a.scale)
	if pw > screenW-2*pad {
		pw = screenW - 2*pad
	}
	inner := pw - 2*pad
	textW := func(s string) int { return r.TextWidth(s, a.scale) }
	_, barH := a.bar.Layout(zenui.Rect{W: inner}, a.lineH, textW)
	titleH := r.LineHeight(2) + 8
	listH, gap := 0, 0
	if a.listed() {
		rowsH := a.lineH + 4
		if len(a.list.Items) > 0 {
			rowsH = a.list.ItemRect(0, zenui.Rect{W: inner, H: 1 << 14}, a.lineH).H
		}
		listH, gap = cardAskRows*rowsH+2, pad
	}
	fixed := 2*pad + titleH + len(a.said)*a.lineH + pad + gap + barH
	if over := fixed + listH - (screenH - 2*pad); over > 0 && listH > 0 {
		listH -= over
		if listH < 2*(a.lineH+4) {
			listH = 2 * (a.lineH + 4)
		}
	}
	ph := fixed + listH
	px, py := (screenW-pw)/2, (screenH-ph)/2
	if py < 0 {
		py = 0
	}
	a.panel = zenui.Rect{X: px, Y: py, W: pw, H: ph}
	y := py + pad + titleH + len(a.said)*a.lineH + pad
	a.listArea = zenui.Rect{X: px + pad, Y: y, W: inner, H: listH}
	y += listH + gap
	a.barArea = zenui.Rect{X: px + pad, Y: y, W: inner, H: barH}
}

// frame runs the dialogue for a frame: the list, the buttons, and the keys. It returns what the frame
// came to, as zenui.Modal does: Cancelled for Escape, Accepted with the place of the button that was
// pressed in buttons, and whether Enter was pressed.
func (a *cardsAsk) frame(in zenui.Input, r zenui.Renderer, screenW, screenH int) (zenui.Status, int, bool) {
	a.setButtons()
	a.layout(r, screenW, screenH)
	enter := false
	for _, k := range in.Keys {
		switch k {
		case zenui.KeyEscape:
			return zenui.Cancelled, 0, false
		case zenui.KeyEnter:
			enter = true
		case zenui.KeyUp, zenui.KeyDown, zenui.KeyHome, zenui.KeyEnd, zenui.KeyPageUp, zenui.KeyPageDown:
			if a.listed() {
				a.step(k)
				a.list.Reveal(a.listArea, a.lineH)
			}
		}
	}
	if a.listed() {
		before := a.list.Selected
		a.list.Update(in, a.listArea, a.lineH)
		if i := a.list.Selected; i != before && (i < 0 || i >= len(a.rows) || !a.rows[i].usable) {
			a.list.Selected = before // a card that cannot be chosen is not chosen by being clicked on
		}
		a.setButtons()
	}
	textW := func(s string) int { return r.TextWidth(s, a.scale) }
	if id, ok := a.bar.Update(in, a.barArea, a.lineH, textW); ok && a.enabled(id) {
		for i, b := range a.buttons {
			if b == id {
				return zenui.Accepted, i, enter
			}
		}
	}
	return zenui.Active, 0, enter
}

// draw draws the dialogue, with what an answer was refused for in the accent colour as the questions of
// the SD Cards window do (cardQuestion.draw).
func (a *cardsAsk) draw(r zenui.Renderer, screenW, screenH int, theme zenui.Theme) {
	a.theme = theme
	a.setButtons()
	a.layout(r, screenW, screenH)
	r.FillRect(zenui.Rect{W: screenW, H: screenH}, zenui.ScaleAlpha(theme.Backdrop, zenui.ModalBackdropDarkening))
	r.FillRect(a.panel, theme.Panel)
	r.StrokeRect(a.panel, theme.Border, theme.BorderThickness)
	pad := r.LineHeight(1)
	x, y := a.panel.X+pad, a.panel.Y+pad
	r.DrawText(a.heading(), x, y, 2, theme.Text)
	y += r.LineHeight(2) + 8
	why := map[int]bool{}
	for _, i := range a.whyAt {
		why[i] = true
	}
	for i, line := range a.said {
		col := theme.Text
		if why[i] {
			col = theme.DirText
		}
		r.DrawText(line, x, y, a.scale, col)
		y += a.lineH
	}
	if a.listed() {
		r.StrokeRect(zenui.Rect{X: a.listArea.X - 1, Y: a.listArea.Y - 1, W: a.listArea.W + 2, H: a.listArea.H + 2}, theme.Border, 1)
		if len(a.rows) == 0 {
			r.DrawText("No card images in the folders looked through: use Browse...", a.listArea.X+4, a.listArea.Y+4, a.scale, theme.DimText)
		}
		a.list.Draw(r, theme, a.listArea, a.scale)
	}
	a.bar.Draw(r, theme, a.barArea, a.scale)
}

// choose takes the file the person chose for the card that is to be chosen for, and looks again.
func (a *cardsAsk) choose(path string) {
	machine, slot, ok := a.target()
	if !ok {
		return
	}
	if a.search.chosen == nil {
		a.search.chosen = map[string]string{}
	}
	a.search.chosen[needKey(machine, slot)] = path
	a.lastDir = filepath.Dir(path)
	if st, err := os.Stat(path); err == nil && st.Mode().IsRegular() {
		known := false
		for _, e := range a.extra {
			known = known || canonical(e) == canonical(path)
		}
		if !known {
			a.extra = append(a.extra, path)
		}
	}
	a.confirm, a.why = false, ""
	a.refind()
	a.build()
}

// load takes Load: it returns the cards of every machine when they are settled, and otherwise says
// what is not, and returns nothing. A file that is not the card it was chosen for is said so once,
// and is gone on with when Load is pressed again.
func (a *cardsAsk) load() (cardPlan, bool) {
	plan := cardPlan{}
	for _, n := range a.needs {
		s, ok := slotsOf(n.found, true)
		if !ok {
			a.why, a.confirm = "A card is needed to go on: choose its file.", false
			if n.label != "" {
				a.why = n.label + ": " + a.why
			}
			a.build()
			return nil, false
		}
		plan[n.machine] = s
	}
	if a.chosenButNot() && !a.confirm {
		a.confirm = true
		a.why = a.warning()
		a.build()
		return nil, false
	}
	return plan, true
}

// chooseAnother lets the person choose again for the card that has a file chosen for it that is not the
// card: the file is let go, and the card is looked for again.
func (a *cardsAsk) chooseAnother() {
	for _, n := range a.needs {
		for _, f := range n.found {
			if f.Path != "" && !f.acceptable() {
				delete(a.search.chosen, needKey(n.machine, f.Rec.Slot))
				a.confirm, a.why = false, ""
				a.refind()
				a.build()
				return
			}
		}
	}
}

// chosenButNot is whether some card has a file chosen for it that is not the card.
func (a *cardsAsk) chosenButNot() bool {
	for _, n := range a.needs {
		for _, f := range n.found {
			if f.Path != "" && !f.acceptable() {
				return true
			}
		}
	}
	return false
}

// ---- in the bar ----

// askForCards puts the dialogue to the person.
func (b *appMenuBar) askForCards(a *cardsAsk) {
	b.closeMenus()
	b.cardsAsk = a
}

// openChooseCardDialog opens the file browser for the card the dialogue wants next.
func (b *appMenuBar) openChooseCardDialog() {
	a := b.cardsAsk
	if a == nil {
		return
	}
	machine, slot, ok := a.target()
	if !ok {
		return
	}
	name := ""
	for _, n := range a.needs {
		if n.machine != machine {
			continue
		}
		for _, f := range n.found {
			if f.Rec.Slot == slot {
				name = f.Rec.Name
			}
		}
	}
	b.diskDialog = zenui.NewDialog(zenui.DialogConfig{
		Mode:     zenui.ModeOpen,
		Title:    fmt.Sprintf("Choose a copy of the card %s", name),
		StartDir: a.browseDir(machine, slot),
		Filters:  []string{"img"},
		FS:       zenui.OSFS{},
	})
	b.dialogKind = dialogChooseCard
}

// browseDir is the folder the file browser opens in for the card of machine in slot: where the
// last card was chosen from; else where the cards are, which are the folder of the card that is in
// use by another machine (it is the card's own folder, with its copies in it), or of any such card;
// else the first of the folders the search looked in that is there.
func (a *cardsAsk) browseDir(machine, slot int) string {
	isDir := func(p string) bool {
		st, err := os.Stat(p)
		return err == nil && st.IsDir()
	}
	if a.lastDir != "" && isDir(a.lastDir) {
		return a.lastDir
	}
	var any string
	for _, n := range a.needs {
		for _, f := range n.found {
			if f.HeldPath == "" || !isDir(filepath.Dir(f.HeldPath)) {
				continue
			}
			if n.machine == machine && f.Rec.Slot == slot {
				return filepath.Dir(f.HeldPath)
			}
			if any == "" {
				any = filepath.Dir(f.HeldPath)
			}
		}
	}
	if any != "" {
		return any
	}
	for _, dir := range a.search.dirs {
		if dir != "" && isDir(dir) {
			return dir
		}
	}
	return "."
}

// updateCardsAsk runs the dialogue for a frame. Enter chooses the card selected in the list while
// there is a card to choose, and loads when there is not; Escape and Cancel drop it.
func (b *appMenuBar) updateCardsAsk(zx *ZenZX) {
	status, result, enter := b.cardsAsk.frame(zenuiraylib.Input(), b.renderer, rl.GetScreenWidth(), rl.GetScreenHeight())
	b.answerCards(zx, status, result, enter)
}

// answerCards takes what a frame of the dialogue came to.
func (b *appMenuBar) answerCards(zx *ZenZX, status zenui.Status, result int, enter bool) {
	a := b.cardsAsk
	if a == nil {
		return // the dialogue has gone: a second answer to it is nothing
	}
	switch {
	case status == zenui.Cancelled:
		b.cardsAsk = nil
		if a.cancel != nil {
			a.cancel()
		}
		zx.notice("Not loaded: %s", a.missingText())
	case status == zenui.Accepted || enter:
		// a button of the dialogue, or Enter; an answer that is neither is no answer (it is not Load)
		button := ""
		switch {
		case status == zenui.Accepted && result >= 0 && result < len(a.buttons):
			button = a.buttons[result]
		case enter:
			button = "Load"
			if a.listed() {
				button = "Choose"
			}
		}
		switch button {
		case "Cancel":
			b.answerCards(zx, zenui.Cancelled, 0, false)
		case "Browse...":
			b.openChooseCardDialog()
		case "Choose":
			if i := a.selectedRow(); i >= 0 {
				a.choose(a.rows[i].path)
			}
		case "Choose another":
			a.chooseAnother()
		case "Load":
			if plan, ok := a.load(); ok {
				b.cardsAsk = nil
				if a.done != nil {
					a.done(plan)
				}
			}
		}
	}
}

// missingText says which cards are missing, by the names they had, for the message that nothing was
// loaded because of it.
func (a *cardsAsk) missingText() string {
	var names []string
	for _, n := range a.needs {
		for _, f := range n.found {
			if f.acceptable() {
				continue
			}
			name := f.Rec.describe()
			if n.label != "" {
				name = n.label + ": " + name
			}
			names = append(names, name)
		}
	}
	switch len(names) {
	case 0:
		return "the cards were not found"
	case 1:
		return names[0] + " was not found"
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1] + " were not found"
}

// warning says what going on with a card that is not the one comes to. A Next that had nothing of the
// person's open on its card carries on from a snapshot over another card that has the same system on
// it; one that was in the middle of something on its files finds another card under it. Whether the
// risk is worth it is for the person to say, so it is said once, and Load goes ahead the second time.
func (a *cardsAsk) warning() string {
	var others, bare []string
	for _, n := range a.needs {
		for _, f := range n.found {
			if f.Path == "" || f.acceptable() {
				continue
			}
			name := filepath.Base(f.Path)
			others = append(others, name)
			if !describeCard(f.Path).system {
				bare = append(bare, name)
			}
		}
	}
	text := "This is not the card the snapshot was made with."
	if len(others) > 1 {
		text = "These are not the cards the snapshot was made with."
	}
	text += " If nothing of yours was on that card, this one should do. If the Next was working with files on it, they may not be where the snapshot expects them, and saving could change what is on this card."
	if len(bare) > 0 {
		text += " " + strings.Join(bare, " and ") + " has no NextZXOS files, so the Next may not start from it."
	}
	return text
}
