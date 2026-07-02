// Package dag builds and analyzes the task dependency graph for a workflow.
package dag

import (
	"fmt"
	"sort"
	"strings"

	"github.com/conduit-io/conduit/internal/flow/ast"
)

// Node is a task vertex in the DAG.
type Node struct {
	Name      string
	Task      *ast.Task
	DependsOn []string
}

// Graph is a directed acyclic graph of tasks.
type Graph struct {
	Workflow string
	Nodes    map[string]*Node
	Order    []string // declaration order for deterministic output
}

// Build constructs a DAG from a workflow AST and validates edges.
func Build(wf *ast.Workflow) (*Graph, error) {
	g := &Graph{Workflow: wf.NameStr(), Nodes: map[string]*Node{}}
	for _, t := range wf.Tasks() {
		if _, dup := g.Nodes[t.Name]; dup {
			return nil, fmt.Errorf("duplicate task %q", t.Name)
		}
		var deps []string
		if v := t.Attr("depends_on"); v != nil {
			deps = v.AsIdentList()
		}
		g.Nodes[t.Name] = &Node{Name: t.Name, Task: t, DependsOn: deps}
		g.Order = append(g.Order, t.Name)
	}
	// Validate dependency targets.
	for _, n := range g.Nodes {
		for _, d := range n.DependsOn {
			if _, ok := g.Nodes[d]; !ok {
				return nil, fmt.Errorf("task %q depends on unknown task %q", n.Name, d)
			}
		}
	}
	if _, err := g.TopoSort(); err != nil {
		return nil, err
	}
	return g, nil
}

// TopoSort returns tasks in a valid execution order (Kahn's algorithm) with
// deterministic tie-breaking by declaration order.
func (g *Graph) TopoSort() ([]string, error) {
	indeg := map[string]int{}
	rank := map[string]int{}
	for i, name := range g.Order {
		indeg[name] = 0
		rank[name] = i
	}
	for _, n := range g.Nodes {
		indeg[n.Name] = len(n.DependsOn)
	}
	// build reverse adjacency (dependency -> dependents)
	dependents := map[string][]string{}
	for _, n := range g.Nodes {
		for _, d := range n.DependsOn {
			dependents[d] = append(dependents[d], n.Name)
		}
	}
	var ready []string
	for name, deg := range indeg {
		if deg == 0 {
			ready = append(ready, name)
		}
	}
	sortByRank(ready, rank)

	var order []string
	for len(ready) > 0 {
		n := ready[0]
		ready = ready[1:]
		order = append(order, n)
		next := append([]string(nil), dependents[n]...)
		sortByRank(next, rank)
		for _, m := range next {
			indeg[m]--
			if indeg[m] == 0 {
				ready = append(ready, m)
			}
		}
		sortByRank(ready, rank)
	}
	if len(order) != len(g.Nodes) {
		return nil, fmt.Errorf("dependency cycle detected among tasks %v", g.remaining(order))
	}
	return order, nil
}

// Layers groups tasks into topological layers; tasks in the same layer have no
// mutual dependencies and may run concurrently.
func (g *Graph) Layers() ([][]string, error) {
	if _, err := g.TopoSort(); err != nil {
		return nil, err
	}
	indeg := map[string]int{}
	rank := map[string]int{}
	for i, name := range g.Order {
		rank[name] = i
	}
	for _, n := range g.Nodes {
		indeg[n.Name] = len(n.DependsOn)
	}
	dependents := map[string][]string{}
	for _, n := range g.Nodes {
		for _, d := range n.DependsOn {
			dependents[d] = append(dependents[d], n.Name)
		}
	}
	var layers [][]string
	remaining := len(g.Nodes)
	done := map[string]bool{}
	for remaining > 0 {
		var layer []string
		for name, deg := range indeg {
			if deg == 0 && !done[name] {
				layer = append(layer, name)
			}
		}
		if len(layer) == 0 {
			return nil, fmt.Errorf("dependency cycle detected")
		}
		sortByRank(layer, rank)
		for _, name := range layer {
			done[name] = true
			remaining--
			for _, m := range dependents[name] {
				indeg[m]--
			}
		}
		layers = append(layers, layer)
	}
	return layers, nil
}

// Roots returns tasks with no dependencies.
func (g *Graph) Roots() []string {
	var roots []string
	for _, name := range g.Order {
		if len(g.Nodes[name].DependsOn) == 0 {
			roots = append(roots, name)
		}
	}
	return roots
}

// DOT renders the graph in Graphviz DOT format.
func (g *Graph) DOT() string {
	var b strings.Builder
	fmt.Fprintf(&b, "digraph %q {\n  rankdir=LR;\n", g.Workflow)
	for _, name := range g.Order {
		fmt.Fprintf(&b, "  %q;\n", name)
	}
	for _, name := range g.Order {
		for _, d := range g.Nodes[name].DependsOn {
			fmt.Fprintf(&b, "  %q -> %q;\n", d, name)
		}
	}
	b.WriteString("}\n")
	return b.String()
}

// Mermaid renders the graph as a Mermaid flowchart.
func (g *Graph) Mermaid() string {
	var b strings.Builder
	b.WriteString("flowchart LR\n")
	for _, name := range g.Order {
		fmt.Fprintf(&b, "  %s[%s]\n", ident(name), name)
	}
	for _, name := range g.Order {
		for _, d := range g.Nodes[name].DependsOn {
			fmt.Fprintf(&b, "  %s --> %s\n", ident(d), ident(name))
		}
	}
	return b.String()
}

func (g *Graph) remaining(order []string) []string {
	seen := map[string]bool{}
	for _, n := range order {
		seen[n] = true
	}
	var rem []string
	for _, name := range g.Order {
		if !seen[name] {
			rem = append(rem, name)
		}
	}
	return rem
}

func sortByRank(s []string, rank map[string]int) {
	sort.SliceStable(s, func(i, j int) bool { return rank[s[i]] < rank[s[j]] })
}

func ident(s string) string {
	return "n_" + strings.NewReplacer("-", "_", ".", "_", " ", "_").Replace(s)
}
