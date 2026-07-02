package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/conduit-io/conduit/internal/flow/ast"
	"github.com/conduit-io/conduit/internal/flow/diag"
	"github.com/conduit-io/conduit/internal/flow/parser"
	"github.com/conduit-io/conduit/internal/flow/sema"
)

// exitError carries a specific process exit code (see docs/70-error-handling.md).
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

func withExit(code int, err error) error {
	if err == nil {
		return nil
	}
	return &exitError{code: code, err: err}
}

func exitCodeFor(err error) int {
	var ee *exitError
	if errors.As(err, &ee) {
		return ee.code
	}
	return 1
}

// Exit code convention.
const (
	exitOK        = 0
	exitError_    = 1 // generic
	exitUsage     = 2
	exitDSL       = 3 // parse/semantic error
	exitRunFailed = 4 // workflow executed but a task failed
)

// parseFlowFile reads, parses and semantically analyzes a .flow file. It prints
// diagnostics to stderr and returns an error if analysis found errors.
func parseFlowFile(path string) (*ast.File, *diag.Diagnostics, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, withExit(exitUsage, fmt.Errorf("read %s: %w", path, err))
	}
	file, perr := parser.ParseBytes(path, data)
	if perr != nil {
		return nil, nil, withExit(exitDSL, perr)
	}
	d := sema.Analyze(file)
	return file, d, nil
}

// printDiagnostics writes diagnostics to stderr.
func printDiagnostics(d *diag.Diagnostics) {
	for _, it := range d.Items {
		fmt.Fprintln(os.Stderr, it.String())
	}
}

// selectWorkflow picks the named workflow or the sole workflow in the file.
func selectWorkflow(file *ast.File, name string) (*ast.Workflow, error) {
	wfs := file.Workflows()
	if len(wfs) == 0 {
		return nil, withExit(exitDSL, fmt.Errorf("no workflows defined"))
	}
	if name == "" {
		if len(wfs) == 1 {
			return wfs[0], nil
		}
		var names []string
		for _, w := range wfs {
			names = append(names, w.NameStr())
		}
		return nil, withExit(exitUsage, fmt.Errorf("multiple workflows (%v); specify --workflow", names))
	}
	for _, w := range wfs {
		if w.NameStr() == name {
			return w, nil
		}
	}
	return nil, withExit(exitUsage, fmt.Errorf("workflow %q not found", name))
}

// writeJSON prints v as indented JSON to stdout.
func writeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// parseParamFlags turns ["k=v", ...] into a map.
func parseParamFlags(kv []string) (map[string]string, error) {
	out := map[string]string{}
	for _, item := range kv {
		for i := 0; i < len(item); i++ {
			if item[i] == '=' {
				out[item[:i]] = item[i+1:]
				break
			}
			if i == len(item)-1 {
				return nil, fmt.Errorf("invalid --param %q: want key=value", item)
			}
		}
	}
	return out, nil
}
