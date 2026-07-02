package parser

import "testing"

const sample = `
// CI workflow
workflow "ci" {
  description: "Build and test the project"
  param branch: string = "main"
  param retries: int = 2
  env: {
    GOFLAGS: "-mod=mod"
    CGO_ENABLED: "0"
  }

  task build {
    run: "go build ./..."
    timeout: "5m"
  }

  task test {
    depends_on: [build]
    when: "params.branch == \"main\""
    run: "go test ./..."
    retry: 2
  }
}
`

func TestParseSample(t *testing.T) {
	f, err := ParseString("ci.flow", sample)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	wfs := f.Workflows()
	if len(wfs) != 1 {
		t.Fatalf("expected 1 workflow, got %d", len(wfs))
	}
	wf := wfs[0]
	if wf.Name != `"ci"` && wf.Name != "ci" {
		t.Logf("workflow name raw = %q", wf.Name)
	}
	if got := len(wf.Params()); got != 2 {
		t.Errorf("expected 2 params, got %d", got)
	}
	tasks := wf.Tasks()
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if tasks[1].Name != "test" {
		t.Errorf("expected second task 'test', got %q", tasks[1].Name)
	}
	// depends_on on test
	var deps []string
	for _, a := range tasks[1].Attrs {
		if a.Name == "depends_on" {
			deps = a.Value.AsIdentList()
		}
	}
	if len(deps) != 1 || deps[0] != "build" {
		t.Errorf("expected depends_on [build], got %v", deps)
	}
}
