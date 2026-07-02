// Package runtime executes a workflow DAG: it schedules ready tasks under a
// concurrency limit, evaluates CEL `when` guards, interpolates ${ } expressions,
// runs shell / builtin / plugin actions, and records state.
package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/conduit-io/conduit/internal/cel"
	"github.com/conduit-io/conduit/internal/dag"
	"github.com/conduit-io/conduit/internal/flow/ast"
	"github.com/conduit-io/conduit/internal/model"
)

// ActionInvoker executes a `uses:` plugin action. Implemented by the plugin
// manager; may be nil when no plugins are configured.
type ActionInvoker interface {
	Invoke(ctx context.Context, action string, inputs map[string]string) (outputs map[string]string, stdout string, err error)
}

// Options configures a run.
type Options struct {
	Concurrency int
	DryRun      bool
	Params      map[string]string
	Env         map[string]string
	Stdout      io.Writer
	Invoker     ActionInvoker
	CEL         *cel.Engine
}

// Engine executes workflows.
type Engine struct {
	cel *cel.Engine
}

// New creates an execution engine, building a CEL engine if one is not supplied.
func New(celEng *cel.Engine) (*Engine, error) {
	if celEng == nil {
		var err error
		celEng, err = cel.New()
		if err != nil {
			return nil, err
		}
	}
	return &Engine{cel: celEng}, nil
}

// NewRunID returns a random run identifier.
func NewRunID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "run_" + hex.EncodeToString(b)
}

// Run executes the workflow described by g and wf.
func (e *Engine) Run(ctx context.Context, g *dag.Graph, wf *ast.Workflow, opts Options) (*model.Run, error) {
	if opts.Concurrency <= 0 {
		opts.Concurrency = runtime.NumCPU()
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	celEng := opts.CEL
	if celEng == nil {
		celEng = e.cel
	}

	// Resolve parameter values (defaults overlaid by supplied params).
	params := resolveParams(wf, opts.Params)
	baseEnv := mergeEnv(wf, opts.Env, celEng, params)

	run := &model.Run{
		ID:        NewRunID(),
		Workflow:  wf.NameStr(),
		Status:    model.StatusRunning,
		Params:    params,
		StartedAt: time.Now(),
		DryRun:    opts.DryRun,
	}
	order, _ := g.TopoSort()
	trByName := map[string]*model.TaskRun{}
	for _, name := range order {
		tr := &model.TaskRun{Name: name, Status: model.StatusPending, DependsOn: g.Nodes[name].DependsOn}
		trByName[name] = tr
		run.Tasks = append(run.Tasks, tr)
	}

	var (
		mu      sync.Mutex // guards status + outputs
		status  = map[string]model.Status{}
		outputs = map[string]any{}
		running int
		doneCh  = make(chan taskResult)
		sem     = make(chan struct{}, opts.Concurrency)
	)
	for _, name := range order {
		status[name] = model.StatusPending
	}

	allDepsTerminal := func(name string) bool {
		for _, d := range g.Nodes[name].DependsOn {
			if !status[d].Terminal() {
				return false
			}
		}
		return true
	}
	anyDepBad := func(name string) bool {
		for _, d := range g.Nodes[name].DependsOn {
			if status[d] != model.StatusSucceeded {
				return true
			}
		}
		return false
	}

	for {
		mu.Lock()
		progressed := true
		for progressed {
			progressed = false
			for _, name := range order {
				if status[name] != model.StatusPending || !allDepsTerminal(name) {
					continue
				}
				if anyDepBad(name) {
					status[name] = model.StatusSkipped
					trByName[name].Status = model.StatusSkipped
					trByName[name].Error = "skipped: upstream dependency did not succeed"
					progressed = true
					continue
				}
				status[name] = model.StatusRunning
				running++
				progressed = true
				node := g.Nodes[name]
				actEnv := snapshotActivation(params, baseEnv, outputs)
				go func() {
					sem <- struct{}{}
					defer func() { <-sem }()
					tr := e.execTask(ctx, node, wf, opts, celEng, actEnv)
					doneCh <- taskResult{name: node.Name, tr: tr}
				}()
			}
		}
		if running == 0 {
			mu.Unlock()
			break
		}
		mu.Unlock()

		res := <-doneCh
		mu.Lock()
		running--
		status[res.name] = res.tr.Status
		*trByName[res.name] = *res.tr
		if res.tr.Outputs != nil {
			outputs[res.name] = map[string]any{"outputs": toAnyMap(res.tr.Outputs), "status": string(res.tr.Status)}
		} else {
			outputs[res.name] = map[string]any{"status": string(res.tr.Status)}
		}
		mu.Unlock()
	}

	run.FinishedAt = time.Now()
	run.Status = model.StatusSucceeded
	for _, tr := range run.Tasks {
		if tr.Status == model.StatusFailed {
			run.Status = model.StatusFailed
			run.Error = fmt.Sprintf("task %q failed", tr.Name)
			break
		}
	}
	return run, nil
}

type taskResult struct {
	name string
	tr   *model.TaskRun
}

// execTask runs a single task with when-guard, interpolation, retries and timeout.
func (e *Engine) execTask(ctx context.Context, node *dag.Node, wf *ast.Workflow, opts Options, celEng *cel.Engine, act cel.Activation) *model.TaskRun {
	t := node.Task
	tr := &model.TaskRun{Name: t.Name, DependsOn: node.DependsOn, Status: model.StatusRunning, StartedAt: time.Now()}

	// when guard
	if v := t.Attr("when"); v != nil {
		expr, _ := v.AsString()
		ok, err := celEng.EvalBool(expr, act)
		if err != nil {
			tr.Status = model.StatusFailed
			tr.Error = fmt.Sprintf("when: %v", err)
			tr.FinishedAt = time.Now()
			return tr
		}
		if !ok {
			tr.Status = model.StatusSkipped
			tr.Error = "skipped: when guard is false"
			tr.FinishedAt = time.Now()
			return tr
		}
	}

	// timeout
	taskCtx := ctx
	if v := t.Attr("timeout"); v != nil {
		if s, ok := v.AsString(); ok {
			if d, err := time.ParseDuration(s); err == nil {
				var cancel context.CancelFunc
				taskCtx, cancel = context.WithTimeout(ctx, d)
				defer cancel()
			}
		}
	}

	retries := 0
	if v := t.Attr("retry"); v != nil {
		if n, ok := v.AsInt(); ok {
			retries = int(n)
		}
	}
	continueOnError := false
	if v := t.Attr("continue_on_error"); v != nil && v.Bool != nil {
		continueOnError = v.Bool.Bool()
	}

	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		tr.Attempts = attempt + 1
		outs, stdout, stderr, code, err := e.runAction(taskCtx, t, opts, celEng, act)
		tr.Stdout, tr.Stderr, tr.ExitCode = stdout, stderr, code
		if outs != nil {
			tr.Outputs = outs
		}
		if err == nil {
			tr.Status = model.StatusSucceeded
			tr.FinishedAt = time.Now()
			return tr
		}
		lastErr = err
		if attempt < retries {
			backoff := time.Duration(1<<attempt) * 200 * time.Millisecond
			select {
			case <-taskCtx.Done():
			case <-time.After(backoff):
			}
		}
	}

	tr.FinishedAt = time.Now()
	tr.Error = lastErr.Error()
	if continueOnError {
		tr.Status = model.StatusSucceeded
		tr.Error = "continue_on_error: " + tr.Error
	} else {
		tr.Status = model.StatusFailed
	}
	return tr
}

// runAction dispatches to shell (run:) or plugin/builtin (uses:).
func (e *Engine) runAction(ctx context.Context, t *ast.Task, opts Options, celEng *cel.Engine, act cel.Activation) (outputs map[string]string, stdout, stderr string, code int, err error) {
	if v := t.Attr("run"); v != nil {
		raw, _ := v.AsString()
		cmdStr, ierr := celEng.Interpolate(raw, act)
		if ierr != nil {
			return nil, "", "", 1, ierr
		}
		if opts.DryRun {
			fmt.Fprintf(opts.Stdout, "[dry-run] %s: %s\n", t.Name, cmdStr)
			return nil, "(dry-run) " + cmdStr, "", 0, nil
		}
		return runShell(ctx, cmdStr, act.Env, opts.Stdout, t.Name)
	}

	if v := t.Attr("uses"); v != nil {
		action, _ := v.AsString()
		inputs := map[string]string{}
		if w := t.Attr("with"); w != nil {
			for k, raw := range w.AsMap() {
				val, ierr := celEng.Interpolate(raw, act)
				if ierr != nil {
					return nil, "", "", 1, ierr
				}
				inputs[k] = val
			}
		}
		if opts.DryRun {
			fmt.Fprintf(opts.Stdout, "[dry-run] %s: uses %s %v\n", t.Name, action, inputs)
			return nil, "(dry-run) uses " + action, "", 0, nil
		}
		return e.invokeAction(ctx, action, inputs, opts)
	}

	// no-op task
	return nil, "", "", 0, nil
}

func runShell(ctx context.Context, cmdStr string, env map[string]string, stream io.Writer, taskName string) (map[string]string, string, string, int, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", cmdStr)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", cmdStr)
	}
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	var outBuf, errBuf capBuf
	cmd.Stdout = io.MultiWriter(&outBuf, prefixWriter(stream, taskName))
	cmd.Stderr = io.MultiWriter(&errBuf, prefixWriter(stream, taskName))
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = 1
		}
	}
	if err != nil {
		return nil, outBuf.String(), errBuf.String(), code, fmt.Errorf("command failed (exit %d): %s", code, cmdStr)
	}
	return map[string]string{"stdout": outBuf.String()}, outBuf.String(), errBuf.String(), 0, nil
}

func (e *Engine) invokeAction(ctx context.Context, action string, inputs map[string]string, opts Options) (map[string]string, string, string, int, error) {
	// Builtin actions available without any plugin.
	switch action {
	case "builtin/print", "core/print":
		msg := inputs["message"]
		if msg == "" {
			msg = inputs["text"]
		}
		fmt.Fprintln(opts.Stdout, msg)
		return map[string]string{"printed": msg}, msg, "", 0, nil
	case "builtin/noop", "core/noop":
		return map[string]string{}, "", "", 0, nil
	}
	if opts.Invoker == nil {
		return nil, "", "", 1, fmt.Errorf("no plugin available for action %q", action)
	}
	outs, stdout, err := opts.Invoker.Invoke(ctx, action, inputs)
	if err != nil {
		return nil, stdout, "", 1, err
	}
	if stdout != "" {
		fmt.Fprint(opts.Stdout, stdout)
	}
	return outs, stdout, "", 0, nil
}

// ---- helpers ----

func resolveParams(wf *ast.Workflow, supplied map[string]string) map[string]string {
	out := map[string]string{}
	for _, p := range wf.Params() {
		if p.Default != nil {
			out[p.Name] = p.Default.Stringify()
		} else {
			out[p.Name] = ""
		}
	}
	for k, v := range supplied {
		out[k] = v
	}
	return out
}

func mergeEnv(wf *ast.Workflow, supplied map[string]string, celEng *cel.Engine, params map[string]string) map[string]string {
	env := map[string]string{}
	if v := wf.Attr("env"); v != nil {
		act := cel.Activation{Params: toAnyMap(params)}
		for k, raw := range v.AsMap() {
			if val, err := celEng.Interpolate(raw, act); err == nil {
				env[k] = val
			} else {
				env[k] = raw
			}
		}
	}
	for k, v := range supplied {
		env[k] = v
	}
	return env
}

func snapshotActivation(params, env map[string]string, tasks map[string]any) cel.Activation {
	tcopy := make(map[string]any, len(tasks))
	for k, v := range tasks {
		tcopy[k] = v
	}
	return cel.Activation{
		Params: toAnyMap(params),
		Env:    env,
		Tasks:  tcopy,
	}
}

func toAnyMap(m map[string]string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		// promote booleans/ints so CEL comparisons behave naturally
		if v == "true" || v == "false" {
			out[k] = v == "true"
			continue
		}
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			out[k] = n
			continue
		}
		out[k] = v
	}
	return out
}
