# Conduit — Performance & Scalability Strategy

> Document ID: `73-performance-scalability`
> Status: Draft (v0.1.0)
> Owner: Platform Architecture — Reliability & Quality
> Last updated: 2026-07-02

Related documents:
- [Non-Functional Requirements](04-non-functional-requirements.md) — the budgets this doc realizes
- [Executive Summary §9](00-executive-summary.md) — headline perf targets
- [Testing Strategy §10](72-testing-strategy.md) — benchmark regression gate in CI
- [Completion Engine](50-completion-engine.md) — completion latency path
- [Parser Design](22-parser-design.md) · [AST Design](23-ast-design.md) · [Expression Engine (CEL)](25-expression-engine.md) — parse/eval caching
- [Workflow DAG](30-workflow-dag.md) · [Execution Runtime](31-execution-runtime.md) — large DAGs, concurrency
- [Internal Message Bus](68-message-bus.md) — streaming, backpressure
- [Recovery Strategy](71-recovery-strategy.md) — server-mode auto-resume

---

## 1. Targets at a Glance

| Metric | Target | How measured |
|---|---|---|
| Cold start (`conduit --version`) | **p50 < 50ms**, p99 < 90ms | benchmark harness, CI matrix |
| Command dispatch (no plugin) | p95 < 15ms | `go test -bench` |
| Completion latency | p95 < 100ms | instrumented completion path |
| Parse throughput (typical `.flow`) | > 5 MB/s single-core | `BenchmarkParse` |
| Plan a 1k-node DAG | < 50ms | `BenchmarkPlan` |
| Plan a 10k-node DAG | < 500ms | `BenchmarkPlanLarge` |
| Steady-state RSS (idle CLI) | < 30 MB | `pprof` heap / RSS sampling |
| Log streaming overhead | < 5% wall-time vs no-capture | throughput bench |

These are the operationalization of the [NFRs](04-non-functional-requirements.md) and the headline
numbers in [00 — Executive Summary](00-executive-summary.md).

---

## 2. Startup Latency Budget (<50ms cold)

Cold start is a *budget*, allocated across phases and enforced in CI. Anything that would blow the
budget must be made lazy.

| Phase | Budget | Technique |
|---|---|---|
| Process + Go runtime init | ~8ms | static binary; minimal `init()` funcs |
| Cobra command tree build | ~6ms | build tree lazily; no plugin discovery here |
| Config load (`conduit.yaml`) | ~10ms | Koanf, single stat+read; skip if not present |
| Plugin discovery | **0ms at startup** | deferred — see §3 |
| Completion metadata | ~4ms | precomputed cache (§8) |
| First command reach | ~10ms | remaining dispatch |

Rules that protect the budget:
- **No `init()`-time I/O or plugin loading.** `init()` functions are audited; heavy work is deferred.
- **No eager plugin handshakes.** A plugin subprocess is only spawned when a command/task actually
  needs it (§3).
- **Version/help fast-path.** `--version`/`-h` short-circuit before any config/plugin work.

```go
// registerPlugins installs lazy command stubs; the real subprocess spawns on first use.
func registerPlugins(root *cobra.Command, mf []Manifest) {
	for _, m := range mf {
		m := m
		stub := &cobra.Command{
			Use:   m.Command,
			Short: m.Summary, // from manifest, no process spawn
			RunE: func(c *cobra.Command, args []string) error {
				client, err := pluginMgr.Ensure(c.Context(), m.Ref) // spawn NOW
				if err != nil { return err }
				return client.Run(c.Context(), args)
			},
		}
		root.AddCommand(stub)
	}
}
```

---

## 3. Lazy Loading of Plugins & Commands

Plugins are discovered from lightweight **manifests** (name, commands, summary, version) — no process
spawn — at startup. The gRPC subprocess ([go-plugin](40-plugin-architecture.md)) is created only when
a command or task first invokes the plugin, then cached and supervised per
[71 — Recovery Strategy §9](71-recovery-strategy.md). Command help/metadata for completion comes from
the manifest, so `conduit <tab>` never spawns anything.

---

## 4. Completion Latency (<100ms p95)

Shell/editor completion is on the human-perceived hot path. It is served from a **precomputed
completion index** (commands, flags, static enums) plus **bounded dynamic providers** with a hard
deadline; a provider that overruns is dropped rather than blocking the prompt.

```go
// Complete returns candidates within a strict budget; slow dynamic providers
// are abandoned so the shell never stalls.
func Complete(ctx context.Context, line string) []Candidate {
	ctx, cancel := context.WithTimeout(ctx, 80*time.Millisecond)
	defer cancel()
	out := index.Static(line) // instant, precomputed (§8)
	if dyn, ok := dynamicFor(line); ok {
		select {
		case c := <-runProvider(ctx, dyn):
			out = append(out, c...)
		case <-ctx.Done(): // over budget → return static-only
		}
	}
	return out
}
```

---

## 5. Parse / Plan Throughput

- **Parser** ([22](22-parser-design.md)) targets >5 MB/s: single-pass lexer, reused token buffers,
  no per-token allocation (§7).
- **Planner** ([30 — Workflow DAG](30-workflow-dag.md)) toposort + readiness computation is
  `O(V+E)`; for 10k nodes it stays <500ms by using integer node IDs and slice-backed adjacency
  (no map-of-maps), and by planning once and caching the immutable plan.

```go
func BenchmarkPlan(b *testing.B) {
	g := buildDAG(1000) // 1k nodes
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := dag.Plan(g); err != nil { b.Fatal(err) }
	}
}
```

---

## 6. Memory Footprint

- **Idle CLI RSS < 30 MB.** Achieved by lazy plugins (§3), streaming logs (§13), and avoiding whole-
  file/whole-run buffering.
- **Bounded buffers everywhere.** Message-bus subscriptions ([68 §4](68-message-bus.md)) and log
  capture use fixed-size ring/overflow buffers, so memory does not grow with run duration.
- **Streaming over accumulation.** Large artifacts/logs are streamed to sinks, never fully resident.

---

## 7. Benchmarking & Profiling

### 7.1 `go test -bench` + `benchstat`

Every perf-critical package ships benchmarks. CI runs them against a committed baseline and fails on
regression >10% (wall-time or allocs).

```bash
go test -run=^$ -bench=. -benchmem -count=10 ./... > new.txt
benchstat baseline.txt new.txt   # fails CI if delta exceeds threshold
```

```go
func BenchmarkColdStart(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		cmd := newRootCommand()          // build tree
		cmd.SetArgs([]string{"--version"})
		_ = cmd.Execute()                // fast-path, no plugins
	}
}
```

### 7.2 Continuous benchmarking in CI

Benchmarks run on a dedicated, pinned runner (perf-stable) on every push to `main`; results are
stored and trended so regressions are caught as a *trend*, not just a single-PR delta. See
[72 — Testing Strategy §10](72-testing-strategy.md) for the gate wiring.

### 7.3 pprof & flamegraphs

CPU/heap/block/mutex profiles are collected in benchmarks (`-cpuprofile`, `-memprofile`) and, in
server mode, exposed on a guarded `/debug/pprof` endpoint. Flamegraphs (via `go tool pprof -http`)
are the primary tool for allocation and hot-path analysis.

---

## 8. Caching

| Cache | Key | Invalidation | Payload |
|---|---|---|---|
| Parsed AST | content hash of `.flow` | file mtime/hash change | immutable AST ([23](23-ast-design.md)) |
| Compiled CEL | expression string + env shape | env/schema change | compiled `cel.Program` ([25](25-expression-engine.md)) |
| Completion index | binary version + manifests | new plugin / upgrade | static candidates (§4) |
| DAG plan | flow hash + inputs shape | flow/inputs change | immutable plan ([30](30-workflow-dag.md)) |

Caches are content-addressed and stored under `~/.cache/conduit/`. CEL programs are compiled once and
reused across evaluations, which is the single biggest expression-path win.

```go
// celCache memoizes compiled programs so repeated evaluation skips compilation.
type celCache struct {
	mu sync.RWMutex
	m  map[string]cel.Program // key: exprHash + envHash
}

func (c *celCache) Program(expr string, env *cel.Env) (cel.Program, error) {
	key := hash(expr, env.Signature())
	c.mu.RLock(); p, ok := c.m[key]; c.mu.RUnlock()
	if ok { return p, nil }
	ast, iss := env.Compile(expr)
	if iss.Err() != nil { return nil, iss.Err() }
	p, err := env.Program(ast)
	if err != nil { return nil, err }
	c.mu.Lock(); c.m[key] = p; c.mu.Unlock()
	return p, nil
}
```

---

## 9. Allocation Reduction

- **`sync.Pool`** for hot, short-lived buffers (lexer token slices, JSON encode buffers, event
  envelopes on the publish path).
- **Pre-sized slices/maps** where the count is known (DAG adjacency, task result maps).
- **Avoid `interface{}` boxing** on hot paths; use concrete types and code-gen where it matters.
- **`-benchmem` gate**: allocation counts are part of the regression gate (§7.1), so alloc creep is
  caught mechanically.

---

## 10. Scalability: Large DAGs (10k+ nodes)

```mermaid
flowchart LR
  PLAN[Plan once<br/>O(V+E), int node IDs] --> SCHED[Scheduler<br/>ready-set frontier]
  SCHED --> POOL[Worker pool<br/>bounded, tuned]
  POOL --> RUN[Task execution]
  RUN -->|settle| SCHED
  RUN -->|events| BUS[(Message Bus<br/>bounded buffers)]
```

- **Integer-ID graph**: nodes are dense integer IDs; adjacency is `[][]int`, so a 10k-node graph is
  cache-friendly and allocation-light.
- **Frontier scheduling**: the scheduler tracks a ready-set and decrements in-degree counters as
  tasks settle — no repeated full-graph scans.
- **Streamed planning**: plan is computed once, immutable, and shared; no per-task re-planning.

---

## 11. High-Concurrency Task Execution & Worker Pool Tuning

Task execution uses a **bounded worker pool** sized from CPU count and I/O-vs-CPU profile, with
per-run concurrency caps to prevent one run from starving others.

```go
// Pool bounds concurrent task execution. Size defaults to GOMAXPROCS for
// CPU-bound flows and is overridable per flow (I/O-bound flows set it higher).
type Pool struct {
	sem chan struct{} // capacity = maxConcurrency
}

func NewPool(max int) *Pool { return &Pool{sem: make(chan struct{}, max)} }

func (p *Pool) Go(ctx context.Context, fn func()) error {
	select {
	case p.sem <- struct{}{}:
		go func() { defer func() { <-p.sem }(); fn() }()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
```

Tuning guidance: CPU-bound → `maxConcurrency = GOMAXPROCS`; I/O-bound (network/plugin) → higher,
bounded by downstream limits and the retry budget ([71 §2.2](71-recovery-strategy.md)) to avoid
overwhelming dependencies.

---

## 12. Streaming for Large Logs / Artifacts

Logs and artifacts are **streamed**, never fully buffered. `step.log` events
([67 — Event Model](67-event-model.md)) flow through the bus with bounded buffers; large artifacts
are piped through `io.Reader`/`io.Writer` with a fixed copy buffer to durable storage.

```go
// streamLogs forwards a step's stdout as bounded step.log events without buffering
// the whole stream in memory.
func streamLogs(ctx context.Context, pub bus.Publisher, r io.Reader, meta LogMeta) error {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 64<<10), 1<<20) // cap line size
	for s.Scan() {
		_ = pub.Publish(ctx, meta.logEvent(s.Text())) // best-effort/backpressured (68 §4)
	}
	return s.Err()
}
```

---

## 13. Server-Mode Horizontal Scaling

Conduit's default is single-binary/single-node, but an optional **server mode** scales horizontally
for fleet use:

- **Stateless API front-ends** behind a load balancer; run state lives in a shared, pluggable state
  store ([32 — State Management](32-state-management.md)).
- **Run affinity / leasing**: a run is leased to one worker via the state store; on worker loss, the
  lease expires and another worker **auto-resumes from the last checkpoint**
  ([71 §5, §11](71-recovery-strategy.md)).
- **Bus bridging**: cross-node event fan-out uses the NATS/Kafka bridge
  ([68 §5.3](68-message-bus.md)), keeping each node's core bus in-process.

---

## 14. Resource Governance / Quotas

- **Per-run limits**: max concurrency, max total tasks, wall-clock deadline
  ([71 §4](71-recovery-strategy.md)), max memory hint.
- **CEL cost limits**: expression evaluation is cost-bounded (sandbox) to prevent DoS via crafted
  expressions ([25](25-expression-engine.md), tested in [72 §5.4](72-testing-strategy.md)).
- **Plugin resource caps**: subprocess CPU/mem limits and RPC timeouts.
- **Quotas in server mode**: per-tenant run/concurrency quotas enforced at the API front-end.

---

## 15. Load Testing

- **Synthetic large flows**: generate 10k-node DAGs and measure plan+execute latency and memory.
- **Sustained-throughput soak**: drive N concurrent runs for hours; watch RSS (no leak/growth) and
  bus queue depth ([68 §4](68-message-bus.md)).
- **Completion storm**: hammer the completion path to validate the p95 budget under load.
- Load tests run **nightly** (non-blocking-but-tracked) per [72 §10](72-testing-strategy.md), with
  results trended alongside continuous benchmarks (§7.2).

```go
func BenchmarkExecute_10kDAG(b *testing.B) {
	g := buildDAG(10_000)
	b.ReportAllocs(); b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := runtime.Execute(context.Background(), g, noopExec); err != nil { b.Fatal(err) }
	}
}
```

---

## 16. Cross-References

- Budgets & NFR provenance: [04 — Non-Functional Requirements](04-non-functional-requirements.md), [00 — Executive Summary §9](00-executive-summary.md).
- Benchmark regression gate & load-test scheduling: [72 — Testing Strategy §10](72-testing-strategy.md).
- Caching sources: [22 — Parser](22-parser-design.md), [23 — AST](23-ast-design.md), [25 — CEL](25-expression-engine.md), [50 — Completion](50-completion-engine.md).
- Large-DAG scheduling & runtime: [30 — Workflow DAG](30-workflow-dag.md), [31 — Execution Runtime](31-execution-runtime.md).
- Streaming/backpressure: [68 — Message Bus](68-message-bus.md); server-mode resume: [71 — Recovery Strategy](71-recovery-strategy.md).
