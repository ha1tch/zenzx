//go:build !headless

package zenzx

import (
	_ "embed"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/ha1tch/zenui"
)

//go:embed help.txt
var helpText string

//go:embed about.txt
var aboutTextTemplate string

// The Help and About windows are zenui.TextModal: a scrollable reader for a
// small markdown subset, shown over the running emulator with a dim
// backdrop. It was written here (after zenimate's help window) and moved
// to zenui in its 0.2.0, with the translucent panel and lighter backdrop
// that the other windows of this program share (zenui.ScaleAlpha,
// ModalPanelOpacity, ModalBackdropDarkening).

// openLink opens a link of one of these windows in the system's browser. The windows ask for
// links only in text that is this program's own (zenui's WithLinks), and zenui makes a link of
// nothing but a web address.
var openLink = rl.OpenURL // a variable, so that a test can see what is opened

// newHelpModal and newAboutModal make the two windows. Their addresses are links.
func newHelpModal() *zenui.TextModal {
	return zenui.NewTextModal("HELP", helpText, false).WithLinks(openLink)
}

func newAboutModal(aboutText string) *zenui.TextModal {
	return zenui.NewTextModal("ABOUT", aboutText, true).WithLinks(openLink)
}
