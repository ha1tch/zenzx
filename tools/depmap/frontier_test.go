package main

import "testing"

func TestFrontierCountsReferencesAndUnexportedNames(t *testing.T) {
	r := &Result{Edges: map[[2]string]*Edge{
		{"fe1.go", "core.go"}: {Count: 5, Names: map[string]int{"Run": 2, "state": 3}},
		{"fe2.go", "core.go"}: {Count: 2, Names: map[string]int{"state": 1, "cpu": 1}},
		{"fe1.go", "fe2.go"}:  {Count: 9, Names: map[string]int{"Draw": 9}},
		{"core.go", "fe1.go"}: {Count: 4, Names: map[string]int{"Hook": 4}},
	}}
	clusters := map[string][]string{"fe-a": {"fe1.go"}, "fe-b": {"fe2.go"}, "core": {"core.go"}}
	refs, names, unexp := Frontier(r, clusters, "fe-")
	if refs != 7 || names != 3 || unexp != 2 {
		t.Errorf("Frontier = (%d refs, %d names, %d unexported), want (7, 3, 2): fe-a/fe-b to each other and core to fe-* do not count", refs, names, unexp)
	}
}
