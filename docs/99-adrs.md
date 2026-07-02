# 99 — Architecture Decision Records (ADRs)

> **Platform:** Conduit · **Binary:** `conduit` (alias `cdt`) · **Module:** `github.com/conduit-io/conduit`
> **Go:** 1.24+ · **Status:** Architecture Baseline v1.0 · **Owner:** Platform Architecture · **Date:** 2026-07-02

**Related documents:** [10 — Platform Architecture](10-platform-architecture.md) · [95 — Roadmap](95-roadmap.md) · [96 — Milestone Plan](96-milestone-plan.md) · [97 — Risk Register](97-risk-register.md) · [98 — Tech Debt Register](98-tech-debt-register.md) · [03 — Functional Requirements](03-functional-requirements.md) · [04 — Non-Functional Requirements](04-non-functional-requirements.md) · [05 — Security Requirements](05-security-requirements.md)

---

## How to read these ADRs

Each record follows the standard Michael-Nygard-style format: **Title, Status, Context, Decision, Consequences (positive / negative / neutral), Alternatives considered**. ADRs are **immutable once Accepted** — a change is a *new* ADR that supersedes the old one. Every ADR is self-contained and decisive: it states *the* decision, not a menu of options.

| Status value | Meaning |
|---|---|
| **Proposed** | Under discussion |
| **Accepted** | Ratified; binding on the codebase |
| **Superseded by ADR-####** | Replaced |
| **Deprecated** | No longer applies |

### Index

| ADR | Title | Status |
|---|---|---|
| [ADR-0001](#adr-0001--go-124-as-implementation-language) | Go 1.24+ as implementation language | Accepted |
| [ADR-0002](#adr-0002--cobra-for-the-cli-command-framework) | Cobra for the CLI command framework | Accepted |
| [ADR-0003](#adr-0003--koanf-for-configuration) | Koanf for configuration | Accepted |
| [ADR-0004](#adr-0004--participle-v2-for-the-flowdsl-parser) | Participle v2 for the FlowDSL parser | Accepted |
| [ADR-0005](#adr-0005--cel-go-as-the-expression-engine) | CEL-Go as the expression engine | Accepted |
| [ADR-0006](#adr-0006--hashicorp-go-plugin-over-grpc-for-plugins) | HashiCorp go-plugin over gRPC for plugins | Accepted |
| [ADR-0007](#adr-0007--bubble-tea-for-the-tui) | Bubble Tea for the TUI | Accepted |
| [ADR-0008](#adr-0008--custom-lsp-server-reusing-the-compiler) | Custom LSP server reusing the compiler | Accepted |
| [ADR-0009](#adr-0009--tree-sitter-grammar-for-syntax-highlighting) | Tree-sitter grammar for syntax highlighting | Accepted |
| [ADR-0010](#adr-0010--dag-based-workflow-execution-model) | DAG-based workflow execution model | Accepted |
| [ADR-0011](#adr-0011--event-sourced-run-state) | Event-sourced run state | Accepted |
| [ADR-0012](#adr-0012--embedded-state-store-boltdb-local-postgres-server) | Embedded state store (BoltDB local / Postgres server) | Accepted |
| [ADR-0013](#adr-0013--opentelemetry-for-observability) | OpenTelemetry for observability | Accepted |
| [ADR-0014](#adr-0014--slog-for-structured-logging) | `slog` for structured logging | Accepted |
| [ADR-0015](#adr-0015--four-independent-version-streams) | Four independent version streams | Accepted |
| [ADR-0016](#adr-0016--monorepo-single-module-layout) | Monorepo / single-module layout | Accepted |
| [ADR-0017](#adr-0017--machine-readable-ai-agent-api) | Machine-readable AI-agent API | Accepted |
| [ADR-0018](#adr-0018--secrets-provider-abstraction) | Secrets provider abstraction | Accepted |
| [ADR-0019](#adr-0019--cel-for-authorization-policies) | CEL for authorization policies | Accepted |

---

## ADR-0001 — Go 1.24+ as implementation language

**Status:** Accepted (2026-07)

**Context.** Conduit is a CLI/automation platform that must ship as a **single static binary**, cross-compile to linux/macOS/windows, orchestrate concurrent work, and be pleasant to iterate on. We need strong concurrency, a large stdlib, and an ecosystem for CLIs, gRPC, and expression engines. Candidate ecosystems: Go, Rust, Node/TypeScript.

**Decision.** Implement Conduit in **Go, minimum version 1.24**. Distribute as static single binaries per OS/arch. Pin the toolchain floor in `go.mod` and CI.

**Consequences.**
- *Positive:* trivial static binaries and cross-compilation; first-class goroutines/`context` for the concurrent DAG runtime ([10 §7](10-platform-architecture.md)); mature Cobra/gRPC/CEL/go-plugin ecosystem; fast compile-edit-test loop; end users need no runtime.
- *Negative:* requires Go 1.24+ toolchain for *contributors* (not users); GC pauses are irrelevant for CLI workloads but noted; CGO dependencies would undermine easy cross-compile (mitigated by preferring pure-Go — see ADR-0012, [RISK-015](97-risk-register.md)).
- *Neutral:* raising the Go floor is itself a versioned decision.

**Alternatives considered.** **Rust** — best-in-class safety/perf but slower iteration and steeper contributor ramp for a broad platform. **Node/TypeScript** — great DX but a runtime dependency and weak static-binary story, unacceptable for a CLI. Related risk: [RISK-010](97-risk-register.md).

---

## ADR-0002 — Cobra for the CLI command framework

**Status:** Accepted (2026-07)

**Context.** Conduit needs subcommands, POSIX/GNU flags, generated shell completion, and help — the standard CLI surface. Options: Cobra, urfave/cli, kong, or hand-rolled `flag`.

**Decision.** Use **Cobra** as the command framework, wired as a *driving adapter* over the application services ([10 §2](10-platform-architecture.md)) so the domain never imports Cobra (enforced by depguard).

**Consequences.**
- *Positive:* de-facto Go CLI standard (kubectl, gh, hugo); built-in completion generators feed the completion engine ([50](50-completion-engine.md)); rich flag/args model; huge community familiarity.
- *Negative:* Cobra brings globals/`init()` patterns we must contain behind the adapter boundary; some boilerplate.
- *Neutral:* completion scripts still need per-shell adapters ([51](51-bash-completion.md)–[54](54-powershell-completion.md)).

**Alternatives considered.** **urfave/cli** — lighter but weaker completion/subcommand ergonomics. **kong** — elegant struct-tag model but smaller ecosystem. **stdlib `flag`** — insufficient for a multi-command platform.

---

## ADR-0003 — Koanf for configuration

**Status:** Accepted (2026-07)

**Context.** Config must merge defaults → file (`conduit.yaml`) → env → flags with clear precedence, support multiple formats, and avoid heavy global state ([60](60-configuration.md)).

**Decision.** Use **Koanf** as the config loader, exposed to the domain via a `ConfigProvider` port.

**Consequences.**
- *Positive:* modular providers/parsers; explicit, testable precedence; far less global state than Viper; easy env/flag overlay.
- *Negative:* smaller community than Viper; we assemble the provider stack ourselves.
- *Neutral:* config schema is independently versioned (ADR-0015; [TD-019](98-tech-debt-register.md)).

**Alternatives considered.** **Viper** — ubiquitous but heavy, global-state-centric, and awkward to test in a hexagonal design. **Hand-rolled** — reinvents precedence and parsing.

---

## ADR-0004 — Participle v2 for the FlowDSL parser

**Status:** Accepted (2026-07)

**Context.** FlowDSL (`*.flow`) needs a typed AST, good error messages, error-tolerant parsing for the LSP, and pure-Go builds. FlowDSL is intentionally non-Turing-complete and keyword-headed ([20](20-dsl-grammar.md)). Options: Participle v2, hand-written recursive descent, ANTLR, goyacc.

**Decision.** Parse FlowDSL with **Participle v2**, using struct-tag grammar productions that map directly to typed AST nodes ([22](22-parser-design.md),[23](23-ast-design.md)). Grammar is LL(k)/PEG-shaped with no left recursion.

**Consequences.**
- *Positive:* grammar-as-Go-structs yields a typed AST with minimal glue; solid error reporting; pure Go (no codegen toolchain in the build); fast to iterate.
- *Negative:* Participle's model constrains grammar shape (no left recursion; limited context-sensitivity) — mitigated by design and an escape hatch to custom lexer states ([RISK-003](97-risk-register.md)); full CEL type-check is layered separately ([TD-004](98-tech-debt-register.md)).
- *Neutral:* the AST contract is stable behind the parser, so the parser could be swapped without breaking downstream stages.

**Alternatives considered.** **Hand-written recursive descent** — maximal control, maximal maintenance. **ANTLR** — powerful but a Java-based codegen toolchain and heavier runtime. **goyacc** — LALR generator with poor error messages and clumsy Go ergonomics.

---

## ADR-0005 — CEL-Go as the expression engine

**Status:** Accepted (2026-07)

**Context.** FlowDSL embeds an expression sublanguage for computed values, `when` guards, and interpolations ([25](25-expression-engine.md)). Expressions may originate from untrusted sources (agents, shared flows), so they must be **sandboxed, non-Turing-complete, type-checked, and cost-bounded**. Options: CEL-Go, Starlark, `expr`, Lua.

**Decision.** Embed **CEL-Go** (Common Expression Language) as the expression engine, with a curated function library, static type-checking at compile time, and enforced **cost limits** (no unbounded loops/regex, per-eval budget).

**Consequences.**
- *Positive:* non-Turing-complete by design → guaranteed termination; static type-checking catches errors at compile time; fast; purpose-built for evaluating untrusted expressions safely; used widely (Kubernetes, Envoy).
- *Negative:* deliberately limited (no general programming); cost model must be tuned ([RISK-017](97-risk-register.md)); sandbox must still be red-teamed ([RISK-001](97-risk-register.md), [M3](96-milestone-plan.md)).
- *Neutral:* CEL is also reused for authz policies (ADR-0019), unifying the expression surface.

**Alternatives considered.** **Starlark** — Turing-complete-ish with loops; harder to bound. **`expr`** — nice but weaker type system and sandbox guarantees. **Lua** — full language, large sandbox-escape surface, unacceptable for untrusted input.

---

## ADR-0006 — HashiCorp go-plugin over gRPC for plugins

**Status:** Accepted (2026-07)

**Context.** Conduit must be extensible with **capability plugins** that may be authored in multiple languages, must not be able to corrupt host memory, and need a stable, versioned contract ([40](40-plugin-architecture.md)). Options: Go `plugin` package, HashiCorp go-plugin (gRPC), WASM.

**Decision.** Use **HashiCorp go-plugin with the gRPC transport**. Plugins are separate OS processes reached over a Unix socket / Windows named pipe, negotiated by the go-plugin handshake (magic cookie + protocol version). The host talks to plugins only through the `CapabilityInvoker` outbound port, isolating go-plugin behind one adapter.

**Consequences.**
- *Positive:* process isolation (a crashing plugin cannot corrupt the host — [RISK-002/007](97-risk-register.md)); language-neutral gRPC contract enables non-Go plugins later (ADR-0007 SDKs post-GA, [TD-007](98-tech-debt-register.md)); battle-tested handshake/versioning; streaming support.
- *Negative:* per-call serialization overhead vs in-process; protocol churn risk isolated by the port ([RISK-002](97-risk-register.md)); alpha sandboxing is process-isolation only ([TD-003](98-tech-debt-register.md)).
- *Neutral:* plugin protocol is an independent version stream (ADR-0015); plugins are signed (cosign) and verified on load ([69](69-threat-model.md)).

**Alternatives considered.** **Go `plugin` package** — same-process, Go-only, notoriously fragile across versions. **WASM** — attractive sandbox but immature host/FFI ergonomics and no mature multi-language capability story in 2026.

---

## ADR-0007 — Bubble Tea for the TUI

**Status:** Accepted (2026-07)

**Context.** Conduit ships an interactive terminal UI for run monitoring and authoring aids ([82](82-developer-experience.md)). It must be composable and, crucially, **testable**. Options: Bubble Tea, tview, termui.

**Decision.** Build the TUI on **Bubble Tea** (with Lip Gloss for styling and Bubbles for widgets), as a driving adapter.

**Consequences.**
- *Positive:* Elm architecture (model/update/view) is deterministic and unit-testable; composable widgets; strong ecosystem and aesthetics.
- *Negative:* Elm-style state modeling is a learning curve; not suited to headless/CI (which uses `--json`).
- *Neutral:* TUI consumes the same application services as the CLI — no divergent logic.

**Alternatives considered.** **tview** — widget-rich but imperative and harder to test. **termui** — dashboard-oriented, less general.

---

## ADR-0008 — Custom LSP server reusing the compiler

**Status:** Accepted (2026-07)

**Context.** Editors need diagnostics, completion, hover, and go-to-definition for `.flow` files ([55](55-intellisense.md),[56](56-lsp-architecture.md)). The cardinal requirement: **editor diagnostics must never diverge from runtime semantics**.

**Decision.** Implement a **custom LSP server** (`conduit lsp`, JSON-RPC 2.0) that **reuses the exact same FlowDSL front-end** used by `conduit run` in-process ([10 §5](10-platform-architecture.md)). No second, editor-only analyzer.

**Consequences.**
- *Positive:* single source of truth for semantics — a diagnostic in the editor is a diagnostic at runtime; new language features light up in the editor for free.
- *Negative:* the front-end must be fast and incremental enough for interactive latency ([RISK-006](97-risk-register.md), [TD-010](98-tech-debt-register.md)); we own the LSP protocol plumbing.
- *Neutral:* highlighting is a separate concern handled by Tree-sitter (ADR-0009).

**Alternatives considered.** **Regex/ad-hoc editor analysis** — inevitably drifts from the compiler; rejected. **Reusing an existing generic LSP framework** — none matches our front-end reuse requirement.

---

## ADR-0009 — Tree-sitter grammar for syntax highlighting

**Status:** Accepted (2026-07)

**Context.** Fast, incremental syntax highlighting and structural editing need a grammar that editors (VS Code, Neovim) understand natively ([58](58-tree-sitter-grammar.md)). The LSP (ADR-0008) handles semantics; highlighting is a distinct, latency-sensitive concern.

**Decision.** Author a **`tree-sitter-flow` grammar** for highlighting and structural queries, kept in the editor/tooling path only — **the core runtime stays CGO-free**.

**Consequences.**
- *Positive:* incremental, resilient highlighting; broad editor support; structural selection/folding.
- *Negative:* a *second* grammar formalism alongside Participle → drift risk, mitigated by validating both against a shared corpus in CI ([TD-009](98-tech-debt-register.md)); tree-sitter is CGO, adding build complexity confined to tooling ([RISK-004](97-risk-register.md)).
- *Neutral:* if native grammar build blocks a platform, highlighting can fall back to LSP semantic tokens.

**Alternatives considered.** **LSP semantic tokens only** — simpler but less responsive and no structural queries. **Regex TextMate grammar** — brittle, non-incremental.

---

## ADR-0010 — DAG-based workflow execution model

**Status:** Accepted (2026-07)

**Context.** FlowDSL workflows are sets of tasks with dependencies and data passing. We need deterministic ordering, parallelism where safe, cycle detection, and resumability ([30](30-workflow-dag.md),[31](31-execution-runtime.md)).

**Decision.** Model workflows as a **directed acyclic graph (DAG)**: plan = build nodes/edges, detect cycles (fail fast), topologically levelize into ready-sets; execute ready-sets concurrently under a bounded worker pool. `for_each`/`matrix` expand the graph; `when` guards prune it.

**Consequences.**
- *Positive:* deterministic, analyzable execution; natural topo-level parallelism ([10 §7](10-platform-architecture.md)); cycles caught before any side effect; clean checkpoint boundaries for resume (ADR-0011).
- *Negative:* purely declarative — no imperative loops/gotos (by design, matches non-Turing-complete FlowDSL); dynamic fan-out must be expressed declaratively.
- *Neutral:* the planner is pure graph math (no I/O), enabling deterministic tests.

**Alternatives considered.** **Imperative/sequential script model** — simpler but loses parallelism, analyzability, and safe resumability. **Reactive/streaming dataflow** — overkill for the target task-orchestration use case.

---

## ADR-0011 — Event-sourced run state

**Status:** Accepted (2026-07)

**Context.** Runs must be **durable, resumable, and auditable**; a crash mid-run must not lose progress or cause duplicate side effects on idempotent tasks ([32](32-state-management.md),[71](71-recovery-strategy.md)).

**Decision.** Persist run state as an **append-only event log** (event sourcing): every task state transition is an event; current run state is a fold over events. Resume replays the log to the last checkpoint.

**Consequences.**
- *Positive:* crash-safe and resumable ([M4](96-milestone-plan.md)); full audit trail for security/compliance; append-only writes reduce corruption surface ([RISK-011](97-risk-register.md)); natural fit for the event bus ([67](67-event-model.md)).
- *Negative:* reconstructing state requires a fold (mitigated by periodic snapshots); log growth needs compaction.
- *Neutral:* the event log underpins observability and post-hoc analysis.

**Alternatives considered.** **Mutable last-write-wins state** — simpler but loses audit trail and complicates safe resume. **External durable orchestrator (Temporal)** — heavy server dependency, contrary to the single-binary goal.

---

## ADR-0012 — Embedded state store (BoltDB local / Postgres server)

**Status:** Accepted (2026-07)

**Context.** Local usage must be **zero-config and dependency-free** (single binary, no server). Server/daemon mode (`conduit serve`) needs a **shared, durable, multi-run** store ([10 §10](10-platform-architecture.md)). The store sits behind the `StateStore` outbound port.

**Decision.** Use **BoltDB** (pure-Go embedded key/value) as the default **local** state store, and **Postgres** as the **server** backend. Both implement the same `StateStore` port; the event log (ADR-0011) is written into whichever adapter is active.

**Consequences.**
- *Positive:* BoltDB is pure Go → preserves easy static cross-compilation ([RISK-015](97-risk-register.md)) and zero-config local UX; Postgres gives shared, scalable server state; single port, swappable adapters.
- *Negative:* two adapters to maintain and test; Postgres backend is deferred past GA ([TD-005](98-tech-debt-register.md), [95 §8](95-roadmap.md)).
- *Neutral:* SQLite was rejected primarily to keep the core CGO-free.

**Alternatives considered.** **SQLite** — excellent embedded SQL but CGO (or immature pure-Go ports) complicates cross-compilation. **BadgerDB** — capable but heavier/more operationally complex than needed for local. **Files/JSON** — no transactionality, corruption-prone.

---

## ADR-0013 — OpenTelemetry for observability

**Status:** Accepted (2026-07)

**Context.** Conduit must emit traces, metrics, and logs that enterprises can route to their existing backends without vendor lock-in ([64](64-observability.md),[66](66-telemetry.md)).

**Decision.** Instrument with the **OpenTelemetry** Go SDK, exporting via **OTLP**. Tracing/metrics/logging are outbound ports (`Tracer`/`Meter`/`Logger`) with an OTel adapter; the domain never imports OTel directly.

**Consequences.**
- *Positive:* vendor-neutral; OTLP fans out to Prometheus/Jaeger/vendors; correlated trace/metric/log context flows through `context.Context` ([10 §8](10-platform-architecture.md)).
- *Negative:* OTel Go API has historically churned ([RISK-020](97-risk-register.md)) — isolated behind ports; some overhead when enabled (off by default in CLI mode).
- *Neutral:* logs bridge from `slog` (ADR-0014) into OTel.

**Alternatives considered.** **Vendor-specific SDKs** — lock-in. **Prometheus + hand-rolled tracing** — no unified, correlated telemetry model.

---

## ADR-0014 — `slog` for structured logging

**Status:** Accepted (2026-07)

**Context.** We need structured, leveled logging that is stdlib-native, low-dependency, and bridgeable to OTel ([65](65-logging.md)).

**Decision.** Use the standard library **`log/slog`** as the logging API, behind a `Logger` port, with a handler that bridges to OpenTelemetry (ADR-0013) and redacts secrets ([RISK-013](97-risk-register.md)).

**Consequences.**
- *Positive:* zero external dependency; structured key/value logs; standard, future-proof; easy to fake in tests; pluggable handlers (JSON for CI, pretty for TUI, OTel bridge for serve).
- *Negative:* newer than zap/zerolog so fewer third-party handlers (we write the ones we need); performance is adequate, not the absolute fastest.
- *Neutral:* redaction is enforced in the handler, not at call sites.

**Alternatives considered.** **zap / zerolog** — faster but external and pre-`slog`; the stdlib standard now covers our needs. **`log`** — unstructured, insufficient.

---

## ADR-0015 — Four independent version streams

**Status:** Accepted (2026-07)

**Context.** Conduit has several surfaces that evolve at different rates and are consumed by different audiences: the CLI itself, the FlowDSL grammar, the plugin gRPC protocol, and the config schema. Coupling them into one version number would force needless breaking bumps and confuse compatibility ([83](83-api-standards-versioning.md)).

**Decision.** Maintain **four independent SemVer streams**: (1) **CLI/binary** version, (2) **FlowDSL grammar** version (`flow/1.0` …), (3) **plugin protocol** version, (4) **config schema** version. Each has its own compatibility and deprecation policy.

**Consequences.**
- *Positive:* a grammar change need not bump the CLI major; plugins negotiate the protocol version independently; users reason about compatibility per surface; reduces false-breaking churn ([RISK-012](97-risk-register.md), [RISK-018](97-risk-register.md)).
- *Negative:* more version bookkeeping and a compatibility matrix to publish and test.
- *Neutral:* all four streams are frozen/guaranteed at [v1.0 GA](96-milestone-plan.md).

**Alternatives considered.** **Single unified version** — simple to state but forces spurious breaking bumps and muddies compatibility. **Unversioned surfaces** — unacceptable for an enterprise platform.

---

## ADR-0016 — Monorepo / single-module layout

**Status:** Accepted (2026-07)

**Context.** The codebase spans core runtime, DSL front-end, LSP, TUI, SDK, and reference plugins ([80](80-repository-structure.md)). We must choose between a single Go module in a monorepo vs multiple modules/repos.

**Decision.** Use a **single Go module** (`github.com/conduit-io/conduit`) in a **monorepo**. Enforce internal boundaries with package structure (`internal/domain`, `internal/adapter`, …) and CI arch-lint (depguard) rather than module boundaries. Reference plugins live in-repo but build as separate binaries.

**Consequences.**
- *Positive:* atomic cross-cutting changes (grammar + LSP + runtime in one PR); one dependency graph; simplest CI; refactors are trivial; arch-lint enforces the hexagonal rule ([10 §2.1](10-platform-architecture.md)).
- *Negative:* consumers importing the public SDK pull a larger module (mitigated by keeping the public SDK surface small and stable, [44](44-public-sdk.md)); no independent per-component versioning at the module level (handled by ADR-0015 streams instead).
- *Neutral:* can extract the SDK into its own module later if external consumption warrants it.

**Alternatives considered.** **Multi-module monorepo** — independent versioning but painful `replace`/tag choreography during rapid development. **Polyrepo** — worst change-atomicity for a tightly coupled platform at this stage.

---

## ADR-0017 — Machine-readable AI-agent API

**Status:** Accepted (2026-07)

**Context.** A core differentiator is a **safe, structured tool surface for AI agents** ([00](00-executive-summary.md),[44](44-public-sdk.md)). Agents need to discover capabilities with typed inputs/permissions and invoke workflows without shelling out to arbitrary commands.

**Decision.** Expose a **machine-readable AI-agent API**: a JSON + gRPC gateway (`/v1/*`) served by `conduit serve`, offering a **typed capability catalog** (capabilities, input/output schemas, permissions, side-effect/destructiveness metadata) plus plan/run/query operations, all behind authz (ADR-0019) with audit and rate limits.

**Consequences.**
- *Positive:* agents get a safe, typed, discoverable contract instead of raw shell; dry-run and typed schemas reduce blast radius; audit trail (ADR-0011) supports governance; catalog also feeds docs-gen ([43](43-documentation-generation.md)).
- *Negative:* a powerful new attack/misuse surface ([RISK-021](97-risk-register.md)) — mitigated by authz, rate limits, human-in-loop for destructive ops; schema/catalog must stay in sync with capabilities.
- *Neutral:* the same catalog powers completion and docs, so it is not agent-only overhead.

**Alternatives considered.** **Shell-out / free-form command exec** — unsafe and unstructured; the exact anti-pattern Conduit exists to replace. **CLI-only, no gateway** — no discovery/typing surface for agents.

---

## ADR-0018 — Secrets provider abstraction

**Status:** Accepted (2026-07)

**Context.** Workflows need secrets (tokens, keys) from heterogeneous backends (env, OS keychain, Vault) without hard-coding a provider, and secrets must never leak into logs/state/traces ([61](61-secrets-management.md),[RISK-013](97-risk-register.md)).

**Decision.** Define a **`SecretResolver` outbound port** with pluggable adapters (env, OS keychain, Vault, …). Secrets are represented as typed, **redacting** values; resolution is late and least-privilege; the logging handler (ADR-0014) and state writer scrub secret values.

**Consequences.**
- *Positive:* backend-agnostic; new providers are new adapters, no core change; redaction is centralized and testable ([M6](96-milestone-plan.md) redaction test); least-privilege resolution.
- *Negative:* only env + one backend ship at beta ([TD-012](98-tech-debt-register.md)); redaction must be vigilant across every sink.
- *Neutral:* the same abstraction supports rotation and per-run scoping.

**Alternatives considered.** **Env-vars only** — simple but insufficient for enterprise/Vault users. **Direct Vault coupling** — locks in one backend and complicates local/dev.

---

## ADR-0019 — CEL for authorization policies

**Status:** Accepted (2026-07)

**Context.** Conduit must authorize *who/what* may run *which* capabilities — for human users and, critically, AI agents via the gateway (ADR-0017). Policies must be expressive, safe to evaluate, auditable, and ideally reuse existing machinery ([63](63-authorization.md)).

**Decision.** Express authorization policies in **CEL** (the same engine as ADR-0005), evaluated **deny-by-default** against a typed request context (identity, capability, attributes). Policy decisions are audit-logged.

**Consequences.**
- *Positive:* reuses the already-embedded, sandboxed, non-Turing-complete CEL engine — one expression language for values *and* authz; safe to evaluate untrusted-adjacent policy; type-checked and cost-bounded; deny-by-default is fail-safe.
- *Negative:* beta ships a coarse, capability-level policy model ([TD-011](98-tech-debt-register.md)); authz gaps are high-impact ([RISK-019](97-risk-register.md)) → policy test suite + pen-test at [M9](96-milestone-plan.md).
- *Neutral:* policies are versioned/audited alongside config.

**Alternatives considered.** **OPA/Rego** — powerful and standard, but adds a second policy language/runtime and dependency when CEL already meets the need. **Hard-coded RBAC** — inflexible; poor fit for attribute-based agent authz. **Casbin** — another model/dependency to learn and secure.

---

## Cross-references

- Architecture context for these decisions → [10 — Platform Architecture](10-platform-architecture.md) (see its §11 Technology Decision Table).
- When each decision is realized → [95 — Roadmap](95-roadmap.md) · [96 — Milestone Plan](96-milestone-plan.md).
- Risks arising from these decisions → [97 — Risk Register](97-risk-register.md).
- Debt these decisions defer → [98 — Tech Debt Register](98-tech-debt-register.md).
