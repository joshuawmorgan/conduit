// Package sema performs semantic analysis over a FlowDSL AST: name resolution,
// structural validation, dependency checking, cycle detection and lint
// warnings. It emits diagnostics and never mutates the AST.
package sema

import (
	"github.com/conduit-io/conduit/internal/flow/ast"
	"github.com/conduit-io/conduit/internal/flow/diag"
)

// Known scalar types for params.
var knownTypes = map[string]bool{
	"string": true, "int": true, "float": true, "bool": true, "list": true, "map": true,
}

// Recognized task attribute names.
var taskAttrs = map[string]bool{
	"run": true, "uses": true, "with": true, "depends_on": true, "when": true,
	"timeout": true, "retry": true, "env": true, "description": true,
	"continue_on_error": true, "outputs": true,
}

// Recognized workflow-level attribute names.
var workflowAttrs = map[string]bool{
	"description": true, "env": true, "timeout": true,
}

// Analyze validates the file and returns diagnostics.
func Analyze(file *ast.File) *diag.Diagnostics {
	d := &diag.Diagnostics{}
	if file == nil {
		return d
	}

	names := map[string]bool{}
	for _, wf := range file.Workflows() {
		if wf.NameStr() == "" {
			d.Errorf("FLOW-E001", wf.Pos, "workflow must have a non-empty name")
		}
		if names[wf.NameStr()] {
			d.Errorf("FLOW-E002", wf.Pos, "duplicate workflow name %q", wf.NameStr())
		}
		names[wf.NameStr()] = true
		analyzeWorkflow(wf, d)
	}
	if len(file.Workflows()) == 0 {
		d.Warnf("FLOW-W001", file.Pos, "file declares no workflows")
	}
	d.Sort()
	return d
}

func analyzeWorkflow(wf *ast.Workflow, d *diag.Diagnostics) {
	// params
	params := map[string]bool{}
	for _, p := range wf.Params() {
		if params[p.Name] {
			d.Errorf("FLOW-E010", p.Pos, "duplicate param %q", p.Name)
		}
		params[p.Name] = true
		if !knownTypes[p.Type] {
			d.Errorf("FLOW-E011", p.Pos, "unknown param type %q for %q", p.Type, p.Name)
		}
	}

	// workflow attrs
	for _, a := range wf.Attrs() {
		if !workflowAttrs[a.Name] {
			d.Warnf("FLOW-W010", a.Pos, "unknown workflow attribute %q", a.Name)
		}
	}

	// tasks
	taskNames := map[string]bool{}
	tasks := wf.Tasks()
	for _, t := range tasks {
		if taskNames[t.Name] {
			d.Errorf("FLOW-E020", t.Pos, "duplicate task %q", t.Name)
		}
		taskNames[t.Name] = true
	}
	if len(tasks) == 0 {
		d.Warnf("FLOW-W020", wf.Pos, "workflow %q has no tasks", wf.NameStr())
	}

	for _, t := range tasks {
		analyzeTask(t, taskNames, d)
	}

	// cycle detection over depends_on
	if cyc := detectCycle(tasks); len(cyc) > 0 {
		d.Errorf("FLOW-E030", wf.Pos, "dependency cycle detected: %v", cyc)
	}
}

func analyzeTask(t *ast.Task, taskNames map[string]bool, d *diag.Diagnostics) {
	hasAction := false
	for _, a := range t.Attrs {
		if !taskAttrs[a.Name] {
			d.Warnf("FLOW-W030", a.Pos, "unknown task attribute %q", a.Name)
		}
		switch a.Name {
		case "run", "uses":
			hasAction = true
		case "depends_on":
			for _, dep := range a.Value.AsIdentList() {
				if !taskNames[dep] {
					d.Errorf("FLOW-E021", a.Pos, "task %q depends on unknown task %q", t.Name, dep)
				}
				if dep == t.Name {
					d.Errorf("FLOW-E022", a.Pos, "task %q cannot depend on itself", t.Name)
				}
			}
		case "retry":
			if _, ok := a.Value.AsInt(); !ok {
				d.Errorf("FLOW-E023", a.Pos, "retry must be an integer")
			}
		}
	}
	if !hasAction {
		d.Warnf("FLOW-W031", t.Pos, "task %q has neither run nor uses; it will be a no-op", t.Name)
	}
}

// detectCycle returns a cycle path if the depends_on graph is cyclic.
func detectCycle(tasks []*ast.Task) []string {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[string]int{}
	adj := map[string][]string{}
	for _, t := range tasks {
		adj[t.Name] = nil
		if v := t.Attr("depends_on"); v != nil {
			adj[t.Name] = v.AsIdentList()
		}
	}
	var stack []string
	var dfs func(n string) []string
	dfs = func(n string) []string {
		color[n] = gray
		stack = append(stack, n)
		for _, m := range adj[n] {
			if _, known := adj[m]; !known {
				continue
			}
			switch color[m] {
			case white:
				if c := dfs(m); c != nil {
					return c
				}
			case gray:
				return append(append([]string{}, stack...), m)
			}
		}
		stack = stack[:len(stack)-1]
		color[n] = black
		return nil
	}
	for _, t := range tasks {
		if color[t.Name] == white {
			if c := dfs(t.Name); c != nil {
				return c
			}
		}
	}
	return nil
}
