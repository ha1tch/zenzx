//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	_ "embed"
	"regexp"
	"strings"

	"github.com/ha1tch/zenui"
)

// The Acknowledgements window (Help > Acknowledgements...) shows ACKNOWLEDGEMENTS.md, the
// file in the source tree, so that there is one text of it, and a person who runs the
// program reads the one a person who reads the repository does. The window is a
// zenui.TextModal, which reads a plain subset of markdown and does not wrap, so the
// file is turned into that here: paragraphs joined and wrapped, each person's name a
// heading, links as their text (and their address where the text is not the address), and
// clickable: they stay "[seen text](address)" for the window, which draws the seen text.

//go:embed ACKNOWLEDGEMENTS.md
var acknowledgementsSource string

// acknowledgementsURL is the file on GitHub, which About points to.
const acknowledgementsURL = "https://github.com/ha1tch/zenzx/blob/main/ACKNOWLEDGEMENTS.md"

// acknowledgementsCols is the width the text is wrapped to, a little under the
// reader's 70 columns.
const acknowledgementsCols = 66

var (
	ackLink     = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	ackEmphasis = regexp.MustCompile(`\*+([^*]+)\*+`)
	ackEntry    = regexp.MustCompile(`^- \*\*(.+?)\*\*\s*(?:—|-)?\s*(.*)$`)
)

func newAcknowledgementsModal() *zenui.TextModal {
	return zenui.NewTextModal("ACKNOWLEDGEMENTS", acknowledgementsModalText(acknowledgementsSource), false).WithLinks(openLink)
}

// acknowledgementsModalText turns the markdown of ACKNOWLEDGEMENTS.md into what a
// TextModal reads. The font has no em dash, nor any other character beyond ASCII, so
// those are written as their ASCII stand-ins.
func acknowledgementsModalText(md string) string {
	type item struct{ kind, head, text string } // kind: h1, entry, para
	var items []item
	cur := -1
	for _, raw := range strings.Split(md, "\n") {
		line := strings.TrimRight(raw, " \t\r")
		switch {
		case line == "":
			cur = -1
		case strings.HasPrefix(line, "# "):
			items = append(items, item{kind: "h1", text: line[2:]})
			cur = -1
		case strings.HasPrefix(line, "- "):
			head, rest := line[2:], ""
			if m := ackEntry.FindStringSubmatch(line); m != nil {
				head, rest = m[1], m[2]
			}
			items = append(items, item{kind: "entry", head: head, text: rest})
			cur = len(items) - 1
		default:
			if cur < 0 {
				items = append(items, item{kind: "para"})
				cur = len(items) - 1
			}
			items[cur].text += " " + strings.TrimSpace(line)
		}
	}

	var out []string
	for _, it := range items {
		switch it.kind {
		case "h1":
			out = append(out, "# "+ackPlain(it.text), "")
		case "entry":
			out = append(out, "## "+ackPlain(it.head))
			out = append(out, zenui.WrapLinkedText(ackPlain(it.text), acknowledgementsCols, "")...)
			out = append(out, "")
		default:
			out = append(out, zenui.WrapLinkedText(ackPlain(it.text), acknowledgementsCols, "")...)
			out = append(out, "")
		}
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n")
}

// ackPlain is a piece of markdown as plain ASCII text, with its links as "[seen](address)".
func ackPlain(s string) string {
	s = ackLink.ReplaceAllStringFunc(s, func(m string) string {
		p := ackLink.FindStringSubmatch(m)
		text, url := p[1], p[2]
		host := strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "http://")
		if i := strings.Index(host, "/"); i >= 0 {
			host = host[:i]
		}
		seen := text
		if !strings.Contains(text, host) { // the text is the address, or has it in it, or it is not
			seen = text + " (" + url + ")"
		}
		return "[" + seen + "](" + url + ")" // still a link, for the window to make clickable
	})
	s = ackEmphasis.ReplaceAllString(s, "$1")
	s = strings.NewReplacer("—", "-", "–", "-", "’", "'", "‘", "'", "“", `"`, "”", `"`).Replace(s)
	return strings.Join(strings.Fields(s), " ")
}
