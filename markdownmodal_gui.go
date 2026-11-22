//go:build !headless

package zenzx

import (
	_ "embed"

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

// newHelpModal and newAboutModal make the two windows.
func newHelpModal() *zenui.TextModal { return zenui.NewTextModal("HELP", helpText, false) }

func newAboutModal(aboutText string) *zenui.TextModal {
	return zenui.NewTextModal("ABOUT", aboutText, true)
}
