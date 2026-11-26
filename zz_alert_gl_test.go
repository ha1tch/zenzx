//go:build !headless

package zenzx

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// TestZZAlertShot draws the dialogue for the cards of a snapshot (a card in use by another machine, a card
// laid out very differently) in a window of the program's own and writes two pictures to look at. It asserts
// nothing, and does nothing unless ZZ_SHOTS names a folder for them:
//
//	DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZZ_SHOTS=/tmp/shots go test -run TestZZAlertShot .
func TestZZAlertShot(t *testing.T) {
	dir := os.Getenv("ZZ_SHOTS")
	if dir == "" {
		t.Skip()
	}
	runtime.LockOSThread()
	rl.SetTraceLogLevel(rl.LogNone)
	rl.InitWindow(960, 768, "alert")
	t.Cleanup(rl.CloseWindow)
	w, h := int(rl.GetScreenWidth()), int(rl.GetScreenHeight())
	b, _ := newTestBar(t, nil, "")

	cards := t.TempDir()
	// sizes: make them look like real ones by naming only; NewCard is the smallest card
	held := searchCard(t, cards, "next-card.img")
	searchCard(t, cards, "next-card-2.img")
	searchCard(t, cards, "next-card-3.img")
	differentCard(t, cards, "games.img")
	os.WriteFile(filepath.Join(cards, "notes.img"), []byte("x"), 0o644)
	rec := searchRecord(t, 1, held)
	search := &cardSearch{dirs: []string{cards}, held: func(p string) int {
		if same(p, held) {
			return 2
		}
		if filepath.Base(p) == "next-card-2.img" {
			return 3
		}
		return 0
	}}
	need := cardNeed{recs: []cardRecord{rec}}
	need.found = search.find(0, need.recs)
	a := newCardsAsk("warhawk.zxs", false, []cardNeed{need}, search, nil, nil)
	b.askForCards(a)
	shot := func(name string) {
		rl.BeginDrawing()
		rl.ClearBackground(rl.NewColor(60, 90, 140, 255))
		b.Draw(w, h)
		rl.DrawRenderBatchActive()
		img := rl.LoadImageFromScreen()
		rl.EndDrawing()
		cols := rl.LoadImageColors(img)
		rl.UnloadImage(img)
		out := image.NewRGBA(image.Rect(0, 0, w, h))
		for i, c := range cols {
			out.SetRGBA(i%w, i/w, color.RGBA{c.R, c.G, c.B, c.A})
		}
		f, _ := os.Create(filepath.Join(dir, name))
		png.Encode(f, out)
		f.Close()
	}
	shot("alert-list.png")
	a.choose(filepath.Join(cards, "games.img"))
	shot("alert-confirm.png")
}
