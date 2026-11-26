//go:build !headless

package zenzx

import (
	"testing"

	"github.com/ha1tch/zenui"
)

// posRenderer remembers where each text was drawn.
type posRenderer struct {
	paintRenderer
	at map[string]int
}

func (p *posRenderer) DrawText(s string, x, y, scale int, c zenui.Colour) {
	p.paintRenderer.DrawText(s, x, y, scale, c)
	if p.at == nil {
		p.at = map[string]int{}
	}
	p.at[s] = x
}

// carets are the carets drawn: thin bars of the text colour, as high as a line.
func carets(r *posRenderer, th zenui.Theme, scale int) []zenui.Rect {
	lh := r.LineHeight(scale)
	var out []zenui.Rect
	for _, f := range r.fills {
		if f.colour == th.Text && f.rect.W == max(1, lh/8) && f.rect.H == lh {
			out = append(out, f.rect)
		}
	}
	return out
}

// A text box in a form of the program has a caret at the end of its text, where the next key goes
// (zenui's form draws it): the DELETE of the SD Cards and Snapshots windows, a new name, and the
// name of an export. It moves as text is typed, and a question that has no box has none.
func TestFormTextBoxesHaveACaret(t *testing.T) {
	th := zenui.LoadTheme(zenui.ThemeDark)
	type form struct {
		name  string
		scale int
		text  *string
		draw  func(r zenui.Renderer)
	}
	del := &cardQuestion{kind: askDelete, path: "/x/card.img", size: 1 << 20}
	del.build()
	ren := &cardQuestion{kind: askRenameName, path: "/x/card.img", size: 1 << 20, text: "card"}
	ren.build()
	snap := &cardQuestion{kind: askDelete, path: "/x/snap.zxs", size: 1 << 10, noun: "snapshot", ext: ".zxs"}
	snap.build()
	exp := &exportAsk{dir: t.TempDir(), name: "spectrum-48k"}
	exp.build()
	forms := []form{
		{"delete", cardAskScale, &del.text, func(r zenui.Renderer) { del.draw(r, 1000, 700, th) }},
		{"rename", cardAskScale, &ren.text, func(r zenui.Renderer) { ren.draw(r, 1000, 700, th) }},
		{"delete a snapshot", cardAskScale, &snap.text, func(r zenui.Renderer) { snap.draw(r, 1000, 700, th) }},
		{"export", exportAskScale, &exp.name, func(r zenui.Renderer) { exp.draw(r, 1000, 700, th) }},
	}
	for _, f := range forms {
		last := 0
		for i, typed := range []string{"", "D", "DELETE"} {
			*f.text = typed
			r := &posRenderer{}
			f.draw(r)
			cs := carets(r, th, f.scale)
			if len(cs) != 1 {
				t.Errorf("%s with %q: %d carets, want 1", f.name, typed, len(cs))
				continue
			}
			if typed != "" {
				// the caret is where the text the box drew ends
				x, ok := r.at[typed]
				if want := x + r.TextWidth(typed, f.scale); !ok || cs[0].X != want {
					t.Errorf("%s with %q: the caret is at x=%d, the text ends at %d (drawn: %v)", f.name, typed, cs[0].X, want, ok)
				}
			}
			if i > 0 && cs[0].X <= last {
				t.Errorf("%s: the caret did not move right as %q was typed", f.name, typed)
			}
			last = cs[0].X
		}
	}
	// a long text keeps the caret on the screen
	long := ""
	for i := 0; i < 200; i++ {
		long += "W"
	}
	del.text = long
	r := &posRenderer{}
	del.draw(r, 1000, 700, th)
	if cs := carets(r, th, cardAskScale); len(cs) != 1 || cs[0].X < 0 || cs[0].X+cs[0].W > 1000 {
		t.Errorf("a long text: carets %v", cs)
	}
	// a question with no box
	sure := &cardQuestion{kind: askRenameSure, path: "/x/a.img", target: "/x/b.img"}
	sure.build()
	r = &posRenderer{}
	sure.draw(r, 1000, 700, th)
	if len(carets(r, th, cardAskScale)) != 0 {
		t.Errorf("a yes-or-no question has a caret")
	}
}
