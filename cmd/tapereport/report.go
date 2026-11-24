package main

import (
	_ "embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
)

//go:embed assets/ZXSpectrum.ttf
var zxFontTTF []byte

//go:embed report.html.tmpl
var reportTemplate string

// TemplateData is what the HTML template actually renders from -- Stats
// plus a few pre-formatted/derived fields template logic shouldn't have to
// compute inline.
type TemplateData struct {
	*Stats
	FontBase64  string
	TZXPctStr   string
	TAPPctStr   string
	Highlight   *BlockStat // the most-used non-standard data-bearing block, if any
	HighlightOK bool       // whether that block is fully wired (fast-load and playback)
}

func renderReport(w io.Writer, s *Stats) error {
	data := TemplateData{
		Stats:      s,
		FontBase64: base64.StdEncoding.EncodeToString(zxFontTTF),
		TZXPctStr:  pctStr(s.TZXOK, s.TZXFiles),
		TAPPctStr:  pctStr(s.TAPOK, s.TAPFiles),
	}

	// The most commercially interesting fact this report can surface
	// automatically: which non-standard data-bearing block sees the most
	// real use, and whether it's actually wired -- computed from whatever
	// corpus was passed in, not assumed to be newdiv's own numbers.
	for i := range s.Blocks {
		b := &s.Blocks[i]
		if b.ID == 0x10 || b.Kind != "data" {
			continue
		}
		if data.Highlight == nil || b.Files > data.Highlight.Files {
			data.Highlight = b
		}
	}
	if data.Highlight != nil {
		data.HighlightOK = data.Highlight.FastLoad == "yes" && data.Highlight.Playback == "yes"
	}

	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"statusClass": statusClass,
		"statusLabel": statusLabel,
		"commas":      commas,
	}).Parse(reportTemplate)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}
	return tmpl.Execute(w, data)
}

func pctStr(ok, total int) string {
	if total == 0 {
		return "n/a"
	}
	if ok == total {
		return "100%"
	}
	return fmt.Sprintf("%.1f%%", 100*float64(ok)/float64(total))
}

func statusClass(v string) string {
	switch v {
	case "yes":
		return "status-fixed"
	case "no":
		return "status-open"
	default:
		return "status-decided"
	}
}

func statusLabel(v string) string {
	switch v {
	case "yes":
		return "✓ yes"
	case "no":
		return "✗ no"
	default:
		return "— n/a"
	}
}

// commas formats an int with thousands separators (1234 -> "1,234").
func commas(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return string(out)
}
