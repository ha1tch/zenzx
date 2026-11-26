package zenzx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// ROMArchivesFile is the file of custom-roms/ that lists the pages Custom ROM > Download more ROMs
// opens: where ROMs to put in the folder are published. It is a JSON array of objects with a
// "text_label" (what the row says) and a "url" (the page, http or https):
//
//	[
//	  {"text_label": "Rodolfo Guerra ROM mods", "url": "https://sites.google.com/view/rodolfoguerra/"}
//	]
//
// Without the file, or when it cannot be used, the menu has defaultROMArchives; a file that is an
// empty array takes the row away.
const ROMArchivesFile = "rom-archives.json"

// romArchive is a page of ROMs: the row's text and the address it opens.
type romArchive struct {
	Label string
	URL   string
}

// defaultROMArchives are the pages of the standard distribution, and what the menu has when
// custom-roms/ has no usable ROMArchivesFile.
var defaultROMArchives = []romArchive{
	{"Rodolfo Guerra ROM mods", "https://sites.google.com/view/rodolfoguerra/"},
	{"Speccy4ever ROM archive", "https://speccy4ever.speccy.org/"},
}

// loadROMArchives reads ROMArchivesFile of dir. A file that is not there gives the default list and
// no warning; a file that cannot be read or is not the JSON above, or whose every entry is
// unusable, gives the default list and says why. An entry with no label, or whose address is not
// an http or https one with a host, is left out, and the warning says how many. An empty array is a
// list of nothing.
func loadROMArchives(dir string) (list []romArchive, warning string) {
	defaults := func() []romArchive { return append([]romArchive(nil), defaultROMArchives...) }
	data, err := os.ReadFile(filepath.Join(dir, ROMArchivesFile))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return defaults(), ""
		}
		return defaults(), fmt.Sprintf("%s could not be read (%v); the standard ROM links are used", ROMArchivesFile, err)
	}
	var raw []struct {
		TextLabel string `json:"text_label"`
		URL       string `json:"url"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return defaults(), fmt.Sprintf("%s is not a list of {\"text_label\", \"url\"} (%v); the standard ROM links are used", ROMArchivesFile, err)
	}
	skipped := 0
	for _, r := range raw {
		label := strings.TrimSpace(r.TextLabel)
		u, err := url.Parse(strings.TrimSpace(r.URL))
		if label == "" || err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			skipped++
			continue
		}
		list = append(list, romArchive{Label: label, URL: u.String()})
	}
	if len(raw) > 0 && len(list) == 0 {
		return defaults(), fmt.Sprintf("no entry of %s has both a text_label and an http(s) url; the standard ROM links are used", ROMArchivesFile)
	}
	if skipped > 0 {
		warning = fmt.Sprintf("%d entr%s of %s left out: each needs a text_label and an http(s) url", skipped, map[bool]string{true: "y", false: "ies"}[skipped == 1], ROMArchivesFile)
	}
	return list, warning
}

// romArchivesStamp says whether ROMArchivesFile of dir has changed since the last stamp: it is
// "absent" for no file, else the file's modification time and size. The menu bar reads the file
// again when the stamp differs, so that editing it shows in the menu without restarting.
func romArchivesStamp(dir string) string {
	st, err := os.Stat(filepath.Join(dir, ROMArchivesFile))
	if err != nil {
		return "absent"
	}
	return fmt.Sprintf("%d:%d", st.ModTime().UnixNano(), st.Size())
}
