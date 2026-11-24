package zenzx

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

func TestParseMulti(t *testing.T) {
	twelve := make([]string, 12)
	for i := range twelve {
		twelve[i] = "128k"
	}
	good := []struct {
		spec  string
		want  []string
		cards []string // nil: none named
	}{
		{"", []string{"plus3"}, nil},
		{"12,128k", twelve, nil},
		{"12:128k", twelve, nil},
		{"4:48k,48k,128k,next", []string{"48k", "48k", "128k", "next"}, nil},
		{"48k,128k", []string{"48k", "128k"}, nil},
		{" 2 : 48K , Plus2 ", []string{"48k", "plus2"}, nil},
		{"3", []string{"plus3", "plus3", "plus3"}, nil},
		{"1", []string{"plus3"}, nil},
		{"next", []string{"next"}, nil},
		{"3:next/sdcard-01,next/sdcard-02,next/sdcard-03", []string{"next", "next", "next"}, []string{"sdcard-01", "sdcard-02", "sdcard-03"}},
		{"next,NEXT/Cards/Two.img,48k", []string{"next", "next", "48k"}, []string{"", "Cards/Two.img", ""}},
		{"next=a.img, next=/abs/b.img", []string{"next", "next"}, []string{"a.img", "/abs/b.img"}},
		{"1,next/only", []string{"next"}, []string{"only"}},
		{`next/C:\cards\a.img`, []string{"next"}, []string{`C:\cards\a.img`}},
	}
	for _, c := range good {
		got, cards, err := parseMulti(c.spec, "plus3")
		want := c.cards
		if want == nil {
			want = make([]string, len(c.want))
		}
		if err != nil || !reflect.DeepEqual(got, c.want) || !reflect.DeepEqual(cards, want) {
			t.Errorf("parseMulti(%q) = %v, %q, %v; want %v, %q", c.spec, got, cards, err, c.want, want)
		}
	}
	bad := map[string]string{
		"4:48k,128k":       "4 machines asked for and 2 models",
		"2,48k,3":          "count belongs at the start",
		"2:3,48k":          "count belongs at the start",
		"0,48k":            "at least 1",
		"-2:48k":           "at least 1",
		"x:48k":            "no such model",
		"48k,,128k":        "empty place",
		"48k,c64":          "no such model",
		"33,48k":           "the most is 32",
		"2,next":           "2 Nexts with no card named",
		"next,48k,next":    "2 Nexts with no card named",
		"next/a,next,next": "2 Nexts with no card named",
		"48k/tape.tap":     "only a Next boots from an SD card",
		"next/":            "names no card",
		"3,next/one":       "cannot share the card",
	}
	for spec, want := range bad {
		if _, _, err := parseMulti(spec, "48k"); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("parseMulti(%q) gave %v, want an error mentioning %q", spec, err, want)
		}
	}
}

func TestPrimaryMachine(t *testing.T) {
	cases := []struct {
		models, cards []string
		want          int
	}{
		{[]string{"48k", "48k", "128k", "next"}, []string{"", "", "", ""}, 3},
		{[]string{"128k", "48k"}, []string{"", ""}, 0},
		{[]string{"48k", "next", "next"}, []string{"", "a", ""}, 2},  // the one on the standard card
		{[]string{"48k", "next", "next"}, []string{"", "a", "b"}, 1}, // each has its own: the first
	}
	for _, c := range cases {
		if p := primaryMachine(c.models, c.cards); p != c.want {
			t.Errorf("primaryMachine(%v, %q) = %d, want %d", c.models, c.cards, p, c.want)
		}
	}
}

// testCards makes n cards a Next can boot from, named card-1.img onwards,
// in a new folder.
func testCards(t *testing.T, n int) (dir string) {
	t.Helper()
	dir = t.TempDir()
	for i := 1; i <= n; i++ {
		path := filepath.Join(dir, fmt.Sprintf("card-%d.img", i))
		root := nextcard.NewDir("")
		// What makes a card one a Next boots from (nextcard.VerifyCard).
		root.Add("TBBLUE.FW", []byte("firmware"))
		root.AddDir("machines").AddDir("next").Add("config.ini", []byte("timing=0\n"))
		if _, err := nextcard.NewCard(path, nextcard.SmallestCard, root); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestResolveNextCards(t *testing.T) {
	dir := testCards(t, 3)
	one, two, three := filepath.Join(dir, "card-1.img"), filepath.Join(dir, "card-2.img"), filepath.Join(dir, "card-3.img")

	// By name in the Next's folder, with and without .img, and by path.
	got, err := resolveNextCards([]string{"", "card-1", "card-2.img", three}, dir, "")
	if err != nil || !reflect.DeepEqual(got, []string{"", one, two, three}) {
		t.Fatalf("resolved %q, %v", got, err)
	}

	bad := []struct {
		cards    []string
		standard string
		want     string
	}{
		{[]string{"card-9"}, "", "no SD card"},
		{[]string{"card-1", "card-1.img"}, "", "is machine 1's card"},
		{[]string{"", "card-2"}, two, "the Next with no card named boots from"},
	}
	for _, c := range bad {
		if _, err := resolveNextCards(c.cards, dir, c.standard); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("resolveNextCards(%q) gave %v, want an error mentioning %q", c.cards, err, c.want)
		}
	}

	notACard := filepath.Join(dir, "notes.img")
	if err := os.WriteFile(notACard, []byte("not a card"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveNextCards([]string{"notes"}, dir, ""); err == nil || !strings.Contains(err.Error(), "not a card a Next can boot from") {
		t.Errorf("a file that is not a card gave %v", err)
	}
}

func TestNextCardsOfNeedsTheBootFiles(t *testing.T) {
	// No card named: nothing is looked for.
	cards, _, err := nextCardsOf([]string{"48k", "next"}, []string{"", ""}, t.TempDir())
	if err != nil || !reflect.DeepEqual(cards, []string{"", ""}) {
		t.Errorf("with no card named: %q, %v", cards, err)
	}
	// A card named and no boot files anywhere it looks: an error that says so.
	t.Setenv(nextFilesEnv, t.TempDir())
	t.Chdir(t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	if _, _, err := nextCardsOf([]string{"next"}, []string{"card-1"}, ""); err == nil || !strings.Contains(err.Error(), "boot files are not in place") {
		t.Errorf("with no boot files: %v", err)
	}
}

func TestParseListen(t *testing.T) {
	if got, err := parseListen("", 4); err != nil || got != nil {
		t.Errorf("no -listen gave %v, %v", got, err)
	}
	if got, _ := parseListen("16384", 1); !reflect.DeepEqual(got, []int{16384}) {
		t.Errorf("one port for one machine gave %v", got)
	}
	if got, _ := parseListen("16384", 4); !reflect.DeepEqual(got, []int{16384, 16385, 16386, 16387}) {
		t.Errorf("one port for four machines gave %v", got)
	}
	if got, _ := parseListen("16384, 16390,2000,2001", 4); !reflect.DeepEqual(got, []int{16384, 16390, 2000, 2001}) {
		t.Errorf("four ports gave %v", got)
	}
	bad := map[string]string{
		"16384,16385": "2 ports for 4 machines",
		"0":           "not a port number",
		"70000":       "not a port number",
		"abc":         "not a port number",
		"65534":       "run past 65535",
		"1,2,2,3":     "given twice",
	}
	for spec, want := range bad {
		if _, err := parseListen(spec, 4); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("parseListen(%q, 4) gave %v, want an error mentioning %q", spec, err, want)
		}
	}
}

func TestParseAutorun(t *testing.T) {
	if got, _ := parseAutorun("", 3); !reflect.DeepEqual(got, []string{"", "", ""}) {
		t.Errorf("no -autorun gave %v", got)
	}
	if got, _ := parseAutorun("a.bas", 3); !reflect.DeepEqual(got, []string{"a.bas", "a.bas", "a.bas"}) {
		t.Errorf("one file gave %v", got)
	}
	if got, _ := parseAutorun("a.bas,,c.bas", 3); !reflect.DeepEqual(got, []string{"a.bas", "", "c.bas"}) {
		t.Errorf("a list with an empty place gave %v", got)
	}
	if _, err := parseAutorun("a.bas,b.bas", 3); err == nil {
		t.Error("two files for three machines were accepted")
	}
}

func TestStartLinksAndAutoruns(t *testing.T) {
	a, b := newSideMachine(t, "48k", 0, "").zx, newSideMachine(t, "128k", 0, "").zx
	machines := []*ZenZX{a, b}
	// Take two free ports by asking for them, then give them back.
	probe := NewLink()
	if err := probe.Listen("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	addr := probe.Addr()
	probe.Close()
	var port int
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			for _, ch := range addr[i+1:] {
				port = port*10 + int(ch-'0')
			}
			break
		}
	}
	links, err := startLinks(machines, "127.0.0.1", []int{port})
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 || a.Link() == nil || b.Link() != nil {
		t.Errorf("one port gave %d links; machine 1 has one: %v, machine 2 has one: %v", len(links), a.Link() != nil, b.Link() != nil)
	}
	// The same port again cannot be opened: the error names the machine.
	if _, err := startLinks([]*ZenZX{b}, "127.0.0.1", []int{port}); err == nil || !strings.Contains(err.Error(), "machine 1") {
		t.Errorf("a port in use gave %v", err)
	}
	links[0].Close()

	err = startAutoruns(machines, []string{"48k", "128k"}, []string{"examples/link/receiver.bas", ""})
	if err != nil || !a.AutorunPending() || b.AutorunPending() {
		t.Errorf("autoruns: %v; machine 1 pending %v, machine 2 pending %v", err, a.AutorunPending(), b.AutorunPending())
	}
	if err := startAutoruns(machines, []string{"48k", "128k"}, []string{"", "no-such.bas"}); err == nil || !strings.Contains(err.Error(), "machine 2") {
		t.Errorf("a missing file gave %v", err)
	}
	// A model that cannot autorun is skipped, not an error.
	if err := startAutoruns(machines, []string{"next", "128k"}, []string{"examples/link/receiver.bas", ""}); err != nil {
		t.Errorf("a Next in the list gave %v", err)
	}
}

func TestParseLayout(t *testing.T) {
	good := map[string]string{
		"":           "One Machine",
		"single":     "One Machine",
		"grid":       "Grid",
		"GRID":       "Grid",
		" columns ":  "Side by Side",
		"side":       "Side by Side",
		"rows":       "Stacked",
		"stacked":    "Stacked",
		"main-left":  "Main Left, Stack Right",
		"main-right": "Stack Left, Main Right",
	}
	for spec, want := range good {
		got, err := parseLayout(spec)
		if err != nil || got != want {
			t.Errorf("parseLayout(%q) = %q, %v; want %q", spec, got, err, want)
		}
	}
	for _, spec := range []string{"tiled", "2x2", "big"} {
		if _, err := parseLayout(spec); err == nil {
			t.Errorf("parseLayout(%q) was accepted", spec)
		}
	}
}
