package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestAnalyzeCountsCrossFileReferences(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"a.go": "package p\n\nfunc A() { B() }\n",
		"b.go": "package p\n\nvar G int\n\nfunc B() { G++ }\n",
		"c.go": "package p\n\ntype T struct{ N int }\n\nfunc (t *T) Get() int { return t.N }\n\nfunc C() { A(); B(); _ = G }\n",
		"d.go": "//go:build ignore\n\npackage p\n\nfunc D() { A() }\n",
	})
	r, err := Analyze(Config{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.TypeErrors) != 0 {
		t.Fatalf("unexpected type errors: %v", r.TypeErrors)
	}
	if len(r.Files) != 3 {
		t.Errorf("files = %v, want a.go b.go c.go (d.go is build-ignored)", r.Files)
	}
	want := map[[2]string]int{
		{"a.go", "b.go"}: 1, // B
		{"c.go", "a.go"}: 1, // A
		{"c.go", "b.go"}: 2, // B, G
	}
	for k, n := range want {
		e := r.Edges[k]
		if e == nil || e.Count != n {
			t.Errorf("edge %v = %+v, want count %d", k, e, n)
		}
	}
	if e := r.Edges[[2]string{"b.go", "b.go"}]; e != nil {
		t.Errorf("same-file reference reported as an edge: %+v", e)
	}
	if len(r.Globals) != 1 || r.Globals[0].Name != "G" || r.Globals[0].File != "b.go" {
		t.Errorf("globals = %+v, want G in b.go", r.Globals)
	}
}

func TestFieldAndMethodReferencesCrossFiles(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"t.go": "package p\n\ntype T struct{ N int }\n\nfunc (t *T) Get() int { return t.N }\n",
		"u.go": "package p\n\nfunc U(t *T) int { t.N = 1; return t.Get() }\n",
	})
	r, err := Analyze(Config{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	e := r.Edges[[2]string{"u.go", "t.go"}]
	if e == nil || e.Names["N"] != 1 || e.Names["Get"] != 1 {
		t.Errorf("u.go -> t.go = %+v, want the field N and the method Get", e)
	}
}

func TestSeamsFindNamesDeclaredOncePerConfiguration(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"core.go": "package p\n\nfunc Shared() {}\n",
		"gui.go":  "//go:build gui\n\npackage p\n\ntype Display struct{}\n\nfunc (d *Display) Draw() {}\n\nfunc OnlyGUI() {}\n",
		"head.go": "//go:build !gui\n\npackage p\n\ntype Display struct{}\n\nfunc (d *Display) Draw() {}\n\nfunc OnlyHeadless() {}\n",
	})
	seams, err := Seams(Config{Dir: dir, Tags: []string{"gui"}}, Config{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][2]string{}
	for _, s := range seams {
		got[s.Name] = [2]string{s.FileA, s.FileB}
	}
	for _, n := range []string{"Display", "Display.Draw"} {
		if g, ok := got[n]; !ok || g != [2]string{"gui.go", "head.go"} {
			t.Errorf("seam %q = %v (found %v), want gui.go / head.go", n, g, ok)
		}
	}
	for _, n := range []string{"Shared", "OnlyGUI", "OnlyHeadless"} {
		if _, ok := got[n]; ok {
			t.Errorf("%q is not a seam", n)
		}
	}
}

func TestClusterEdgesDropsInternalAndNamesUnassigned(t *testing.T) {
	r := &Result{Edges: map[[2]string]*Edge{
		{"a.go", "b.go"}: {Count: 3, Names: map[string]int{"X": 3}},
		{"a.go", "c.go"}: {Count: 2, Names: map[string]int{"Y": 2}},
		{"c.go", "z.go"}: {Count: 1, Names: map[string]int{"Z": 1}},
	}}
	got := ClusterEdges(r, map[string][]string{"core": {"a.go", "b.go"}, "other": {"c.go"}})
	if len(got) != 2 {
		t.Fatalf("pairs = %+v, want core->other and other->(unassigned)", got)
	}
	if got[0].From != "core" || got[0].To != "other" || got[0].Count != 2 {
		t.Errorf("first pair = %+v", got[0])
	}
	if got[1].To != "(unassigned)" {
		t.Errorf("second pair = %+v, want the unassigned target", got[1])
	}
}
