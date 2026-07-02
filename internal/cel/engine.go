// Package cel wraps google/cel-go to provide the FlowDSL expression engine.
// It exposes a sandboxed environment (params, env, tasks) used to evaluate
// `when` guards and `${ ... }` string interpolation. Compiled programs are
// cached; evaluation is bounded by a cost limit and carries no I/O bindings.
package cel

import (
	"fmt"
	"strings"
	"sync"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// costLimit caps CEL evaluation to prevent runaway comprehensions.
const costLimit = 1_000_000

// Engine is a reusable, concurrency-safe CEL evaluator.
type Engine struct {
	env   *cel.Env
	mu    sync.Mutex
	cache map[string]cel.Program
}

// New constructs the FlowDSL CEL environment.
func New() (*Engine, error) {
	env, err := cel.NewEnv(
		cel.Variable("params", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("env", cel.MapType(cel.StringType, cel.StringType)),
		cel.Variable("tasks", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("matrix", cel.MapType(cel.StringType, cel.DynType)),
	)
	if err != nil {
		return nil, fmt.Errorf("build cel env: %w", err)
	}
	return &Engine{env: env, cache: map[string]cel.Program{}}, nil
}

// Activation is the set of variables available to an expression.
type Activation struct {
	Params map[string]any
	Env    map[string]string
	Tasks  map[string]any
	Matrix map[string]any
}

func (a Activation) vars() map[string]any {
	return map[string]any{
		"params": nz(a.Params),
		"env":    nzs(a.Env),
		"tasks":  nz(a.Tasks),
		"matrix": nz(a.Matrix),
	}
}

func nz(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}
func nzs(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

// program compiles (or returns a cached) CEL program for expr.
func (e *Engine) program(expr string) (cel.Program, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if p, ok := e.cache[expr]; ok {
		return p, nil
	}
	astObj, iss := e.env.Compile(expr)
	if iss != nil && iss.Err() != nil {
		return nil, fmt.Errorf("compile %q: %w", expr, iss.Err())
	}
	prg, err := e.env.Program(astObj, cel.CostLimit(costLimit))
	if err != nil {
		return nil, fmt.Errorf("plan %q: %w", expr, err)
	}
	e.cache[expr] = prg
	return prg, nil
}

// Eval evaluates an expression and returns the raw CEL value.
func (e *Engine) Eval(expr string, act Activation) (ref.Val, error) {
	prg, err := e.program(expr)
	if err != nil {
		return nil, err
	}
	out, _, err := prg.Eval(act.vars())
	if err != nil {
		return nil, fmt.Errorf("eval %q: %w", expr, err)
	}
	return out, nil
}

// EvalBool evaluates an expression expected to yield a boolean.
func (e *Engine) EvalBool(expr string, act Activation) (bool, error) {
	if strings.TrimSpace(expr) == "" {
		return true, nil
	}
	out, err := e.Eval(expr, act)
	if err != nil {
		return false, err
	}
	if b, ok := out.Value().(bool); ok {
		return b, nil
	}
	return false, fmt.Errorf("expression %q did not evaluate to bool (got %v)", expr, out.Type())
}

// Interpolate expands ${ ... } expressions inside a string. A literal "$${" is
// an escaped dollar-brace and is emitted as "${".
func (e *Engine) Interpolate(s string, act Activation) (string, error) {
	var b strings.Builder
	for i := 0; i < len(s); {
		// Escaped: $${ -> ${
		if i+2 < len(s) && s[i] == '$' && s[i+1] == '$' && s[i+2] == '{' {
			b.WriteString("${")
			i += 3
			continue
		}
		if i+1 < len(s) && s[i] == '$' && s[i+1] == '{' {
			end, expr := scanExpr(s, i+2)
			if end < 0 {
				return "", fmt.Errorf("unterminated ${ } interpolation")
			}
			out, err := e.Eval(expr, act)
			if err != nil {
				return "", err
			}
			b.WriteString(stringify(out))
			i = end
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String(), nil
}

// scanExpr returns the index just past the matching '}' and the expression
// text between braces, handling nested braces and quoted strings.
func scanExpr(s string, start int) (int, string) {
	depth := 1
	inStr := false
	var esc bool
	for i := start; i < len(s); i++ {
		c := s[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1, strings.TrimSpace(s[start:i])
			}
		}
	}
	return -1, ""
}

func stringify(v ref.Val) string {
	switch t := v.Value().(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		if v.Type() == types.NullType {
			return ""
		}
		return fmt.Sprint(v.Value())
	}
}
