package sema

import (
	"testing"

	"github.com/conduit-io/conduit/internal/flow/parser"
)

func analyze(t *testing.T, src string) []string {
	t.Helper()
	f, err := parser.ParseString("t.flow", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	d := Analyze(f)
	var codes []string
	for _, it := range d.Items {
		codes = append(codes, it.Code)
	}
	return codes
}

func has(codes []string, code string) bool {
	for _, c := range codes {
		if c == code {
			return true
		}
	}
	return false
}

func TestCleanWorkflowHasNoErrors(t *testing.T) {
	codes := analyze(t, `workflow "w" {
	  param n: string = "x"
	  task a { run: "echo hi" }
	  task b { depends_on: [a] run: "echo bye" }
	}`)
	for _, c := range codes {
		if c[len(c)-4] == 'E' {
			t.Fatalf("unexpected error diagnostic %s", c)
		}
	}
}

func TestUnknownDependency(t *testing.T) {
	if !has(analyze(t, `workflow "w" { task a { depends_on: [ghost] run: "x" } }`), "FLOW-E021") {
		t.Fatal("expected FLOW-E021")
	}
}

func TestDuplicateTask(t *testing.T) {
	if !has(analyze(t, `workflow "w" { task a { run: "x" } task a { run: "y" } }`), "FLOW-E020") {
		t.Fatal("expected FLOW-E020")
	}
}

func TestUnknownParamType(t *testing.T) {
	if !has(analyze(t, `workflow "w" { param p: frob = "x" task a { run: "z" } }`), "FLOW-E011") {
		t.Fatal("expected FLOW-E011")
	}
}

func TestNoActionWarning(t *testing.T) {
	if !has(analyze(t, `workflow "w" { task a { description: "nothing" } }`), "FLOW-W031") {
		t.Fatal("expected FLOW-W031")
	}
}
