//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenui/zenuiraylib"
)

// ============================================================================
// The Binary window (Snapshot > Load Binary...)
//
// Three tabs over the front machine's memory.
//
//	Load file      is -bin, -binaddr, -binstart and -binpage: a file of bytes put in memory at an
//	               address, in the memory as the CPU sees it now or in a RAM page, and started or not.
//	Peek and Poke  shows a run of bytes as a hex dump, as the machine runs, and writes bytes of the
//	               person's own, at an address in the same two ways.
//	Hot Memory     is a hex editor over the memory as it runs: pick the CPU's 64K or a RAM page from a
//	               list, and edit it in place (hotmem_gui.go).
//
// What a page is, and what is refused, is pagedmem.go and hotmem.go. Nothing is typed here that can
// be refused afterwards (inputs_gui.go): an address is hex digits that cannot pass the end of the
// memory chosen, the memory is chosen from a list, the bytes to write are hex digits, and what
// cannot be done with what is on the tab is said where the note is, with the button that would do
// it off. The dumps are drawn in a monospaced font (monofont_gui.go).
// ============================================================================

// binaryResult is what a frame of the window ended in.
type binaryResult int

const (
	binaryOpen   binaryResult = iota // still on screen
	binaryClosed                     // closed
)

// The window's tabs.
const (
	binTabLoad = iota
	binTabPeek
	binTabHot
)

// The window's buttons.
const (
	binBtnBrowse = "browse"
	binBtnLoad   = "load"
	binBtnPoke   = "poke"
	binBtnBack   = "back"
	binBtnNext   = "next"
	binBtnClose  = "close"
)

// binDumpRows is how many rows the dumps show when the screen is tall enough.
const binDumpRows = 12

// binMaxBytes is the most bytes the Poke box takes.
const binMaxBytes = 64

// binInput is a box of the tab showing, for Tab to move between.
type binInput struct {
	text  *zenui.TextField
	hex   *hexBox // nil for a box of text
	area  zenui.Rect
	limit int // the largest value a hex box can hold
}

// binaryWindow is the window, open for one machine.
type binaryWindow struct {
	zx   *ZenZX
	tabs *zenui.Tabs
	hot  *hotEditor

	// Load file
	file     *zenui.TextField
	loadMem  *spacePicker
	addr     *hexBox
	startChk *zenui.Checkbox
	start    *hexBox
	// startTouched is set when the start address has been typed: until then it is the load address.
	startTouched bool

	// Peek and Poke
	peekMem *spacePicker
	pAddr   *hexBox
	pBytes  *zenui.TextField

	buttons *zenui.ButtonBar
	browse  *zenui.Dialog

	// what the tab showing allows: the buttons that are on
	canLoad, canPoke, canBack, canNext bool

	dump   []string // the dump's lines, as last read
	dumpAt string   // what the dump is of, for its heading

	note    string
	refused bool   // the note says why something cannot be done
	result  string // what was done last, said until an input changes
	sig     string // the inputs as they were when it was done
	said    string // what the bar says as a toast when the window closes

	// mono draws the dumps: a monospaced face. nil draws them in the program's own.
	mono zenui.Renderer

	// handWanted is whether the pointer is over a tab: the program makes it a hand, as it does over
	// the tabs of the Preferences window.
	handWanted bool

	panel, tabsArea, bodyArea, barArea zenui.Rect
	fileA, addrA, chkA, startA         zenui.Rect // Load file's boxes
	pAddrA, pBytesA                    zenui.Rect // Peek and Poke's
	inputs                             []binInput // the boxes of the tab showing, in the order Tab moves through them
	scale, lineH, labelW, rows, per    int
}

// newBinaryWindow opens the window on a tab for machine zx.
func newBinaryWindow(zx *ZenZX, tab int) *binaryWindow {
	w := &binaryWindow{zx: zx, hot: newHotEditor(zx), buttons: zenui.NewButtonBar(),
		tabs:    &zenui.Tabs{Labels: []string{"Load file", "Peek and Poke", "Hot Memory"}, Selected: tab},
		loadMem: newSpacePicker(zx.memory), peekMem: newSpacePicker(zx.memory),
		file: zenui.NewTextField(), addr: newHexBox(4, 0x8000), start: newHexBox(4, 0x8000),
		pAddr: newHexBox(4, 0x8000), pBytes: zenui.NewTextField(), startChk: &zenui.Checkbox{Label: "Start at"}}
	w.file.MaxLen = 255
	w.pBytes.MaxLen = 2 * binMaxBytes
	w.pBytes.Accept = func(r rune) bool { _, ok := hexDigit(r); return ok }
	w.start.Disabled = true
	return w
}

// monoOr is the renderer for the dumps: the monospaced one, or r when there is none.
func (w *binaryWindow) monoOr(r zenui.Renderer) zenui.Renderer {
	if w.mono != nil {
		return w.mono
	}
	return r
}

// paged says whether this machine has memory to choose from.
func (w *binaryWindow) paged() bool { return w.loadMem.paged() }

// blurAll ends any edit that is going on, and closes the lists.
func (w *binaryWindow) blurAll() {
	w.file.Blur()
	w.addr.Blur()
	w.start.Blur()
	w.pAddr.Blur()
	w.pBytes.Blur()
	w.hot.goBox.Blur()
	w.hot.dropOpen = false
	w.loadMem.dropOpen = false
	w.peekMem.dropOpen = false
}

// picker is the choice of memory of the tab showing, or nil when it has none.
func (w *binaryWindow) picker() *spacePicker {
	switch w.tabs.Selected {
	case binTabLoad:
		return w.loadMem
	case binTabPeek:
		return w.peekMem
	}
	return nil
}

// active is the index of the box being edited, or -1.
func (w *binaryWindow) active() int {
	for i, in := range w.inputs {
		if in.text.Active() {
			return i
		}
	}
	return -1
}

// focus starts an edit in box i of the tab showing, with the caret at the end of its text, and ends
// the others.
func (w *binaryWindow) focus(i int, textW func(string) int) {
	for j, in := range w.inputs {
		if j != i {
			in.text.Blur()
			continue
		}
		b := in.text.Bounds(in.area, w.lineH)
		click := zenui.Input{MousePressed: true, MouseX: b.X + b.W - 1, MouseY: b.Y + b.H/2}
		if in.hex != nil {
			in.hex.Update(click, in.area, w.lineH, textW, in.limit)
		} else {
			in.text.Update(click, in.area, w.lineH, textW)
		}
	}
}

// setButtons makes the row of buttons for the tab showing, with the ones that cannot do anything off.
func (w *binaryWindow) setButtons() {
	switch w.tabs.Selected {
	case binTabPeek:
		w.buttons.Buttons = []zenui.BarButton{
			{ID: binBtnPoke, Label: "Poke", Disabled: !w.canPoke},
			{ID: binBtnBack, Label: "Back", Disabled: !w.canBack}, {ID: binBtnNext, Label: "Forward", Disabled: !w.canNext},
			{ID: binBtnClose, Label: "Close"},
		}
	case binTabHot:
		w.buttons.Buttons = []zenui.BarButton{{ID: binBtnClose, Label: "Close"}}
	default:
		w.buttons.Buttons = []zenui.BarButton{
			{ID: binBtnBrowse, Label: "Browse..."}, {ID: binBtnLoad, Label: "Load", Disabled: !w.canLoad}, {ID: binBtnClose, Label: "Close"},
		}
	}
}

// say sets the note under the fields; refused makes it the reason something cannot be done.
func (w *binaryWindow) say(refused bool, format string, a ...any) {
	w.note, w.refused = fmt.Sprintf(format, a...), refused
}

// inputsSig is every input of the two tabs as it is: what the result of an action is about.
func (w *binaryWindow) inputsSig() string {
	return fmt.Sprint(w.file.Text, "|", w.loadMem.cur, w.addr.Text, w.startChk.Checked, w.start.Text, "|",
		w.peekMem.cur, w.pAddr.Text, w.pBytes.Text)
}

// keepInRange holds the address boxes to the memory chosen, which is smaller when a page is, and the
// start to what a start can be: the CPU's, and only there.
func (w *binaryWindow) keepInRange() {
	w.addr.keep(w.loadMem.size() - 1)
	w.pAddr.keep(w.peekMem.size() - 1)
	w.start.keep(0xFFFF)
	inPage := w.loadMem.page() >= 0
	if inPage {
		w.startChk.Checked = false
	}
	w.startChk.Disabled = inPage
	w.start.Disabled = !w.startChk.Checked
	if !w.start.Active() && !w.startTouched {
		w.start.Set(w.addr.Value())
	}
}

// problem says why n bytes cannot be written at addr in a memory, or "" when they can.
func (w *binaryWindow) problem(p *spacePicker, addr, n int) string {
	var err error
	if p.page() < 0 {
		err = w.zx.memory.checkPokeMap(uint16(addr), n)
	} else {
		err = w.zx.memory.checkPokePage(p.page(), addr, n)
	}
	if err == nil {
		return ""
	}
	return strings.Replace(err.Error(), "(name a RAM page instead)", "(choose a RAM page in Memory)", 1)
}

// loadState works out whether the file can be loaded as things are, and says so in the note.
func (w *binaryWindow) loadState() {
	w.canLoad = false
	path := strings.TrimSpace(w.file.Text)
	if path == "" {
		w.say(false, "Name a file, or choose one with Browse...")
		return
	}
	st, err := os.Stat(path)
	switch {
	case err != nil:
		w.say(true, "%s: %s", filepath.Base(path), plainError(err))
		return
	case st.IsDir():
		w.say(true, "%s is a folder, not a file.", filepath.Base(path))
		return
	case st.Size() == 0:
		w.say(true, "%s is empty.", filepath.Base(path))
		return
	}
	n, addr := int(st.Size()), w.addr.Value()
	if msg := w.problem(w.loadMem, addr, n); msg != "" {
		w.say(true, "%s", msg)
		return
	}
	w.canLoad = true
	s := fmt.Sprintf("%d bytes, from 0x%04X to 0x%04X.", n, addr, addr+n-1)
	if w.startChk.Checked {
		s += fmt.Sprintf(" Then the program counter is set to 0x%04X.", w.start.Value())
	}
	w.say(false, "%s", s)
}

// pokeBytes is the bytes in the Poke box, and whether it has a digit left over.
func (w *binaryWindow) pokeBytes() (data []byte, odd bool) {
	t := w.pBytes.Text
	if len(t)%2 == 1 {
		t, odd = t[:len(t)-1], true
	}
	data, _ = hex.DecodeString(t)
	return data, odd
}

// peekLinear is where the dump is in its memory counted in bytes from the start of the first page,
// and how many bytes the memory has: for stepping across the end of a page.
func (w *binaryWindow) peekLinear() (lin, total int) {
	if w.peekMem.page() < 0 {
		return w.pAddr.Value(), 0x10000
	}
	l := w.zx.memory.PageLayout()
	return w.peekMem.page()*l.Size + w.pAddr.Value(), l.Count * l.Size
}

// peekState reads the dump as the machine has it, and works out whether the bytes can be written.
func (w *binaryWindow) peekState() {
	p, addr := w.peekMem, w.pAddr.Value()
	var data []byte
	if p.page() < 0 {
		data = w.zx.memory.PeekMap(uint16(addr), w.dumpBytes())
		w.dumpAt = "the memory as the CPU sees it"
	} else {
		data, _ = w.zx.memory.PeekPage(p.page(), addr, w.dumpBytes())
		w.dumpAt = fmt.Sprintf("%s %d", w.zx.memory.PageLayout().pageWord(), p.page())
	}
	w.dump = hexDump(addr, data, max(w.per, 1))

	lin, total := w.peekLinear()
	w.canBack, w.canNext = lin > 0, lin+w.dumpBytes() < total

	w.canPoke = false
	bytes, odd := w.pokeBytes()
	switch {
	case w.result != "":
		w.say(false, "%s", w.result)
	case len(w.pBytes.Text) == 0:
		w.say(false, "Type the bytes to write as hex digits, two to a byte.")
	case len(bytes) == 0:
		w.say(false, "A byte is two digits: one more.")
	default:
		if msg := w.problem(p, addr, len(bytes)); msg != "" {
			w.say(true, "%s", msg)
			break
		}
		if odd {
			w.say(false, "One digit more makes the last byte: two digits to a byte.")
			break
		}
		w.canPoke = true
		w.say(false, "%d bytes to write at 0x%04X.", len(bytes), addr)
	}
}

// refreshState works out what the tab showing allows and says, for the buttons and the note.
func (w *binaryWindow) refreshState() {
	w.keepInRange()
	if sig := w.inputsSig(); sig != w.sig {
		w.result, w.sig = "", sig
	}
	switch w.tabs.Selected {
	case binTabLoad:
		w.loadState()
	case binTabPeek:
		w.peekState()
	}
	w.setButtons()
}

// doLoad is the Load button: the file into memory, and the window closed when it was.
func (w *binaryWindow) doLoad() {
	if !w.canLoad {
		return
	}
	path := strings.TrimSpace(w.file.Text)
	start := -1
	if w.startChk.Checked {
		start = w.start.Value()
	}
	msg, err := w.zx.LoadBINAt(path, w.loadMem.page(), uint16(w.addr.Value()), start)
	if err != nil {
		w.say(true, "Not loaded: %s", plainError(err)) // the file changed since it was looked at
		return
	}
	w.said = fmt.Sprintf("%s: %s", filepath.Base(path), msg)
	sayf("Binary: %s\n", w.said)
}

// dumpBytes is how many bytes the Peek and Poke dump shows.
func (w *binaryWindow) dumpBytes() int { return max(w.rows, 1) * max(w.per, 1) }

// hexDump lays bytes out as rows of perRow: the address, the bytes in hex, then as the characters
// they are when they are printable. The address is the one of the window, not of the page. It is
// meant for a monospaced face.
func hexDump(addr int, data []byte, perRow int) []string {
	var out []string
	for i := 0; i < len(data); i += perRow {
		row := data[i:min(i+perRow, len(data))]
		var digits, text strings.Builder
		for _, b := range row {
			fmt.Fprintf(&digits, "%02X ", b)
			if b >= 32 && b < 127 {
				text.WriteByte(b)
			} else {
				text.WriteByte('.')
			}
		}
		out = append(out, fmt.Sprintf("%04X  %-*s %s", addr+i, perRow*3, digits.String(), text.String()))
	}
	return out
}

// doPoke is the Poke button: the bytes written, and the dump read again.
func (w *binaryWindow) doPoke() {
	if !w.canPoke {
		return
	}
	data, _ := w.pokeBytes()
	p, addr := w.peekMem, w.pAddr.Value()
	var err error
	if p.page() < 0 {
		err = w.zx.memory.PokeMap(uint16(addr), data)
	} else {
		err = w.zx.memory.PokePage(p.page(), addr, data)
	}
	if err != nil {
		w.say(true, "Not written: %s", plainError(err)) // the memory was paged since it was looked at
		return
	}
	sayf("Poke: %d bytes at 0x%04X\n", len(data), addr)
	w.pBytes.Text = ""
	w.sig = w.inputsSig() // emptying the box is not a change that ends the result
	w.result = fmt.Sprintf("%d bytes written at 0x%04X.", len(data), addr)
}

// doStep moves the dump on or back by what it shows, across the end of a page when it is in one.
func (w *binaryWindow) doStep(dir int) {
	lin, total := w.peekLinear()
	lin = min(max(lin+dir*w.dumpBytes(), 0), total-1)
	if w.peekMem.page() < 0 {
		w.pAddr.Set(lin)
		return
	}
	size := w.zx.memory.PageLayout().Size
	for i, sp := range w.peekMem.spaces {
		if sp.Page == lin/size {
			w.peekMem.set(i)
		}
	}
	w.pAddr.Set(lin % size)
}

// do carries out a button.
func (w *binaryWindow) do(id string) binaryResult {
	switch id {
	case binBtnClose:
		return binaryClosed
	case binBtnBrowse:
		start := filepath.Dir(strings.TrimSpace(w.file.Text))
		if st, err := os.Stat(start); err != nil || !st.IsDir() {
			start = "."
		}
		w.blurAll()
		w.browse = zenui.NewDialog(zenui.DialogConfig{
			Mode: zenui.ModeOpen, Title: "Open a Binary", StartDir: start, FS: zenui.OSFS{},
		})
	case binBtnLoad:
		w.doLoad()
		if w.said != "" {
			return binaryClosed
		}
	case binBtnPoke:
		w.doPoke()
	case binBtnBack:
		w.doStep(-1)
	case binBtnNext:
		w.doStep(1)
	}
	return binaryOpen
}

// layout places the window's parts. The window is as tall as its tallest tab, so that it does not
// change size as the tabs are chosen.
func (w *binaryWindow) layout(r zenui.Renderer, screenW, screenH int) {
	w.keepInRange()
	w.setButtons()
	mono := w.monoOr(r)
	w.scale = zenui.TextBodyScale(r, screenW)
	w.lineH = max(r.LineHeight(w.scale), 1)
	mlh := max(mono.LineHeight(w.scale), 1)
	mcw := max(mono.TextWidth("0", w.scale), 1)
	pad := r.LineHeight(1)

	// wide enough for the usual text panel, and for a dump of 16 bytes a row
	pw := max(zenui.TextPanelWidth(r, w.scale), hotRowChars(16)*mcw+2*hotPad+2*pad)
	pw = min(pw, screenW-2*pad)
	inner := pw - 2*pad
	w.per = hotPerRow(mcw, inner)
	textW := func(s string) int { return r.TextWidth(s, w.scale) }
	_, barH := w.buttons.Layout(zenui.Rect{W: inner}, w.lineH, textW)
	titleH := r.LineHeight(2) + 8
	tabsH := w.tabs.Height(w.lineH)
	fieldH := w.lineH + w.lineH/2
	rowH := fieldH + w.lineH/3

	w.labelW = 0
	for _, s := range []string{"File", "Memory", "Address", "Bytes", "Go to"} {
		w.labelW = max(w.labelW, textW(s))
	}
	check := zenui.MarkSize(w.lineH) + max(4, w.lineH/3) + textW(w.startChk.Label)
	w.labelW = max(w.labelW, check) + pad

	paged := 0
	if w.paged() {
		paged = 1
	}
	noteH := 2 * w.lineH
	body := func(rows int) int {
		load := (3+paged)*rowH + noteH
		peek := (2+paged)*rowH + (rows+1)*mlh + w.lineH/2 + noteH
		hot := hotHeight(r, mono, w.scale, rows) + noteH
		return max(load, max(peek, hot))
	}
	fixed := 2*pad + titleH + tabsH + pad + 2*pad + barH
	w.rows = binDumpRows
	for w.rows > hotMinRows && fixed+body(w.rows) > screenH-2*pad {
		w.rows--
	}
	bodyH := body(w.rows)
	ph := fixed + bodyH
	px, py := (screenW-pw)/2, max((screenH-ph)/2, 0)
	w.panel = zenui.Rect{X: px, Y: py, W: pw, H: ph}
	y := py + pad + titleH
	w.tabsArea = zenui.Rect{X: px + pad, Y: y, W: inner, H: tabsH}
	y += tabsH + pad
	w.bodyArea = zenui.Rect{X: px + pad, Y: y, W: inner, H: bodyH}

	// the rows of the boxes: each at the label's width, a hex box as wide as its digits
	x := px + pad + w.labelW
	hexW := hexWidth(r, w.scale, 4)
	row := func(i int) int { return y + i*rowH }
	room := func(sel zenui.Rect) int { return w.bodyArea.Y + bodyH - (sel.Y + sel.H) }
	w.inputs = w.inputs[:0]
	switch w.tabs.Selected {
	case binTabLoad:
		i := 0
		w.fileA = zenui.Rect{X: x, Y: row(i), W: inner - w.labelW, H: fieldH}
		i++
		if w.paged() {
			sel := zenui.Rect{X: x, Y: row(i), W: inner - w.labelW, H: fieldH}
			w.loadMem.place(sel, w.lineH, room(sel))
			i++
		}
		w.addrA = zenui.Rect{X: x, Y: row(i), W: hexW, H: fieldH}
		i++
		w.chkA = zenui.Rect{X: px + pad, Y: row(i), W: w.labelW, H: fieldH}
		w.startA = zenui.Rect{X: x, Y: row(i), W: hexW, H: fieldH}
		w.inputs = append(w.inputs,
			binInput{text: w.file, area: w.fileA},
			binInput{text: w.addr.TextField, hex: w.addr, area: w.addrA, limit: w.loadMem.size() - 1})
		if !w.start.Disabled {
			w.inputs = append(w.inputs, binInput{text: w.start.TextField, hex: w.start, area: w.startA, limit: 0xFFFF})
		}
	case binTabPeek:
		i := 0
		if w.paged() {
			sel := zenui.Rect{X: x, Y: row(i), W: inner - w.labelW, H: fieldH}
			w.peekMem.place(sel, w.lineH, room(sel))
			i++
		}
		w.pAddrA = zenui.Rect{X: x, Y: row(i), W: hexW, H: fieldH}
		i++
		w.pBytesA = zenui.Rect{X: x, Y: row(i), W: inner - w.labelW, H: fieldH}
		w.inputs = append(w.inputs,
			binInput{text: w.pAddr.TextField, hex: w.pAddr, area: w.pAddrA, limit: w.peekMem.size() - 1},
			binInput{text: w.pBytes, area: w.pBytesA})
	case binTabHot:
		// the editor takes the height the other tabs need, as more rows
		rows := w.rows + max(0, (bodyH-noteH-hotHeight(r, mono, w.scale, w.rows))/mlh)
		w.hot.layout(r, mono, w.scale, zenui.Rect{X: w.bodyArea.X, Y: y, W: inner, H: bodyH - noteH}, w.labelW, rows)
	}
	y += bodyH + 2*pad
	w.barArea = zenui.Rect{X: px + pad, Y: y, W: inner, H: barH}
}

// Update runs the window for a frame.
func (w *binaryWindow) Update(in zenui.Input, r zenui.Renderer, screenW, screenH int) binaryResult {
	w.handWanted = false
	if w.browse != nil {
		switch w.browse.Update(in) {
		case zenui.Accepted:
			w.file.Text = w.browse.Result()
			w.browse = nil
		case zenui.Cancelled:
			w.browse = nil
		}
		return binaryOpen
	}
	w.layout(r, screenW, screenH)
	for _, t := range w.tabs.Layout(w.tabsArea, w.lineH) {
		if t.Contains(in.MouseX, in.MouseY) {
			w.handWanted = true
		}
	}
	textW := func(s string) int { return r.TextWidth(s, w.scale) }
	hot := w.tabs.Selected == binTabHot
	was := w.active()
	editing := was >= 0 || hot && w.hot.editing()

	if _, changed := w.tabs.Update(in, w.tabsArea, w.lineH); changed {
		w.blurAll()
		w.layout(r, screenW, screenH)
		w.refreshState()
		return binaryOpen
	}
	var enter, tab, esc, pgUp, pgDn bool
	for _, k := range in.Keys {
		switch k {
		case zenui.KeyPageUp:
			pgUp = true
		case zenui.KeyPageDown:
			pgDn = true
		case zenui.KeyEnter:
			enter = true
		case zenui.KeyTab:
			tab = true
		case zenui.KeyEscape:
			esc = true
		}
	}

	if hot {
		listWasOpen := w.hot.dropOpen
		used := w.hot.update(in, r)
		switch {
		case tab:
			if w.hot.goBox.Active() {
				w.hot.goTo(w.hot.goBox.Value()) // leaving the box goes where it says
				w.hot.goBox.Blur()
			} else if !w.hot.dropOpen {
				w.hot.toggleMode()
			}
		case esc && !used && !editing:
			return binaryClosed
		}
		w.note, w.refused = w.hot.note, w.hot.refused
		if listWasOpen || w.hot.dropOpen {
			return binaryOpen // the list had the pointer and the keys
		}
		if id, ok := w.buttons.Update(in, w.barArea, w.lineH, textW); ok {
			return w.do(id)
		}
		return binaryOpen
	}

	// the list of memories, when it is open, has the keyboard and the pointer
	pk := w.picker()
	if pk.dropOpen {
		pk.update(in, w.lineH)
		w.refreshState()
		return binaryOpen
	}
	if pk.paged() && in.MousePressed && pk.selRect.Contains(in.MouseX, in.MouseY) {
		w.blurAll()
		pk.open()
		return binaryOpen
	}

	w.updateBoxes(in, textW)
	w.refreshState()
	if w.tabs.Selected == binTabPeek {
		// Page Up and Page Down page the dump as Back and Forward do, and stop where they stop
		if pgUp && w.canBack {
			w.doStep(-1)
			w.refreshState()
		}
		if pgDn && w.canNext {
			w.doStep(1)
			w.refreshState()
		}
	}
	switch {
	case tab:
		next := 0
		if was >= 0 {
			next = (was + 1) % len(w.inputs)
		}
		w.focus(next, textW)
		return binaryOpen
	case esc && was < 0:
		return binaryClosed
	case enter && was >= 0:
		if w.tabs.Selected == binTabPeek {
			return w.do(binBtnPoke)
		}
		return w.do(binBtnLoad)
	}
	if id, ok := w.buttons.Update(in, w.barArea, w.lineH, textW); ok {
		return w.do(id)
	}
	return binaryOpen
}

// updateBoxes runs the boxes of the tab showing, the start one following the load address until it
// has been typed in.
func (w *binaryWindow) updateBoxes(in zenui.Input, textW func(string) int) {
	switch w.tabs.Selected {
	case binTabLoad:
		w.file.Update(in, w.fileA, w.lineH, textW)
		w.addr.Update(in, w.addrA, w.lineH, textW, w.loadMem.size()-1)
		w.startChk.Update(in, w.chkA, w.lineH, textW)
		w.start.Disabled = !w.startChk.Checked
		if w.start.Update(in, w.startA, w.lineH, textW, 0xFFFF) == zenui.TextChanged {
			w.startTouched = true
		}
	case binTabPeek:
		w.pAddr.Update(in, w.pAddrA, w.lineH, textW, w.peekMem.size()-1)
		w.pBytes.Update(in, w.pBytesA, w.lineH, textW)
		w.pBytes.Text = strings.ToUpper(w.pBytes.Text)
	}
}

// Draw draws the window over the machine's picture.
func (w *binaryWindow) Draw(r zenui.Renderer, screenW, screenH int, theme zenui.Theme) {
	if w.browse != nil {
		w.browse.Draw(r, screenW, screenH, theme)
		return
	}
	w.layout(r, screenW, screenH)
	if w.tabs.Selected != binTabHot {
		w.refreshState()
	}
	mono := w.monoOr(r)
	r.FillRect(zenui.Rect{W: screenW, H: screenH}, zenui.ScaleAlpha(theme.Backdrop, zenui.ModalBackdropDarkening))
	r.FillRect(w.panel, theme.Panel)
	r.StrokeRect(w.panel, theme.Border, theme.BorderThickness)

	pad := r.LineHeight(1)
	x, y := w.panel.X+pad, w.panel.Y+pad
	r.DrawText("LOAD BINARY", x, y, 2, theme.Text)
	w.tabs.Draw(r, theme, w.tabsArea, w.scale)

	fieldH := w.lineH + w.lineH/2
	cols := w.bodyArea.W / max(r.TextWidth("n", w.scale), 1)
	noteY := w.bodyArea.Y + w.bodyArea.H - 2*w.lineH
	label := func(s string, a zenui.Rect) { r.DrawText(s, x, a.Y+(fieldH-w.lineH)/2, w.scale, theme.Text) }

	switch w.tabs.Selected {
	case binTabHot:
		w.hot.draw(r, mono, theme, x)
	case binTabLoad:
		label("File", w.fileA)
		w.file.Draw(r, theme, w.fileA, w.scale)
		if w.paged() {
			w.loadMem.draw(r, theme, w.scale, x, "Memory")
		}
		label("Address", w.addrA)
		w.addr.Draw(r, theme, w.addrA, w.scale)
		w.startChk.Draw(r, theme, w.chkA, w.scale)
		w.start.Draw(r, theme, w.startA, w.scale)
	case binTabPeek:
		if w.paged() {
			w.peekMem.draw(r, theme, w.scale, x, "Memory")
		}
		label("Address", w.pAddrA)
		w.pAddr.Draw(r, theme, w.pAddrA, w.scale)
		label("Bytes", w.pBytesA)
		w.pBytes.Draw(r, theme, w.pBytesA, w.scale)
		w.drawDump(r, mono, theme, w.pBytesA.Y+w.pBytesA.H+w.lineH/3)
	}

	col := theme.DimText
	if w.refused {
		col = theme.DirText // the reason something cannot be done, in the accent colour, as the questions' are
	}
	for i, line := range zenui.WrapText(w.note, cols, "") {
		if i > 1 {
			break
		}
		r.DrawText(line, x, noteY+i*w.lineH, w.scale, col)
	}
	w.buttons.Draw(r, theme, w.barArea, w.scale)
	// the list of memories, over the note and the buttons too
	if w.tabs.Selected == binTabHot {
		w.hot.drawList(r, theme)
	} else {
		w.picker().drawList(r, theme, w.scale)
	}
}

// drawDump draws the dump of Peek and Poke from y in the monospaced face: always as many rows as
// there is room for, so that what is under it does not move.
func (w *binaryWindow) drawDump(r, mono zenui.Renderer, theme zenui.Theme, y int) {
	x := w.bodyArea.X
	r.DrawText("In "+w.dumpAt+":", x, y, w.scale, theme.DimText)
	mlh := max(mono.LineHeight(w.scale), 1)
	y += mlh
	for i := 0; i < w.rows; i++ {
		if i < len(w.dump) {
			line := w.dump[i]
			mono.DrawText(line[:4], x, y, w.scale, theme.DirText)
			mono.DrawText(line[4:], x+4*mono.TextWidth("0", w.scale), y, w.scale, theme.Text)
		}
		y += mlh
	}
}

// repeatKeys are the keys a held key goes on pressing: the ones that move through a dump or take
// text away. zenui's input has a key as pressed on the frame it goes down only, so a held Page Down
// did one page.
var repeatKeys = []struct {
	rl int32
	k  zenui.Key
}{
	{rl.KeyUp, zenui.KeyUp}, {rl.KeyDown, zenui.KeyDown}, {rl.KeyLeft, zenui.KeyLeft}, {rl.KeyRight, zenui.KeyRight},
	{rl.KeyPageUp, zenui.KeyPageUp}, {rl.KeyPageDown, zenui.KeyPageDown}, {rl.KeyBackspace, zenui.KeyBackspace},
}

// withKeyRepeat adds to the keys of a frame the ones that are repeating on it, once each.
func withKeyRepeat(in zenui.Input, repeating func(key int32) bool) zenui.Input {
	for _, r := range repeatKeys {
		if !repeating(r.rl) {
			continue
		}
		seen := false
		for _, k := range in.Keys {
			seen = seen || k == r.k
		}
		if !seen {
			in.Keys = append(in.Keys, r.k)
		}
	}
	return in
}

// openBinary opens the Binary window on a tab.
func (b *appMenuBar) openBinary(zx *ZenZX, tab int) {
	b.closeMenus()
	b.bin = newBinaryWindow(zx, tab)
	b.bin.mono = b.monoRenderer()
}

// updateBinary runs the Binary window and acts on what it ends in.
func (b *appMenuBar) updateBinary(zx *ZenZX) {
	w := b.bin
	w.mono = b.monoRenderer()
	if w.Update(withKeyRepeat(zenuiraylib.Input(), rl.IsKeyPressedRepeat), b.renderer, rl.GetScreenWidth(), rl.GetScreenHeight()) == binaryClosed {
		b.bin = nil
		if w.said != "" {
			zx.notice("%s", w.said)
		}
	}
	setHandCursor(&b.binHand, b.bin != nil && b.bin.handWanted) // the arrow again when the window goes
}
