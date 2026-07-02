package runtime

import (
	"context"
	"io"
	"testing"

	"github.com/conduit-io/conduit/internal/dag"
	"github.com/conduit-io/conduit/internal/flow/parser"
	"github.com/conduit-io/conduit/internal/model"
)

func run(t *testing.T, src string, params map[string]string) *model.Run {
	t.Helper()
	f, err := parser.ParseString("t.flow", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	wf := f.Workflows()[0]
	g, err := dag.Build(wf)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	eng, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := eng.Run(context.Background(), g, wf, Options{
		Params: params,
		Stdout: io.Discard,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return r
}

func status(r *model.Run, name string) model.Status {
	for _, tr := range r.Tasks {
		if tr.Name == name {
			return tr.Status
		}
	}
	return ""
}

func TestSuccessfulRun(t *testing.T) {
	r := run(t, `workflow "w" {
	  task a { run: "echo a" }
	  task b { depends_on: [a] run: "echo b" }
	}`, nil)
	if r.Status != model.StatusSucceeded {
		t.Fatalf("expected succeeded, got %s", r.Status)
	}
}

func TestWhenGuardSkips(t *testing.T) {
	r := run(t, `workflow "w" {
	  param on: bool = false
	  task a { when: "params.on" run: "echo a" }
	}`, nil)
	if status(r, "a") != model.StatusSkipped {
		t.Fatalf("expected a skipped, got %s", status(r, "a"))
	}
}

func TestWhenNegationRuns(t *testing.T) {
	// Regression for the bool-default bug: !false must be true.
	r := run(t, `workflow "w" {
	  param skip: bool = false
	  task a { when: "!params.skip" run: "echo a" }
	}`, nil)
	if status(r, "a") != model.StatusSucceeded {
		t.Fatalf("expected a succeeded, got %s", status(r, "a"))
	}
}

func TestFailurePropagates(t *testing.T) {
	r := run(t, `workflow "w" {
	  task a { run: "exit 1" }
	  task b { depends_on: [a] run: "echo b" }
	}`, nil)
	if status(r, "a") != model.StatusFailed {
		t.Fatalf("expected a failed, got %s", status(r, "a"))
	}
	if status(r, "b") != model.StatusSkipped {
		t.Fatalf("expected b skipped, got %s", status(r, "b"))
	}
	if r.Status != model.StatusFailed {
		t.Fatalf("expected run failed, got %s", r.Status)
	}
}

func TestParamOverrideAndInterpolation(t *testing.T) {
	r := run(t, `workflow "w" {
	  param name: string = "world"
	  task a { run: "echo ${params.name}" }
	}`, map[string]string{"name": "conduit"})
	// task succeeds and stdout captured contains the interpolated value
	for _, tr := range r.Tasks {
		if tr.Name == "a" && tr.Status != model.StatusSucceeded {
			t.Fatalf("expected success, got %s (%s)", tr.Status, tr.Error)
		}
	}
}

func TestContinueOnError(t *testing.T) {
	r := run(t, `workflow "w" {
	  task a { run: "exit 1" continue_on_error: true }
	  task b { depends_on: [a] run: "echo b" }
	}`, nil)
	if status(r, "a") != model.StatusSucceeded {
		t.Fatalf("expected a coerced to succeeded, got %s", status(r, "a"))
	}
	if status(r, "b") != model.StatusSucceeded {
		t.Fatalf("expected b to run, got %s", status(r, "b"))
	}
}
