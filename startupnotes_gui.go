//go:build !headless

package zenzx

import "strings"

// What is found wrong before the window is up (a settings file that could not be read, snapshots
// moved into their folder, a menu bar that could not be built, sound that would not start) was
// said on the console only, which most people who use the window never read. startupNotice turns
// the lines into what the window says: one is a toast, several are a text window, one line each.

// startupNotice is what the window says for the notes: the toast, or the title and text of a text
// window. Both are empty when there is nothing to say.
func startupNotice(notes []string) (toast, title, text string) {
	var kept []string
	for _, n := range notes {
		if n = strings.TrimSpace(n); n != "" {
			kept = append(kept, n)
		}
	}
	switch len(kept) {
	case 0:
		return "", "", ""
	case 1:
		return kept[0], "", ""
	}
	return "", "NOTICES", "## Found while starting\n\n- " + strings.Join(kept, "\n- ")
}

// sayStartupNotes says them in the window: a toast through the drawer, or the bar's text window.
// Without a bar (it could not be built) the lines are joined into one toast.
func sayStartupNotes(notes []string, bar *appMenuBar, drawer *mountDrawer) {
	toast, title, text := startupNotice(notes)
	switch {
	case toast != "":
		drawer.say(toast)
	case title != "" && bar != nil:
		bar.showInfo(title, text)
	case title != "":
		drawer.say(strings.ReplaceAll(strings.TrimPrefix(text, "## Found while starting\n\n- "), "\n- ", "; "))
	}
}
