# Conduit — Error Handling Strategy

> Document ID: `70-error-handling`
> Status: Draft (v0.1.0)
> Owner: Platform Architecture — Reliability & Quality
> Last updated: 2026-07-02

Related documents:
- [Recovery Strategy](71-recovery-strategy.md) — what we *do* about retryable/transient errors
- [Event Model](67-event-model.md) — `error.raised` event payload
- [Internal Message Bus](68-message-bus.md) — error propagation to subscribers
- [DSL Grammar](20-dsl-grammar.md) · [Parser Design](22-parser-design.md) · [Semantic Analysis](24-semantic-analysis.md) — sources of DSL diagnostics
- [Expression Engine (CEL)](25-expression-engine.md) — CEL evaluation errors
- [Plugin Architecture](40-plugin-architecture.md) — plugin error boundary
- [Workflow DAG Design](30-workflow-dag.md) — DAG error aggregation
- [Observability Architecture](64-observability.md) · [Logging](65-logging.md)

---

## 1. Principles

1. **Every failure has a category, a code, and a remediation.** No bare `errors.New("failed")` on a
   user-visible path.
2. **Errors are values, wrapped not swallowed.** We use Go 1.24 `errors.Is/As`, `%w` wrapping, and
   typed/sentinel errors — never string matching.
3. **Two audiences, one source.** A human sees a formatted, colorized diagnostic; an AI agent or
   script sees the *same* error as structured JSON. Both derive from one `*ConduitError`.
4. **Panics are bugs; errors are expectations.** Expected failures are returned as errors. Panics are
   contained at defined recovery boundaries and converted into a `system` error + `error.raised`
   event.
5. **Deterministic exit codes.** The process exit code is a stable function of the error category so
   CI and agents can branch on it.

---

## 2. Error Taxonomy

| Category | `Category` const | Meaning | Typical exit | Retryable? |
|---|---|---|---|---|
| **User error** | `CategoryUser` | Bad invocation: unknown flag, missing arg, invalid input | 2 | no |
| **Config error** | `CategoryConfig` | Invalid/absent `conduit.yaml`, bad profile, env binding | 3 | no |
| **DSL error** | `CategoryDSL` | Lex/parse/type/semantic error in a `.flow` file | 4 | no |
| **Plugin error** | `CategoryPlugin` | Plugin handshake/exec/protocol failure | 5 | sometimes |
| **System error** | `CategorySystem` | I/O, OS, internal invariant, recovered panic | 70 | no |
| **Transient error** | `CategoryTransient` | Timeout, network blip, throttling, breaker-open | 75 | **yes** |

`CategoryTransient` is the hinge between this document and
[71 — Recovery Strategy](71-recovery-strategy.md): transient errors are the ones the retry/backoff
and circuit-breaker machinery acts on.

```go
package cerr

// Category classifies an error for exit codes, retry decisions, and telemetry.
type Category string

const (
	CategoryUser      Category = "user"
	CategoryConfig    Category = "config"
	CategoryDSL       Category = "dsl"
	CategoryPlugin    Category = "plugin"
	CategorySystem    Category = "system"
	CategoryTransient Category = "transient"
)

// Retryable reports whether errors of this category are, by default, retryable.
func (c Category) Retryable() bool { return c == CategoryTransient }
```

---

## 3. Error Types

### 3.1 The core `ConduitError`

```go
package cerr

// ConduitError is the single structured error type surfaced to users and agents.
// It implements error, unwraps to its cause, and renders to both text and JSON.
type ConduitError struct {
	Code        string         // e.g. "CONDUIT-E1004" (registry, §6)
	Category    Category       // §2
	Summary     string         // one-line, imperative, no secrets
	Detail      string         // optional multi-line explanation
	Remediation string         // actionable next step ("run `conduit lint`")
	Retryable   bool           // overrides Category default when set
	Diag        *Diagnostic    // optional source-located diagnostic (DSL/CLI), §5
	Fields      map[string]any // structured context (task, plugin, path...)
	cause       error          // wrapped underlying error
}

func (e *ConduitError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Summary, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Summary)
}

func (e *ConduitError) Unwrap() error { return e.cause }

// IsRetryable respects an explicit override, else falls back to category.
func (e *ConduitError) IsRetryable() bool {
	return e.Retryable || e.Category.Retryable()
}
```

### 3.2 Construction & wrapping

```go
// New builds a ConduitError from a registry entry (§6) plus context.
func New(code string, cat Category, summary string) *ConduitError {
	return &ConduitError{Code: code, Category: cat, Summary: summary}
}

// Wrap attaches a cause, preserving %w semantics for errors.Is/As.
func (e *ConduitError) Wrap(cause error) *ConduitError { e.cause = cause; return e }

func (e *ConduitError) WithRemediation(s string) *ConduitError { e.Remediation = s; return e }
func (e *ConduitError) WithField(k string, v any) *ConduitError {
	if e.Fields == nil {
		e.Fields = map[string]any{}
	}
	e.Fields[k] = v
	return e
}
```

### 3.3 Sentinel errors (for `errors.Is`)

```go
// Sentinels mark well-known conditions callers branch on with errors.Is.
var (
	ErrNotFound       = New("CONDUIT-E1001", CategoryUser, "resource not found")
	ErrInvalidInput   = New("CONDUIT-E1002", CategoryUser, "invalid input")
	ErrConfigInvalid  = New("CONDUIT-E2001", CategoryConfig, "invalid configuration")
	ErrPluginCrashed  = New("CONDUIT-E5003", CategoryPlugin, "plugin process crashed")
	ErrTimeout        = New("CONDUIT-E7501", CategoryTransient, "operation timed out")
	ErrBreakerOpen    = New("CONDUIT-E7502", CategoryTransient, "circuit breaker open")
)
```

Because `ConduitError` has value identity, `errors.Is(err, cerr.ErrTimeout)` works after wrapping.
For typed extraction:

```go
var ce *cerr.ConduitError
if errors.As(err, &ce) {
	log.Warn("conduit error", "code", ce.Code, "category", ce.Category)
}
```

---

## 4. Exit Code Convention

The CLI's top-level runner maps the terminal error to a process exit code so shells, CI, and agents
can branch deterministically.

| Exit | Meaning | Source category |
|---|---|---|
| `0` | Success | — |
| `1` | Generic/unclassified failure | fallback |
| `2` | Usage / user error | `CategoryUser` |
| `3` | Configuration error | `CategoryConfig` |
| `4` | DSL / flow compilation error | `CategoryDSL` |
| `5` | Plugin error | `CategoryPlugin` |
| `70` | Internal/system error (recovered panic) | `CategorySystem` |
| `75` | Transient/temporary — safe to retry the whole invocation | `CategoryTransient` |
| `130` | Interrupted (SIGINT) | context canceled |

```go
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, context.Canceled) {
		return 130
	}
	var ce *ConduitError
	if errors.As(err, &ce) {
		switch ce.Category {
		case CategoryUser:
			return 2
		case CategoryConfig:
			return 3
		case CategoryDSL:
			return 4
		case CategoryPlugin:
			return 5
		case CategorySystem:
			return 70
		case CategoryTransient:
			return 75
		}
	}
	return 1
}
```

Exit codes `64–78` intentionally follow BSD `sysexits.h` conventions where they overlap (e.g. `70`
`EX_SOFTWARE`), easing interop with existing tooling.

---

## 5. Structured Diagnostics (DSL & CLI)

DSL errors from the [parser](22-parser-design.md) and [semantic analyzer](24-semantic-analysis.md)
carry a **`Diagnostic`** with a precise source span, severity, and (where possible) a fix-it hint.
This is the same structure the [LSP server](56-lsp-architecture.md) publishes to editors, so CLI and
IDE report identically.

```go
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityHint    Severity = "hint"
)

// Position is 1-based line, 0-based column (LSP-compatible on export).
type Position struct {
	Line, Column int
}

type Span struct {
	File       string
	Start, End Position
}

type FixIt struct {
	Message     string
	Span        Span
	Replacement string
}

// Diagnostic is a source-located message attached to a ConduitError (Diag field).
type Diagnostic struct {
	Span     Span
	Severity Severity
	Code     string   // shares the CONDUIT-Exxxx registry
	Message  string
	FixIts   []FixIt
	Notes    []string // secondary, related spans/hints
}
```

Human rendering (colorized, caret-annotated):

```text
error[CONDUIT-E4007]: unknown task reference "buld"
  --> deploy.flow:14:18
   |
14 |   depends_on: [buld]
   |               ^^^^ no task named "buld" in this flow
   |
   = help: did you mean "build"?
   = docs: https://docs.conduit.io/errors/CONDUIT-E4007
```

---

## 6. Error Code Registry (`CONDUIT-Exxxx`)

Every user-facing error has a stable code from a central registry. Codes are **immutable once
published** (a code never changes meaning) and grouped by category for readability.

| Range | Category | Example |
|---|---|---|
| `E1xxx` | User | `CONDUIT-E1001` not found · `E1002` invalid input |
| `E2xxx` | Config | `CONDUIT-E2001` invalid config · `E2003` unknown profile |
| `E4xxx` | DSL | `CONDUIT-E4001` parse error · `E4007` unknown task ref · `E4012` type mismatch |
| `E5xxx` | Plugin | `CONDUIT-E5001` handshake failed · `E5003` crashed · `E5004` protocol version |
| `E7xxx` | System | `CONDUIT-E7001` internal invariant · `E7002` recovered panic |
| `E75xx` | Transient | `CONDUIT-E7501` timeout · `E7502` breaker open · `E7503` throttled |

The registry is a single source-of-truth table, validated by a test that fails if a code is
duplicated or missing a remediation/docs URL:

```go
// Registry entry: metadata for one CONDUIT-Exxxx code.
type RegEntry struct {
	Code        string
	Category    Category
	Summary     string
	Remediation string
	DocsURL     string
}

var Codes = map[string]RegEntry{
	"CONDUIT-E4007": {
		Code: "CONDUIT-E4007", Category: CategoryDSL,
		Summary:     "unknown task reference",
		Remediation: "check the task name; run `conduit lint <file>` to list valid references",
		DocsURL:     "https://docs.conduit.io/errors/CONDUIT-E4007",
	},
	// ...
}
```

---

## 7. User-facing vs Machine-facing (JSON) Rendering

The global `--output` flag (`text` default, `json`) selects the renderer. Agents and scripts request
`json`; the payload matches the `error.raised` event body in [67 — Event Model §3](67-event-model.md)
so the *same* structure appears whether an error is streamed as an event or returned at process exit.

```go
// MarshalJSON emits the stable machine contract for AI agents / scripts.
func (e *ConduitError) MarshalJSON() ([]byte, error) {
	type diagJSON = Diagnostic
	out := struct {
		Code        string         `json:"code"`
		Category    Category       `json:"category"`
		Message     string         `json:"message"`
		Detail      string         `json:"detail,omitempty"`
		Remediation string         `json:"remediation,omitempty"`
		Retryable   bool           `json:"retryable"`
		Diagnostic  *diagJSON      `json:"diagnostic,omitempty"`
		Fields      map[string]any `json:"fields,omitempty"`
		DocsURL     string         `json:"docs_url,omitempty"`
	}{
		Code: e.Code, Category: e.Category, Message: e.Summary,
		Detail: e.Detail, Remediation: e.Remediation,
		Retryable: e.IsRetryable(), Diagnostic: e.Diag, Fields: e.Fields,
		DocsURL: Codes[e.Code].DocsURL,
	}
	return json.Marshal(out)
}
```

Example machine output:

```json
{
  "code": "CONDUIT-E5003",
  "category": "plugin",
  "message": "plugin process crashed",
  "remediation": "re-run with --plugin-log-level=debug; the plugin will be restarted automatically",
  "retryable": true,
  "fields": { "plugin": "git", "pid": 48213, "signal": "SIGSEGV" },
  "docs_url": "https://docs.conduit.io/errors/CONDUIT-E5003"
}
```

---

## 8. Panic Recovery Boundaries

Conduit installs `recover()` at exactly three boundaries; nowhere else. A recovered panic becomes a
`CategorySystem` `ConduitError` (`CONDUIT-E7002`) plus an `error.raised` event, and never crashes the
whole process when isolation is possible.

1. **Command boundary** — the Cobra `RunE` wrapper. A panic in a command handler is converted to a
   system error and exit code `70`, with the stack captured to the debug log.
2. **Task/goroutine boundary** — every task goroutine in the [DAG runtime](31-execution-runtime.md)
   runs under a recover; a panicking task fails *that task* (and its DAG subtree) without killing
   sibling tasks.
3. **Plugin boundary** — plugins run out-of-process ([go-plugin/gRPC](40-plugin-architecture.md)), so
   a plugin panic is observed as a gRPC error / process exit, mapped to `ErrPluginCrashed`, and
   handled by the restart policy in [71 — Recovery Strategy](71-recovery-strategy.md).

```go
// guard converts a panic into a system-category ConduitError.
func guard(where string, fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			err = New("CONDUIT-E7002", CategorySystem, "internal panic recovered").
				WithField("where", where).
				WithField("stack", string(stack)).
				Wrap(fmt.Errorf("panic: %v", r))
		}
	}()
	return fn()
}
```

---

## 9. Error Aggregation in DAG Runs

A DAG run can fail in multiple branches concurrently. The runtime aggregates per-task errors into a
single **`RunError`** that preserves each failure while presenting one top-level code/exit.

```go
// RunError aggregates multiple task failures from a DAG run.
type RunError struct {
	RunID     string
	Failures  []TaskFailure // ordered by task completion time
	Policy    FailurePolicy // FailFast | ContinueOnError (see 71-recovery §6)
}

type TaskFailure struct {
	Task string
	Err  *ConduitError
}

func (r *RunError) Error() string {
	return fmt.Sprintf("run %s failed: %d task(s) failed", r.RunID, len(r.Failures))
}

// As/Is delegate so errors.As(runErr, &ce) finds the first ConduitError,
// and errors.Is(runErr, cerr.ErrTimeout) is true if any failure is a timeout.
func (r *RunError) Unwrap() []error {
	out := make([]error, len(r.Failures))
	for i, f := range r.Failures {
		out[i] = f.Err
	}
	return out
}

// Category returns the most severe category across failures for exit-code mapping:
// system > plugin > dsl > config > user, with transient collapsing to the
// underlying cause so a partly-transient run still reports non-zero deterministically.
func (r *RunError) Category() Category { /* max over r.Failures */ return CategorySystem }
```

`Unwrap() []error` uses the Go 1.20+ multi-error interface, so `errors.Is`/`As` traverse every
branch. The aggregated error is emitted as a `run.failed` event whose `failed_tasks[]` mirrors
`Failures`.

---

## 10. Actionable Messages & Remediation

Guidelines enforced by review and a lint that checks registry entries:

- **Imperative, specific summaries.** "unknown task reference \"buld\"", not "invalid flow".
- **Always a remediation** for user/config/DSL errors: the exact command or edit to try.
- **Fix-its where the fix is unambiguous** (typo → "did you mean \"build\"?").
- **A docs URL** per code (`https://docs.conduit.io/errors/<CODE>`).
- **Never leak secrets**: summaries/fields pass the redactor from
  [61 — Secrets Management](61-secrets-management.md) before display or emission.

---

## 11. Cross-References

- Transient errors → retry/backoff/circuit-breaker behavior: [71 — Recovery Strategy](71-recovery-strategy.md).
- The `error.raised` event and its JSON parity with §7: [67 — Event Model](67-event-model.md).
- DSL diagnostic sources: [22 — Parser Design](22-parser-design.md), [24 — Semantic Analysis](24-semantic-analysis.md), [25 — Expression Engine](25-expression-engine.md).
- Plugin crash mapping: [40 — Plugin Architecture](40-plugin-architecture.md).
- Diagnostics rendering in editors: [56 — LSP Architecture](56-lsp-architecture.md).
