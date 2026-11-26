//go:build !headless

package zenzx

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenui/zenuiraylib"
	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// ============================================================================
// The SD Cards window
//
// A Next that boots NextZXOS has two card slots: the card it boots from in
// the first, any card in the second. With several machines in one window the
// rule is that a card image is in one slot of one machine: two machines
// writing one image would destroy it. This window is where that is seen and
// arranged. It lists the card images in the Next's folder with what each is
// and who has it, and puts the chosen one in a slot, copies it, or makes a
// new one.
//
// It also opens by itself when a machine is to become a Next and the card
// the Next boots from is in another machine: then it asks for a card of this
// machine's own, which is usually a copy of that one.
// ============================================================================

// cardUse is where a card image is: machine 0 is nowhere, and machines count
// from 1 as the dock numbers them; slot is 1 or 2.
type cardUse struct{ machine, slot int }

// cardEntry is one card image the window lists.
type cardEntry struct {
	path   string
	size   int64
	system bool   // a Next can boot from it
	why    string // why not, when it cannot
}

// describeCard reads what the window needs to know of a card image.
func describeCard(path string) cardEntry {
	e := cardEntry{path: path}
	f, err := os.Open(path)
	if err != nil {
		e.why = err.Error()
		return e
	}
	defer f.Close()
	if st, err := f.Stat(); err == nil {
		e.size = st.Size()
	}
	if err := nextcard.VerifyCard(f); err != nil {
		e.why = err.Error()
	} else {
		e.system = true
	}
	return e
}

// listCards returns the card images in dir, and those of extra that are not
// in it, in the order of their names.
func listCards(dir string, extra []string) []cardEntry {
	seen := map[string]bool{}
	var paths []string
	add := func(p string) {
		abs, err := filepath.Abs(p)
		if err != nil || seen[abs] {
			return
		}
		seen[abs] = true
		paths = append(paths, p)
	}
	if names, err := filepath.Glob(filepath.Join(dir, "*.img")); err == nil {
		sort.Strings(names)
		for _, p := range names {
			add(p)
		}
	}
	for _, p := range extra {
		if _, err := os.Stat(p); err == nil {
			add(p)
		}
	}
	cards := make([]cardEntry, 0, len(paths))
	for _, p := range paths {
		cards = append(cards, describeCard(p))
	}
	return cards
}

// freeCardName returns a file name in dir that is not taken: stem.img, or
// stem-2.img, stem-3.img and so on.
func freeCardName(dir, stem string) string { return freeImageName(dir, stem, ".img") }

// freeImageName is freeCardName for any extension; the disc window names
// its files with it too.
func freeImageName(dir, stem, ext string) string {
	p := filepath.Join(dir, stem+ext)
	for n := 2; ; n++ {
		if _, err := os.Stat(p); err != nil {
			return p
		}
		p = filepath.Join(dir, fmt.Sprintf("%s-%d%s", stem, n, ext))
	}
}

// copyStem is the stem a copy of the card at path is named from: the card's
// own, without a number a former copy gave it.
func copyStem(path string) string {
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if i := strings.LastIndex(stem, "-"); i > 0 {
		digits := stem[i+1:]
		if digits != "" && strings.Trim(digits, "0123456789") == "" {
			stem = stem[:i]
		}
	}
	return stem
}

// copyCard copies the card image src to dst, which must not exist, telling
// progress how far it is. A copy that fails or is cancelled leaves no file.
func copyCard(ctx context.Context, src, dst string, progress func(done, total int64)) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	st, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			os.Remove(dst)
		}
	}()
	buf := make([]byte, 1<<20)
	var done int64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, rerr := in.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return werr
			}
			done += int64(n)
			progress(done, st.Size())
		}
		if rerr == io.EOF {
			return nil
		}
		if rerr != nil {
			return rerr
		}
	}
}

// cardsResult is what a frame of the window ended in.
type cardsResult int

const (
	cardsOpen   cardsResult = iota // still on screen
	cardsClosed                    // closed
	cardsBoot                      // closed: the machine is to boot from bootCard
)

// cardsWindow is the SD Cards window, open for one machine.
type cardsWindow struct {
	zx      *ZenZX
	machine int  // its number in the dock
	must    bool // the machine is to become a Next and needs a card to boot from
	dir     string
	// taken is the card the machine would have booted from, in must mode.
	taken    string
	readOnly bool
	use      func(path string) cardUse

	cards    []cardEntry
	extra    []string
	list     *zenui.List
	buttons  *zenui.ButtonBar
	browse   *zenui.Dialog
	ask      *cardQuestion // Rename or Delete, asking: sdcards_manage_gui.go
	note     string
	bootCard string

	mu      sync.Mutex
	copying bool
	done    int64
	total   int64
	copyErr error
	copyDst string
	copied  bool
	cancel  context.CancelFunc
	// making says the work in hand is making copyDst bootable, not copying
	// a card to it: the same progress bar and the same Stop serve both.
	making bool

	theme                              zenui.Theme // as last drawn with, for the cards' pictures
	panel, listArea, barArea, progress zenui.Rect
	head                               []string
	scale, lineH                       int
}

// newCardsWindow opens the window for machine number machine. dir is the
// folder the Next's files are in; use tells where a card image is among the
// window's machines.
func newCardsWindow(zx *ZenZX, machine int, dir string, readOnly bool, use func(string) cardUse) *cardsWindow {
	w := &cardsWindow{zx: zx, machine: machine, dir: dir, readOnly: readOnly, use: use,
		list: zenui.NewList(), buttons: zenui.NewButtonBar()}
	w.list.NoToolbar = true
	if zx.next != nil {
		w.extra = append(w.extra, zx.next.files.Card)
	}
	if second := zx.SecondCard(); second != "" {
		w.extra = append(w.extra, second)
	}
	w.refresh()
	return w
}

// refresh reads the folder again and keeps the selection on the same card.
func (w *cardsWindow) refresh() {
	keep := ""
	if w.list.Selected >= 0 && w.list.Selected < len(w.cards) {
		keep = w.cards[w.list.Selected].path
	}
	w.cards = listCards(w.dir, w.extra)
	w.list.Selected = -1
	for i, c := range w.cards {
		if c.path == keep {
			w.list.Selected = i
		}
	}
	if w.list.Selected < 0 && len(w.cards) > 0 {
		w.list.Selected = 0
	}
}

// selectPath puts the selection on the card at path.
func (w *cardsWindow) selectPath(path string) {
	for i, c := range w.cards {
		if same(c.path, path) {
			w.list.Selected = i
		}
	}
}

// where says who has a card, in the window's words.
func (w *cardsWindow) where(u cardUse) string {
	switch {
	case u.machine == 0:
		return "free"
	case u.machine == w.machine:
		return fmt.Sprintf("slot %d here", u.slot)
	}
	return fmt.Sprintf("machine %d, slot %d", u.machine, u.slot)
}

// row is a card's line in the list.
func (w *cardsWindow) row(c cardEntry) string {
	kind := "data"
	if c.system {
		kind = "boots"
	}
	name := filepath.Base(c.path)
	if len(name) > 26 {
		name = name[:25] + "~"
	}
	return fmt.Sprintf("%-26s %5d MB  %-5s  %s", name, c.size>>20, kind, w.where(w.use(c.path)))
}

// cardPicture draws a small SD card beside a row: filled for a card a Next
// boots from, in outline for any other.
func (w *cardsWindow) cardPicture(system bool) func(r zenui.Renderer, area zenui.Rect) {
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
		if system {
			r.FillRect(body, w.theme.Text)
			r.FillRect(top, w.theme.Text)
			return
		}
		r.StrokeRect(body, w.theme.DimText, 1)
		r.StrokeRect(top, w.theme.DimText, 1)
	}
}

// selected returns the chosen card, if there is one.
func (w *cardsWindow) selected() (cardEntry, bool) {
	if w.list.Selected < 0 || w.list.Selected >= len(w.cards) {
		return cardEntry{}, false
	}
	return w.cards[w.list.Selected], true
}

// The window's buttons.
const (
	cardBtnBoot      = "boot"
	cardBtnSlot2     = "slot2"
	cardBtnRemove    = "remove"
	cardBtnCopy      = "copy"
	cardBtnRename    = "rename"
	cardBtnDelete    = "delete"
	cardBtnBootable  = "bootable"
	cardBtnNewSmall  = "new-small"
	cardBtnNewLarge  = "new-large"
	cardBtnOther     = "other"
	cardBtnClose     = "close"
	cardBtnStopCopy  = "stop"
	cardsWindowLines = 5 // rows of the list on show
)

// setButtons makes the row of buttons for the present state. What a button
// may do is decided here and nowhere else: do acts only on enabled buttons.
func (w *cardsWindow) setButtons() {
	w.mu.Lock()
	copying := w.copying
	w.mu.Unlock()
	if copying {
		label := "Stop copying"
		if w.making {
			label = "Stop"
		}
		w.buttons.Buttons = []zenui.BarButton{{ID: cardBtnStopCopy, Label: label}}
		return
	}
	c, ok := w.selected()
	u := cardUse{}
	if ok {
		u = w.use(c.path)
	}
	free := ok && u.machine == 0
	mine := ok && u.machine == w.machine
	running := usesSDCards(w.zx)
	boot := "Boot from it"
	if running {
		boot = "Restart from it"
	}
	w.buttons.Buttons = []zenui.BarButton{
		{ID: cardBtnBoot, Label: boot, Disabled: !(ok && c.system && (free || (mine && u.slot == 2)))},
		{ID: cardBtnSlot2, Label: "Put in slot 2", Disabled: !(running && free)},
		{ID: cardBtnRemove, Label: "Take out of slot 2", Disabled: !(mine && u.slot == 2)},
		{ID: cardBtnCopy, Label: "Copy", Disabled: !ok},
		{ID: cardBtnRename, Label: "Rename", Disabled: !(free && !w.isStandard(c.path))},
		{ID: cardBtnDelete, Label: "Delete", Disabled: !free},
		{ID: cardBtnBootable, Label: "Make bootable", Disabled: !(free && !c.system && w.systemSource() != "")},
		{ID: cardBtnNewSmall, Label: "New 280 MB"},
		{ID: cardBtnNewLarge, Label: "New 1 GB"},
		{ID: cardBtnOther, Label: "Other file..."},
		{ID: cardBtnClose, Label: "Close"},
	}
}

// do carries out a button.
func (w *cardsWindow) do(id string) cardsResult {
	for _, b := range w.buttons.Buttons {
		if b.ID == id && b.Disabled {
			return cardsOpen
		}
	}
	c, _ := w.selected()
	switch id {
	case cardBtnClose:
		return cardsClosed
	case cardBtnBoot:
		if u := w.use(c.path); u.machine == w.machine && u.slot == 2 {
			w.zx.RemoveSecondCard()
		}
		w.bootCard = c.path
		return cardsBoot
	case cardBtnSlot2:
		protected, err := w.zx.InsertSecondCard(c.path, w.readOnly)
		if err != nil {
			w.note = "That card could not be used: " + err.Error()
		} else {
			w.note = fmt.Sprintf("%s is in slot 2 (write-protected: %v). %s", filepath.Base(c.path), protected, remountHint)
		}
	case cardBtnRemove:
		w.zx.RemoveSecondCard()
		w.note = "Slot 2 is empty. " + remountHint
	case cardBtnCopy:
		w.startCopy(c)
	case cardBtnRename:
		w.startRename(c)
	case cardBtnDelete:
		w.startDelete(c)
	case cardBtnBootable:
		w.startBootable(c)
	case cardBtnNewSmall, cardBtnNewLarge:
		size := int64(nextcard.SmallestCard)
		if id == cardBtnNewLarge {
			size = 1 << 30
		}
		path, err := newEmptyCard(freeCardName(w.dir, "card"), size)
		if err != nil {
			w.note = "The card could not be made: " + err.Error()
		} else {
			w.note = fmt.Sprintf("Made %s, empty, %d MB.", filepath.Base(path), size>>20)
			w.refresh()
			w.selectPath(path)
		}
	case cardBtnOther:
		w.browse = zenui.NewDialog(zenui.DialogConfig{
			Mode: zenui.ModeOpen, Title: "Another SD Card Image", StartDir: w.dir,
			Filters: []string{"img"}, FS: zenui.OSFS{},
		})
	case cardBtnStopCopy:
		if w.cancel != nil {
			w.cancel()
		}
	}
	return cardsOpen
}

// startCopy copies card c to a new file beside the others, in the
// background; the window shows how far it is.
func (w *cardsWindow) startCopy(c cardEntry) {
	dst := freeCardName(w.dir, copyStem(c.path))
	ctx, cancel := context.WithCancel(context.Background())
	w.mu.Lock()
	w.copying, w.done, w.total, w.copyErr, w.copyDst, w.copied = true, 0, c.size, nil, dst, false
	w.mu.Unlock()
	w.cancel = cancel
	w.note = fmt.Sprintf("Copying %s to %s.", filepath.Base(c.path), filepath.Base(dst))
	go func() {
		err := copyCard(ctx, c.path, dst, func(done, total int64) {
			w.mu.Lock()
			w.done, w.total = done, total
			w.mu.Unlock()
		})
		if err == nil {
			// the copy is a card of its own: a snapshot made over the original is not made over it
			// (a file that has no partition table to put a signature in is left as it is)
			_ = nextcard.Resign(dst)
		}
		w.mu.Lock()
		w.copying, w.copyErr, w.copied = false, err, true
		w.mu.Unlock()
	}()
}

// systemSource is the card the boot files are copied from to make another
// bootable: the standard card in the window's folder, when it is there and
// is itself one a Next boots from. It is "" when there is none.
func (w *cardsWindow) systemSource() string {
	std := nextcard.In(w.dir).Card
	for _, c := range w.cards {
		if c.system && sameCardFile(c.path, std) {
			return c.path
		}
	}
	return ""
}

// sameCardFile reports whether two names are the same file.
func sameCardFile(a, b string) bool {
	sa, err := os.Stat(a)
	if err != nil {
		return false
	}
	sb, err := os.Stat(b)
	return err == nil && os.SameFile(sa, sb)
}

// startBootable makes card c one a Next can boot from, by copying the
// boot files onto it from the standard card, in the background. What is on
// the card is kept.
func (w *cardsWindow) startBootable(c cardEntry) {
	source := w.systemSource()
	ctx, cancel := context.WithCancel(context.Background())
	w.mu.Lock()
	w.copying, w.done, w.total, w.copyErr, w.copyDst, w.copied = true, 0, c.size, nil, c.path, false
	w.mu.Unlock()
	w.making = true
	w.cancel = cancel
	w.note = fmt.Sprintf("Making %s bootable.", filepath.Base(c.path))
	go func() {
		_, err := nextcard.MakeBootable(ctx, source, c.path, func(done, total int64) {
			w.mu.Lock()
			w.done, w.total = done, total
			w.mu.Unlock()
		})
		w.mu.Lock()
		w.copying, w.copyErr, w.copied = false, err, true
		w.mu.Unlock()
	}()
}

// finishCopy takes note of a copy, or of making a card bootable, that has
// ended.
func (w *cardsWindow) finishCopy() {
	w.mu.Lock()
	copied, err, dst := w.copied, w.copyErr, w.copyDst
	w.copied = false
	w.mu.Unlock()
	if !copied {
		return
	}
	w.cancel = nil
	if w.making {
		w.making = false
		switch {
		case err == context.Canceled:
			w.note = "Stopped; the card is as it was."
		case err != nil:
			w.note = "The card could not be made bootable: " + err.Error()
		default:
			w.note = fmt.Sprintf("%s boots a Next now: the %d files NextZXOS needs to start were put on it. What it held is still there.", filepath.Base(dst), len(nextcard.BootFiles))
		}
		w.refresh()
		w.selectPath(dst)
		return
	}
	switch {
	case err == context.Canceled:
		w.note = "The copy was stopped; nothing was kept."
	case err != nil:
		w.note = "The copy failed: " + err.Error()
	default:
		w.note = fmt.Sprintf("%s is a copy, free to use.", filepath.Base(dst))
		w.refresh()
		w.selectPath(dst)
	}
}

// headLines is the text above the list.
func (w *cardsWindow) headLines() []string {
	if w.must {
		return []string{
			fmt.Sprintf("## Machine %d needs a card of its own to be a Next", w.machine),
			fmt.Sprintf("%s is in %s. Two machines cannot", filepath.Base(w.taken), w.where(w.use(w.taken))),
			"share a card: each would write over the other. Choose a free card",
			"that boots, or Copy one to make it, and then Boot from it.",
		}
	}
	slot1, slot2 := "none: this machine is not a Next booting NextZXOS", "empty"
	if w.zx.next != nil {
		slot1 = filepath.Base(w.zx.next.files.Card)
	}
	if second := w.zx.SecondCard(); second != "" {
		slot2 = filepath.Base(second)
	}
	return []string{
		fmt.Sprintf("## Machine %d", w.machine),
		"Slot 1, the card it boots from: " + slot1,
		"Slot 2: " + slot2,
		"A card is in one slot of one machine at a time.",
	}
}

// layout places the window's parts.
func (w *cardsWindow) layout(r zenui.Renderer, screenW, screenH int) {
	w.setButtons()
	w.head = w.headLines()
	w.scale = zenui.TextBodyScale(r, screenW)
	w.lineH = r.LineHeight(w.scale)
	if w.lineH < 1 {
		w.lineH = 1
	}
	pad := r.LineHeight(1)
	pw := zenui.TextPanelWidth(r, w.scale)
	if pw > screenW-2*pad {
		pw = screenW - 2*pad
	}
	inner := pw - 2*pad
	textW := func(s string) int { return r.TextWidth(s, w.scale) }
	_, barH := w.buttons.Layout(zenui.Rect{W: inner}, w.lineH, textW)
	titleH := r.LineHeight(2) + 8
	// A row is as tall as the list makes it, which is more than a line: it
	// has the picture beside the name.
	rowsH := w.lineH + 4
	if len(w.list.Items) > 0 {
		rowsH = w.list.ItemRect(0, zenui.Rect{W: inner, H: 1 << 14}, w.lineH).H
	}
	listH := cardsWindowLines*rowsH + 2
	rowH := w.lineH + 8
	fixed := 2*pad + titleH + len(w.head)*w.lineH + pad + pad + rowH + w.lineH + pad + barH
	if over := fixed + listH - (screenH - 2*pad); over > 0 {
		listH -= over
		if listH < 2*(w.lineH+4) {
			listH = 2 * (w.lineH + 4)
		}
	}
	ph := fixed + listH
	px, py := (screenW-pw)/2, (screenH-ph)/2
	if py < 0 {
		py = 0
	}
	w.panel = zenui.Rect{X: px, Y: py, W: pw, H: ph}
	y := py + pad + titleH + len(w.head)*w.lineH + pad
	w.listArea = zenui.Rect{X: px + pad, Y: y, W: inner, H: listH}
	y += listH + pad
	w.progress = zenui.Rect{X: px + pad, Y: y, W: inner, H: rowH}
	y += rowH + w.lineH + pad
	w.barArea = zenui.Rect{X: px + pad, Y: y, W: inner, H: barH}
}

// Update runs the window for a frame.
func (w *cardsWindow) Update(in zenui.Input, r zenui.Renderer, screenW, screenH int) cardsResult {
	if w.ask != nil {
		return w.updateAsk(in)
	}
	if w.browse != nil {
		switch w.browse.Update(in) {
		case zenui.Accepted:
			path := w.browse.Result()
			w.browse = nil
			w.extra = append(w.extra, path)
			w.refresh()
			w.selectPath(path)
		case zenui.Cancelled:
			w.browse = nil
		}
		return cardsOpen
	}
	w.finishCopy()
	w.layout(r, screenW, screenH)
	w.list.Items = w.list.Items[:0]
	for _, c := range w.cards {
		w.list.Items = append(w.list.Items, zenui.ListItem{Name: w.row(c), Thumb: w.cardPicture(c.system)})
	}

	w.mu.Lock()
	copying := w.copying
	w.mu.Unlock()
	for _, k := range in.Keys {
		switch k {
		case zenui.KeyEscape:
			if copying {
				return w.do(cardBtnStopCopy)
			}
			return cardsClosed
		case zenui.KeyEnter:
			if !copying {
				return w.do(cardBtnBoot)
			}
		case zenui.KeyUp, zenui.KeyDown, zenui.KeyHome, zenui.KeyEnd, zenui.KeyPageUp, zenui.KeyPageDown:
			w.list.Step(k)
			w.list.Reveal(w.listArea, w.lineH)
		}
	}
	w.list.Update(in, w.listArea, w.lineH)
	textW := func(s string) int { return r.TextWidth(s, w.scale) }
	if id, ok := w.buttons.Update(in, w.barArea, w.lineH, textW); ok {
		return w.do(id)
	}
	return cardsOpen
}

// Draw draws the window over the machine's picture.
func (w *cardsWindow) Draw(r zenui.Renderer, screenW, screenH int, theme zenui.Theme) {
	if w.browse != nil {
		w.browse.Draw(r, screenW, screenH, theme)
		return
	}
	w.theme = theme
	w.layout(r, screenW, screenH)
	r.FillRect(zenui.Rect{W: screenW, H: screenH}, zenui.ScaleAlpha(theme.Backdrop, zenui.ModalBackdropDarkening))
	r.FillRect(w.panel, theme.Panel)
	r.StrokeRect(w.panel, theme.Border, theme.BorderThickness)

	pad := r.LineHeight(1)
	x, y := w.panel.X+pad, w.panel.Y+pad
	title := "SD CARDS"
	if w.must {
		title = "SD CARDS: A CARD FOR THIS NEXT"
	}
	r.DrawText(title, x, y, 2, theme.Text)
	y += r.LineHeight(2) + 8
	for _, line := range w.head {
		zenui.DrawTextLine(r, line, x, y, w.scale, theme)
		y += w.lineH
	}

	r.StrokeRect(zenui.Rect{X: w.listArea.X - 1, Y: w.listArea.Y - 1, W: w.listArea.W + 2, H: w.listArea.H + 2}, theme.Border, 1)
	if len(w.cards) == 0 {
		r.DrawText("No card images in "+w.dir, w.listArea.X+4, w.listArea.Y+4, w.scale, theme.DimText)
	}
	w.list.Draw(r, theme, w.listArea, w.scale)

	w.mu.Lock()
	copying, done, total := w.copying, w.done, w.total
	w.mu.Unlock()
	noteY := w.progress.Y + w.progress.H + 2
	if copying {
		zenui.DrawProgressBar(r, w.progress, done, total, theme)
		r.DrawText(zenui.ProgressText(done, total), w.progress.X, noteY, w.scale, theme.Text)
	} else {
		note := w.note
		if c, ok := w.selected(); ok && note == "" && !c.system {
			note = "It does not boot a Next: " + c.why
			if w.use(c.path).machine == 0 && w.systemSource() != "" {
				note += ". Make bootable puts the system on it."
			}
		}
		for i, line := range zenui.WrapText(note, 68, "") {
			if i > 1 {
				break
			}
			r.DrawText(line, w.progress.X, w.progress.Y+i*w.lineH, w.scale, theme.DimText)
		}
	}
	w.buttons.Draw(r, theme, w.barArea, w.scale)
	if w.ask != nil {
		w.ask.draw(r, screenW, screenH, theme)
	}
}

// busy reports that the window must not be closed from outside: a copy is
// being written.
func (w *cardsWindow) busy() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.copying
}

// whereIs tells where a card image is among the window's machines. With no
// dock there is one machine, and only its own slots to look in.
func (b *appMenuBar) whereIs(path string) cardUse {
	if b.cardUse != nil {
		return b.cardUse(path)
	}
	return cardUse{}
}

// numberOf is zx's number in the dock, 1 when there is no dock.
func (b *appMenuBar) numberOf(zx *ZenZX) int {
	if b.machineNumber != nil {
		return b.machineNumber(zx)
	}
	return 1
}

// openCards opens the SD Cards window for a machine that is a Next booting
// NextZXOS.
func (b *appMenuBar) openCards(zx *ZenZX) {
	if zx.next == nil {
		return
	}
	b.cards = newCardsWindow(zx, b.numberOf(zx), zx.next.files.Dir, b.nextReadOnly, b.whereIs)
}

// needsOwnCard reports that zx cannot become a Next with the card a Next
// boots from, because another machine has it, and opens the SD Cards window
// to ask for one of its own. The machine is left as it is until one is
// chosen.
func (b *appMenuBar) needsOwnCard(zx *ZenZX) bool {
	if b.nextClassic {
		return false
	}
	f, ok, _ := locateNextFiles(b.nextFilesDir)
	if !ok {
		return false
	}
	u := b.whereIs(f.Card)
	if u.machine == 0 || u.machine == b.numberOf(zx) {
		return false
	}
	b.cards = newCardsWindow(zx, b.numberOf(zx), f.Dir, b.nextReadOnly, b.whereIs)
	b.cards.must, b.cards.taken = true, f.Card
	return true
}

// updateCards runs the SD Cards window and acts on what it ends in.
func (b *appMenuBar) updateCards(zx *ZenZX) {
	w := b.cards
	switch w.Update(zenuiraylib.Input(), b.renderer, rl.GetScreenWidth(), rl.GetScreenHeight()) {
	case cardsClosed:
		b.cards = nil
	case cardsBoot:
		b.cards = nil
		f := nextcard.In(w.dir)
		f.Card = w.bootCard
		if w.must {
			zx.detachNextBootChain()
			switchModelLive(zx, "next", b.romDir)
			b.setCurrentModel("next")
		}
		b.attachNext(zx, f)
	}
}
