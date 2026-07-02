# 25 — FlowDSL Expression Engine (CEL-Go Integration)

> **Codename:** Conduit · **DSL:** FlowDSL (`*.flow`) · **Module:** `github.com/conduit-io/conduit`
> **Expr package:** `github.com/conduit-io/conduit/internal/flow/expr`
> **Expression engine:** CEL-Go (`github.com/google/cel-go`) · **Go:** 1.24+
> **Status:** Language Baseline v1.0 · **Owner:** Language & Compiler · **Date:** 2026-07-02

FlowDSL embeds **CEL** (Common Expression Language) as its expression sublanguage. This document specifies the
**custom CEL environment**, the **standard function library**, **custom macros**, the **type provider** bridging
FlowDSL types ↔ CEL, the **cost limits / sandbox**, **compilation & caching**, the **runtime evaluation context**, and
**security** properties.

**Related documents**
- [20 §9 `${{ }}` & CEL embedding](20-dsl-grammar.md#9-formal-specification-of--interpolation--cel-embedding)
- [21 §3 Interpolation states](21-lexer-design.md#3-interpolation-lexer-states) — raw `CelText` capture.
- [22 §4.3 Custom capture](22-parser-design.md#4-grammar-to-ast-mapping) — `EmbeddedExpr`.
- [23 §2 EmbeddedExpr / §7 Source mapping](23-ast-design.md#2-node-taxonomy)
- [24 §6 CEL env construction](24-semantic-analysis.md#6-cel-environment-construction--expression-type-checking-pass-4) — the caller.

---

## 1. Why CEL

CEL is a **non-Turing-complete**, statically-typed, sandboxable expression language with a Go implementation, mature
cost accounting, and no ambient I/O — an ideal fit for a declarative workflow DSL where expressions must be **safe,
bounded, and analyzable**. FlowDSL never lets user code do arbitrary computation; CEL supplies pure, side-effect-free
value derivation for interpolations, guards (`when`), and iteration sources (`for_each`/`matrix`).

Boundaries:
- **FlowDSL owns** structure, scoping, types-of-record; **CEL owns** expression syntax/semantics.
- Sema ([24](24-semantic-analysis.md)) compiles & type-checks CEL at analysis time; the runtime ([31](31-execution-runtime.md))
  evaluates cached programs against a per-instance activation.

---

## 2. Custom CEL environment

The environment declares the **variables**, **functions**, **macros**, and **types** visible to FlowDSL expressions.
A base `cel.Env` is built once; per-scope variable sets are layered on cheaply.

```go
package expr

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/ext"
)

// Env wraps a base cel.Env plus FlowDSL type-provider and standard library.
type Env struct {
	base *cel.Env
}

// NewBaseEnv builds the shared, immutable base environment.
func NewBaseEnv() (*Env, error) {
	base, err := cel.NewEnv(
		cel.CustomTypeProvider(newFlowTypeProvider()), // §3
		cel.CustomTypeAdapter(newFlowTypeAdapter()),

		// Ambient objects always available to FlowDSL expressions:
		cel.Variable("params",  cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("inputs",  cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("env",     cel.MapType(cel.StringType, cel.StringType)),
		cel.Variable("secrets", cel.MapType(cel.StringType, cel.StringType)),
		cel.Variable("matrix",  cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("outputs", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("ctx",     flowCtxType), // run metadata: branch, actor, run_id, now, os…

		// Standard function library (§4) and CEL extensions:
		ext.Strings(), ext.Math(), ext.Encoders(), ext.Lists(), ext.Sets(),
		flowStdLib(),   // custom functions (§4)
		flowMacros(),   // custom macros (§4.3)

		// Safety knobs (§6):
		cel.HomogeneousAggregateLiterals(),
		cel.EagerlyValidateDeclarations(true),
		cel.DefaultUTCTimeZone(true),
	)
	if err != nil { return nil, err }
	return &Env{base: base}, nil
}

// WithVars returns a child env exposing scope-specific typed variables.
// Called by sema Pass 4 per EmbeddedExpr (doc 24 §6).
func (e *Env) WithVars(vars []VarDecl) *Env {
	opts := make([]cel.EnvOption, 0, len(vars))
	for _, v := range vars {
		opts = append(opts, cel.Variable(v.Name, toCelType(v.Type))) // §3 bridge
	}
	child, _ := e.base.Extend(opts...) // cheap; shares parser/checker
	return &Env{base: child}
}
```

### 2.1 Exposed variables

| Name | CEL type | Contents |
|---|---|---|
| `params` | `map<string, dyn>` | resolved workflow params |
| `inputs` | `map<string, dyn>` | invocation inputs |
| `env` | `map<string, string>` | merged env (step>task>workflow) |
| `secrets` | `map<string, string>` | resolved secret refs (redacted in logs) |
| `matrix` | `map<string, dyn>` | current matrix cell |
| `outputs` | `map<string, dyn>` | outputs of already-completed tasks/steps |
| `ctx` | `FlowCtx` | `branch, actor, run_id, now, os, arch, workspace` |
| loop var(s) | per `for_each` | value (+ optional key/index) |

Scope determines which of these are in scope (sema, [24 §3](24-semantic-analysis.md#3-name-resolution--scoping)).

---

## 3. Type provider & FlowDSL ↔ CEL bridging

A custom `ref.TypeProvider`/`types.Adapter` maps FlowDSL's type system ([24 §5](24-semantic-analysis.md#5-type-system--type-checking))
to CEL and back so both agree.

```go
// flowType ↔ CEL type mapping.
func toCelType(t *ast.TypeRef) *cel.Type {
	switch {
	case t == nil:            return cel.DynType
	case t.Scalar != nil:
		switch *t.Scalar {
		case "string":    return cel.StringType
		case "int":       return cel.IntType
		case "float":     return cel.DoubleType
		case "bool":      return cel.BoolType
		case "duration":  return cel.DurationType
		case "timestamp": return cel.TimestampType
		case "any", "null": return cel.DynType
		}
	case t.List != nil:  return cel.ListType(toCelType(t.List))
	case t.MapVal != nil: return cel.MapType(cel.StringType, toCelType(t.MapVal))
	case t.Object != nil: return flowObjectType(t) // registered struct-like type
	case t.Named != nil:  return cel.ObjectType(*t.Named)
	}
	return cel.DynType
}

// fromCelType maps an inferred CEL type back to a FlowDSL Type so sema can
// enforce declared/expected types (when→bool, for_each→list, ${{}}→string).
func fromCelType(t *cel.Type) sema.Type { /* inverse of toCelType */ }
```

FlowDSL `duration` and `timestamp` map to CEL's native `google.protobuf.Duration`/`Timestamp`, so arithmetic like
`ctx.now - inputs.started > 5m` type-checks and evaluates correctly.

---

## 4. Standard function library

Beyond CEL's built-ins and the `ext.*` packages, FlowDSL registers a **curated, pure, sandbox-safe** library. All
functions are **deterministic** and **I/O-free** (the `fs-safe` group operates on *path strings*, never the filesystem).

```go
// flowStdLib registers FlowDSL's custom functions as a single EnvOption.
func flowStdLib() cel.EnvOption {
	return cel.Lib(&flowLib{})
}

type flowLib struct{}

func (flowLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		// --- crypto-hash (pure) ---
		cel.Function("sha256",
			cel.Overload("sha256_string", []*cel.Type{cel.StringType}, cel.StringType,
				cel.UnaryBinding(func(v ref.Val) ref.Val {
					sum := sha256.Sum256([]byte(v.(types.String)))
					return types.String(hex.EncodeToString(sum[:]))
				}))),
		cel.Function("hmac_sha256", /* (key, msg) → hex */ ),

		// --- json ---
		cel.Function("json.encode", /* dyn → string */ ),
		cel.Function("json.decode", /* string → dyn */ ),

		// --- regex ---
		cel.Function("regex.matches",  /* (s, pattern) → bool  (RE2, linear-time) */ ),
		cel.Function("regex.replace",  /* (s, pattern, repl) → string */ ),
		cel.Function("regex.find_all", /* (s, pattern) → list<string> */ ),

		// --- fs-safe (string-only path ops; NO disk access) ---
		cel.Function("path.join",     /* (list<string>) → string */ ),
		cel.Function("path.base",     /* string → string */ ),
		cel.Function("path.ext",      /* string → string */ ),
		cel.Function("path.clean",    /* string → string (rejects .. traversal → error) */ ),

		// --- math (beyond ext.Math) ---
		cel.Function("clamp",   /* (x, lo, hi) → number */ ),

		// --- string / list / map helpers layered atop ext.* where needed ---
		cel.Function("slugify", /* string → string */ ),
		cel.Function("default", /* (dyn, dyn) → dyn : null-coalescing helper */ ),
	}
}
func (flowLib) ProgramOptions() []cel.ProgramOption { return nil }
```

| Group | Representative functions |
|---|---|
| **string** | `ext.Strings()` (`.startsWith`, `.split`, `.replace`, …), `slugify`, `trim`, `indent` |
| **list** | `ext.Lists()`/`ext.Sets()`, `flatten`, `distinct` |
| **map** | `keys`, `values`, `merge`, `get(m, k, default)` |
| **math** | `ext.Math()` (`math.greatest`, `math.least`), `clamp`, `abs` |
| **fs-safe** | `path.join/base/ext/clean` (no I/O; `..` traversal rejected) |
| **crypto-hash** | `sha256`, `sha1`, `hmac_sha256` (hash only; no keygen/rand) |
| **json** | `json.encode`, `json.decode` |
| **regex** | `regex.matches/replace/find_all` (RE2 — no catastrophic backtracking) |

Explicitly **absent**: file/network/exec/time-of-day-nondeterminism/random — see [§7 Security](#7-security).

### 4.3 Custom macros

CEL macros are compile-time rewrites (no runtime cost of their own). FlowDSL adds a few ergonomic ones:

```go
func flowMacros() cel.EnvOption {
	return cel.Macros(
		// coalesce(a, b, c) → first non-null/non-empty  (rewrites to nested ?: )
		cel.ReceiverVarArgMacro("coalesce", expandCoalesce),
		// x?.field → has(x, 'field') ? x.field : null  (safe navigation)
		cel.GlobalMacro("optget", 2, expandOptGet),
	)
}
```

Comprehension macros (`all`, `exists`, `map`, `filter`) come from CEL but are **bounded** by comprehension limits
([§6](#6-cost-limits--sandbox)).

---

## 5. Compilation & caching

Compilation happens **once**, during semantic analysis; the resulting `cel.Program` is cached and reused for every
runtime evaluation.

```go
// CompiledCel holds a type-checked, compiled CEL program plus provenance.
type CompiledCel struct {
	AST     *cel.Ast
	Program cel.Program
	Type    *cel.Type     // result type (mapped back via fromCelType)
	Cost    cost.Estimate // static cost bounds (min,max) for planning
	SrcPos  ast.Position  // start of the EmbeddedExpr in .flow source (doc 23 §7)
}

// Compile parses + type-checks source in env, then builds a runnable program.
// Called by sema Pass 4 (doc 24 §6).
func Compile(source string, env *Env) (*CompiledCel, sema.Type, []CelError) {
	astOut, iss := env.base.Compile(source)   // parse + check
	if iss != nil && iss.Err() != nil {
		return nil, sema.Type{}, mapIssues(iss) // → FLOW-E1301/1302/1303 (doc 24 §7)
	}
	prg, err := env.base.Program(astOut,
		cel.CostLimit(maxCost),                 // §6 hard ceiling
		cel.EvalOptions(cel.OptOptimize|cel.OptTrackCost),
		cel.InterruptCheckFrequency(1024),      // cooperative cancellation
	)
	if err != nil { return nil, sema.Type{}, []CelError{fromErr(err)} }
	est, _ := env.base.EstimateCost(astOut, costEstimator{})
	c := &CompiledCel{AST: astOut, Program: prg, Type: astOut.OutputType(),
		Cost: est, SrcPos: /*from caller*/}
	return c, fromCelType(astOut.OutputType()), nil
}
```

- **Program cache:** keyed by `(source, envFingerprint)`. Identical expressions across a file share one compiled
  program. Cache is per-analysis and, for hot LSP paths, an LRU across analyses.
- **Error mapping:** CEL issue offsets are relative to `source`; `SrcPos.Offset + issueOffset` yields the true source
  range ([21 §5](21-lexer-design.md#5-position-tracking)).

---

## 6. Cost limits & sandbox

CEL is bounded so a malicious/accidental expression cannot hang or exhaust resources.

```go
const (
	maxCost              = 1_000_000 // abstract CEL cost units (compile-checked + runtime-enforced)
	maxComprehensionSize = 10_000    // element ceiling for all/exists/map/filter
	evalTimeout          = 250 * time.Millisecond
)
```

| Control | Mechanism |
|---|---|
| **Static cost estimate** | `env.EstimateCost` at compile; expressions whose *max* exceeds `maxCost` are rejected → `FLOW-E1320`. |
| **Runtime cost ceiling** | `cel.CostLimit(maxCost)` aborts evaluation that exceeds the budget → runtime diag. |
| **Comprehension bounds** | Custom checker rejects/limits unbounded comprehensions; nested comprehension depth capped. |
| **Timeout / cancellation** | Programs run under a `context.WithTimeout`; `InterruptCheckFrequency` makes long loops cancellable. |
| **No I/O** | The env registers **no** file/net/exec/random/time-of-wall-clock functions; `ctx.now` is a fixed run timestamp, not live. |
| **Homogeneous literals** | `HomogeneousAggregateLiterals` prevents type-confusion in list/map literals. |
| **Memory** | Result sizes bounded by comprehension limits; string ops on `ext.Strings` are linear. |

```go
// Evaluate runs a compiled program against a runtime activation (doc 31).
func Evaluate(ctx context.Context, c *CompiledCel, act Activation) (ref.Val, error) {
	ctx, cancel := context.WithTimeout(ctx, evalTimeout)
	defer cancel()
	out, det, err := c.Program.ContextEval(ctx, act.vars) // det carries actual cost
	if err != nil { return nil, wrapEvalError(err, c.SrcPos) } // maps back to source
	if det != nil && det.ActualCost() != nil && *det.ActualCost() > maxCost {
		return nil, ErrCostExceeded
	}
	return out, nil
}
```

---

## 7. Security

CEL expressions are **untrusted user input** and are contained accordingly:

- **No ambient authority.** Pure functions only; no filesystem, network, process, clock-nondeterminism, or randomness.
  `path.*` operate on strings and reject `..` traversal (`FLOW-E1330`).
- **Secret handling.** `secrets` values are wrapped so they are **redacted** in any diagnostic, log, or error message;
  the evaluation runtime marks derived values tainted so a secret cannot be exfiltrated via a log-emitting step without
  passing the taint policy ([61 — Secrets Management](61-secrets-management.md)).
- **Deterministic & bounded** (see [§6](#6-cost-limits--sandbox)) — DoS-resistant; RE2 regex prevents ReDoS.
- **No dynamic code.** There is no `eval`/reflection surface exposed; CEL cannot construct new functions.
- **Type-safety.** All expressions are type-checked at analysis time ([24 §6](24-semantic-analysis.md#6-cel-environment-construction--expression-type-checking-pass-4)),
  so runtime type errors are effectively eliminated.

Full threat analysis: [69 — Threat Model](69-threat-model.md) (`THREAT-` entries for expression injection & secret
exfiltration).

---

## 8. Runtime evaluation context

At execution ([31 — Execution Runtime](31-execution-runtime.md)) each `CompiledCel` is evaluated against an
**Activation** assembled per task/step/matrix-cell/loop-iteration:

```go
// Activation supplies concrete values for the env's variables.
type Activation struct {
	vars map[string]any // params, inputs, env, secrets, matrix, outputs, ctx, loop vars
}

func BuildActivation(scope *RuntimeScope) Activation {
	return Activation{vars: map[string]any{
		"params":  scope.Params,
		"inputs":  scope.Inputs,
		"env":     scope.Env,
		"secrets": scope.Secrets, // redaction-wrapped
		"matrix":  scope.MatrixCell,
		"outputs": scope.CompletedOutputs,
		"ctx":     scope.Ctx,
		// loop var(s) injected by name for for_each
	}}
}
```

Evaluation order respects the DAG ([30](30-workflow-dag.md)): a task's `when`/interpolations only see `outputs` of
already-completed dependencies, guaranteeing referential validity checked statically in [24 §4](24-semantic-analysis.md#4-structural--dependency-validation).

---

## 9. Cross-references

- `${{ }}` syntax & embedding contract → [20 §9](20-dsl-grammar.md#9-formal-specification-of--interpolation--cel-embedding)
- Raw CEL capture in the lexer/parser → [21 §3](21-lexer-design.md#3-interpolation-lexer-states), [22 §4.3](22-parser-design.md#4-grammar-to-ast-mapping)
- `EmbeddedExpr` node & source mapping → [23 §2/§7](23-ast-design.md#2-node-taxonomy)
- CEL env construction & type-check pass → [24 §6](24-semantic-analysis.md#6-cel-environment-construction--expression-type-checking-pass-4)
- Runtime evaluation & activation → [31 — Execution Runtime](31-execution-runtime.md)
- Secret redaction/taint → [61 — Secrets Management](61-secrets-management.md)
- Expression-related threats → [69 — Threat Model](69-threat-model.md)
