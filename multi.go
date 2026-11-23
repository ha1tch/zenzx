package zenzx

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// Several machines from the command line: what -multi, -listen and
// -autorun mean when there is more than one machine. The parsing is here,
// shared by the GUI and the headless program; each does its own running.

// multiMax is the most machines -multi will start.
const multiMax = 32

// machineModels are the models a machine can be, as -model names them.
var machineModels = []string{
	"48k", "128k", "plus2", "plus2a", "plus3",
	"spanish48k", "spanish128k", "spanishplus2", "spanishplus3",
	"ts2068", "next",
}

func knownModel(model string) bool {
	for _, m := range machineModels {
		if m == model {
			return true
		}
	}
	return false
}

const multiHelp = "Start several machines. A count and a model: 12,128k. A count and one model each: 4:48k,48k,128k,next. " +
	"A list alone: 48k,128k. A count alone: 3 (of -model). A Next may name the SD card it boots from, next/card-01, " +
	"and every Next but one must: 3:next/card-01,next/card-02,next/card-03. " +
	"-tape, -snapshot, -script and the other single-machine options go to the first machine (to the Next, if there is one)"

const listenHelp = "Give each machine a link: a TCP port on which bytes sent reach a program on the machine through I/O ports 16384 (data) and 16385 (control), " +
	"and bytes the program writes to port 16384 come back. One port number serves the first machine and the next numbers the others (16384 gives 16384, 16385, ...); " +
	"or one number each, separated by commas. See docs/MANUAL.md, The link"

const listenHostHelp = "Address the -listen ports are opened on. The default accepts connections from this computer only"

const autorunHelp = "Load and run a BASIC program from a text file (line numbers, one line each) when the machine has started: no tape or disc to prepare. " +
	"With several machines, one file for all of them or one each, separated by commas; an empty place leaves that machine alone. Not on a Next or a TS2068"

const layoutHelp = "With several machines, how the window tiles them at the start, the same choices the View menu has: " +
	"single (one machine fills the window, the default), columns (side by side), rows (one above another), grid, main-left or main-right (one large, the rest stacked beside it). " +
	"Changed any time from the View menu. The headless program renders each machine to its own screenshot and ignores this"

const focusFollowHelp = "With several machines, move the keyboard focus to the machine whose player beacon (the byte at 0xBFFE) is set, so the controls follow the player across screens. " +
	"For an example whose program maintains that beacon, such as examples/world; off by default, since an ordinary multi-machine session has no such convention"

// layoutNames maps the -layout values to the View menu's own layout names
// (deckLayoutNames, machinedeck_gui.go), which is where a name becomes a
// layout. The keys are the short names and a couple of aliases.
var layoutNames = map[string]string{
	"single":     "One Machine",
	"columns":    "Side by Side",
	"side":       "Side by Side",
	"rows":       "Stacked",
	"stacked":    "Stacked",
	"grid":       "Grid",
	"main-right": "Stack Left, Main Right",
	"main-left":  "Main Left, Stack Right",
}

// parseLayout turns a -layout value into the View menu's name for that
// layout; "" is the default single view. It does not need the GUI, so the
// headless program can check the value too.
func parseLayout(spec string) (string, error) {
	spec = strings.ToLower(strings.TrimSpace(spec))
	if spec == "" {
		return "One Machine", nil
	}
	if name, ok := layoutNames[spec]; ok {
		return name, nil
	}
	return "", fmt.Errorf("-layout %q: want single, columns, rows, grid, main-left or main-right", spec)
}

// parseMulti turns a -multi value into the models to start and, for each,
// the SD card named for it ("" for none). An empty value is one machine of
// the default model.
//
// A card is named after the model and a slash, next/card-01 (an equals sign
// does as well: next=card-01), and only a Next takes one. What the name
// stands for is settled later, by resolveNextCards.
func parseMulti(spec, defaultModel string) (models, cards []string, err error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return []string{defaultModel}, []string{""}, nil
	}
	count, counted := 0, false
	list := spec
	// "4:" at the start is the count. A colon further on belongs to a path.
	if head, tail, ok := strings.Cut(spec, ":"); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(head)); err == nil {
			count, counted, list = n, true, tail
		}
	}
	for _, item := range strings.Split(list, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			return nil, nil, fmt.Errorf("-multi %q: an empty place in the list", spec)
		}
		if n, err := strconv.Atoi(item); err == nil {
			// A number in the list is the count, written first: "12,128k" or "3".
			if counted || len(models) != 0 {
				return nil, nil, fmt.Errorf("-multi %q: a count belongs at the start, once", spec)
			}
			count, counted = n, true
			continue
		}
		model, card := item, ""
		if i := strings.IndexAny(item, "/="); i >= 0 {
			model, card = item[:i], strings.TrimSpace(item[i+1:])
			if card == "" {
				return nil, nil, fmt.Errorf("-multi %q: %q names no card after the model", spec, item)
			}
		}
		model = strings.ToLower(strings.TrimSpace(model))
		if !knownModel(model) {
			return nil, nil, fmt.Errorf("-multi %q: no such model %q (models: %s)", spec, model, strings.Join(machineModels, ", "))
		}
		if card != "" && model != "next" {
			return nil, nil, fmt.Errorf("-multi %q: %q: only a Next boots from an SD card", spec, item)
		}
		models = append(models, model)
		cards = append(cards, card)
	}
	if len(models) == 0 {
		models, cards = []string{defaultModel}, []string{""}
	}
	switch {
	case !counted:
		// a list alone
	case count < 1:
		return nil, nil, fmt.Errorf("-multi %q: the count must be at least 1", spec)
	case len(models) == 1:
		one, card := models[0], cards[0]
		if card != "" && count > 1 {
			return nil, nil, fmt.Errorf("-multi %q: %d machines cannot share the card %s; name one for each", spec, count, card)
		}
		models, cards = make([]string, count), make([]string, count)
		for i := range models {
			models[i] = one
		}
		cards[0] = card
	case len(models) != count:
		return nil, nil, fmt.Errorf("-multi %q: %d machines asked for and %d models named", spec, count, len(models))
	}
	if len(models) > multiMax {
		return nil, nil, fmt.Errorf("-multi %q: %d machines; the most is %d", spec, len(models), multiMax)
	}
	bare := 0
	for i, m := range models {
		if m == "next" && cards[i] == "" {
			bare++
		}
	}
	if bare > 1 {
		return nil, nil, fmt.Errorf("-multi %q: %d Nexts with no card named; two cannot boot from the same SD card, so name one for each but one at most: next/card-01,next/card-02", spec, bare)
	}
	return models, cards, nil
}

// primaryMachine is the place in the list of the machine that is built the
// way a single machine is, with every option of the command line. It is a
// Next if there is one, since that is the machine the options about a
// Next's boot files are for: the one with no card named, which boots from
// the standard card as a Next alone does, or the first of them if each has
// its own. Otherwise it is the first machine.
func primaryMachine(models, cards []string) int {
	first := -1
	for i, m := range models {
		if m != "next" {
			continue
		}
		if cards[i] == "" {
			return i
		}
		if first < 0 {
			first = i
		}
	}
	if first >= 0 {
		return first
	}
	return 0
}

// resolveNextCards turns the card names of -multi into files. A name is
// looked for as it is written and with .img added, first from where the
// program was started and then in the folder of the Next's boot files,
// dir. Each must be a card a Next can boot from, and no two machines may
// have the same one: standard is the card a Next with none named boots
// from ("" if there is no such Next), and counts.
func resolveNextCards(cards []string, dir, standard string) ([]string, error) {
	out := make([]string, len(cards))
	taken := map[string]int{}
	if standard != "" {
		if abs, err := filepath.Abs(standard); err == nil {
			taken[abs] = 0
		}
	}
	for i, name := range cards {
		if name == "" {
			continue
		}
		found, err := findCardFile(name, dir)
		if err != nil {
			return nil, fmt.Errorf("machine %d: %w", i+1, err)
		}
		f, err := os.Open(found)
		if err != nil {
			return nil, fmt.Errorf("machine %d: %w", i+1, err)
		}
		err = nextcard.VerifyCard(f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("machine %d: %s is not a card a Next can boot from: %w", i+1, found, err)
		}
		abs, err := filepath.Abs(found)
		if err != nil {
			return nil, fmt.Errorf("machine %d: %w", i+1, err)
		}
		if other, ok := taken[abs]; ok {
			if other == 0 {
				return nil, fmt.Errorf("machine %d: %s is the card the Next with no card named boots from; two machines cannot have one card", i+1, found)
			}
			return nil, fmt.Errorf("machine %d: %s is machine %d's card; two machines cannot have one card", i+1, found, other)
		}
		taken[abs] = i + 1
		out[i] = found
	}
	return out, nil
}

// findCardFile turns the name of a card into a file: as it is written and
// with .img added, first from where the program was started and then in
// dir, the folder of the Next's boot files.
func findCardFile(name, dir string) (string, error) {
	var tried []string
	for _, p := range []string{name, name + ".img", filepath.Join(dir, name), filepath.Join(dir, name+".img")} {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
		tried = append(tried, p)
	}
	return "", fmt.Errorf("no SD card %q (looked for %s)", name, strings.Join(tried, ", "))
}

// nextCardsOf settles the cards of -multi before any machine is made. With
// none named it does nothing. Otherwise the Next's boot files have to be in
// place already, because the boot ROM and the DivMMC ROM come from their
// folder whichever card a machine boots from.
func nextCardsOf(models, cards []string, filesFlag string) ([]string, nextcard.Files, error) {
	named := false
	for _, c := range cards {
		named = named || c != ""
	}
	if !named {
		return cards, nextcard.Files{}, nil
	}
	f, ok, note := locateNextFiles(filesFlag)
	if !ok {
		if note == "" {
			note = "none were found"
		}
		return nil, f, fmt.Errorf("-multi names SD cards, but the Next's boot files are not in place (%s); start a Next on its own once, or run with -next-firmware setup, and they are fetched", note)
	}
	standard := ""
	for i, m := range models {
		if m == "next" && cards[i] == "" {
			standard = f.Card
		}
	}
	resolved, err := resolveNextCards(cards, f.Dir, standard)
	return resolved, f, err
}

// bootNextFrom gives a machine that is already a Next the boot files of f
// with card in place of its card, and starts it from the boot ROM. It
// returns the line that describes what was attached.
func bootNextFrom(zx *ZenZX, f nextcard.Files, card string, readOnly, debug bool) (string, error) {
	f.Card = card
	protected, err := zx.attachNextBootChain(f, readOnly, debug)
	if err != nil {
		return "", err
	}
	zx.Reset()
	zx.io.nextResetRequest = nextResetHard
	return describeNextChain(f, protected), nil
}

// parseListen turns a -listen value into one TCP port for each of n
// machines. An empty value is no ports at all.
func parseListen(spec string, n int) ([]int, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, nil
	}
	var ports []int
	for _, item := range strings.Split(spec, ",") {
		p, err := strconv.Atoi(strings.TrimSpace(item))
		if err != nil || p < 1 || p > 65535 {
			return nil, fmt.Errorf("-listen %q: %q is not a port number (1 to 65535)", spec, strings.TrimSpace(item))
		}
		ports = append(ports, p)
	}
	if len(ports) == 1 && n > 1 {
		first := ports[0]
		if first+n-1 > 65535 {
			return nil, fmt.Errorf("-listen %q: %d machines from port %d run past 65535", spec, n, first)
		}
		for i := 1; i < n; i++ {
			ports = append(ports, first+i)
		}
	}
	if len(ports) != n {
		return nil, fmt.Errorf("-listen %q: %d ports for %d machines; give one port, or one for each", spec, len(ports), n)
	}
	seen := map[int]bool{}
	for _, p := range ports {
		if seen[p] {
			return nil, fmt.Errorf("-listen %q: port %d is given twice", spec, p)
		}
		seen[p] = true
	}
	return ports, nil
}

// parseAutorun turns an -autorun value into one file for each of n
// machines, "" where a machine has none.
func parseAutorun(spec string, n int) ([]string, error) {
	files := make([]string, n)
	if strings.TrimSpace(spec) == "" {
		return files, nil
	}
	items := strings.Split(spec, ",")
	if len(items) == 1 {
		for i := range files {
			files[i] = strings.TrimSpace(items[0])
		}
		return files, nil
	}
	if len(items) != n {
		return nil, fmt.Errorf("-autorun %q: %d files for %d machines; give one file, or one place for each machine", spec, len(items), n)
	}
	for i, item := range items {
		files[i] = strings.TrimSpace(item)
	}
	return files, nil
}

// startLinks opens a link for each machine on its port and attaches it. On
// an error the links already opened are closed.
func startLinks(machines []*ZenZX, host string, ports []int) ([]*Link, error) {
	var links []*Link
	for i, port := range ports {
		l := NewLink()
		if err := l.Listen(fmt.Sprintf("%s:%d", host, port)); err != nil {
			for _, open := range links {
				open.Close()
			}
			return nil, fmt.Errorf("machine %d: %w", i+1, err)
		}
		machines[i].AttachLink(l)
		links = append(links, l)
	}
	return links, nil
}

// startAutoruns arranges each machine's autorun. A machine whose model
// cannot autorun is reported and left alone; a file that cannot be read or
// is not BASIC is an error.
func startAutoruns(machines []*ZenZX, models, files []string) error {
	for i, file := range files {
		if file == "" {
			continue
		}
		if _, ok := autorunKeys[models[i]]; !ok {
			sayf("Machine %d (%s): -autorun is not available on this model; %s was not loaded\n", i+1, models[i], file)
			continue
		}
		if err := machines[i].AutorunFile(file, models[i]); err != nil {
			return fmt.Errorf("machine %d: %w", i+1, err)
		}
	}
	return nil
}
