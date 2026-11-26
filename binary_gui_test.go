//go:build !headless

package zenzx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/zenui"
)

// The Binary window (binary_gui.go, hotmem_gui.go, inputs_gui.go): Snapshot > Load Binary...

const (
	binScreenW = 1000
	binScreenH = 700
)

func binWindow(t *testing.T, model string, tab int) (*binaryWindow, *ZenZX, *paintRenderer) {
	t.Helper()
	zx := machineOf(t, model)
	w := newBinaryWindow(zx, tab)
	r := &paintRenderer{}
	w.Update(zenui.Input{}, r, binScreenW, binScreenH) // lays it out
	return w, zx, r
}

func binFile(t *testing.T, data ...byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "prog.bin")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func binFrame(w *binaryWindow, r zenui.Renderer, in zenui.Input) binaryResult {
	return w.Update(in, r, binScreenW, binScreenH)
}

func binPress(x, y int) zenui.Input { return zenui.Input{MousePressed: true, MouseX: x, MouseY: y} }

func binCentre(r zenui.Rect) (int, int) { return r.X + r.W/2, r.Y + r.H/2 }

func binClick(w *binaryWindow, r zenui.Renderer, rect zenui.Rect) {
	x, y := binCentre(rect)
	binFrame(w, r, binPress(x, y))
}

func binType(w *binaryWindow, r zenui.Renderer, s string) {
	binFrame(w, r, zenui.Input{Chars: []rune(s)})
}

func binKey(w *binaryWindow, r zenui.Renderer, k ...zenui.Key) binaryResult {
	return binFrame(w, r, zenui.Input{Keys: k})
}

// binEnter clicks a box and types into it, as a person does.
func binEnter(w *binaryWindow, r zenui.Renderer, box zenui.Rect, s string) {
	binClick(w, r, box)
	binType(w, r, s)
}

// pick chooses the RAM page of a picker, or the CPU's memory for -1.
func pick(t *testing.T, p *spacePicker, page int) {
	t.Helper()
	for i, sp := range p.spaces {
		if sp.Page == page {
			p.set(i)
			return
		}
	}
	t.Fatalf("no space for page %d", page)
}

// button is a button of the row as the window has it now.
func button(t *testing.T, w *binaryWindow, id string) zenui.BarButton {
	t.Helper()
	for _, b := range w.buttons.Buttons {
		if b.ID == id {
			return b
		}
	}
	t.Fatalf("no %q button on tab %d", id, w.tabs.Selected)
	return zenui.BarButton{}
}

// The menu has the row, and it opens a window that is modal and drawn by the fade.
func TestLoadBinaryRowOpensTheWindow(t *testing.T) {
	inSnapshotDir(t)
	b, zx := newTestBar(t, nil, "")
	load := snapRow(t, b, "Load Binary...")
	b.snapSpec.pick(zx, load, -1)
	if b.bin == nil || !b.modalOpen() || !b.Active() {
		t.Fatal("Load Binary... did not open the window")
	}
	if b.bin.tabs.Selected != binTabLoad {
		t.Errorf("it opened on tab %d, want Load file", b.bin.tabs.Selected)
	}
	if b.modalDrawer(binScreenW, binScreenH) == nil {
		t.Error("the window is not drawn by the fade")
	}
}

// A file goes in memory as the CPU sees it, at the address, and is started when asked; on every
// model.
func TestLoadFileOnEveryModel(t *testing.T) {
	for _, c := range pagingByModel {
		w, zx, r := binWindow(t, c.model, binTabLoad)
		w.file.Text = binFile(t, 1, 2, 3, 4, 5)
		w.addr.Set(0x9000)
		w.startChk.Checked = true
		binFrame(w, r, zenui.Input{})
		if !w.canLoad || button(t, w, binBtnLoad).Disabled {
			t.Errorf("%s: Load is off: %q", c.model, w.note)
			continue
		}
		if got := w.do(binBtnLoad); got != binaryClosed {
			t.Errorf("%s: Load left the window %v (%s)", c.model, got, w.note)
			continue
		}
		if got := zx.memory.PeekMap(0x9000, 5); string(got) != "\x01\x02\x03\x04\x05" {
			t.Errorf("%s: memory at 0x9000 is %v", c.model, got)
		}
		if zx.cpu.PC != 0x9000 {
			t.Errorf("%s: PC is 0x%04X, want 0x9000", c.model, zx.cpu.PC)
		}
		if !strings.Contains(w.said, "5 bytes loaded at 0x9000") || !strings.Contains(w.said, "started at 0x9000") {
			t.Errorf("%s: it said %q", c.model, w.said)
		}
	}
}

// Without Start at, the file is loaded and the machine goes on where it was.
func TestLoadFileWithoutStartLeavesPC(t *testing.T) {
	w, zx, r := binWindow(t, "128k", binTabLoad)
	w.file.Text = binFile(t, 9, 9)
	pc := zx.cpu.PC
	binFrame(w, r, zenui.Input{})
	if w.do(binBtnLoad) != binaryClosed || zx.cpu.PC != pc {
		t.Errorf("PC went from 0x%04X to 0x%04X: %q", pc, zx.cpu.PC, w.note)
	}
}

// A page names RAM wherever it is paged: on every model that has pages, and the CPU's view agrees
// when the page is paged in; a model with none has no list to choose from.
func TestLoadFileInAPageOnEveryModel(t *testing.T) {
	for _, c := range pagingByModel {
		w, zx, r := binWindow(t, c.model, binTabLoad)
		if c.kind == pagesNone {
			if w.paged() || w.loadMem.paged() {
				t.Errorf("%s: a model with no pages is offered a choice of memory", c.model)
			}
			continue
		}
		w.file.Text = binFile(t, 0xDE, 0xAD, 0xBE)
		w.startChk.Checked = true // asked for, and then a page is chosen: it cannot be started
		pick(t, w.loadMem, 3)
		w.addr.Set(0x0100)
		binFrame(w, r, zenui.Input{})
		if w.startChk.Checked || !w.startChk.Disabled || !w.start.Disabled {
			t.Errorf("%s: Start at is %v/%v in a page, want it off and unavailable", c.model, w.startChk.Checked, w.startChk.Disabled)
		}
		if got := w.do(binBtnLoad); got != binaryClosed {
			t.Errorf("%s: %q", c.model, w.note)
			continue
		}
		base := pagedIn(zx, 3)
		if got := zx.memory.PeekMap(base+0x0100, 3); string(got) != "\xDE\xAD\xBE" {
			t.Errorf("%s: page 3 reads %v when paged in", c.model, got)
		}
		if zx.cpu.PC == 0x0100 {
			t.Errorf("%s: a load into a page moved PC", c.model)
		}
	}
}

// What cannot be done is said, with Load off, the window stays open, and nothing in memory changes.
func TestLoadIsOffUnlessItCanBeDone(t *testing.T) {
	good := binFile(t, 1, 2, 3)
	empty := binFile(t)
	cases := []struct {
		name   string
		model  string
		file   string
		page   int // -1: the CPU's memory
		addr   int
		want   string
		wantOK bool
	}{
		{"no file", "128k", "", -1, 0x8000, "Name a file", false},
		{"missing file", "128k", "/no/such/file.bin", -1, 0x8000, "file.bin", false},
		{"a folder", "128k", filepath.Dir(good), -1, 0x8000, "folder", false},
		{"empty file", "128k", empty, -1, 0x8000, "empty", false},
		{"rom", "128k", good, -1, 0x0100, "ROM", false},
		{"rom on 48k", "48k", good, -1, 0x3FFF, "ROM", false},
		{"rom on next", "next", good, -1, 0x0000, "ROM", false},
		{"past the last bank's end", "128k", good, 7, 0x3FFF, "last bank", false},
		{"top of the 64K", "128k", good, -1, 0xFFFE, "64K", false},
		{"fine", "128k", good, -1, 0x8000, "3 bytes", true},
		{"fine in a page", "128k", good, 1, 0x3FFD, "3 bytes", true},
		{"fine on 48k", "48k", good, -1, 0x4000, "3 bytes", true},
	}
	for _, c := range cases {
		w, zx, r := binWindow(t, c.model, binTabLoad)
		pc := zx.cpu.PC
		w.file.Text = c.file
		pick(t, w.loadMem, c.page)
		w.addr.Set(c.addr)
		before := zx.memory.PeekMap(0x8000, 16)
		binFrame(w, r, zenui.Input{})
		if w.canLoad != c.wantOK || button(t, w, binBtnLoad).Disabled == c.wantOK {
			t.Errorf("%s: canLoad %v, Load disabled %v, want it %v", c.name, w.canLoad, button(t, w, binBtnLoad).Disabled, c.wantOK)
		}
		if (w.refused == c.wantOK && c.name != "no file") || !strings.Contains(w.note, c.want) {
			t.Errorf("%s: note %q (refused %v), want it to say %q", c.name, w.note, w.refused, c.want)
		}
		if !c.wantOK {
			if got := w.do(binBtnLoad); got != binaryOpen || w.said != "" {
				t.Errorf("%s: Load did something: %v %q", c.name, got, w.said)
			}
			if zx.cpu.PC != pc || string(zx.memory.PeekMap(0x8000, 16)) != string(before) {
				t.Errorf("%s: something changed", c.name)
			}
		}
	}
}

// The reason for a ROM address names the way out, and not the old page number box.
func TestLoadRomReasonPointsAtTheList(t *testing.T) {
	w, _, r := binWindow(t, "128k", binTabLoad)
	w.file.Text = binFile(t, 1)
	w.addr.Set(0x0100)
	binFrame(w, r, zenui.Input{})
	if strings.Contains(w.note, "name a RAM page") || !strings.Contains(w.note, "choose a RAM page in Memory") {
		t.Errorf("the note is %q", w.note)
	}
}

// The boxes take only what can be used: hex digits, in capitals, no more than four, and no more than
// the memory has.
func TestAddressBoxesTakeOnlyAddresses(t *testing.T) {
	w, _, r := binWindow(t, "128k", binTabLoad)
	binEnter(w, r, w.addrA, "zq-12 g!")
	if w.addr.Text != "12" {
		t.Errorf("typed zq-12 g!: the box holds %q, want 12", w.addr.Text)
	}
	binType(w, r, "abcdef")
	if w.addr.Text != "12AB" {
		t.Errorf("typed abcdef after 12: %q, want 12AB (four digits, capitals)", w.addr.Text)
	}
	binKey(w, r, zenui.KeyEnter)
	if w.addr.Text != "12AB" || w.addr.Active() {
		t.Errorf("after Enter: %q, active %v", w.addr.Text, w.addr.Active())
	}
	// in a bank the address cannot pass 0x3FFF, however it is typed
	pick(t, w.loadMem, 2)
	binFrame(w, r, zenui.Input{})
	if w.addr.Text != "12AB" {
		t.Errorf("a bank was chosen with the address at 0x12AB, which fits it: %q", w.addr.Text)
	}
	binEnter(w, r, w.addrA, "ffff")
	if w.addr.Text != "3FFF" {
		t.Errorf("typed ffff in a bank: %q, want 3FFF", w.addr.Text)
	}
	// and choosing a bank when the address is past its end brings the address back
	pick(t, w.loadMem, -1)
	binFrame(w, r, zenui.Input{})
	w.addr.Blur()
	w.addr.Set(0xC000)
	pick(t, w.loadMem, 2)
	binFrame(w, r, zenui.Input{})
	if w.addr.Value() != 0x3FFF {
		t.Errorf("an address of 0xC000 with a bank chosen: 0x%X, want 0x3FFF", w.addr.Value())
	}
}

// A box left half typed is made whole when it is left.
func TestHexBoxIsPaddedWhenLeft(t *testing.T) {
	w, _, r := binWindow(t, "48k", binTabLoad)
	binEnter(w, r, w.addrA, "7")
	if w.addr.Text != "7" {
		t.Fatalf("while typing: %q", w.addr.Text)
	}
	binKey(w, r, zenui.KeyEnter)
	if w.addr.Text != "0007" {
		t.Errorf("after Enter: %q, want 0007", w.addr.Text)
	}
	binEnter(w, r, w.addrA, "")
	binKey(w, r, zenui.KeyBackspace)
	binClick(w, r, w.fileA) // leave it empty
	if w.addr.Text != "0000" {
		t.Errorf("left empty: %q, want 0000", w.addr.Text)
	}
}

// A click in a hex box selects what is in it: the first digit replaces, and Left puts the caret in
// the number instead.
func TestHexBoxFirstDigitReplaces(t *testing.T) {
	b := newHexBox(4, 0x8000)
	textW := func(s string) int { return 8 * len(s) }
	area := zenui.Rect{X: 0, Y: 0, W: 100, H: 20}
	click := binPress(50, 10)
	b.Update(click, area, 16, textW, 0xFFFF)
	if !b.Active() || !b.fresh {
		t.Fatalf("a click: active %v, fresh %v", b.Active(), b.fresh)
	}
	b.Update(zenui.Input{Chars: []rune("1")}, area, 16, textW, 0xFFFF)
	if b.Text != "1" || b.fresh {
		t.Errorf("one digit after the click: %q (fresh %v), want 1", b.Text, b.fresh)
	}
	b.Blur()
	b.Set(0x8000)
	b.Update(click, area, 16, textW, 0xFFFF)
	b.Update(zenui.Input{Keys: []zenui.Key{zenui.KeyBackspace}}, area, 16, textW, 0xFFFF)
	if b.Text != "" {
		t.Errorf("Backspace after the click: %q, want it empty", b.Text)
	}
	b.Blur()
	b.Set(0x8000)
	b.Update(click, area, 16, textW, 0xFFFF)
	b.Update(zenui.Input{Keys: []zenui.Key{zenui.KeyLeft}}, area, 16, textW, 0xFFFF)
	if b.fresh || b.Text != "8000" {
		t.Errorf("Left: fresh %v, text %q", b.fresh, b.Text)
	}
	if b.Value() != 0x8000 || (&hexBox{TextField: zenui.NewTextField()}).Value() != 0 {
		t.Error("Value is not the number in the box, or 0 when it has none")
	}
}

// The start address follows the load address until it is typed, and is off while Start at is.
func TestStartAddressFollowsTheLoadAddress(t *testing.T) {
	w, _, r := binWindow(t, "128k", binTabLoad)
	w.file.Text = binFile(t, 1, 2, 3)
	if !w.start.Disabled || w.startChk.Checked {
		t.Fatal("Start at begins on")
	}
	binEnter(w, r, w.startA, "ff") // the box is off: nothing is typed
	if w.start.Active() || w.start.Text != "8000" {
		t.Errorf("a click on the off Start box: active %v, %q", w.start.Active(), w.start.Text)
	}
	binClick(w, r, zenui.Rect{X: w.chkA.X, Y: w.chkA.Y, W: 4, H: w.chkA.H}) // the check mark
	if !w.startChk.Checked || w.start.Disabled {
		t.Fatalf("a click on Start at: checked %v, box off %v", w.startChk.Checked, w.start.Disabled)
	}
	binEnter(w, r, w.addrA, "9100")
	binKey(w, r, zenui.KeyEnter)
	if w.start.Text != "9100" {
		t.Errorf("the load address went to 9100 and the start is %q", w.start.Text)
	}
	binEnter(w, r, w.startA, "9123")
	binKey(w, r, zenui.KeyEnter)
	binEnter(w, r, w.addrA, "A000")
	binKey(w, r, zenui.KeyEnter)
	if w.start.Text != "9123" {
		t.Errorf("a start that was typed moved to %q with the load address", w.start.Text)
	}
	if !strings.Contains(w.note, "0x9123") {
		t.Errorf("the note does not say where it starts: %q", w.note)
	}
}

// Tab goes round the boxes that can be typed in, and Enter loads only when Load is on.
func TestLoadTabAndEnter(t *testing.T) {
	w, zx, r := binWindow(t, "128k", binTabLoad)
	binKey(w, r, zenui.KeyTab)
	if w.active() != 0 {
		t.Fatalf("Tab: box %d, want the file", w.active())
	}
	binKey(w, r, zenui.KeyTab)
	binKey(w, r, zenui.KeyTab)
	if w.active() != 0 {
		t.Errorf("with Start at off, Tab goes file, address, file: box %d", w.active())
	}
	binKey(w, r, zenui.KeyTab)
	w.startChk.Checked = true
	binFrame(w, r, zenui.Input{})
	binKey(w, r, zenui.KeyTab)
	if w.active() != 2 || w.inputs[2].text != w.start.TextField {
		t.Errorf("with Start at on, the third Tab is box %d", w.active())
	}
	// Enter in a box with no file does not load
	w.file.Text = ""
	if got := binKey(w, r, zenui.KeyEnter); got != binaryOpen || w.said != "" {
		t.Errorf("Enter with no file: %v %q", got, w.said)
	}
	// and with one it does
	w.file.Text = binFile(t, 7)
	w.startChk.Checked = false
	w.start.Blur()
	binClick(w, r, w.fileA)
	if got := binKey(w, r, zenui.KeyEnter); got != binaryClosed || zx.memory.Read(0x8000) != 7 {
		t.Errorf("Enter with a file: %v, byte 0x%02X", got, zx.memory.Read(0x8000))
	}
}

// Escape leaves the box being edited, and only then closes the window.
func TestLoadEscape(t *testing.T) {
	w, _, r := binWindow(t, "128k", binTabLoad)
	binClick(w, r, w.fileA)
	if binKey(w, r, zenui.KeyEscape) != binaryOpen || w.file.Active() {
		t.Error("Escape in the file box closed the window, or not the box")
	}
	if binKey(w, r, zenui.KeyEscape) != binaryClosed {
		t.Error("Escape with nothing being edited did not close the window")
	}
}

// With memory to choose from, the Memory list is on the tab, and it has the keyboard and the pointer
// while it is open.
func TestLoadMemoryListOnTheTab(t *testing.T) {
	w, _, r := binWindow(t, "next", binTabLoad)
	if !w.paged() {
		t.Fatal("the Next has pages")
	}
	binClick(w, r, w.loadMem.selRect)
	if !w.loadMem.dropOpen || len(w.loadMem.drop.Items) != 225 {
		t.Fatalf("open %v with %d entries", w.loadMem.dropOpen, len(w.loadMem.drop.Items))
	}
	if binKey(w, r, zenui.KeyEscape) != binaryOpen || w.loadMem.dropOpen {
		t.Error("Escape in the list closed the window, or not the list")
	}
	binClick(w, r, w.loadMem.selRect)
	binKey(w, r, zenui.KeyDown)
	binKey(w, r, zenui.KeyEnter)
	if w.loadMem.dropOpen || w.loadMem.page() != 0 {
		t.Errorf("Down, Enter: open %v, page %d, want page 0", w.loadMem.dropOpen, w.loadMem.page())
	}
	// a click on an entry that is over the Close button does not press it
	binClick(w, r, w.loadMem.selRect)
	item := w.loadMem.drop.ItemRect(2, w.loadMem.popRect, w.lineH)
	x, y := binCentre(item)
	if binFrame(w, r, binPress(x, y)) != binaryOpen || w.loadMem.page() != 1 {
		t.Errorf("a click on the second page: page %d", w.loadMem.page())
	}
}

// On a model that has no pages there is no list, and the rows close up.
func TestMemoryRowIsLeftOutWhenThereIsNoChoice(t *testing.T) {
	for _, c := range []struct {
		model string
		rows  int
	}{{"48k", 1}, {"ts2068", 1}, {"128k", 2}, {"next", 2}} {
		w, _, _ := binWindow(t, c.model, binTabLoad)
		rowH := w.lineH + w.lineH/2 + w.lineH/3
		if got := w.addrA.Y - w.fileA.Y; got != c.rows*rowH {
			t.Errorf("%s: the address is %d below the file, want %d rows of %d", c.model, got, c.rows, rowH)
		}
		w, _, _ = binWindow(t, c.model, binTabPeek)
		if got := w.pBytesA.Y - w.pAddrA.Y; got != rowH {
			t.Errorf("%s: the bytes are %d below the address on Peek and Poke, want %d", c.model, got, rowH)
		}
	}
}

// Poke writes bytes in a page and the dump shows them at once, with no button to press for it.
func TestPeekAndPoke(t *testing.T) {
	w, zx, r := binWindow(t, "128k", binTabPeek)
	pick(t, w.peekMem, 4)
	w.pAddr.Set(0x10)
	binEnter(w, r, w.pBytesA, "41424344")
	if !w.canPoke || button(t, w, binBtnPoke).Disabled {
		t.Fatalf("Poke is off: %q", w.note)
	}
	w.do(binBtnPoke)
	if w.refused {
		t.Fatalf("poke refused: %s", w.note)
	}
	if got, _ := zx.memory.PeekPage(4, 0x10, 4); string(got) != "ABCD" {
		t.Errorf("bank 4 holds %q, want ABCD", got)
	}
	binFrame(w, r, zenui.Input{})
	if len(w.dump) != w.rows {
		t.Fatalf("the dump has %d rows, want %d", len(w.dump), w.rows)
	}
	if !strings.HasPrefix(w.dump[0], "0010  41 42 43 44 ") || !strings.Contains(w.dump[0], " ABCD") {
		t.Errorf("the dump reads back as %q", w.dump[0])
	}
	if w.dumpBytes() != w.rows*w.per {
		t.Errorf("the dump is %d bytes, want %d rows of %d", w.dumpBytes(), w.rows, w.per)
	}
	if w.pBytes.Text != "" || !strings.Contains(w.note, "4 bytes written at 0x0010") || w.refused {
		t.Errorf("after the poke the box holds %q and the note says %q", w.pBytes.Text, w.note)
	}
	binFrame(w, r, zenui.Input{})
	if !strings.Contains(w.note, "4 bytes written") {
		t.Errorf("the result was forgotten with no change: %q", w.note)
	}
	if w.canPoke {
		t.Error("Poke is on with nothing to write")
	}
	// it goes when an input changes
	binEnter(w, r, w.pBytesA, "0")
	if strings.Contains(w.note, "written") {
		t.Errorf("a new byte was started and the note still says %q", w.note)
	}

	// the dump is live: the machine changes a byte and it shows without a button
	if err := zx.memory.PokePage(4, 0x10, []byte{0x5A}); err != nil {
		t.Fatal(err)
	}
	binFrame(w, r, zenui.Input{})
	if !strings.HasPrefix(w.dump[0], "0010  5A 42 43 44 ") {
		t.Errorf("the dump did not follow the machine: %q", w.dump[0])
	}

	// the CPU's memory, with bank 4 paged in at the top: the same bytes
	zx.memory.SetPaging(4)
	pick(t, w.peekMem, -1)
	w.pAddr.Set(0xC010)
	binFrame(w, r, zenui.Input{})
	if !strings.Contains(w.dump[0], "5A 42 43 44") || !strings.Contains(w.dumpAt, "CPU") {
		t.Errorf("the CPU's view of bank 4 reads %q (%s)", w.dump[0], w.dumpAt)
	}
}

// Only hex digits can be typed as bytes, in capitals, and Poke is on only for whole bytes that can
// be written.
func TestPokeIsOffUnlessItCanBeDone(t *testing.T) {
	cases := []struct {
		name    string
		page    int
		addr    int
		typed   string
		want    string
		wantBox string
		ok      bool
	}{
		{"nothing", -1, 0x8000, "", "Type the bytes", "", false},
		{"half a byte", -1, 0x8000, "4", "two digits", "4", false},
		{"a byte and a half", -1, 0x8000, "414", "One digit more", "414", false},
		{"not digits", -1, 0x8000, "zz4g1x", "41 would", "", false}, // filled in below
		{"rom", -1, 0x0100, "01", "ROM", "01", false},
		{"top of memory", -1, 0xFFFF, "0102", "64K", "0102", false},
		{"end of the last bank", 7, 0x3FFF, "0102", "last bank", "0102", false},
		{"good", -1, 0x8000, "ab", "1 bytes to write at 0x8000", "AB", true},
		{"good at the end of a bank", 1, 0x3FFE, "aabb", "2 bytes", "AABB", true},
	}
	for _, c := range cases {
		w, zx, r := binWindow(t, "128k", binTabPeek)
		pick(t, w.peekMem, c.page)
		w.pAddr.Set(c.addr)
		binEnter(w, r, w.pBytesA, c.typed)
		wantBox, want := c.wantBox, c.want
		if c.name == "not digits" {
			wantBox, want = "41", "One digit more" // z, g and x were not taken: 4 and 1 are one byte
			wantBox = "41"
			want = "1 bytes to write"
			c.ok = true
		}
		if w.pBytes.Text != wantBox {
			t.Errorf("%s: typed %q, the box holds %q, want %q", c.name, c.typed, w.pBytes.Text, wantBox)
		}
		if w.canPoke != c.ok || button(t, w, binBtnPoke).Disabled == c.ok {
			t.Errorf("%s: canPoke %v, button off %v, want it %v", c.name, w.canPoke, button(t, w, binBtnPoke).Disabled, c.ok)
		}
		if !strings.Contains(w.note, want) || w.refused == c.ok && c.name != "nothing" && c.name != "half a byte" && c.name != "a byte and a half" {
			t.Errorf("%s: note %q (refused %v), want %q", c.name, w.note, w.refused, want)
		}
		if !c.ok {
			rom := zx.memory.PeekMap(0x0100, 4)
			w.do(binBtnPoke)
			if string(zx.memory.PeekMap(0x0100, 4)) != string(rom) || w.result != "" {
				t.Errorf("%s: Poke did something while off", c.name)
			}
		}
	}
}

// The Bytes box takes no more than the most it can poke, and Enter in it pokes when it can.
func TestPokeBoxLimitsAndEnter(t *testing.T) {
	w, zx, r := binWindow(t, "48k", binTabPeek)
	binEnter(w, r, w.pBytesA, strings.Repeat("a", 2*binMaxBytes+10))
	if len(w.pBytes.Text) != 2*binMaxBytes {
		t.Errorf("the box holds %d digits, want %d", len(w.pBytes.Text), 2*binMaxBytes)
	}
	w.pBytes.Text = "CAFE"
	w.pAddr.Set(0x9000)
	binFrame(w, r, zenui.Input{})
	binKey(w, r, zenui.KeyEnter)
	if got := zx.memory.PeekMap(0x9000, 2); string(got) != "\xCA\xFE" {
		t.Errorf("Enter wrote %v", got)
	}
}

// Back and Forward step the dump by what it shows, across the end of a page, and are off at the ends.
func TestPeekStepsAcrossPages(t *testing.T) {
	w, _, r := binWindow(t, "next", binTabPeek)
	n := w.dumpBytes()
	pick(t, w.peekMem, 6)
	w.pAddr.Set(0x1FF8)
	binFrame(w, r, zenui.Input{})
	if button(t, w, binBtnNext).Disabled {
		t.Fatal("Forward is off in the middle")
	}
	w.do(binBtnNext)
	if w.peekMem.page() != 7 || w.pAddr.Value() != n-8 {
		t.Errorf("Forward from page 6, 0x1FF8 went to page %d, 0x%X; want page 7, 0x%X", w.peekMem.page(), w.pAddr.Value(), n-8)
	}
	w.do(binBtnBack)
	if w.peekMem.page() != 6 || w.pAddr.Value() != 0x1FF8 {
		t.Errorf("Back came to page %d, 0x%X; want page 6, 0x1FF8", w.peekMem.page(), w.pAddr.Value())
	}
	pick(t, w.peekMem, 223)
	w.pAddr.Set(0x1FFF)
	binFrame(w, r, zenui.Input{})
	if !button(t, w, binBtnNext).Disabled || w.canNext {
		t.Error("Forward is on at the last byte of the last page")
	}
	w.pAddr.Set(0x1FFF - n + 1)
	binFrame(w, r, zenui.Input{})
	if !w.canNext == false && w.pAddr.Value()+n < 0x2000 {
		t.Error("a dump that is the last of the memory still has Forward on")
	}
	pick(t, w.peekMem, -1)
	w.pAddr.Set(0)
	binFrame(w, r, zenui.Input{})
	if !button(t, w, binBtnBack).Disabled {
		t.Error("Back is on at 0")
	}
	w.pAddr.Set(0xFFFF - n + 1)
	binFrame(w, r, zenui.Input{})
	if !button(t, w, binBtnNext).Disabled {
		t.Error("Forward is on when the dump reaches the top of the CPU's memory")
	}
	if button(t, w, binBtnBack).Disabled {
		t.Error("Back is off in the middle of the CPU's memory")
	}
}

// On a model with one memory, Back and Forward move the address, and stop at the ends.
func TestPeekStepsOnA48K(t *testing.T) {
	w, _, r := binWindow(t, "48k", binTabPeek)
	n := w.dumpBytes()
	w.pAddr.Set(0x8000)
	binFrame(w, r, zenui.Input{})
	w.do(binBtnNext)
	if w.pAddr.Value() != 0x8000+n {
		t.Errorf("Forward: 0x%X, want 0x%X", w.pAddr.Value(), 0x8000+n)
	}
	w.do(binBtnBack)
	w.do(binBtnBack)
	w.do(binBtnBack)
	w.do(binBtnBack)
	w.do(binBtnBack)
	if w.pAddr.Value() < 0 || w.pAddr.Value() > 0x8000 {
		t.Errorf("Back: 0x%X", w.pAddr.Value())
	}
}

// The Hot Memory tab: typing a byte writes it as the second digit goes in, in a page and in the
// CPU's memory, and the cursor moves on.
func TestHotEditorTypesBytes(t *testing.T) {
	w, zx, r := binWindow(t, "128k", binTabHot)
	h := w.hot
	h.selectSpace(1) // bank 0
	h.goTo(0x1D00)
	binType(w, r, "4")
	if got := zx.memory.hotRead(h.space(), 0x1D00, 1)[0]; got&0xF0 != 0x40 || h.nib != 1 {
		t.Errorf("after one digit: 0x%02X, nib %d", got, h.nib)
	}
	binType(w, r, "2")
	if got := zx.memory.hotRead(h.space(), 0x1D00, 1)[0]; got != 0x42 {
		t.Errorf("after two digits: 0x%02X, want 0x42", got)
	}
	if h.pos != 0x1D01 || h.nib != 0 {
		t.Errorf("the cursor is at 0x%04X, nib %d: want 0x1D01, 0", h.pos, h.nib)
	}
	binType(w, r, "zq g") // not hex digits
	if h.pos != 0x1D01 || h.nib != 0 {
		t.Errorf("letters that are not digits moved the cursor: 0x%04X, nib %d", h.pos, h.nib)
	}
	binType(w, r, "fA")
	if got := zx.memory.hotRead(h.space(), 0x1D01, 1)[0]; got != 0xFA {
		t.Errorf("fA wrote 0x%02X", got)
	}

	// the CPU's memory: RAM is written, ROM is refused and says so
	h.selectSpace(0)
	h.goTo(0x9000)
	binType(w, r, "c3")
	if got := zx.memory.Read(0x9000); got != 0xC3 {
		t.Errorf("the CPU reads 0x%02X at 0x9000", got)
	}
	h.goTo(0x0100)
	rom := zx.memory.Read(0x0100)
	binType(w, r, "5")
	if zx.memory.Read(0x0100) != rom || !h.refused || !strings.Contains(h.note, "ROM") || h.nib != 0 {
		t.Errorf("a digit typed on ROM: byte 0x%02X (was 0x%02X), note %q, nib %d", zx.memory.Read(0x0100), rom, h.note, h.nib)
	}
	if w.note != h.note || !w.refused {
		t.Errorf("the window does not show the editor's note: %q", w.note)
	}
}

// In ASCII mode a character typed is the byte, and the cursor moves on; the keys that are not
// printable ASCII are not taken, and digits are characters, not nibbles.
func TestHotEditorTypesCharacters(t *testing.T) {
	w, zx, r := binWindow(t, "48k", binTabHot)
	h := w.hot
	h.goTo(0x9000)
	h.setMode(hotASCII)
	binType(w, r, "Hi 4~")
	for i, want := range []byte{'H', 'i', ' ', '4', '~'} {
		if got := zx.memory.Read(uint16(0x9000 + i)); got != want {
			t.Errorf("byte %d is 0x%02X, want %q", i, got, want)
		}
	}
	if h.pos != 0x9005 || h.nib != 0 {
		t.Errorf("the cursor is at 0x%04X, nib %d: want 0x9005, 0", h.pos, h.nib)
	}
	binFrame(w, r, zenui.Input{Chars: []rune{7, 0x7F, 200, 'é', 0x1F600}})
	if h.pos != 0x9005 || zx.memory.Read(0x9005) != 0 {
		t.Errorf("a key outside printable ASCII was taken: at 0x%04X, byte 0x%02X", h.pos, zx.memory.Read(0x9005))
	}
	// ROM is refused in this mode too, and says so
	h.goTo(0x0100)
	rom := zx.memory.Read(0x0100)
	binType(w, r, "A")
	if zx.memory.Read(0x0100) != rom || !h.refused || h.pos != 0x0100 {
		t.Errorf("a character typed on ROM: byte 0x%02X, refused %v, at 0x%04X", zx.memory.Read(0x0100), h.refused, h.pos)
	}
	// at the end of the space the cursor stays, and the byte is still written
	h.goTo(0xFFFF)
	binType(w, r, "Z")
	if zx.memory.Read(0xFFFF) != 'Z' || h.pos != 0xFFFF {
		t.Errorf("at the last byte: 0x%02X, cursor 0x%04X", zx.memory.Read(0xFFFF), h.pos)
	}
}

// Hex or ASCII is chosen with Tab, with the two buttons, and by clicking in the column to type in;
// the choice drops a half-typed byte.
func TestHotEditorSwitchesMode(t *testing.T) {
	w, zx, r := binWindow(t, "48k", binTabHot)
	h := w.hot
	h.goTo(0x9000)
	if h.mode != hotHex {
		t.Fatal("the editor begins in ASCII")
	}
	binType(w, r, "4") // half a byte
	binKey(w, r, zenui.KeyTab)
	if h.mode != hotASCII || h.nib != 0 {
		t.Errorf("Tab: mode %d, nib %d, want ASCII, 0", h.mode, h.nib)
	}
	binType(w, r, "A")
	if got := zx.memory.Read(0x9000); got != 'A' {
		t.Errorf("an A in ASCII mode: 0x%02X, want 0x41", got)
	}
	binKey(w, r, zenui.KeyTab)
	if h.mode != hotHex {
		t.Error("a second Tab did not come back to hex")
	}
	binClick(w, r, h.modeRect[1])
	if h.mode != hotASCII {
		t.Error("the ASCII button did not choose ASCII")
	}
	binClick(w, r, h.modeRect[1])
	if h.mode != hotASCII {
		t.Error("the ASCII button when it is on turned it off")
	}
	binClick(w, r, h.modeRect[0])
	if h.mode != hotHex {
		t.Error("the Hex button did not choose hex")
	}
	// a click in a column: the characters make it ASCII, the hex digits hex
	g := h.gridRect
	y := g.Y + hotPad + h.lineH/2
	binFrame(w, r, binPress(g.X+hotPad+(7+3*h.perRow+2)*h.charW+h.charW/2, y))
	if h.mode != hotASCII {
		t.Error("a click on a character did not choose ASCII")
	}
	binFrame(w, r, binPress(g.X+hotPad+(6+3)*h.charW+1, y))
	if h.mode != hotHex {
		t.Error("a click on a hex digit did not choose hex")
	}
	// Tab leaves the Go to box instead of switching, and goes where it says
	binClick(w, r, h.goRect)
	binType(w, r, "a123")
	binKey(w, r, zenui.KeyTab)
	if h.goBox.Active() || h.mode != hotHex || h.pos != 0xA123 {
		t.Errorf("Tab in the Go to box: active %v, mode %d, at 0x%04X", h.goBox.Active(), h.mode, h.pos)
	}
}

// The place where the next key goes flashes: lit for half a second, dark for half a second, lit
// again when something is typed or the cursor moves; and not at all on a byte that cannot be written.
func TestHotCaretFlashes(t *testing.T) {
	w, _, r := binWindow(t, "48k", binTabHot)
	h := w.hot
	h.goTo(0x9000)
	theme := zenui.LoadTheme(zenui.ThemeDark)
	blocks := func() int {
		r.fills = nil
		w.Draw(r, binScreenW, binScreenH, theme)
		n := 0
		for _, f := range r.fills {
			if f.colour == theme.SelFill && f.rect.W == h.charW && f.rect.H == h.lineH {
				n++
			}
		}
		return n
	}
	if !h.caretOn() || blocks() != 1 {
		t.Errorf("a cursor just moved: lit %v, %d blocks, want lit, 1", h.caretOn(), blocks())
	}
	binFrame(w, r, zenui.Input{DeltaTime: hotBlink})
	if h.caretOn() || blocks() != 0 {
		t.Errorf("after %vs: lit %v, %d blocks, want dark, 0", hotBlink, h.caretOn(), blocks())
	}
	binFrame(w, r, zenui.Input{DeltaTime: hotBlink})
	if !h.caretOn() || blocks() != 1 {
		t.Errorf("after %vs: lit %v, %d blocks, want lit again", 2*hotBlink, h.caretOn(), blocks())
	}
	binFrame(w, r, zenui.Input{DeltaTime: hotBlink})
	binType(w, r, "1")
	if !h.caretOn() || h.idle != 0 {
		t.Errorf("a digit typed in the dark phase: lit %v, idle %v", h.caretOn(), h.idle)
	}
	binFrame(w, r, zenui.Input{DeltaTime: hotBlink})
	binKey(w, r, zenui.KeyRight)
	if !h.caretOn() {
		t.Error("moving the cursor did not light it")
	}
	// the block is on the digit that is next: the second after the first is typed
	h.goTo(0x9000)
	hexBlock := func() int {
		r.fills = nil
		w.Draw(r, binScreenW, binScreenH, theme)
		for _, f := range r.fills {
			if f.colour == theme.SelFill && f.rect.W == h.charW && f.rect.H == h.lineH {
				return f.rect.X
			}
		}
		return -1
	}
	first := hexBlock()
	binType(w, r, "7")
	if second := hexBlock(); second != first+h.charW || first < 0 {
		t.Errorf("the block is at x=%d after one digit, want %d", second, first+h.charW)
	}
	// in ASCII mode it is on the character
	h.setMode(hotASCII)
	if x := hexBlock(); x < h.gridRect.X+(7+3*h.perRow)*h.charW {
		t.Errorf("in ASCII mode the block is at x=%d, not in the characters", x)
	}
	// ROM: outlined, never lit
	h.goTo(0x0100)
	if blocks() != 0 {
		t.Error("a block is flashing on a byte of ROM")
	}
}

// Keys move the cursor and the dump follows it; the wheel scrolls; a click chooses a byte.
func TestHotEditorMoves(t *testing.T) {
	w, _, r := binWindow(t, "48k", binTabHot)
	h := w.hot
	h.goTo(0x8000)
	per := h.perRow
	for _, c := range []struct {
		k    zenui.Key
		want int
	}{
		{zenui.KeyRight, 0x8001}, {zenui.KeyDown, 0x8001 + per}, {zenui.KeyLeft, 0x8000 + per},
		{zenui.KeyUp, 0x8000}, {zenui.KeyEnd, 0x8000 + per - 1}, {zenui.KeyHome, 0x8000},
		{zenui.KeyPageDown, 0x8000 + h.rows*per}, {zenui.KeyPageUp, 0x8000},
	} {
		binKey(w, r, c.k)
		if h.pos != c.want {
			t.Errorf("%v: the cursor is at 0x%04X, want 0x%04X", c.k, h.pos, c.want)
		}
	}
	// it stops at the ends of the space
	h.goTo(0)
	binKey(w, r, zenui.KeyLeft)
	binKey(w, r, zenui.KeyUp)
	if h.pos != 0 {
		t.Errorf("went below 0: 0x%04X", h.pos)
	}
	h.goTo(0xFFFF)
	binKey(w, r, zenui.KeyRight)
	binKey(w, r, zenui.KeyDown)
	if h.pos != 0xFFFF {
		t.Errorf("went past 0xFFFF: 0x%04X", h.pos)
	}
	// the cursor's row is always in the dump
	for _, p := range []int{0, 0x1234, 0x8000, 0xFFFF, 0x0100} {
		h.goTo(p)
		binFrame(w, r, zenui.Input{})
		if p < h.top || p >= h.top+h.rows*h.perRow {
			t.Errorf("0x%04X is not in the dump that starts at 0x%04X", p, h.top)
		}
	}
	// a click on a byte of the hex columns, and on a character
	h.goTo(0x8000)
	binFrame(w, r, zenui.Input{})
	g := h.gridRect
	x := g.X + hotPad + (6+3*2)*h.charW + h.charW/2 // third byte of the first row
	y := g.Y + hotPad + 2*h.lineH + h.lineH/2       // third row
	binFrame(w, r, binPress(x, y))
	if want := h.top + 2*h.perRow + 2; h.pos != want || h.mode != hotHex {
		t.Errorf("a click on the third byte of the third row put the cursor at 0x%04X (mode %d), want 0x%04X, hex", h.pos, h.mode, want)
	}
	x = g.X + hotPad + (7+3*h.perRow+5)*h.charW + h.charW/2 // sixth character
	binFrame(w, r, binPress(x, y))
	if want := h.top + 2*h.perRow + 5; h.pos != want || h.mode != hotASCII {
		t.Errorf("a click on the sixth character put the cursor at 0x%04X (mode %d), want 0x%04X, ASCII", h.pos, h.mode, want)
	}
	// the wheel scrolls three rows
	h.setTop(10)
	top := h.top
	binFrame(w, r, zenui.Input{WheelY: -1, MouseX: g.X + 5, MouseY: g.Y + 5})
	if h.top != top+3*h.perRow {
		t.Errorf("the wheel moved the dump from 0x%04X to 0x%04X, want 3 rows", top, h.top)
	}
}

// Go to is a box of hex digits that cannot pass the end of the memory: there is no address to refuse.
func TestHotEditorGoTo(t *testing.T) {
	w, _, r := binWindow(t, "128k", binTabHot)
	h := w.hot
	binClick(w, r, h.goRect)
	if !h.goBox.Active() {
		t.Fatal("a click on the Go to box did not start an edit")
	}
	binType(w, r, "c123")
	binKey(w, r, zenui.KeyEnter)
	if h.pos != 0xC123 || h.refused || h.goBox.Text != "C123" {
		t.Errorf("Go to c123: 0x%04X, box %q, %q", h.pos, h.goBox.Text, h.note)
	}
	h.selectSpace(3) // bank 2: 16K
	binClick(w, r, h.goRect)
	binType(w, r, "ffff")
	if h.goBox.Text != "3FFF" {
		t.Errorf("ffff in a 16K bank: the box holds %q, want 3FFF", h.goBox.Text)
	}
	binKey(w, r, zenui.KeyEnter)
	if h.pos != 0x3FFF || h.refused {
		t.Errorf("Go to 3FFF in a bank: %q, 0x%04X", h.note, h.pos)
	}
	binClick(w, r, h.goRect)
	binType(w, r, "pear") // only e and a are digits
	if h.goBox.Text != "EA" {
		t.Errorf("pear: the box holds %q, want EA", h.goBox.Text)
	}
	binKey(w, r, zenui.KeyEscape)
	if h.goBox.Active() || h.goBox.Text != "3FFF" || h.pos != 0x3FFF {
		t.Errorf("Escape: active %v, box %q, at 0x%04X: want the old address back", h.goBox.Active(), h.goBox.Text, h.pos)
	}
}

// The drop-down lists the CPU's space and the pages, and choosing one shows it.
func TestHotEditorDropDown(t *testing.T) {
	for _, c := range pagingByModel {
		w, _, r := binWindow(t, c.model, binTabHot)
		h := w.hot
		binClick(w, r, h.selRect)
		if !h.dropOpen || len(h.drop.Items) != 1+c.count {
			t.Fatalf("%s: open %v with %d entries, want %d", c.model, h.dropOpen, len(h.drop.Items), 1+c.count)
		}
		if h.drop.Selected != 0 {
			t.Errorf("%s: %d is chosen, want the CPU's", c.model, h.drop.Selected)
		}
		if !strings.Contains(h.drop.Items[0].Name, "CPU") {
			t.Errorf("%s: the first entry is %q", c.model, h.drop.Items[0].Name)
		}
		if c.kind == pagesNone {
			// nothing but the CPU's: choosing it is fine, and Escape closes the list only
			if binKey(w, r, zenui.KeyEscape) != binaryOpen || h.dropOpen {
				t.Errorf("%s: Escape did not just close the list", c.model)
			}
			continue
		}
		// a click on an entry: the second
		item := h.drop.ItemRect(1, h.popRect, h.uiLineH)
		x, y := binCentre(item)
		binFrame(w, r, binPress(x, y))
		if h.dropOpen || h.cur != 1 {
			t.Errorf("%s: after a click on the second entry: open %v, space %d", c.model, h.dropOpen, h.cur)
		}
		if h.space().Page != 0 || h.pos != 0 {
			t.Errorf("%s: page %d at 0x%X, want page 0 at 0", c.model, h.space().Page, h.pos)
		}
		// by keys
		binClick(w, r, h.selRect)
		binKey(w, r, zenui.KeyDown)
		binKey(w, r, zenui.KeyEnter)
		if h.dropOpen || h.cur != 2 {
			t.Errorf("%s: Down, Enter chose space %d (open %v), want 2", c.model, h.cur, h.dropOpen)
		}
		// a click away closes it and changes nothing
		binClick(w, r, h.selRect)
		binFrame(w, r, binPress(w.panel.X+2, w.panel.Y+w.panel.H-2))
		if h.dropOpen || h.cur != 2 {
			t.Errorf("%s: a click away: open %v, space %d", c.model, h.dropOpen, h.cur)
		}
	}
}

// Escape closes the list, then leaves the Go to box, and only then closes the window; Tab does not
// move to the box, it chooses hex or ASCII.
func TestHotEditorEscapeAndTab(t *testing.T) {
	w, _, r := binWindow(t, "next", binTabHot)
	h := w.hot
	binClick(w, r, h.selRect)
	if binKey(w, r, zenui.KeyEscape) != binaryOpen || h.dropOpen {
		t.Error("Escape in the list closed the window, or not the list")
	}
	binKey(w, r, zenui.KeyTab)
	if h.goBox.Active() || h.mode != hotASCII {
		t.Errorf("Tab: Go to box active %v, mode %d: want it to choose ASCII", h.goBox.Active(), h.mode)
	}
	binClick(w, r, h.goRect)
	if binKey(w, r, zenui.KeyEscape) != binaryOpen || h.goBox.Active() {
		t.Error("Escape in the Go to box closed the window, or not the box")
	}
	if binKey(w, r, zenui.KeyEscape) != binaryClosed {
		t.Error("Escape in the dump did not close the window")
	}
}

// What the running machine does to memory shows, and lights what changed; and the editor shows
// the paging as it is now.
func TestHotEditorIsLive(t *testing.T) {
	w, zx, r := binWindow(t, "128k", binTabHot)
	h := w.hot
	h.goTo(0x8000)
	binFrame(w, r, zenui.Input{})
	if err := zx.memory.hotWrite(hotCPU, 0x8003, 0x99); err != nil {
		t.Fatal(err)
	}
	binFrame(w, r, zenui.Input{})
	i := 0x8003 - h.top
	if h.prev[i] != 0x99 || h.age[i] != hotFlash {
		t.Errorf("byte 0x8003 reads 0x%02X lit %d, want 0x99 lit %d", h.prev[i], h.age[i], hotFlash)
	}
	if h.age[i-1] != 0 || h.age[i+1] != 0 {
		t.Error("a byte beside it is lit too")
	}
	for n := 0; n < hotFlash; n++ {
		binFrame(w, r, zenui.Input{})
	}
	if h.age[i] != 0 {
		t.Errorf("the light has not gone out after %d frames: %d", hotFlash, h.age[i])
	}
	// the program pages another bank in at the top
	h.goTo(0xC000)
	zx.memory.SetPaging(0)
	zx.memory.hotWrite(hotSpace{Page: 0}, 0, 0x11)
	binFrame(w, r, zenui.Input{})
	if top := 0xC000 - h.top; h.prev[top] != 0x11 {
		t.Errorf("the top of memory reads 0x%02X with bank 0 paged in", h.prev[0xC000-h.top])
	}
	zx.memory.SetPaging(1)
	zx.memory.hotWrite(hotSpace{Page: 1}, 0, 0x22)
	binFrame(w, r, zenui.Input{})
	if h.prev[0xC000-h.top] != 0x22 {
		t.Errorf("the top of memory reads 0x%02X with bank 1 paged in", h.prev[0xC000-h.top])
	}
	// a new space starts without lights
	h.selectSpace(2)
	binFrame(w, r, zenui.Input{})
	for _, a := range h.age {
		if a != 0 {
			t.Fatal("a byte is lit after choosing a space")
		}
	}
}

// The dumps are drawn in the monospaced renderer, and the rest in the program's own.
func TestHexIsDrawnInTheMonospacedFace(t *testing.T) {
	theme := zenui.LoadTheme(zenui.ThemeDark)
	for _, tab := range []int{binTabPeek, binTabHot} {
		w, _, ui := binWindow(t, "128k", tab)
		mono := &paintRenderer{}
		w.mono = mono
		if tab == binTabPeek {
			w.pAddr.Set(0x4000)
		} else {
			w.hot.goTo(0x8000)
		}
		binFrame(w, ui, zenui.Input{})
		ui.texts, mono.texts = nil, nil
		w.Draw(ui, binScreenW, binScreenH, theme)
		joined := strings.Join(mono.texts, "|")
		if tab == binTabHot {
			if !strings.Contains(joined, "8000") || !strings.Contains(joined, "00") {
				t.Errorf("Hot Memory: the mono renderer drew %q", joined)
			}
		} else if !strings.Contains(joined, "4000") || !strings.Contains(joined, "00 00") {
			t.Errorf("Peek: the mono renderer drew %q", joined)
		}
		for _, s := range ui.texts {
			if s == "00" || strings.Contains(s, "00 00") {
				t.Errorf("tab %d: the program's renderer drew a dump cell %q", tab, s)
			}
		}
	}
}

// Whatever the screen, the window is on it and its parts are in it, on each tab, with the list
// open, and it does not change size between tabs.
func TestBinaryWindowFits(t *testing.T) {
	for _, model := range []string{"48k", "128k", "next"} {
		for _, sz := range [][2]int{{1280, 720}, {1000, 700}, {800, 600}, {640, 480}, {480, 320}} {
			var panel zenui.Rect
			for tab := binTabLoad; tab <= binTabHot; tab++ {
				w := newBinaryWindow(machineOf(t, model), tab)
				r := &paintRenderer{}
				w.Update(zenui.Input{}, r, sz[0], sz[1])
				if tab == binTabLoad {
					panel = w.panel
				} else if w.panel != panel {
					t.Errorf("%s %v: tab %d is %+v, the first was %+v", model, sz, tab, w.panel, panel)
				}
				p := w.panel
				if p.X < 0 || p.Y < 0 || p.X+p.W > sz[0] || p.Y+p.H > sz[1] {
					t.Errorf("%s %v tab %d: the window %+v is off the screen", model, sz, tab, p)
				}
				in := func(name string, r zenui.Rect) {
					if r.X < p.X || r.Y < p.Y || r.X+r.W > p.X+p.W || r.Y+r.H > p.Y+p.H {
						t.Errorf("%s %v tab %d: %s %+v is outside the window %+v", model, sz, tab, name, r, p)
					}
				}
				in("tabs", w.tabsArea)
				in("body", w.bodyArea)
				in("buttons", w.barArea)
				switch tab {
				case binTabLoad:
					in("file", w.fileA)
					in("address", w.addrA)
					in("start", w.startA)
					in("check", w.chkA)
				case binTabPeek:
					in("address", w.pAddrA)
					in("bytes", w.pBytesA)
				case binTabHot:
					in("grid", w.hot.gridRect)
					in("selector", w.hot.selRect)
					in("go to", w.hot.goRect)
					in("hex button", w.hot.modeRect[0])
					in("ascii button", w.hot.modeRect[1])
					x, y := binCentre(w.hot.selRect)
					w.Update(binPress(x, y), r, sz[0], sz[1])
					in("drop-down", w.hot.popRect)
				}
				if tab != binTabHot && w.paged() {
					pk := w.picker()
					in("memory", pk.selRect)
					x, y := binCentre(pk.selRect)
					w.Update(binPress(x, y), r, sz[0], sz[1])
					in("memory list", pk.popRect)
				}
			}
		}
	}
}

// With a wide enough window the dump is 16 bytes a row, else 8, else 4.
func TestHotRowWidth(t *testing.T) {
	if hotPerRow(10, 71*10+2*hotPad) != 16 || hotPerRow(10, 71*10+2*hotPad-1) != 8 ||
		hotPerRow(10, 39*10+2*hotPad) != 8 || hotPerRow(10, 39*10+2*hotPad-1) != 4 || hotPerRow(10, 5) != 4 {
		t.Error("the row width does not follow the room")
	}
	w, _, _ := binWindow(t, "48k", binTabHot)
	if w.per != 16 || w.hot.perRow != 16 {
		t.Errorf("at 1000 pixels the dumps are %d and %d bytes a row, want 16", w.per, w.hot.perRow)
	}
}

// The pointer is a hand over the tabs, as it is over the tabs of Preferences, and an arrow elsewhere.
func TestBinaryTabsWantTheHandCursor(t *testing.T) {
	w, _, r := binWindow(t, "128k", binTabLoad)
	tabs := w.tabs.Layout(w.tabsArea, w.lineH)
	if len(tabs) != 3 {
		t.Fatalf("%d tab rectangles", len(tabs))
	}
	for i, tr := range tabs {
		x, y := binCentre(tr)
		binFrame(w, r, zenui.Input{MouseX: x, MouseY: y})
		if !w.handWanted {
			t.Errorf("over tab %d the pointer is not a hand", i)
		}
	}
	binFrame(w, r, zenui.Input{MouseX: w.bodyArea.X + 2, MouseY: w.bodyArea.Y + w.bodyArea.H - 2})
	if w.handWanted {
		t.Error("over the body the pointer is a hand")
	}
	binFrame(w, r, zenui.Input{MouseX: -5, MouseY: -5})
	if w.handWanted {
		t.Error("off the window the pointer is a hand")
	}
}
