# Conduit — Functional Requirements

> Document ID: `03-functional-requirements`
> Status: Draft (v0.1.0)
> Owner: Principal Product Architect
> Last updated: 2026-07-02

Related documents:
- [Executive Summary](00-executive-summary.md)
- [Product Vision](01-product-vision.md)
- [Product Requirements (PRD)](02-prd.md)
- [Non-Functional Requirements](04-non-functional-requirements.md)
- [Security Requirements](05-security-requirements.md)
- [Platform Architecture](10-platform-architecture.md)

---

## 1. Conventions

- Requirement IDs are `FR-###`, globally unique and stable.
- RFC 2119 keywords: **MUST**, **SHOULD**, **MAY** (and negations) are normative.
- Priority: `M` (Must / MVP), `S` (Should / Beta), `C` (Could / GA+), aligned to the [PRD MoSCoW matrix](02-prd.md).
- Each requirement lists: **Statement**, **Rationale**, **Acceptance Criteria (AC)**, **Priority**.
- Canonical stack per [Executive Summary](00-executive-summary.md): Cobra, Koanf/`conduit.yaml`, CEL-Go, go-plugin/gRPC, Bubble Tea, LSP + `tree-sitter-flow`, binary `conduit`/`cdt`, module `github.com/conduit-io/conduit`, Go 1.24+.

### Capability areas
1. Command Framework (FR-001–FR-014)
2. FlowDSL (FR-015–FR-028)
3. DAG / Workflow (FR-029–FR-040)
4. Execution Runtime (FR-041–FR-050)
5. State & Checkpoints (FR-051–FR-057)
6. Plugins (FR-058–FR-067)
7. Completion & Shell Integration (FR-068–FR-072)
8. LSP / IDE Tooling (FR-073–FR-079)
9. Config (FR-080–FR-085)
10. Secrets (FR-086–FR-091)
11. AuthN / AuthZ (FR-092–FR-097)
12. Observability (FR-098–FR-104)
13. Expressions (CEL) (FR-105–FR-109)
14. AI-Agent API (FR-110–FR-118)
15. TUI (FR-119–FR-122)

---

## 2. Command Framework

### FR-001 — Root command and aliasing
- **Statement:** The system **MUST** expose a root command `conduit` and a functionally identical alias `cdt`.
- **Rationale:** Ergonomics; short alias for frequent use.
- **AC:** Invoking `cdt <args>` behaves identically to `conduit <args>` for all subcommands.
- **Priority:** M

### FR-002 — Subcommand hierarchy
- **Statement:** The framework **MUST** support arbitrarily nested subcommands built on Cobra.
- **Rationale:** Composable CLIs (persona Priya).
- **AC:** A three-level subcommand (`conduit a b c`) resolves and executes its handler.
- **Priority:** M

### FR-003 — Typed flags
- **Statement:** Commands **MUST** support typed flags (string, int, float, bool, duration, enum, string-slice, path) with validation.
- **Rationale:** Reject invalid input early with clear errors.
- **AC:** Passing an invalid value for a typed flag exits non-zero, names the flag and expected type, and does not run the body.
- **Priority:** M

### FR-004 — Typed positional arguments
- **Statement:** Commands **MUST** support typed, count-validated positional args (exact, min, range, variadic).
- **Rationale:** Correctness and good error messages.
- **AC:** Wrong arg count yields a usage error and non-zero exit.
- **Priority:** M

### FR-005 — Required vs. optional flags/args
- **Statement:** The framework **MUST** distinguish required from optional inputs and enforce required ones.
- **Rationale:** Prevent under-specified invocations.
- **AC:** Omitting a required flag yields an error naming it.
- **Priority:** M

### FR-006 — Flag defaults and env binding
- **Statement:** Flags **MUST** support default values and **SHOULD** bind to environment variables (e.g., `CONDUIT_*`).
- **Rationale:** Twelve-factor configuration.
- **AC:** An unset flag with an env binding takes the env value; explicit flag overrides env.
- **Priority:** M

### FR-007 — Pre/post execution hooks
- **Statement:** The framework **MUST** support pre-run and post-run hooks at command and global scope.
- **Rationale:** Cross-cutting concerns (auth, telemetry).
- **AC:** A registered global pre-hook runs before every command body; a post-hook runs after, including on error.
- **Priority:** M

### FR-008 — Structured, machine-readable output
- **Statement:** Commands **MUST** support `--output {text,json,yaml}` (default `text`).
- **Rationale:** Scripting and agent consumption.
- **AC:** `--output json` emits schema-valid JSON to stdout; human text goes to stderr where applicable.
- **Priority:** M

### FR-009 — Consistent exit codes
- **Statement:** The system **MUST** use documented, consistent exit codes (0 success; distinct codes for usage error, execution failure, permission denied, cancellation).
- **Rationale:** Reliable automation and CI gating.
- **AC:** Each failure class maps to its documented exit code.
- **Priority:** M

### FR-010 — Help and usage generation
- **Statement:** Every command **MUST** auto-generate help (`-h`/`--help`) including flags, args, examples.
- **Rationale:** Discoverability.
- **AC:** `conduit <cmd> --help` lists all flags/args with types and defaults.
- **Priority:** M

### FR-011 — Version and build info
- **Statement:** `conduit --version` **MUST** print version (SemVer), commit, build date, Go version, and OS/arch.
- **Rationale:** Support and provenance.
- **AC:** Output includes all fields and matches embedded build metadata.
- **Priority:** M

### FR-012 — Global flags
- **Statement:** The system **MUST** provide global flags: `--config`, `--verbose/-v`, `--quiet`, `--no-color`, `--output`, `--profile`.
- **Rationale:** Uniform control across commands.
- **AC:** Global flags are accepted before or after the subcommand path.
- **Priority:** M

### FR-013 — Command groups / categories
- **Statement:** Help output **SHOULD** organize subcommands into named groups.
- **Rationale:** Navigability for large CLIs.
- **AC:** Grouped help renders category headers.
- **Priority:** S

### FR-014 — Deprecation signaling
- **Statement:** Commands/flags **MUST** support deprecation notices that warn without breaking, per SemVer.
- **Rationale:** Backward-compatibility promise.
- **AC:** Using a deprecated flag warns on stderr and still functions until removal in a major release.
- **Priority:** S

---

## 3. FlowDSL

### FR-015 — `*.flow` file format
- **Statement:** The system **MUST** define and parse FlowDSL from `*.flow` files.
- **Rationale:** Declarative, reviewable automation.
- **AC:** A syntactically valid `*.flow` parses without error; an invalid one reports line/column diagnostics.
- **Priority:** M

### FR-016 — Flow declaration with typed inputs
- **Statement:** A flow **MUST** declare typed, optionally-defaulted inputs.
- **Rationale:** Validated invocation from CLI and agents.
- **AC:** Invoking a flow with a wrong-typed input is rejected pre-execution.
- **Priority:** M

### FR-017 — Step declaration
- **Statement:** A flow **MUST** support named steps that invoke commands, plugins, or shell.
- **Rationale:** Core unit of work.
- **AC:** Each step has a unique name within its flow; duplicates are a parse error.
- **Priority:** M

### FR-018 — Explicit dependencies
- **Statement:** Steps **MUST** declare dependencies (e.g., `needs`/`after`) forming a DAG.
- **Rationale:** Deterministic ordering.
- **AC:** Declared dependencies are honored at runtime (see FR-029).
- **Priority:** M

### FR-019 — Data passing between steps
- **Statement:** Steps **MUST** be able to consume typed outputs of upstream steps.
- **Rationale:** Real pipelines pass data, not just order.
- **AC:** A downstream step reads a named output of an upstream step; missing outputs are a type/compile error where statically knowable.
- **Priority:** M

### FR-020 — Imports / modules
- **Statement:** FlowDSL **MUST** support importing reusable flow modules by path or registry reference.
- **Rationale:** Golden-path distribution (persona Priya).
- **AC:** An imported flow/step is usable and namespaced; cyclic imports are rejected.
- **Priority:** S

### FR-021 — CEL expressions in DSL
- **Statement:** FlowDSL **MUST** allow CEL-Go expressions for conditions, templating, and derived values.
- **Rationale:** Dynamic behavior without a general language.
- **AC:** A CEL condition gates a step; invalid CEL is a compile-time diagnostic.
- **Priority:** M

### FR-022 — Conditional execution
- **Statement:** Steps **MUST** support conditional execution via a CEL predicate (`when`).
- **Rationale:** Branching flows.
- **AC:** A step with a false `when` is skipped and marked `skipped`.
- **Priority:** M

### FR-023 — Loops / fan-out (matrix)
- **Statement:** FlowDSL **SHOULD** support fan-out over a collection (matrix/`for-each`) producing parallel step instances.
- **Rationale:** Multi-target operations (regions, services).
- **AC:** A fan-out over N items yields N step instances that may run in parallel and aggregate outputs.
- **Priority:** S

### FR-024 — Static type checking
- **Statement:** The DSL compiler **MUST** type-check inputs, outputs, and expression results before execution.
- **Rationale:** Catch errors before side effects.
- **AC:** Type mismatches are reported with location; a type-clean flow runs.
- **Priority:** M

### FR-025 — Comments and documentation
- **Statement:** FlowDSL **MUST** support comments and step/flow docstrings surfaced in help and the agent API.
- **Rationale:** Self-documenting flows.
- **AC:** A flow docstring appears in `conduit describe <flow>` and the agent tool schema.
- **Priority:** S

### FR-026 — Secrets references (no literals)
- **Statement:** FlowDSL **MUST** allow referencing secrets by name and **MUST NOT** encourage secret literals; a linter **SHOULD** flag inline secrets.
- **Rationale:** Prevent secret leakage (persona Nadia).
- **AC:** A secret reference resolves at runtime via a provider; a suspected literal secret triggers a lint warning.
- **Priority:** S

### FR-027 — Deterministic formatting (`conduit fmt`)
- **Statement:** The system **MUST** provide a canonical formatter for `*.flow`.
- **Rationale:** Consistent, diff-friendly code.
- **AC:** Running the formatter twice is idempotent; output matches the published style.
- **Priority:** S

### FR-028 — Flow validation command (`conduit validate`)
- **Statement:** The system **MUST** provide static validation of `*.flow` without executing it.
- **Rationale:** CI linting.
- **AC:** `conduit validate x.flow` exits zero for valid, non-zero with diagnostics otherwise.
- **Priority:** M

---

## 4. DAG / Workflow

### FR-029 — Dependency resolution & topological ordering
- **Statement:** The engine **MUST** compute a valid topological order from declared dependencies.
- **Rationale:** Correct execution order.
- **AC:** Given B,C after A, A completes before B and C start.
- **Priority:** M

### FR-030 — Cycle detection
- **Statement:** The engine **MUST** reject cyclic dependency graphs at compile time.
- **Rationale:** Prevent deadlock/non-termination.
- **AC:** A cyclic flow fails validation with the cycle path reported.
- **Priority:** M

### FR-031 — Parallel execution
- **Statement:** Independent steps **MUST** execute concurrently up to a configurable concurrency limit.
- **Rationale:** Performance.
- **AC:** Two independent steps run concurrently; `--max-concurrency N` caps parallelism.
- **Priority:** M

### FR-032 — Per-step retries with backoff
- **Statement:** Steps **MUST** support retries with configurable count, backoff strategy (fixed/exponential), and jitter.
- **Rationale:** Self-heal transient failures.
- **AC:** A step failing twice then succeeding, configured for 3 attempts, ultimately succeeds; backoff timing is observed.
- **Priority:** M

### FR-033 — Per-step timeouts
- **Statement:** Steps **MUST** support timeouts that cancel and fail the step.
- **Rationale:** Bound runaway steps.
- **AC:** A step exceeding its timeout is cancelled and marked `timed_out`.
- **Priority:** M

### FR-034 — Compensation / rollback
- **Statement:** Steps **MUST** support declared compensation actions executed on downstream failure in reverse dependency order.
- **Rationale:** Consistent state after partial failure (persona Sam).
- **AC:** On failure after step A completed, A's compensation runs; run marked `compensated`.
- **Priority:** S

### FR-035 — Dry-run / plan
- **Statement:** The engine **MUST** support `--dry-run` producing the resolved DAG and intended actions without side effects.
- **Rationale:** Safe preview.
- **AC:** No side-effecting step runs; plan output lists ordered actions.
- **Priority:** M

### FR-036 — Continue-on-error policy
- **Statement:** Flows **SHOULD** allow per-step `continue_on_error` and flow-level failure policy.
- **Rationale:** Best-effort steps.
- **AC:** A `continue_on_error` step's failure does not abort the flow but is recorded.
- **Priority:** S

### FR-037 — Step-level environment & working directory
- **Statement:** Steps **MUST** support per-step env vars and working directory.
- **Rationale:** Isolation and correctness.
- **AC:** A step's declared env/cwd apply only to that step.
- **Priority:** M

### FR-038 — Output aggregation
- **Statement:** The engine **MUST** aggregate step outputs into a run result addressable by step name.
- **Rationale:** Downstream consumption and reporting.
- **AC:** Run result JSON contains each step's typed outputs and status.
- **Priority:** M

### FR-039 — Deterministic run identity
- **Statement:** Each run **MUST** have a stable, unique run ID; each step instance a stable step ID.
- **Rationale:** Correlation across logs/traces/state.
- **AC:** Run/step IDs appear consistently in logs, traces, state, and audit.
- **Priority:** M

### FR-040 — Partial DAG execution (targets)
- **Statement:** The engine **SHOULD** support running a subgraph (`--only step`, `--from step`, `--until step`).
- **Rationale:** Debugging and resumption.
- **AC:** `--only B` runs B and its required upstream, skipping unrelated steps.
- **Priority:** S

---

## 5. Execution Runtime

### FR-041 — Local execution
- **Statement:** The runtime **MUST** execute flows locally with no external server dependency.
- **Rationale:** Single-binary principle.
- **AC:** A flow runs on a network-isolated host.
- **Priority:** M

### FR-042 — Graceful cancellation
- **Statement:** The runtime **MUST** honor cancellation (SIGINT/SIGTERM, context) and stop steps cleanly.
- **Rationale:** Safe aborts.
- **AC:** Ctrl-C cancels in-flight steps, runs applicable compensation if configured, and exits with the cancellation code.
- **Priority:** M

### FR-043 — Per-step resource limits
- **Statement:** The runtime **SHOULD** enforce per-step CPU/memory/time limits where the OS permits.
- **Rationale:** Prevent host exhaustion.
- **AC:** A step exceeding a memory limit is terminated and marked failed with a resource-limit reason.
- **Priority:** S

### FR-044 — Shell/step process isolation
- **Statement:** Shell steps **MUST** run as child processes with controlled env and no implicit shell injection of untrusted input.
- **Rationale:** Safety.
- **AC:** Untrusted input is passed as arguments, not interpolated into a shell string, unless explicitly opted in.
- **Priority:** M

### FR-045 — Streaming logs
- **Statement:** The runtime **MUST** stream step stdout/stderr in real time, tagged by step.
- **Rationale:** Live observability.
- **AC:** Interleaved output is attributable to its step.
- **Priority:** M

### FR-046 — Deterministic environment resolution
- **Statement:** The runtime **MUST** resolve env from a documented precedence (flow < step < flag < secret provider).
- **Rationale:** Predictability.
- **AC:** Precedence is honored and documented.
- **Priority:** M

### FR-047 — Idempotency hints
- **Statement:** Steps **SHOULD** allow declaring idempotency to inform retry/resume safety.
- **Rationale:** Safe re-execution.
- **AC:** A non-idempotent step is not auto-retried unless explicitly allowed.
- **Priority:** S

### FR-048 — Concurrency safety for shared state
- **Statement:** The runtime **MUST** serialize writes to shared run state to avoid races.
- **Rationale:** Correctness under parallelism.
- **AC:** Concurrent step outputs are recorded without corruption (race-detector clean).
- **Priority:** M

### FR-049 — Exit-on-first-failure default
- **Statement:** By default a step failure **MUST** abort dependent steps while allowing independent in-flight steps to complete or cancel per policy.
- **Rationale:** Predictable failure semantics.
- **AC:** Dependents of a failed step do not start; policy governs siblings.
- **Priority:** M

### FR-050 — Reproducible run summary
- **Statement:** The runtime **MUST** emit a structured run summary (status, durations, per-step results).
- **Rationale:** Reporting and CI.
- **AC:** Summary is emitted as text and (with `--output json`) machine-readable.
- **Priority:** M

---

## 6. State & Checkpoints

### FR-051 — Run state persistence
- **Statement:** The runtime **MUST** persist run state (step statuses, outputs) to a local store by default.
- **Rationale:** Inspection and resume.
- **AC:** After a run, state is queryable via `conduit runs`.
- **Priority:** S

### FR-052 — Checkpointing
- **Statement:** The engine **MUST** checkpoint completed step outputs so resumption skips completed work.
- **Rationale:** Avoid repeating expensive steps.
- **AC:** A resumed run does not re-execute already-completed, idempotent steps.
- **Priority:** S

### FR-053 — Resume failed run
- **Statement:** The system **MUST** support `conduit resume <run-id>` continuing from the last checkpoint.
- **Rationale:** Long pipelines (persona Dan).
- **AC:** Resume continues at the first incomplete step.
- **Priority:** S

### FR-054 — Pluggable state backends
- **Statement:** State persistence **MUST** be pluggable (local embedded default; remote backend interface).
- **Rationale:** Shared/team state.
- **AC:** Switching backend via config changes storage without flow changes.
- **Priority:** S

### FR-055 — Run history & querying
- **Statement:** The system **SHOULD** provide `conduit runs list/show` to inspect past runs.
- **Rationale:** Audit and debugging.
- **AC:** History lists runs with status, time, and IDs; show renders per-step detail.
- **Priority:** S

### FR-056 — State retention / GC
- **Statement:** The system **SHOULD** support configurable retention and garbage collection of run state.
- **Rationale:** Bound disk usage.
- **AC:** State older than the retention window is pruned.
- **Priority:** C

### FR-057 — State schema versioning
- **Statement:** Persisted state **MUST** be versioned and migratable across Conduit upgrades.
- **Rationale:** Forward compatibility.
- **AC:** An older state store opens and migrates on upgrade without data loss.
- **Priority:** S

---

## 7. Plugins

### FR-058 — go-plugin/gRPC protocol
- **Statement:** Plugins **MUST** be loaded out-of-process via HashiCorp go-plugin over gRPC.
- **Rationale:** Isolation and language-agnostic extensibility.
- **AC:** A conforming gRPC plugin loads and its capabilities are invocable from a flow.
- **Priority:** S

### FR-059 — Plugin capability discovery
- **Statement:** The host **MUST** query a plugin's declared capabilities (commands, step types, providers) with typed schemas.
- **Rationale:** Integration and agent exposure.
- **AC:** `conduit plugin describe <p>` lists capabilities with input/output schemas.
- **Priority:** S

### FR-060 — Version/protocol negotiation
- **Statement:** Host and plugin **MUST** negotiate a protocol version and refuse incompatible plugins.
- **Rationale:** Stability across upgrades.
- **AC:** An incompatible-protocol plugin is rejected with a clear message.
- **Priority:** S

### FR-061 — Plugin lifecycle management
- **Statement:** The host **MUST** manage plugin process lifecycle (start, health-check, graceful shutdown, crash handling).
- **Rationale:** Reliability.
- **AC:** A crashed plugin fails the dependent step, not the host; the host reclaims the process.
- **Priority:** S

### FR-062 — Plugin signing & trust verification
- **Statement:** The host **MUST** verify plugin signatures/provenance against a trust policy before execution.
- **Rationale:** Supply-chain safety (see [Security Requirements](05-security-requirements.md)).
- **AC:** Unsigned/untrusted plugin is refused under default policy with an audit event.
- **Priority:** S

### FR-063 — Plugin sandboxing
- **Statement:** Plugins **MUST** run with least privilege (restricted filesystem/network/env) enforced by the host where the OS permits.
- **Rationale:** Contain third-party code.
- **AC:** A plugin cannot access paths/network outside its granted policy.
- **Priority:** S

### FR-064 — Plugin registry
- **Statement:** The system **MUST** support installing plugins from a registry (`conduit plugin install <ref>`).
- **Rationale:** Distribution.
- **AC:** Installing a registry plugin fetches, verifies, and registers it.
- **Priority:** S

### FR-065 — Local/dev plugins
- **Statement:** The system **SHOULD** support loading local plugin binaries for development.
- **Rationale:** Author workflow.
- **AC:** A local plugin path loads under an explicit dev-trust flag.
- **Priority:** S

### FR-066 — Plugin resource limits & timeouts
- **Statement:** Plugin invocations **MUST** support timeouts and resource limits.
- **Rationale:** Prevent hangs/exhaustion.
- **AC:** A hung plugin call times out and fails the step.
- **Priority:** S

### FR-067 — Plugin audit logging
- **Statement:** Plugin loads and invocations **MUST** be audit-logged (identity, version, hash, outcome).
- **Rationale:** Compliance and forensics.
- **AC:** Each plugin invocation produces an audit record.
- **Priority:** S

---

## 8. Completion & Shell Integration

### FR-068 — Multi-shell completion
- **Statement:** The system **MUST** generate completion scripts for bash, zsh, fish, and PowerShell.
- **Rationale:** Native feel across platforms.
- **AC:** `conduit completion <shell>` emits a working script for each shell.
- **Priority:** M

### FR-069 — Dynamic completion
- **Statement:** Completion **MUST** support dynamic values (flow names, step names, plugin capabilities, enum flags).
- **Rationale:** Useful, context-aware suggestions.
- **AC:** Completing a flow-name arg lists available `*.flow` in scope.
- **Priority:** M

### FR-070 — Completion latency budget
- **Statement:** Dynamic completion **MUST** meet the latency budget (p95 < 100ms) — see [NFRs](04-non-functional-requirements.md).
- **Rationale:** Responsiveness.
- **AC:** Benchmarked completion p95 < 100ms.
- **Priority:** M

### FR-071 — Completion install helper
- **Statement:** The system **SHOULD** provide guidance/automation to install completion per shell.
- **Rationale:** Onboarding.
- **AC:** `conduit completion --help` documents per-shell install.
- **Priority:** S

### FR-072 — Flag/description completion metadata
- **Statement:** Completion **SHOULD** surface flag descriptions where the shell supports it.
- **Rationale:** Discoverability.
- **AC:** zsh/fish show descriptions alongside completions.
- **Priority:** S

---

## 9. LSP / IDE Tooling

### FR-073 — LSP server
- **Statement:** The system **MUST** provide an LSP server for FlowDSL.
- **Rationale:** Editor-grade authoring.
- **AC:** An LSP-capable editor connects and receives capabilities.
- **Priority:** S

### FR-074 — tree-sitter grammar
- **Statement:** The system **MUST** publish a `tree-sitter-flow` grammar for `*.flow`.
- **Rationale:** Highlighting and structural editing.
- **AC:** The grammar parses valid flows and powers highlighting.
- **Priority:** S

### FR-075 — Diagnostics
- **Statement:** The LSP **MUST** provide real-time diagnostics (syntax, type, unresolved refs) as the user types.
- **Rationale:** Fast feedback.
- **AC:** Introducing a type error surfaces a diagnostic at the correct range.
- **Priority:** S

### FR-076 — Completion (LSP)
- **Statement:** The LSP **MUST** provide completion for step names, inputs, CEL identifiers, and imports.
- **Rationale:** Authoring speed.
- **AC:** Completing an upstream output reference lists valid outputs.
- **Priority:** S

### FR-077 — Hover & signature help
- **Statement:** The LSP **SHOULD** provide hover docs and signature help for steps/plugins/CEL functions.
- **Rationale:** In-context docs.
- **AC:** Hover over a step shows its docstring and I/O types.
- **Priority:** S

### FR-078 — Go-to-definition / find references
- **Statement:** The LSP **SHOULD** support go-to-definition and find-references across imported flows.
- **Rationale:** Navigation.
- **AC:** Jumping to an imported step opens its definition.
- **Priority:** S

### FR-079 — Formatting (LSP)
- **Statement:** The LSP **SHOULD** expose document formatting using the canonical formatter (FR-027).
- **Rationale:** Consistency.
- **AC:** Editor "format document" reformats to canonical style.
- **Priority:** S

---

## 10. Config

### FR-080 — `conduit.yaml` via Koanf
- **Statement:** The system **MUST** load configuration from `conduit.yaml` using Koanf.
- **Rationale:** Canonical config.
- **AC:** A valid `conduit.yaml` is loaded and applied.
- **Priority:** M

### FR-081 — Layered configuration precedence
- **Statement:** Config **MUST** merge in documented precedence: built-in defaults < system < user < project `conduit.yaml` < env < flags.
- **Rationale:** Environment-specific overrides.
- **AC:** A flag overrides env, which overrides file, which overrides defaults.
- **Priority:** M

### FR-082 — Environment variable binding
- **Statement:** Config keys **MUST** bind to `CONDUIT_`-prefixed env vars.
- **Rationale:** Twelve-factor.
- **AC:** Setting `CONDUIT_LOG_LEVEL=debug` sets the corresponding key.
- **Priority:** M

### FR-083 — Config validation
- **Statement:** The system **MUST** validate config against a schema and fail fast with clear errors.
- **Rationale:** Prevent silent misconfiguration.
- **AC:** An unknown/invalid key produces a descriptive error (strict mode) or warning (lenient mode).
- **Priority:** M

### FR-084 — Config discovery
- **Statement:** The system **MUST** discover `conduit.yaml` by walking up from cwd, honoring `--config` override.
- **Rationale:** Project-scoped config.
- **AC:** Running in a subdirectory finds the nearest ancestor `conduit.yaml`.
- **Priority:** M

### FR-085 — Config introspection
- **Statement:** The system **SHOULD** provide `conduit config show` rendering the effective merged config with sources.
- **Rationale:** Debuggability.
- **AC:** Output shows each effective key and its originating layer; secrets are redacted.
- **Priority:** S

---

## 11. Secrets

### FR-086 — Pluggable secret providers
- **Statement:** The system **MUST** support pluggable secret providers (env, file, OS keychain, Vault, cloud KMS).
- **Rationale:** Integrate with existing secret stores.
- **AC:** A secret resolves via a configured provider at runtime.
- **Priority:** S

### FR-087 — Least-privilege injection
- **Statement:** Secrets **MUST** be injected only into steps that reference them, scoped to that step's process.
- **Rationale:** Minimize exposure.
- **AC:** A secret is absent from steps that do not reference it.
- **Priority:** S

### FR-088 — Redaction in all outputs
- **Statement:** Secret values **MUST** be redacted in logs, traces, dry-run output, errors, and run summaries.
- **Rationale:** Prevent leakage (persona Nadia).
- **AC:** Redaction tests confirm no secret value appears in any output channel.
- **Priority:** S

### FR-089 — No secret persistence in plaintext
- **Statement:** Secrets **MUST NOT** be persisted to run state or checkpoints in plaintext.
- **Rationale:** Data-at-rest safety.
- **AC:** State inspection shows references/handles, never plaintext secret values.
- **Priority:** S

### FR-090 — Secret rotation compatibility
- **Statement:** The system **SHOULD** resolve secrets at execution time so rotation takes effect without re-authoring flows.
- **Rationale:** Operational hygiene.
- **AC:** Rotating a secret in the provider changes the resolved value on next run.
- **Priority:** S

### FR-091 — Secret access auditing
- **Statement:** Secret resolution **MUST** be audit-logged by reference (not value).
- **Rationale:** Compliance.
- **AC:** Each secret access yields an audit record with the reference and consumer step.
- **Priority:** S

---

## 12. AuthN / AuthZ

### FR-092 — Identity resolution
- **Statement:** The system **MUST** resolve an operator/agent identity (OS user, token, or agent principal) for sensitive operations.
- **Rationale:** Attribution and RBAC.
- **AC:** Sensitive commands record the acting identity.
- **Priority:** S

### FR-093 — Role-based access control
- **Statement:** The system **MUST** support RBAC gating commands/flows/tools by role/permission.
- **Rationale:** Least privilege (persona Priya/Nadia).
- **AC:** A user lacking permission is denied with the permission-denied exit code and an audit event.
- **Priority:** S

### FR-094 — Per-tool permission scopes (agent API)
- **Statement:** Each flow/command exposed as an agent tool **MUST** carry a declared permission scope enforced at invocation.
- **Rationale:** Safe agent actions (persona Mira).
- **AC:** An agent invoking a tool outside its granted scope is denied and audited.
- **Priority:** S

### FR-095 — Token/credential handling
- **Statement:** Auth tokens **MUST** be handled as secrets (never logged) and support expiry/refresh.
- **Rationale:** Security.
- **AC:** Tokens are redacted; expired tokens are rejected.
- **Priority:** S

### FR-096 — SSO/OIDC integration hook
- **Statement:** The system **SHOULD** provide an interface to integrate enterprise identity (OIDC/SSO) for authenticating operators/agents.
- **Rationale:** Enterprise adoption.
- **AC:** An OIDC-configured deployment authenticates via the provider.
- **Priority:** C

### FR-097 — Authorization audit
- **Statement:** All allow/deny decisions **MUST** be audit-logged.
- **Rationale:** Compliance/forensics.
- **AC:** Each authz decision produces an audit record with identity, resource, decision.
- **Priority:** S

---

## 13. Observability

### FR-098 — Structured logging
- **Statement:** The system **MUST** emit structured logs (JSON option) with run/step IDs and levels.
- **Rationale:** Machine-parseable diagnostics.
- **AC:** Logs include correlation IDs and are valid JSON in JSON mode.
- **Priority:** M

### FR-099 — Log levels & verbosity
- **Statement:** The system **MUST** support configurable log levels and `-v/--verbose`, `--quiet`.
- **Rationale:** Signal control.
- **AC:** Level filtering works; verbose adds detail without changing exit semantics.
- **Priority:** M

### FR-100 — OpenTelemetry traces
- **Statement:** The runtime **MUST** emit OTel traces with spans per run and per step.
- **Rationale:** Distributed debugging and timing.
- **AC:** A run produces a trace with a root span and child step spans; exportable via OTLP.
- **Priority:** S

### FR-101 — Metrics
- **Statement:** The system **SHOULD** emit metrics (run counts, durations, failures, retries) via OTel/Prometheus-compatible export.
- **Rationale:** Fleet monitoring.
- **AC:** Metrics are scrapeable/exportable and reflect run activity.
- **Priority:** S

### FR-102 — Audit log stream
- **Statement:** The system **MUST** produce a tamper-evident-friendly audit log for security-relevant events (see FR-067, FR-091, FR-097).
- **Rationale:** Compliance.
- **AC:** Audit events are separable from operational logs and export to a sink.
- **Priority:** S

### FR-103 — Correlation with external systems
- **Statement:** The system **SHOULD** propagate/accept trace context (W3C traceparent) to correlate with callers.
- **Rationale:** End-to-end tracing (CI, agents).
- **AC:** An incoming traceparent is honored and continued.
- **Priority:** S

### FR-104 — Progress reporting
- **Statement:** The runtime **MUST** report machine-readable progress events consumable by TUI/agent/CI.
- **Rationale:** Live status.
- **AC:** Progress events stream with step transitions.
- **Priority:** S

---

## 14. Expressions (CEL)

### FR-105 — CEL-Go integration
- **Statement:** The system **MUST** evaluate expressions using CEL-Go.
- **Rationale:** Safe, typed, embeddable expressions.
- **AC:** A CEL expression referencing flow inputs/step outputs evaluates correctly.
- **Priority:** M

### FR-106 — Typed CEL environment
- **Statement:** The CEL environment **MUST** be typed with flow inputs, step outputs, and a curated function set.
- **Rationale:** Compile-time safety.
- **AC:** Referencing an unknown identifier is a compile-time error.
- **Priority:** M

### FR-107 — CEL sandbox limits
- **Statement:** CEL evaluation **MUST** enforce cost/time/recursion limits to prevent abuse — see [Security Requirements](05-security-requirements.md).
- **Rationale:** DoS prevention.
- **AC:** An over-budget expression is rejected/aborted with a resource error.
- **Priority:** M

### FR-108 — No ambient side effects
- **Statement:** CEL functions **MUST NOT** perform I/O or side effects; only pure, whitelisted functions are exposed.
- **Rationale:** Safety and determinism.
- **AC:** No exposed CEL function can read files/network.
- **Priority:** M

### FR-109 — Custom function extension (governed)
- **Statement:** The system **SHOULD** allow registering additional pure CEL functions via a governed, reviewed mechanism.
- **Rationale:** Extensibility without unsafe escape.
- **AC:** A registered custom function is available and remains side-effect-free.
- **Priority:** C

---

## 15. AI-Agent API

### FR-110 — Tool discovery endpoint
- **Statement:** The system **MUST** expose flows/commands as discoverable tools with JSON-schema input/output.
- **Rationale:** Agents need a machine-readable catalog (persona Mira).
- **AC:** Discovery returns each tool with name, description (from docstrings), and I/O schema.
- **Priority:** S

### FR-111 — Typed invocation
- **Statement:** The agent API **MUST** accept typed, schema-validated invocations and return typed results.
- **Rationale:** Safe, structured calls.
- **AC:** An invocation with a schema-invalid payload is rejected before execution.
- **Priority:** S

### FR-112 — Per-invocation permissioning
- **Statement:** The agent API **MUST** enforce per-tool permission scopes (FR-094) on every invocation.
- **Rationale:** Least privilege for agents.
- **AC:** Out-of-scope invocations are denied and audited.
- **Priority:** S

### FR-113 — Sandboxed execution for agent calls
- **Statement:** Agent-invoked flows **MUST** run under the same sandboxing/limits as CLI runs (plugins, CEL, resources).
- **Rationale:** Consistent safety.
- **AC:** An agent-invoked flow cannot exceed configured resource/sandbox limits.
- **Priority:** S

### FR-114 — Full audit of agent actions
- **Statement:** Every agent invocation **MUST** produce an audit record (principal, tool, inputs hash, outcome).
- **Rationale:** Reviewability (persona Mira/Nadia).
- **AC:** Each agent call yields an audit record with typed I/O metadata.
- **Priority:** S

### FR-115 — Streaming/async results
- **Statement:** The agent API **SHOULD** support streaming progress and async long-running invocations with a handle.
- **Rationale:** Long flows.
- **AC:** A long invocation returns a handle and streams progress until completion.
- **Priority:** S

### FR-116 — Dry-run over the agent API
- **Statement:** The agent API **MUST** support dry-run invocation returning the plan without side effects.
- **Rationale:** Let agents preview/plan safely.
- **AC:** A dry-run agent call returns the plan and performs no side effects.
- **Priority:** S

### FR-117 — Rate limiting & quotas
- **Statement:** The agent API **SHOULD** support per-principal rate limits/quotas.
- **Rationale:** Abuse and cost control.
- **AC:** Exceeding a quota yields a throttling response, audited.
- **Priority:** C

### FR-118 — Transport & framing
- **Statement:** The agent API **MUST** be exposed over a documented transport (gRPC; optional HTTP/JSON) with authenticated framing.
- **Rationale:** Interoperability with agent frameworks.
- **AC:** A client connects over the documented transport with authentication and lists tools.
- **Priority:** S

---

## 16. TUI

### FR-119 — Live run view
- **Statement:** The system **MUST** provide a Bubble Tea TUI showing live step status and logs during a run.
- **Rationale:** Operator situational awareness (persona Sam).
- **AC:** Running with `--tui` shows a live-updating DAG/step view.
- **Priority:** S

### FR-120 — Run inspection
- **Statement:** The TUI **SHOULD** allow inspecting a past run's per-step logs/outputs.
- **Rationale:** Post-hoc debugging.
- **AC:** Selecting a completed run shows step details.
- **Priority:** S

### FR-121 — Non-interactive fallback
- **Statement:** The TUI **MUST** degrade gracefully to plain output when stdout is not a TTY (CI, pipes).
- **Rationale:** Portability.
- **AC:** In a non-TTY context, output is plain and script-safe.
- **Priority:** M

### FR-122 — Accessibility in TUI
- **Statement:** The TUI **SHOULD** respect `--no-color`, honor `NO_COLOR`, and remain usable on screen readers/limited terminals.
- **Rationale:** Accessibility — see [NFRs](04-non-functional-requirements.md).
- **AC:** With `NO_COLOR` set, the TUI renders without color and remains legible.
- **Priority:** S

---

## 17. Traceability

| Capability area | FR range | Epic (PRD) |
|-----------------|----------|------------|
| Command Framework | FR-001–014 | E1 |
| FlowDSL | FR-015–028 | E2 |
| DAG/Workflow | FR-029–040 | E3 |
| Execution Runtime | FR-041–050 | E4 |
| State & Checkpoints | FR-051–057 | E5 |
| Plugins | FR-058–067 | E6 |
| Completion | FR-068–072 | E15 |
| LSP/IDE | FR-073–079 | E10 |
| Config | FR-080–085 | E7 |
| Secrets | FR-086–091 | E7 |
| AuthN/AuthZ | FR-092–097 | E13 |
| Observability | FR-098–104 | E9 |
| Expressions (CEL) | FR-105–109 | E8 |
| AI-Agent API | FR-110–118 | E12 |
| TUI | FR-119–122 | E11 |

Security-critical FRs (FR-062/063/067, FR-086–091, FR-092–097, FR-107/108, FR-112–114) map to `SEC-###` in [Security Requirements](05-security-requirements.md). Performance-critical FRs (FR-070) map to budgets in [Non-Functional Requirements](04-non-functional-requirements.md).
