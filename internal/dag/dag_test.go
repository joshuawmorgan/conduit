package dag

import (
	"testing"

	"github.com/conduit-io/conduit/internal/flow/parser"
)

func build(t *testing.T, src string) *Graph {
	t.Helper()
	f, err := parser.ParseString("t.flow", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	g, err := Build(f.Workflows()[0])
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	return g
}

func TestTopoSortRespectsDeps(t *testing.T) {
	g := build(t, `workflow "w" {
	  task a { run: "x" }
	  task b { depends_on: [a] run: "x" }
	  task c { depends_on: [b] run: "x" }
	}`)
	order, err := g.TopoSort()
	if err != nil {
		t.Fatal(err)
	}
	pos := map[string]int{}
	for i, n := range order {
		pos[n] = i
	}
	if !(pos["a"] < pos["b"] && pos["b"] < pos["c"]) {
		t.Fatalf("bad order: %v", order)
	}
}

func TestLayersGroupParallel(t *testing.T) {
	g := build(t, `workflow "w" {
	  task a { run: "x" }
	  task b { depends_on: [a] run: "x" }
	  task c { depends_on: [a] run: "x" }
	  task d { depends_on: [b, c] run: "x" }
	}`)
	layers, err := g.Layers()
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != 3 {
		t.Fatalf("expected 3 layers, got %d: %v", len(layers), layers)
	}
	if len(layers[1]) != 2 {
		t.Fatalf("expected 2 parallel tasks in layer 1, got %v", layers[1])
	}
}

func TestCycleRejected(t *testing.T) {
	f, _ := parser.ParseString("t.flow", `workflow "w" {
	  task a { depends_on: [b] run: "x" }
	  task b { depends_on: [a] run: "x" }
	}`)
	if _, err := Build(f.Workflows()[0]); err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestUnknownDependencyRejected(t *testing.T) {
	f, _ := parser.ParseString("t.flow", `workflow "w" {
	  task a { depends_on: [ghost] run: "x" }
	}`)
	if _, err := Build(f.Workflows()[0]); err == nil {
		t.Fatal("expected unknown-dependency error")
	}
}
