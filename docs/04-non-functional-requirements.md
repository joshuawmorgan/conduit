# Conduit — Non-Functional Requirements

> Document ID: `04-non-functional-requirements`
> Status: Draft (v0.1.0)
> Owner: Principal Product Architect
> Last updated: 2026-07-02

Related documents:
- [Executive Summary](00-executive-summary.md)
- [Product Vision](01-product-vision.md)
- [Product Requirements (PRD)](02-prd.md)
- [Functional Requirements](03-functional-requirements.md)
- [Security Requirements](05-security-requirements.md)
- [Platform Architecture](10-platform-architecture.md)

---

## 1. Conventions

- Requirement IDs are `NFR-###`, stable and unique.
- RFC 2119 keywords (**MUST**, **SHOULD**, **MAY**) are normative.
- Each NFR states a concrete, measurable target and a **measurement method**. "Enforced in CI" means a regression against the target fails the build.
- Latency targets are stated as percentiles on the reference environment (see §12) unless noted.
- Priority: `M` (MVP), `S` (Beta), `C` (GA+).

---

## 2. Performance

### NFR-001 — Cold-start latency
- **Target:** `conduit --version` (and no-op command) **MUST** complete with p50 < 50ms and p95 < 90ms on the reference environment.
- **Rationale:** The "instant CLI" promise (differentiator D7).
- **Measurement:** Benchmark harness runs the binary N=200 times per OS/arch in CI; reports percentiles; regression > 10% over baseline fails the build.
- **Priority:** M

### NFR-002 — Completion latency
- **Target:** Shell/LSP completion **MUST** return with p95 < 100ms and p99 < 150ms.
- **Rationale:** Completion must feel native (FR-070).
- **Measurement:** Instrumented completion path benchmarked in CI with representative catalogs (100+ flows).
- **Priority:** M

### NFR-003 — Flow parse + type-check throughput
- **Target:** A 200-step `*.flow` **MUST** parse and type-check in < 150ms p95.
- **Rationale:** Fast validate/LSP feedback.
- **Measurement:** Compiler benchmark over fixture flows in CI.
- **Priority:** M

### NFR-004 — DAG scheduling overhead
- **Target:** Per-step scheduling overhead (engine bookkeeping, excluding step work) **MUST** be < 1ms p95.
- **Rationale:** Engine must not dominate runtime for many small steps.
- **Measurement:** Micro-benchmark with no-op steps; total overhead / step count.
- **Priority:** S

### NFR-005 — Plugin invocation overhead
- **Target:** gRPC round-trip overhead for a plugin call **SHOULD** be < 5ms p95 (excluding plugin work) on localhost.
- **Rationale:** Keep out-of-process cost acceptable.
- **Measurement:** Benchmark a no-op plugin capability.
- **Priority:** S

### NFR-006 — CEL evaluation latency
- **Target:** A typical CEL predicate **MUST** evaluate in < 1ms p95 within the enforced cost budget.
- **Rationale:** Expressions gate steps frequently.
- **Measurement:** CEL micro-benchmark; correlate with cost limit (SEC in [Security Requirements](05-security-requirements.md)).
- **Priority:** M

### NFR-007 — Startup memory footprint
- **Target:** Resident memory for a no-op invocation **MUST** be < 40MB RSS.
- **Rationale:** Runs on constrained CI/dev hosts.
- **Measurement:** RSS sampled at exit of a no-op run in CI.
- **Priority:** M

### NFR-008 — Binary size
- **Target:** The stripped release binary **SHOULD** be < 40MB per platform.
- **Rationale:** Distribution and container image size.
- **Measurement:** CI reports artifact size; regression > 15% flags review.
- **Priority:** S

### NFR-009 — Profile-guided optimization
- **Target:** Release builds **SHOULD** use Go PGO to optimize hot paths (startup, completion, scheduler).
- **Rationale:** Meet latency budgets sustainably.
- **Measurement:** PGO enabled in release pipeline; before/after benchmark recorded.
- **Priority:** S

---

## 3. Scalability

### NFR-010 — Flow size
- **Target:** The engine **MUST** handle flows of at least 1,000 steps without super-linear slowdown (scheduling O(V+E)).
- **Rationale:** Large orchestrations.
- **Measurement:** Scaling benchmark at 10/100/1,000 steps; verify near-linear.
- **Priority:** S

### NFR-011 — Concurrency scaling
- **Target:** The runtime **MUST** scale parallel step execution up to a configurable limit (default = NumCPU) without deadlock or unbounded goroutine growth.
- **Rationale:** Efficient use of hosts.
- **Measurement:** Load test at high fan-out; monitor goroutine count and completion.
- **Priority:** M

### NFR-012 — State store scaling
- **Target:** The default local state store **SHOULD** handle ≥ 10,000 retained runs with query latency < 100ms p95.
- **Rationale:** Run history usability.
- **Measurement:** Seed store; benchmark `runs list/show`.
- **Priority:** S

### NFR-013 — Log/trace volume
- **Target:** Observability export **MUST** apply backpressure/sampling so high-volume runs do not exhaust memory.
- **Rationale:** Stability under load.
- **Measurement:** Stress test with high log rate; verify bounded memory.
- **Priority:** S

---

## 4. Reliability

### NFR-014 — Deterministic execution
- **Target:** Given identical inputs/config and idempotent steps, runs **MUST** produce identical DAG ordering decisions and identical result structure.
- **Rationale:** Reproducibility.
- **Measurement:** Repeated runs of fixture flows compared for structural equivalence.
- **Priority:** M

### NFR-015 — Crash-free session rate
- **Target:** ≥ 99.95% of sessions **MUST** complete without an unrecovered panic.
- **Rationale:** Trust.
- **Measurement:** Opt-in telemetry / panic reporting aggregated per release.
- **Priority:** M

### NFR-016 — Flow success rate (non-user-error)
- **Target:** ≥ 99.9% of runs **MUST** succeed excluding user/logic errors.
- **Rationale:** Engine reliability.
- **Measurement:** Runtime telemetry classifying failure causes.
- **Priority:** S

### NFR-017 — Graceful degradation
- **Target:** Optional subsystems (TUI, tracing, plugins, LSP) **MUST** fail soft: their unavailability degrades features but never aborts a valid run.
- **Rationale:** Robustness.
- **Measurement:** Fault-injection tests disabling each subsystem.
- **Priority:** M

### NFR-018 — Data integrity of state
- **Target:** State/checkpoint writes **MUST** be atomic and crash-consistent (no partial/corrupt records after abrupt termination).
- **Rationale:** Safe resume (FR-052/053).
- **Measurement:** Kill-during-write fuzz test; verify store opens and resumes.
- **Priority:** S

### NFR-019 — Race-free concurrency
- **Target:** The codebase **MUST** be clean under the Go race detector in CI.
- **Rationale:** Correctness under parallelism (FR-048).
- **Measurement:** `-race` test suite runs on every PR.
- **Priority:** M

### NFR-020 — Error message quality
- **Target:** User-facing errors **MUST** state what failed, where (file/line/step), and a next action; no bare stack traces by default.
- **Rationale:** Usability and support cost.
- **Measurement:** Error-quality review checklist; snapshot tests of key errors.
- **Priority:** M

---

## 5. Availability

### NFR-021 — No mandatory external dependency at runtime
- **Target:** Core execution **MUST** succeed with no network/server dependency (single-binary principle).
- **Rationale:** Availability independent of infrastructure.
- **Measurement:** Run full fixture suite on a network-isolated host.
- **Priority:** M

### NFR-022 — Optional service availability targets
- **Target:** Where an optional hosted component (registry, remote state, agent API server) is deployed, it **SHOULD** target ≥ 99.9% availability.
- **Rationale:** Enterprise expectations for shared services.
- **Measurement:** SLO monitoring on the hosted component.
- **Priority:** C

### NFR-023 — Offline capability
- **Target:** Previously installed/verified plugins and cached registry metadata **MUST** be usable offline.
- **Rationale:** Air-gapped and flaky-network environments.
- **Measurement:** Offline test after warm cache.
- **Priority:** S

---

## 6. Portability

### NFR-024 — OS support
- **Target:** The binary **MUST** run on Windows 10+/11, macOS 13+, and mainstream Linux (glibc and musl).
- **Rationale:** Cross-team adoption.
- **Measurement:** CI matrix builds and smoke-tests each OS.
- **Priority:** M

### NFR-025 — Architecture support
- **Target:** The binary **MUST** ship for amd64 and arm64 on all supported OSes.
- **Rationale:** Apple Silicon, ARM CI/servers.
- **Measurement:** CI cross-compiles and (where possible) tests each arch.
- **Priority:** M

### NFR-026 — Behavioral parity
- **Target:** Command semantics, exit codes, and flow results **MUST** be identical across OS/arch (modulo documented OS-specific features like sandboxing granularity).
- **Rationale:** "Runs the same everywhere."
- **Measurement:** Shared conformance suite executed on every OS/arch in CI.
- **Priority:** M

### NFR-027 — Shell completion parity
- **Target:** Completion **MUST** work on bash, zsh, fish, and PowerShell (FR-068).
- **Rationale:** Native feel on Windows too.
- **Measurement:** Per-shell completion tests, including PowerShell on Windows.
- **Priority:** M

### NFR-028 — Path and line-ending handling
- **Target:** The system **MUST** handle Windows paths, drive letters, and CRLF/LF line endings in `*.flow`/config correctly.
- **Rationale:** Windows-first personas exist.
- **Measurement:** Cross-platform path/line-ending fixtures.
- **Priority:** M

### NFR-029 — No cgo by default
- **Target:** Default builds **SHOULD** be pure-Go (CGO_ENABLED=0) for static, portable binaries.
- **Rationale:** Simplify distribution; avoid glibc coupling.
- **Measurement:** Release build config asserts CGO disabled.
- **Priority:** S

---

## 7. Usability

### NFR-030 — Time-to-first-flow
- **Target:** A new user **MUST** be able to author and run their first flow in < 15 minutes using quickstart docs.
- **Rationale:** Adoption (KR-A1 in [PRD](02-prd.md)).
- **Measurement:** Moderated onboarding study, n ≥ 20.
- **Priority:** M

### NFR-031 — Discoverability
- **Target:** Every command/flag **MUST** be self-documenting via help, and every flow discoverable via `conduit describe`.
- **Rationale:** Reduce doc dependence.
- **Measurement:** Coverage check that all commands/flags have help text.
- **Priority:** M

### NFR-032 — Consistent CLI grammar
- **Target:** Commands **MUST** follow a consistent `noun verb`/`verb noun` convention and flag naming style documented in a style guide.
- **Rationale:** Predictability.
- **Measurement:** Lint rule enforcing naming conventions.
- **Priority:** S

### NFR-033 — Actionable diagnostics
- **Target:** DSL/config diagnostics **MUST** include precise ranges and, where feasible, suggested fixes.
- **Rationale:** Authoring speed.
- **Measurement:** Snapshot tests on diagnostic output quality.
- **Priority:** S

### NFR-034 — Progressive disclosure
- **Target:** Default output **SHOULD** be concise; detail available via `-v`; full machine detail via `--output json`.
- **Rationale:** Serve novices and experts.
- **Measurement:** UX review of default vs. verbose output.
- **Priority:** S

---

## 8. Maintainability

### NFR-035 — Test coverage
- **Target:** Core packages (parser, type-checker, engine, runtime, state) **MUST** maintain ≥ 80% line coverage; overall ≥ 70%.
- **Rationale:** Change safety.
- **Measurement:** Coverage gate in CI.
- **Priority:** M

### NFR-036 — Modular architecture
- **Target:** The codebase **MUST** enforce clear module boundaries (no import cycles; UI/agent surfaces depend on core, not vice versa).
- **Rationale:** Long-term evolvability.
- **Measurement:** Dependency-graph lint; `go vet`/architecture tests.
- **Priority:** M

### NFR-037 — API stability & SemVer
- **Target:** Public Go API, CLI surface, FlowDSL, plugin protocol, and config schema **MUST** follow SemVer 2.0.0 with documented compatibility guarantees.
- **Rationale:** Ecosystem trust (differentiator; PRD principle 10).
- **Measurement:** API-diff tooling gates breaking changes to major versions.
- **Priority:** M

### NFR-038 — Documentation completeness
- **Target:** Every FR-visible feature **MUST** have reference docs and at least one example by GA.
- **Rationale:** Adoptability.
- **Measurement:** Docs coverage checklist mapped to FR IDs.
- **Priority:** S

### NFR-039 — Reproducible builds
- **Target:** Release builds **MUST** be byte-for-byte reproducible from source given the pinned toolchain.
- **Rationale:** Supply-chain trust (see [Security Requirements](05-security-requirements.md)).
- **Measurement:** Two independent builds compared for equality in CI.
- **Priority:** M

### NFR-040 — Linting & static analysis
- **Target:** CI **MUST** run `go vet`, staticcheck-class linters, and gosec with zero high-severity findings.
- **Rationale:** Quality and security hygiene.
- **Measurement:** Lint/sec gates on every PR.
- **Priority:** M

---

## 9. Internationalization (i18n) & Localization

### NFR-041 — UTF-8 correctness
- **Target:** All input/output (flows, config, logs, TUI) **MUST** be UTF-8 clean and handle multibyte content without corruption.
- **Rationale:** Global usage.
- **Measurement:** Multibyte fixtures in tests.
- **Priority:** M

### NFR-042 — Localizable messages
- **Target:** User-facing strings **SHOULD** be externalized to enable localization; default locale is English.
- **Rationale:** Future localization.
- **Measurement:** Audit that strings are catalog-referenced, not hard-coded, in user-facing paths.
- **Priority:** C

### NFR-043 — Locale-aware formatting
- **Target:** Where dates/numbers are shown to humans, formatting **SHOULD** respect locale; machine output **MUST** use stable, locale-independent formats (RFC 3339, canonical numbers).
- **Rationale:** Correct scripting and human readability.
- **Measurement:** Tests asserting machine output is locale-independent.
- **Priority:** S

---

## 10. Accessibility

### NFR-044 — Color independence
- **Target:** No information **MUST** be conveyed by color alone; `--no-color` and `NO_COLOR` **MUST** be honored (FR-122).
- **Rationale:** Color-blind and non-color terminals.
- **Measurement:** Snapshot tests with color disabled; contrast/symbol checks.
- **Priority:** M

### NFR-045 — Screen-reader friendliness
- **Target:** TUI and CLI output **SHOULD** remain intelligible when read linearly by assistive tech.
- **Rationale:** Inclusive tooling.
- **Measurement:** Manual assistive-tech review of key flows.
- **Priority:** S

### NFR-046 — Keyboard-only operation
- **Target:** The TUI **MUST** be fully operable via keyboard without a mouse.
- **Rationale:** Terminal norms and accessibility.
- **Measurement:** Keyboard-only test pass of all TUI actions.
- **Priority:** S

### NFR-047 — Configurable output width/verbosity
- **Target:** Output **SHOULD** adapt to terminal width and honor `--quiet` for minimal, parse-friendly output.
- **Rationale:** Small terminals and pipelines.
- **Measurement:** Rendering tests at varied widths.
- **Priority:** S

---

## 11. Resource Limits & Efficiency

### NFR-048 — Bounded memory under load
- **Target:** Peak memory **MUST** remain bounded and configurable under high fan-out; the engine **MUST NOT** buffer unbounded step output in memory (stream to disk/sink beyond a threshold).
- **Rationale:** Stability on constrained hosts.
- **Measurement:** High-fan-out stress test with memory ceiling assertions.
- **Priority:** M

### NFR-049 — File descriptor & goroutine hygiene
- **Target:** The runtime **MUST** close descriptors and reap goroutines/child processes; no leaks across a run.
- **Rationale:** Long-running/host stability.
- **Measurement:** Leak detectors (goroutine snapshot diff, fd count) in tests.
- **Priority:** M

### NFR-050 — Configurable global limits
- **Target:** Operators **MUST** be able to cap concurrency, per-step CPU/memory/time, and total run time via `conduit.yaml`.
- **Rationale:** Safe multi-tenant/shared hosts.
- **Measurement:** Config-driven limit tests (ties to FR-043, FR-050).
- **Priority:** S

### NFR-051 — CPU efficiency at idle/wait
- **Target:** While waiting on step I/O or sleeps, the engine **MUST NOT** busy-wait; idle CPU **MUST** approach ~0%.
- **Rationale:** Efficiency.
- **Measurement:** CPU sampling during an I/O-bound flow.
- **Priority:** S

---

## 12. Compatibility & Reference Environment

### NFR-052 — Go toolchain compatibility
- **Target:** The project **MUST** build with Go 1.24+ and track the two most recent Go minor releases.
- **Rationale:** Language features and security fixes.
- **Measurement:** CI matrix across supported Go versions.
- **Priority:** M

### NFR-053 — FlowDSL backward compatibility
- **Target:** Within a major version, newer Conduit **MUST** run all `*.flow` valid in prior minor versions (additive-only DSL changes).
- **Rationale:** Protect authored flows.
- **Measurement:** Compatibility corpus of prior-version flows run on new releases.
- **Priority:** M

### NFR-054 — Config schema compatibility
- **Target:** Config schema changes **MUST** be backward compatible within a major version; deprecations warn before removal.
- **Rationale:** Stable operations.
- **Measurement:** Prior-version `conduit.yaml` corpus validated on new releases.
- **Priority:** M

### NFR-055 — Plugin protocol compatibility
- **Target:** The plugin gRPC protocol **MUST** be versioned; hosts **MUST** support the current and immediately-prior protocol minor version.
- **Rationale:** Ecosystem stability (FR-060).
- **Measurement:** Cross-version host/plugin compatibility tests.
- **Priority:** S

### NFR-056 — Reference environment (for measurements)
- **Definition:** Unless stated otherwise, targets are measured on: 4 vCPU / 8GB RAM CI runners, SSD-backed, on Linux amd64, macOS arm64, and Windows amd64. Percentiles use N ≥ 200 samples with warm and cold variants reported separately.
- **Rationale:** Comparable, repeatable measurement.
- **Priority:** M

---

## 13. NFR ↔ Requirement Traceability

| NFR area | NFR range | Related FRs | Notes |
|----------|-----------|-------------|-------|
| Performance | 001–009 | FR-070, FR-105–107 | Startup/completion/CEL budgets |
| Scalability | 010–013 | FR-031, FR-051–055 | Fan-out & state |
| Reliability | 014–020 | FR-048, FR-052/053 | Determinism, resume, races |
| Availability | 021–023 | FR-041, FR-064 | Single-binary, offline |
| Portability | 024–029 | FR-068 | OS/arch/shell parity |
| Usability | 030–034 | FR-010, FR-025, FR-033-visible | Onboarding & help |
| Maintainability | 035–040 | FR-014, FR-057 | SemVer, reproducible builds |
| i18n | 041–043 | — | UTF-8 & formats |
| Accessibility | 044–047 | FR-122 | Color/keyboard/screen-reader |
| Resource limits | 048–051 | FR-043, FR-050, FR-066 | Bounded resources |
| Compatibility | 052–056 | FR-060 | Toolchain/DSL/config/plugin |

Security-oriented non-functional concerns (sandbox limits, audit integrity) are specified in [Security Requirements](05-security-requirements.md).
