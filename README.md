# Conduit

> Enterprise-grade CLI & workflow automation platform, in Go.

Conduit runs **FlowDSL** (`*.flow`) workflows as DAGs with CEL expressions, a
plugin architecture, shell completion and IDE integration. This repository
contains both the [architecture & design documentation](docs/README.md) and a
working reference implementation of the core engine.

```
conduit run examples/ci.flow --param branch=main
```

## Status

| Area | State |
|---|---|
| FlowDSL front-end (lexer → parser → AST → sema) | ✅ implemented (Participle v2) |
| CEL expression engine (`when`, `${ }` interpolation) | ✅ implemented (cel-go) |
| DAG planner (topo sort, layers, cycle detection) | ✅ implemented |
| Execution runtime (concurrent scheduler, retries, timeouts) | ✅ implemented |
| Shell / builtin / **plugin** actions (go-plugin) | ✅ implemented |
| State store (JSON journal) | ✅ implemented |
| Config (Koanf, layered) | ✅ implemented |
| CLI: `run validate fmt graph lint init runs docs meta plugin version` | ✅ implemented |
| Shell completion (bash/zsh/fish/powershell) | ✅ via Cobra + dynamic completers |
| LSP server / Tree-sitter grammar / TUI | 🗺️ designed (see docs), not yet built |

See the full [12-month roadmap](docs/95-roadmap.md) and [milestone plan](docs/96-milestone-plan.md).

## Quick start

```bash
# Build the CLI
go build -o conduit ./cmd/conduit        # or: task build

# Scaffold a project
./conduit init myproj && cd myproj
./conduit run example.flow

# Explore the bundled examples
./conduit validate examples/ci.flow
./conduit graph    examples/ci.flow                 # Mermaid DAG
./conduit graph    examples/ci.flow --format dot
./conduit run      examples/ci.flow --param branch=main
./conduit run      examples/ci.flow --output json   # machine/agent mode
./conduit fmt      examples/ci.flow --write

# Plugins (go-plugin subprocess over RPC)
go build -o .conduit/plugins/conduit-plugin-greet ./plugins/greet
./conduit plugin list
./conduit run examples/deploy.flow --param env=staging
```

## FlowDSL at a glance

```flow
workflow "ci" {
  description: "Build, test and package"
  param branch: string = "main"
  param skip_lint: bool = false
  env: { CI: "true" }

  task build { run: "go build ./..." }

  task test {
    depends_on: [build]
    run: "go test ./..."
    retry: 1
  }

  task lint {
    depends_on: [build]
    when: "!params.skip_lint"        # CEL guard
    run: "golangci-lint run"
  }

  task package {
    depends_on: [test, lint]
    when: "params.branch == \"main\""
    run: "echo packaging ${params.branch}"   # ${ } interpolation via CEL
  }
}
```

- **Scalar attributes**: `key: value`. **Blocks**: `keyword label { ... }`.
- `depends_on` builds the DAG; independent tasks run concurrently.
- `when:` is a CEL boolean; `${ ... }` is CEL string interpolation.
- `run:` executes a shell command; `uses:` invokes a builtin or plugin action.

## Architecture

The implementation follows the design documents in [`docs/`](docs/README.md).
Package layout (see [docs/80](docs/80-repository-structure.md)):

```
cmd/conduit            # entry point
internal/
  buildinfo            # version stamping
  model                # execution-domain types (Run, TaskRun, Status)
  config               # Koanf layered configuration
  flow/
    ast                # FlowDSL AST + Participle grammar tags
    parser             # lexer + parser (Participle v2)
    sema               # semantic analysis + diagnostics
    format             # canonical formatter (conduit fmt)
    diag               # diagnostic model
  cel                  # CEL environment: when guards + interpolation
  dag                  # graph build, topo sort, layers, cycle detection
  runtime              # concurrent scheduler + step executors
  state                # run persistence (memory + JSON file journal)
  plugin               # host-side plugin manager (go-plugin)
  meta                 # command metadata catalog (docs/completion/AI)
  cli                  # Cobra command tree
pkg/plugin             # PUBLIC plugin SDK/contract
plugins/greet          # sample plugin
examples/*.flow        # sample workflows
```

## Development

```bash
task build      # build binary
task plugins    # build first-party plugins
task test       # unit tests
task vet        # go vet
task docs       # regenerate docs/cli
task demo       # build + run example workflows
```

Requires Go 1.24+ (developed and tested against the Go 1.25 toolchain).

## Machine / AI-agent mode

Every command supports `--output json` for structured, parseable output, and
`conduit meta dump` emits the full machine-readable command catalog (used for
docs generation, completion, and AI-agent tool discovery). Exit codes follow a
documented convention (`0` ok, `2` usage, `3` DSL error, `4` run failed).

## License

Apache-2.0. See [LICENSE](LICENSE).
