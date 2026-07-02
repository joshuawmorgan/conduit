# 91 — Sample FlowDSL Files

> **Codename:** Conduit · **Binary:** `conduit` (alias `cdt`) · **Module:** `github.com/conduit-io/conduit`
> **DSL:** FlowDSL (`*.flow`) · **Expressions:** CEL-Go · **Go:** 1.24+
> **Status:** Samples v1.0 · **Owner:** Developer Experience · **Date:** 2026-07-02

This document is a **teaching gallery** of complete, runnable `.flow` programs of increasing complexity.
Every construct in the [FlowDSL Grammar](20-dsl-grammar.md) is exercised at least once. Each sample is a
self-contained file, richly commented, followed by a short **grammar-in-practice** note that maps the sample
back to the normative productions in [20 — DSL Grammar](20-dsl-grammar.md).

**Related documents**
- [20 — DSL Grammar Specification](20-dsl-grammar.md) — the normative EBNF these files conform to.
- [25 — Expression Engine](25-expression-engine.md) — the CEL environment (`params`, `inputs`, `matrix`, `secrets`, `ctx`, prior `output`s).
- [31 — Execution Runtime](31-execution-runtime.md) — how `retry`/`timeout`/`on_error` and the DAG are executed.
- [40 — Plugin Architecture](40-plugin-architecture.md) — `uses:` action resolution over go-plugin/gRPC.
- [90 — Sample Workflows](90-sample-workflows.md) — end-to-end scenarios with CLI commands & output.
- [92 — Sample Plugins](92-sample-plugins.md) — the plugins referenced by `uses:` here.

> **Conventions used in every sample**
> - Files pin the grammar with `flow "1.0"`.
> - Comments use `#`.
> - Plugin actions are referenced as `uses: <plugin>/<action>@<version>` (canonical short form); the fully
>   qualified `plugin://<plugin>/<action>@<version>` URI form is equivalent and shown once for contrast.
> - Interpolation is `${{ cel.expr }}`; `when:` and `for_each:` take **bare** CEL.

---

## Index

| # | File | Showcases |
|---|---|---|
| a | [`hello.flow`](#a-hello-world--helloflow) | minimal `workflow`/`task`/`step`/`run` |
| b | [`build-test.flow`](#b-sequential-buildtest--build-testflow) | `depends_on`, `output`, sequential ordering |
| c | [`fanout.flow`](#c-parallel-fan-outfan-in--fanoutflow) | parallel tasks, fan-in via `depends_on` list |
| d | [`matrix-build.flow`](#d-matrix-build-across-osarch--matrix-buildflow) | `matrix`, `env`, interpolation |
| e | [`conditional-deploy.flow`](#e-conditional-deploy-with-cel-when--conditional-deployflow) | `when:` CEL guards, `param`/`input` |
| f | [`resilient.flow`](#f-retries--timeouts--on_error-compensation--resilientflow) | `retry`, `timeout`, `on_error`, `on_success` |
| g | [`with-template.flow`](#g-reusable-template--import--params--with-templateflow) | `template`, `import`, `param` signatures |
| h | [`cloud-deploy.flow`](#h-secrets--plugin-uses-cloud-deploy--cloud-deployflow) | `secret`, `ref(...)`, `uses:` cloud action |
| i | [`dynamic-foreach.flow`](#i-dynamic-for_each-over-a-plugin-provided-list--dynamic-foreachflow) | `for_each` over plugin output, `as value, index` |

---

## (a) Hello world — `hello.flow`

The smallest useful program. One workflow, one task, one step, one shell `run`.

```flow
flow "1.0"

# A workflow is the top-level unit. The identifier `hello` names the run.
workflow hello {
  description = "The smallest useful FlowDSL program."

  # A task is a schedulable node in the DAG. With no `depends_on`, it is a root.
  task greet {
    # A step is the smallest executable unit. An anonymous step is allowed.
    step {
      # `run` executes a shell command using the default shell (see conduit.yaml).
      run "echo 'Hello, Conduit'"
    }
  }
}
```

Run it:

```console
$ conduit run hello.flow
✔ hello · greet · step#1  (0.01s)
Hello, Conduit
Run r-1a2b succeeded in 0.02s
```

**Grammar-in-practice.** Exercises `WorkflowDecl → TaskDecl → StepDecl → RunDecl` (§6, §7.4) plus the
`MetaAssign` `description = …` (§7.7). The anonymous `step {}` uses the optional-`IDENT` form of `StepDecl`.

---

## (b) Sequential build/test — `build-test.flow`

Two tasks in a strict order. `test` waits for `build` via `depends_on`, and consumes an `output` the build
task published. Outputs are the canonical way to pass typed data between tasks.

```flow
flow "1.0"

workflow build_test {
  description = "Compile, then test the produced binary — strictly sequential."

  task build {
    step compile {
      run "go build -o bin/app ./cmd/app"
    }
    # Outputs are typed and become available to downstream tasks as
    # `tasks.build.outputs.artifact` inside CEL.
    output artifact: string = "bin/app"
  }

  task test {
    # `depends_on` creates the DAG edge build → test; test cannot start until
    # build reaches a terminal success state.
    depends_on build

    step unit {
      # Interpolate the upstream output into the command string.
      run "go test ./... -run Unit -args -bin=${{ tasks.build.outputs.artifact }}"
    }
    step integration {
      # Steps within a task run in declaration order by default.
      run "go test ./... -run Integration"
    }
  }
}
```

```console
$ conduit run build-test.flow
✔ build · compile              (3.10s)
✔ test  · unit                 (1.44s)
✔ test  · integration          (5.02s)
Run r-77aa succeeded in 9.58s
```

**Grammar-in-practice.** `DependsOnDecl` (§7.3) with a single `IDENT`; `OutputDecl` (§7.1) with an explicit
`TypeRef` (`string`); cross-task reference via CEL `tasks.<id>.outputs.<name>` (see [25](25-expression-engine.md)).
Steps run in source order — there is no imperative control flow, only declaration order and the DAG.

---

## (c) Parallel fan-out/fan-in — `fanout.flow`

Three independent tasks fan **out** from a common root and fan **in** to a gate task. Tasks with no ordering
dependency between them are scheduled concurrently by the runtime (bounded by the worker pool, see
[31 — Execution Runtime](31-execution-runtime.md)).

```flow
flow "1.0"

workflow fanout {
  description = "Fan out three linters/scanners in parallel, then gate on all of them."

  task checkout {
    step { run "git fetch --depth=1 && git checkout ${{ ctx.commit }}" }
  }

  # --- fan-out: these three all depend only on `checkout`, so they run concurrently ---
  task lint {
    depends_on checkout
    step { run "golangci-lint run ./..." }
  }
  task vet {
    depends_on checkout
    step { run "go vet ./..." }
  }
  task scan {
    depends_on checkout
    step { uses: security/scan@v2 with { path = ".", fail_on = "high" } }
  }

  # --- fan-in: `gate` waits for the whole set via a depends_on list ---
  task gate {
    depends_on (lint, vet, scan)   # IdentList form: all must succeed
    step { run "echo 'all checks green'" }
  }
}
```

```console
$ conduit run fanout.flow --var commit=HEAD
✔ checkout                     (0.40s)
⣾ lint    ⣾ vet    ⣾ scan      (running in parallel)
✔ lint                          (12.1s)
✔ vet                           (2.30s)
✔ scan                          (18.7s)
✔ gate                          (0.01s)
Run r-9c3d succeeded in 19.3s
```

**Grammar-in-practice.** The fan-in uses `DependsOnDecl` with the **`IdentList`** form `(lint, vet, scan)`
(§7.3). Concurrency is *implicit*: the grammar has no `parallel` keyword — parallelism is a property of the DAG
derived from the absence of edges. Mixing `run` and `uses:` steps in one workflow is shown here.

---

## (d) Matrix build across os/arch — `matrix-build.flow`

`matrix` expands a task into the Cartesian product of its axes. Each cell gets its own `matrix.<axis>` bindings
inside CEL, its own env, and runs as an independent DAG node.

```flow
flow "1.0"

workflow matrix_build {
  description = "Cross-compile across the os × arch product; upload each artifact."

  task cross_compile {
    # 3 os × 2 arch = 6 cells. Cells run in parallel up to the pool limit.
    matrix {
      os   = ["linux", "darwin", "windows"]
      arch = ["amd64", "arm64"]
    }

    # Per-cell environment: values are interpolated from the active matrix bindings.
    env {
      GOOS   = "${{ matrix.os }}"
      GOARCH = "${{ matrix.arch }}"
      EXT    = "${{ matrix.os == 'windows' ? '.exe' : '' }}"
    }

    step build {
      run "go build -o dist/app-${{ matrix.os }}-${{ matrix.arch }}${{ env.EXT }} ./cmd/app"
    }
    step upload {
      uses: oci/push@v2 with {
        artifact = "dist/app-${{ matrix.os }}-${{ matrix.arch }}${{ env.EXT }}"
        tag      = "app:${{ ctx.version }}-${{ matrix.os }}-${{ matrix.arch }}"
      }
    }
  }
}
```

```console
$ conduit run matrix-build.flow --var version=1.4.0
✔ cross_compile [os=linux   arch=amd64]  (2.9s)
✔ cross_compile [os=linux   arch=arm64]  (3.1s)
✔ cross_compile [os=darwin  arch=amd64]  (3.0s)
✔ cross_compile [os=darwin  arch=arm64]  (3.2s)
✔ cross_compile [os=windows arch=amd64]  (3.4s)
✔ cross_compile [os=windows arch=arm64]  (3.6s)
Run r-b2e1 succeeded in 6.8s   (6 cells, max 4 concurrent)
```

**Grammar-in-practice.** `MatrixDecl` with two `MatrixAxis` bindings (§7.3). The runtime materializes one DAG
node per product cell; `matrix.os`/`matrix.arch` are exposed in the CEL environment for that cell only. The
ternary in `EXT` is CEL (§11 precedence table). `EnvDecl` in block form (§7.2).

---

## (e) Conditional deploy with CEL `when` — `conditional-deploy.flow`

`when:` is a **bare CEL guard** that must type to `bool`. A `false` guard marks the node *skipped* (not
failed), and downstream `depends_on` treats a skip as satisfied-but-not-run.

```flow
flow "1.0"

# `param` = author/operator-tunable input with a default & validation block.
param environment: string = "dev" {
  description = "Target environment."
  enum        = ["dev", "staging", "prod"]
}

# `input` = required-at-invocation value (no default → must be supplied).
input version: string {
  description = "Semver tag to deploy."
  required    = true
}

workflow conditional_deploy {
  env {
    IS_PROD = environment == "prod"
  }

  task build {
    step { run "go build -o bin/app ./cmd/app" }
  }

  task deploy_staging {
    depends_on build
    # Guard: only run this node when targeting staging.
    when environment == "staging"
    step { uses: k8s/apply@v3 with { manifest = "deploy/staging.yaml", image = "app:${{ inputs.version }}" } }
  }

  task deploy_prod {
    depends_on build
    # Compound guard: prod only, and never on a pre-release tag.
    when environment == "prod" && !inputs.version.contains("-rc")
    step approve {
      # `uses:` an interactive approval gate action; blocks until approved.
      uses: gate/approval@v1 with { approvers = ["sre", "release-mgr"], timeout = "2h" }
    }
    step rollout {
      depends_on approve
      uses: k8s/apply@v3 with { manifest = "deploy/prod.yaml", image = "app:${{ inputs.version }}" }
    }
  }
}
```

```console
$ conduit run conditional-deploy.flow --var environment=staging --input version=1.4.0
✔ build                                  (3.0s)
✔ deploy_staging                         (8.2s)
⊘ deploy_prod              skipped (when=false)
Run r-51fa succeeded in 11.3s
```

**Grammar-in-practice.** `ParamDecl` with an `enum` validation block and `InputDecl` with `required = true`
(§7.1). Two `WhenDecl` guards (§7.3): a simple equality and a compound `&&` expression calling the CEL
`string.contains` member. Note the `depends_on approve` **inside a task** to order sibling steps.

---

## (f) Retries + timeouts + `on_error` compensation — `resilient.flow`

Reliability primitives compose: a `retry` block re-runs a node on failure with backoff; `timeout` bounds
wall-clock; `on_error` runs a compensation sub-flow; `on_success` runs only on the happy path.

```flow
flow "1.0"

input db_url: string { required = true }

workflow resilient_migration {
  description = "Run a DB migration with retries, a hard timeout, and rollback compensation."

  task migrate {
    # Whole-task deadline. Exceeding it cancels the task context (and any plugin RPC).
    timeout 5m

    # Retry with exponential backoff, but only for transient errors (CEL predicate on the error).
    retry {
      max     = 4
      delay   = 2s
      backoff = "exponential"          # 2s, 4s, 8s, 16s (jittered)
      when    = error.transient == true
    }

    step apply {
      uses: db/migrate@v1 with { url = "${{ inputs.db_url }}", dir = "./migrations" }
    }

    # Compensation: runs if the task ultimately fails (after retries exhausted).
    on_error {
      step rollback {
        uses: db/migrate@v1 with { url = "${{ inputs.db_url }}", direction = "down", steps = 1 }
      }
      step page {
        uses: pagerduty/alert@v1 with { severity = "critical", summary = "migration failed & rolled back" }
      }
    }

    # Runs only when `apply` (and retries) succeed.
    on_success {
      step announce {
        uses: slack/post@v1 with { channel = "#ops", text = "migration to ${{ inputs.db_url }} OK" }
      }
    }
  }
}
```

```console
$ conduit run resilient.flow --input db_url=postgres://... 
⚠ migrate · apply   attempt 1/4 failed: connection reset  (retry in 2s, transient=true)
✔ migrate · apply   attempt 2/4 ok            (7.7s)
✔ migrate · announce (on_success)             (0.30s)
Run r-c0de succeeded in 11.9s
```

**Grammar-in-practice.** `TimeoutDecl` with a `DURATION` literal `5m`; `RetryDecl` in **block form** with
`max`/`delay`/`backoff` and a `when` predicate over the `error` CEL object (§7.5). `OnErrorDecl` and
`OnSuccessDecl` carry nested step blocks (§7.5). The `error.transient` binding is documented in
[70 — Error Handling](70-error-handling.md).

---

## (g) Reusable `template` + `import` + params — `with-template.flow`

A `template` is a parameterized bundle of declarations expanded via `uses: template …`. Libraries live in
their own `.flow` files (no top-level `workflow`) and are pulled in with `import`.

`lib/checks.flow` (a library file — declarations only, no workflow):

```flow
flow "1.0"

# A template takes a typed parameter signature and expands to step/task declarations.
template smoke_test(url: string, timeout: duration = 30s) {
  step check {
    timeout timeout
    run <<-SH
      set -euo pipefail
      curl --fail --max-time ${{ int(timeout / 1s) }} "${{ url }}/healthz"
    SH
  }
}

template retry_curl(url: string, attempts: int = 3) {
  step hit {
    retry { max = attempts, delay = 1s, backoff = "linear" }
    run "curl --fail '${{ url }}'"
  }
}
```

`with-template.flow` (the workflow that imports & instantiates them):

```flow
flow "1.0"

# Bind the library under the `checks` namespace.
import "./lib/checks.flow" as checks

param base_url: string = "https://staging.example.com"

workflow verify_release {
  description = "Instantiate shared templates to smoke-test a deployment."

  task health {
    # Expand a template with bound params. `with { … }` supplies the signature args.
    uses template checks.smoke_test with { url = "${{ params.base_url }}", timeout = 45s }
  }

  task probe {
    depends_on health
    uses template checks.retry_curl with { url = "${{ params.base_url }}/ready", attempts = 5 }
  }
}
```

```console
$ conduit run with-template.flow
✔ health · check   (0.6s)
✔ probe  · hit     (0.4s)
Run r-7e21 succeeded in 1.1s
```

**Grammar-in-practice.** `TemplateDecl` with a `ParamSig` including a defaulted `duration` param (§7.6); a
library file with **zero** top-level workflows (permitted by §4); `ImportDecl` with `as` namespacing (§5);
and `TemplateUse` (`uses template <name> with { … }`, §7.6). The heredoc `<<-SH` allows an indented closer and
keeps interpolation active (§8.4).

---

## (h) Secrets + plugin `uses:` cloud deploy — `cloud-deploy.flow`

Secrets are declared with `secret` and resolved lazily by the Secrets Manager (never printed; redacted in
logs — see [61 — Secrets Management](61-secrets-management.md)). Here they feed a cloud-deploy plugin action.

```flow
flow "1.0"

input region: string = "us-east-1"

workflow cloud_deploy {
  description = "Push an image and deploy to a managed cloud runtime using resolved secrets."

  # Secrets resolve via a provider ref (see the `vault` plugin in doc 92) or by name.
  secret {
    REGISTRY_TOKEN = ref("vault://kv/ci/registry#token")
    CLOUD_API_KEY  = ref("vault://kv/prod/cloud#api_key")
  }

  task push_image {
    step {
      # Fully-qualified URI form of a plugin action (equivalent to `oci/push@v2`).
      uses: "plugin://oci/push@v2" with {
        image = "registry.example.com/app:${{ ctx.version }}"
        # Secrets are passed as opaque handles; the value is injected in-plugin, never logged.
        auth  = "${{ secrets.REGISTRY_TOKEN }}"
      }
    }
  }

  task deploy {
    depends_on push_image
    env { REGION = "${{ inputs.region }}" }
    step run_deploy {
      uses: cloud/deploy@v4 with {
        service   = "app"
        image     = "registry.example.com/app:${{ ctx.version }}"
        region    = "${{ env.REGION }}"
        api_key   = "${{ secrets.CLOUD_API_KEY }}"
        replicas  = 3
      }
    }
    on_error {
      step rollback { uses: cloud/deploy@v4 with { service = "app", rollback = true, api_key = "${{ secrets.CLOUD_API_KEY }}" } }
    }
  }
}
```

```console
$ conduit run cloud-deploy.flow --var version=1.4.0 --input region=us-west-2
✔ push_image                   (14.9s)
✔ deploy · run_deploy          (22.1s)
Run r-a17c succeeded in 37.4s
# Note: secret values appear as «redacted» in all logs and JSON output.
```

**Grammar-in-practice.** `SecretDecl` block form with `SecretRef` = `ref("…")` (§7.2). Two `uses:` actions
shown in both **short** (`cloud/deploy@v4`) and **URI** (`"plugin://oci/push@v2"`) `ActionRef` forms (§7.4).
Secret interpolation `${{ secrets.X }}` yields a redacting handle; see the redaction sequence in
[93 §6](93-sequence-diagrams.md#6-secret-resolution--redaction).

---

## (i) Dynamic `for_each` over a plugin-provided list — `dynamic-foreach.flow`

`for_each` iterates a CEL collection. The collection can be **computed at runtime** from a prior task's output
(e.g. a plugin that lists resources). Each iteration is a distinct DAG node with its own loop bindings.

```flow
flow "1.0"

param cluster: string = "prod-euw1"

workflow rolling_restart {
  description = "Discover services from a plugin, then restart each one with an index-aware guard."

  # Task 1: a plugin lists services and publishes a typed list output.
  task discover {
    step list {
      uses: k8s/list_services@v3 with { cluster = "${{ params.cluster }}", namespace = "app" }
    }
    # The plugin action populates this output (list<string>).
    output services: list<string> = "${{ steps.list.outputs.names }}"
  }

  # Task 2: fan out over the discovered list. `as svc, idx` binds value + index.
  task restart {
    depends_on discover

    # Bare CEL collection sourced from the upstream output.
    for_each ${{ tasks.discover.outputs.services }} as svc, idx {
      # Per-iteration guard: skip anything the operator excluded.
      when !(svc in ["canary", "debug"])

      step roll {
        env { SVC = "${{ svc }}", ORDINAL = "${{ idx }}" }
        run "kubectl -n app rollout restart deploy/${{ svc }} && echo restarted #${{ idx }}"
      }
    }
  }
}
```

```console
$ conduit run dynamic-foreach.flow --var cluster=prod-euw1
✔ discover · list                        (0.9s)   → services=[api, worker, canary, web]
✔ restart[svc=api    idx=0]              (4.1s)
✔ restart[svc=worker idx=1]              (4.3s)
⊘ restart[svc=canary idx=2]  skipped (when=false)
✔ restart[svc=web    idx=3]              (4.0s)
Run r-d4f0 succeeded in 9.6s
```

**Grammar-in-practice.** `ForEachDecl` with the two-binding `as svc, idx` form (value + index) and a
runtime-computed `EmbeddedExpr` collection (§7.3). Each iteration owns a `WhenDecl` guard evaluated with its
loop bindings in scope. `OutputDecl` typed as `list<string>` (§7.1). Data flows plugin → `output` → `for_each`
entirely within CEL — see the [Sample Workflows ETL scenario](90-sample-workflows.md#4-data-etl-pipeline) for a
larger version.

---

## Feature coverage matrix

| Construct | Sample(s) |
|---|---|
| `workflow` / `task` / `step` | all |
| `run:` (shell) | a, b, c, d, i |
| `uses:` (plugin action) | c, d, e, f, g, h, i |
| `depends_on` (single & list) | b, c, e, f, g, h, i |
| `when:` (CEL guard) | e, i |
| `for_each` (+ index) | i |
| `matrix` | d |
| `param` / `input` / `output` | b, e, g, h, i |
| `env` | d, e, h, i |
| `secret` / `ref(...)` | h |
| `retry` | f, g |
| `timeout` | f, g |
| `on_error` / `on_success` | f, h |
| `template` / `import` | g |
| heredoc `<<-` | g |
| string interpolation `${{ }}` | b, d, e, f, g, h, i |

---

## Cross-references

- Normative grammar for every construct above → [20 — DSL Grammar](20-dsl-grammar.md)
- CEL variables/functions available in `${{ }}`, `when`, `for_each` → [25 — Expression Engine](25-expression-engine.md)
- How the DAG, retries, timeouts, and compensation execute → [31 — Execution Runtime](31-execution-runtime.md)
- Plugins referenced by `uses:` (`greet`, `git`, `vault`, …) → [92 — Sample Plugins](92-sample-plugins.md)
- End-to-end runnable scenarios with output & DAGs → [90 — Sample Workflows](90-sample-workflows.md)
