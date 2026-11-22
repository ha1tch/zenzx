// Command depmap reports the file-level dependency structure of one Go
// package under one build configuration: which file references which, how the
// references fall between named clusters of files, which names are declared
// once per build tag (the seams between configurations), the package-level
// variables, and the process-level calls (os.Exit, flag, log, fmt.Print...)
// that tie a package to being a program.
//
// It exists to measure an extraction: run it before moving code and again
// afterwards, and the references that cross the boundary should only shrink.
//
//	depmap -dir ../zenzx -tags headless -clusters docs/machine_clusters.json
//	depmap -dir ../zenzx -goos windows -tags purego -alt-tags headless -alt-goos linux
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Config selects which files of one package a build configuration compiles.
type Config struct {
	Dir    string
	GOOS   string
	GOARCH string
	Tags   []string
	Cgo    bool
}

func (c Config) context() build.Context {
	ctx := build.Default
	if c.GOOS != "" {
		ctx.GOOS = c.GOOS
	}
	if c.GOARCH != "" {
		ctx.GOARCH = c.GOARCH
	}
	ctx.BuildTags = append([]string(nil), c.Tags...)
	ctx.CgoEnabled = c.Cgo
	return ctx
}

// Files returns the base names of the non-test Go files the configuration
// compiles, honouring //go:build lines and GOOS/GOARCH file-name suffixes.
func (c Config) Files() ([]string, error) {
	ctx := c.context()
	entries, err := os.ReadDir(c.Dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		ok, err := ctx.MatchFile(c.Dir, n)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out, nil
}

// Edge counts the references from one file to names declared in another.
type Edge struct {
	Count int
	Names map[string]int
}

// Global is a package-level variable.
type Global struct{ Name, File, Type string }

// Result is the analysis of one configuration.
type Result struct {
	Files      []string
	Edges      map[[2]string]*Edge // [referencing file, declaring file]
	Globals    []Global
	Calls      map[string]map[string]int // file -> "os.Exit" -> count
	TypeErrors []string
}

// Analyze type-checks the package under the configuration and collects the
// cross-file references. Type errors do not stop it; they are reported, since
// a partial map is still useful but must be known to be partial.
func Analyze(c Config) (*Result, error) {
	files, err := c.Files()
	if err != nil {
		return nil, err
	}
	saved := build.Default
	build.Default = c.context() // the source importer reads this
	defer func() { build.Default = saved }()

	// go/build resolves module imports by running `go list` in the current
	// directory, so analyse from inside the target package's own module.
	abs, err := filepath.Abs(c.Dir)
	if err != nil {
		return nil, err
	}
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if err := os.Chdir(abs); err != nil {
		return nil, err
	}
	defer func() { _ = os.Chdir(wd) }()

	fset := token.NewFileSet()
	var asts []*ast.File
	for _, n := range files {
		f, err := parser.ParseFile(fset, filepath.Join(abs, n), nil, 0)
		if err != nil {
			return nil, err
		}
		asts = append(asts, f)
	}
	res := &Result{Files: files, Edges: map[[2]string]*Edge{}, Calls: map[string]map[string]int{}}
	conf := types.Config{
		Importer:    importer.ForCompiler(fset, "source", nil),
		FakeImportC: true,
		Error:       func(err error) { res.TypeErrors = append(res.TypeErrors, err.Error()) },
	}
	info := &types.Info{Uses: map[*ast.Ident]types.Object{}}
	pkg, _ := conf.Check("pkg", fset, asts, info)
	if pkg == nil {
		return nil, fmt.Errorf("type-check produced no package")
	}
	base := func(p token.Pos) string { return filepath.Base(fset.Position(p).Filename) }

	for id, obj := range info.Uses {
		if obj.Pkg() != pkg || !obj.Pos().IsValid() {
			continue
		}
		member := false
		switch o := obj.(type) {
		case *types.Var:
			member = o.IsField()
		case *types.Func:
			if sig, ok := o.Type().(*types.Signature); ok {
				member = sig.Recv() != nil
			}
		}
		if !member && obj.Parent() != pkg.Scope() {
			continue // a local
		}
		from, to := base(id.Pos()), base(obj.Pos())
		if from == to {
			continue
		}
		k := [2]string{from, to}
		e := res.Edges[k]
		if e == nil {
			e = &Edge{Names: map[string]int{}}
			res.Edges[k] = e
		}
		e.Count++
		e.Names[obj.Name()]++
	}

	for _, name := range pkg.Scope().Names() {
		if v, ok := pkg.Scope().Lookup(name).(*types.Var); ok {
			res.Globals = append(res.Globals, Global{
				Name: name, File: base(v.Pos()),
				Type: types.TypeString(v.Type(), func(*types.Package) string { return "" }),
			})
		}
	}

	for i, f := range asts {
		fname := files[i]
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			x, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			pn, ok := info.Uses[x].(*types.PkgName)
			if !ok {
				return true
			}
			path, name := pn.Imported().Path(), sel.Sel.Name
			if path == "os" || path == "flag" || path == "log" || (path == "fmt" && strings.HasPrefix(name, "Print")) {
				if res.Calls[fname] == nil {
					res.Calls[fname] = map[string]int{}
				}
				res.Calls[fname][path+"."+name]++
			}
			return true
		})
	}
	return res, nil
}

// Seam is a name declared in a file exclusive to one configuration and again
// in a file exclusive to the other: two definitions behind one name.
type Seam struct{ Name, FileA, FileB string }

func topLevelNames(path string) ([]string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil || len(d.Recv.List) == 0 {
				out = append(out, d.Name.Name)
				continue
			}
			out = append(out, recvName(d.Recv.List[0].Type)+"."+d.Name.Name)
		case *ast.GenDecl:
			for _, s := range d.Specs {
				switch s := s.(type) {
				case *ast.TypeSpec:
					out = append(out, s.Name.Name)
				case *ast.ValueSpec:
					for _, n := range s.Names {
						if n.Name != "_" {
							out = append(out, n.Name)
						}
					}
				}
			}
		}
	}
	return out, nil
}

func recvName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return recvName(t.X)
	case *ast.IndexExpr:
		return recvName(t.X)
	case *ast.IndexListExpr:
		return recvName(t.X)
	case *ast.Ident:
		return t.Name
	}
	return "?"
}

// Seams compares two configurations of the same directory.
func Seams(a, b Config) ([]Seam, error) {
	fa, err := a.Files()
	if err != nil {
		return nil, err
	}
	fb, err := b.Files()
	if err != nil {
		return nil, err
	}
	inB, inA := map[string]bool{}, map[string]bool{}
	for _, f := range fb {
		inB[f] = true
	}
	for _, f := range fa {
		inA[f] = true
	}
	collect := func(dir string, files []string, other map[string]bool) (map[string]string, error) {
		m := map[string]string{}
		for _, f := range files {
			if other[f] {
				continue
			}
			names, err := topLevelNames(filepath.Join(dir, f))
			if err != nil {
				return nil, err
			}
			for _, n := range names {
				m[n] = f
			}
		}
		return m, nil
	}
	onlyA, err := collect(a.Dir, fa, inB)
	if err != nil {
		return nil, err
	}
	onlyB, err := collect(b.Dir, fb, inA)
	if err != nil {
		return nil, err
	}
	var out []Seam
	for n, f := range onlyA {
		if g, ok := onlyB[n]; ok {
			out = append(out, Seam{Name: n, FileA: f, FileB: g})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func fanSets(r *Result) (in, out map[string]map[string]bool) {
	in, out = map[string]map[string]bool{}, map[string]map[string]bool{}
	for k := range r.Edges {
		if out[k[0]] == nil {
			out[k[0]] = map[string]bool{}
		}
		if in[k[1]] == nil {
			in[k[1]] = map[string]bool{}
		}
		out[k[0]][k[1]] = true
		in[k[1]][k[0]] = true
	}
	return in, out
}

type pair struct {
	From, To string
	Count    int
	Names    map[string]int
}

// ClusterEdges folds the file edges into edges between clusters, dropping
// those inside one cluster. Files in no cluster fall under "(unassigned)".
func ClusterEdges(r *Result, clusters map[string][]string) []pair {
	of := map[string]string{}
	for name, fs := range clusters {
		for _, f := range fs {
			of[f] = name
		}
	}
	get := func(f string) string {
		if c, ok := of[f]; ok {
			return c
		}
		return "(unassigned)"
	}
	agg := map[[2]string]*pair{}
	for k, e := range r.Edges {
		a, b := get(k[0]), get(k[1])
		if a == b {
			continue
		}
		p := agg[[2]string{a, b}]
		if p == nil {
			p = &pair{From: a, To: b, Names: map[string]int{}}
			agg[[2]string{a, b}] = p
		}
		p.Count += e.Count
		for n, c := range e.Names {
			p.Names[n] += c
		}
	}
	var out []pair
	for _, p := range agg {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].From+out[i].To < out[j].From+out[j].To
	})
	return out
}

// Frontier measures the references from clusters whose name starts with
// prefix into clusters whose name does not: how many references, how many
// distinct names they use, and how many of those names are unexported. The
// last figure is what would have to be exported if the prefixed clusters
// moved to a package of their own.
func Frontier(r *Result, clusters map[string][]string, prefix string) (refs, names, unexported int) {
	seen := map[string]bool{}
	for _, p := range ClusterEdges(r, clusters) {
		if strings.HasPrefix(p.From, prefix) && !strings.HasPrefix(p.To, prefix) {
			refs += p.Count
			for n := range p.Names {
				seen[n] = true
			}
		}
	}
	for n := range seen {
		if c := n[0]; c >= 'a' && c <= 'z' || c == '_' {
			unexported++
		}
	}
	return refs, len(seen), unexported
}

func topNames(m map[string]int, n int) string {
	type kv struct {
		K string
		V int
	}
	var s []kv
	for k, v := range m {
		s = append(s, kv{k, v})
	}
	sort.Slice(s, func(i, j int) bool {
		if s[i].V != s[j].V {
			return s[i].V > s[j].V
		}
		return s[i].K < s[j].K
	})
	var parts []string
	for i, e := range s {
		if i == n {
			parts = append(parts, fmt.Sprintf("+%d more", len(s)-n))
			break
		}
		parts = append(parts, e.K)
	}
	return strings.Join(parts, ", ")
}

// Report writes the analysis as plain text.
func Report(w io.Writer, c Config, r *Result, clusters map[string][]string, seams []Seam, top int, frontier string) {
	fmt.Fprintf(w, "configuration: goos=%s tags=%v cgo=%v\n", orDefault(c.GOOS, build.Default.GOOS), c.Tags, c.Cgo)
	fmt.Fprintf(w, "files: %d, cross-file edges: %d, type errors: %d\n", len(r.Files), len(r.Edges), len(r.TypeErrors))
	for i, e := range r.TypeErrors {
		if i == 5 {
			fmt.Fprintf(w, "  ... %d more\n", len(r.TypeErrors)-5)
			break
		}
		fmt.Fprintf(w, "  type error: %s\n", e)
	}

	in, out := fanSets(r)
	refsIn := map[string]int{}
	for k, e := range r.Edges {
		refsIn[k[1]] += e.Count
	}
	type row struct {
		File            string
		RefsIn, In, Out int
	}
	var rows []row
	for _, f := range r.Files {
		rows = append(rows, row{f, refsIn[f], len(in[f]), len(out[f])})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].RefsIn != rows[j].RefsIn {
			return rows[i].RefsIn > rows[j].RefsIn
		}
		return rows[i].File < rows[j].File
	})
	fmt.Fprintf(w, "\nmost referenced files (refs in, files in, files out):\n")
	for i, x := range rows {
		if i == top {
			break
		}
		fmt.Fprintf(w, "  %-28s %5d %4d %4d\n", x.File, x.RefsIn, x.In, x.Out)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Out != rows[j].Out {
			return rows[i].Out > rows[j].Out
		}
		return rows[i].File < rows[j].File
	})
	fmt.Fprintf(w, "\nwidest dependencies (files out):\n")
	for i, x := range rows {
		if i == top {
			break
		}
		fmt.Fprintf(w, "  %-28s %5d %4d %4d\n", x.File, x.RefsIn, x.In, x.Out)
	}

	if len(clusters) > 0 {
		fmt.Fprintf(w, "\nreferences between clusters (from -> to: count, most used names):\n")
		for _, p := range ClusterEdges(r, clusters) {
			fmt.Fprintf(w, "  %-18s -> %-18s %5d  %s\n", p.From, p.To, p.Count, topNames(p.Names, 6))
		}
	}

	if frontier != "" && len(clusters) > 0 {
		refs, names, unexp := Frontier(r, clusters, frontier)
		fmt.Fprintf(w, "\nfrontier %q: %d references into the other clusters use %d distinct names, %d of them unexported\n", frontier, refs, names, unexp)
	}

	fmt.Fprintf(w, "\npackage-level variables: %d\n", len(r.Globals))
	byFile := map[string][]string{}
	for _, g := range r.Globals {
		byFile[g.File] = append(byFile[g.File], g.Name+" "+g.Type)
	}
	var gf []string
	for f := range byFile {
		gf = append(gf, f)
	}
	sort.Strings(gf)
	for _, f := range gf {
		fmt.Fprintf(w, "  %-28s %s\n", f, strings.Join(byFile[f], "; "))
	}

	fmt.Fprintf(w, "\nprocess-level calls by file:\n")
	var cf []string
	for f := range r.Calls {
		cf = append(cf, f)
	}
	sort.Strings(cf)
	for _, f := range cf {
		var parts []string
		for k, v := range r.Calls[f] {
			parts = append(parts, fmt.Sprintf("%s x%d", k, v))
		}
		sort.Strings(parts)
		fmt.Fprintf(w, "  %-28s %s\n", f, strings.Join(parts, ", "))
	}

	if seams != nil {
		fmt.Fprintf(w, "\nseams: %d names declared once per configuration:\n", len(seams))
		for _, s := range seams {
			fmt.Fprintf(w, "  %-34s %-26s %s\n", s.Name, s.FileA, s.FileB)
		}
	}
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func splitTags(s string) []string {
	var out []string
	for _, t := range strings.Split(s, ",") {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func main() {
	dir := flag.String("dir", "", "package directory to analyse (required)")
	goos := flag.String("goos", "", "GOOS for the configuration (default: the host's)")
	tags := flag.String("tags", "", "comma-separated build tags")
	cgo := flag.Bool("cgo", false, "treat cgo as enabled")
	clustersPath := flag.String("clusters", "", "JSON file mapping a cluster name to its file names")
	altGOOS := flag.String("alt-goos", "", "compare against a second configuration with this GOOS (enables the seams report)")
	altTags := flag.String("alt-tags", "", "tags of the second configuration")
	top := flag.Int("top", 12, "rows in the ranked tables")
	frontier := flag.String("frontier", "", "cluster-name prefix: report what the matching clusters use from all the others")
	flag.Parse()
	if *dir == "" {
		fmt.Fprintln(os.Stderr, "depmap: -dir is required")
		os.Exit(2)
	}
	c := Config{Dir: *dir, GOOS: *goos, Tags: splitTags(*tags), Cgo: *cgo}
	r, err := Analyze(c)
	if err != nil {
		fmt.Fprintln(os.Stderr, "depmap:", err)
		os.Exit(1)
	}
	var clusters map[string][]string
	if *clustersPath != "" {
		b, err := os.ReadFile(*clustersPath)
		if err == nil {
			err = json.Unmarshal(b, &clusters)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "depmap:", err)
			os.Exit(1)
		}
	}
	var seams []Seam
	if *altGOOS != "" || *altTags != "" {
		seams, err = Seams(c, Config{Dir: *dir, GOOS: *altGOOS, Tags: splitTags(*altTags), Cgo: *cgo})
		if err != nil {
			fmt.Fprintln(os.Stderr, "depmap:", err)
			os.Exit(1)
		}
		if seams == nil {
			seams = []Seam{}
		}
	}
	Report(os.Stdout, c, r, clusters, seams, *top, *frontier)
}
