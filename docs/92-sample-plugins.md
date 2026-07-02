# 92 — Sample Plugin Implementations

> **Codename:** Conduit · **Binary:** `conduit` (alias `cdt`) · **Module:** `github.com/conduit-io/conduit`
> **Plugin transport:** HashiCorp **go-plugin** over **gRPC** (Unix socket / Windows named pipe) · **Go:** 1.24+
> **Status:** Samples v1.0 · **Owner:** Extensibility · **Date:** 2026-07-02

This document contains **three complete, compilable-looking sample plugins** built on the Conduit Plugin SDK
(`github.com/conduit-io/conduit/sdk/plugin`). Each plugin is a standalone Go binary that Conduit launches as a
subprocess and talks to over gRPC via the go-plugin handshake.

**Related documents**
- [40 — Plugin Architecture](40-plugin-architecture.md) — handshake, lifecycle, capability model (normative).
- [41 — Extension SDK](41-extension-sdk.md) — the `sdk/plugin` helpers used below.
- [44 — Public SDK](44-public-sdk.md) · [50 — Completion Engine](50-completion-engine.md) — completion contributions.
- [61 — Secrets Management](61-secrets-management.md) — the secret-provider interface (`vault` plugin).
- [91 — Sample DSL Files](91-sample-dsl-files.md) — how `uses:` references these plugins from `.flow`.

> **Samples in this doc**
> | Plugin | Kind | Contributes | Section |
> |---|---|---|---|
> | `greet` | Action | one action `greet/say` | [(a)](#a-greet--minimal-action-plugin) |
> | `git` | Action + Completion | actions + dynamic completions | [(b)](#b-git--actions--completions-plugin) |
> | `vault` | Secret Provider | resolves `ref("vault://…")` | [(c)](#c-vault--secret-provider-plugin) |

---

## 0. The plugin contract (shared)

All plugins implement one or more gRPC-served interfaces defined by the SDK. The core service every plugin
implements is `CapabilityProvider`; action and secret plugins additionally implement the relevant capability
interface.

### 0.1 Protobuf (excerpt from `sdk/plugin/proto/plugin.proto`)

```protobuf
syntax = "proto3";
package conduit.plugin.v1;
option go_package = "github.com/conduit-io/conduit/sdk/plugin/proto;proto";

// Every plugin implements this. The host calls Describe() right after handshake.
service CapabilityProvider {
  rpc Describe (DescribeRequest) returns (Manifest);
  rpc Health   (HealthRequest)   returns (HealthResponse);
}

// Action plugins implement this to run `uses:` actions.
service ActionRunner {
  rpc Execute (ExecuteRequest) returns (stream ExecuteEvent); // streams logs + final result
  rpc Complete(CompleteRequest) returns (CompleteResponse);   // dynamic completions (optional)
}

// Secret-provider plugins implement this to resolve ref("scheme://…").
service SecretProvider {
  rpc Resolve (ResolveRequest) returns (ResolveResponse);
}

message Manifest {
  string name = 1;                 // "git"
  string version = 2;              // "1.2.0"
  string api_version = 3;          // "plugin/1.0"
  repeated ActionSpec actions = 4; // actions this plugin serves
  repeated string secret_schemes = 5; // e.g. ["vault"]
}
message ActionSpec {
  string name = 1;                 // "clone"  → referenced as git/clone@v1
  string version = 2;              // "v1"
  map<string, ParamSpec> inputs = 3;
  map<string, ParamSpec> outputs = 4;
}
message ParamSpec { string type = 1; bool required = 2; string description = 3; }

message ExecuteRequest {
  string action = 1;               // "clone"
  map<string, Value> inputs = 2;   // resolved `with { … }` args
  map<string, string> env = 3;
  string workdir = 4;
}
message ExecuteEvent {
  oneof body {
    string log_line = 1;           // streamed stdout/stderr
    ExecuteResult result = 2;      // terminal event
  }
}
message ExecuteResult {
  bool success = 1;
  map<string, Value> outputs = 2;
  string error = 3;
  bool transient = 4;              // hints the runtime's retry `when = error.transient`
}

message CompleteRequest  { string action = 1; string arg = 2; string prefix = 3; }
message CompleteResponse { repeated Completion items = 1; }
message Completion { string value = 1; string description = 2; }

message ResolveRequest  { string uri = 1; }   // "vault://kv/ci/registry#token"
message ResolveResponse { bytes value = 1; bool secret = 2; }

message Value { oneof v { string s = 1; int64 i = 2; double d = 3; bool b = 4; } }
message DescribeRequest {}
message HealthRequest {}
message HealthResponse { bool ok = 1; string detail = 2; }
```

### 0.2 SDK-side Go interfaces (what you implement)

```go
// Package plugin (github.com/conduit-io/conduit/sdk/plugin) wraps the gRPC layer so
// plugin authors implement plain Go interfaces.
package plugin

import "context"

// Every plugin implements Describe + Health.
type Capability interface {
	Describe(ctx context.Context) (Manifest, error)
	Health(ctx context.Context) error
}

// Action plugins implement Execute; Emit streams log lines back to the host.
type Action interface {
	Capability
	Execute(ctx context.Context, req ExecuteRequest, emit func(line string)) (ExecuteResult, error)
}

// Optional: contribute dynamic shell/LSP completions for an action's args.
type Completer interface {
	Complete(ctx context.Context, action, arg, prefix string) ([]Completion, error)
}

// Secret-provider plugins implement Resolve for their URI scheme(s).
type SecretProvider interface {
	Capability
	Resolve(ctx context.Context, uri string) ([]byte, error)
}

// Serve wires the given implementation into go-plugin/gRPC and blocks. It performs
// the magic-cookie handshake and registers the appropriate gRPC services based on
// which interfaces `impl` satisfies.
func Serve(impl Capability) // implemented in the SDK
```

### 0.3 go-plugin handshake (shared by all three `main.go`s)

```go
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "CONDUIT_PLUGIN",
	MagicCookieValue: "b6f1c0a2-conduit-plugin-v1",
}
```

The host verifies the cookie + protocol version before any RPC; mismatches fail fast. See
[40 §Handshake](40-plugin-architecture.md) and the lifecycle diagram in
[93 §5](93-sequence-diagrams.md#5-plugin-load--handshake--health--shutdown).

---

## (a) `greet` — minimal action plugin

The smallest possible action plugin: one action, `greet/say`, that prints a greeting and returns it as an output.

### Layout

```
plugins/greet/
├── go.mod
├── main.go
└── plugin.yaml        # manifest (also served via Describe)
```

### `plugin.yaml` (manifest)

```yaml
name: greet
version: 1.0.0
api_version: plugin/1.0
description: A minimal greeting action.
actions:
  - name: say
    version: v1
    inputs:
      to:    { type: string, required: true, description: "Who to greet." }
      shout: { type: bool,   required: false, description: "Uppercase the greeting." }
    outputs:
      message: { type: string, description: "The rendered greeting." }
```

### `main.go`

```go
package main

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/conduit-io/conduit/sdk/plugin"
)

type greetPlugin struct{}

func (greetPlugin) Describe(context.Context) (sdk.Manifest, error) {
	return sdk.Manifest{
		Name: "greet", Version: "1.0.0", APIVersion: "plugin/1.0",
		Actions: []sdk.ActionSpec{{
			Name: "say", Version: "v1",
			Inputs: map[string]sdk.ParamSpec{
				"to":    {Type: "string", Required: true},
				"shout": {Type: "bool"},
			},
			Outputs: map[string]sdk.ParamSpec{"message": {Type: "string"}},
		}},
	}, nil
}

func (greetPlugin) Health(context.Context) error { return nil }

func (greetPlugin) Execute(ctx context.Context, req sdk.ExecuteRequest, emit func(string)) (sdk.ExecuteResult, error) {
	if req.Action != "say" {
		return sdk.ExecuteResult{}, fmt.Errorf("unknown action %q", req.Action)
	}
	to, _ := req.Inputs["to"].AsString()
	if to == "" {
		return sdk.ExecuteResult{Success: false, Error: "input 'to' is required"}, nil
	}
	msg := fmt.Sprintf("Hello, %s!", to)
	if shout, _ := req.Inputs["shout"].AsBool(); shout {
		msg = strings.ToUpper(msg)
	}
	emit(msg) // streamed to the run log in real time
	return sdk.ExecuteResult{
		Success: true,
		Outputs: map[string]sdk.Value{"message": sdk.StringValue(msg)},
	}, nil
}

func main() { sdk.Serve(greetPlugin{}) } // handshake + gRPC serve; blocks
```

### Reference it from a `.flow`

```flow
flow "1.0"
workflow hello_plugin {
  task greet {
    step {
      uses: greet/say@v1 with { to = "Conduit", shout = true }
    }
    output greeting: string = "${{ steps[0].outputs.message }}"
  }
}
```

### Build / install / test

```console
# build
$ cd plugins/greet && go build -o ../../bin/conduit-plugin-greet .

# install into the local plugin registry (~/.conduit/plugins)
$ conduit plugin install ./bin/conduit-plugin-greet
installed greet@1.0.0 (actions: say@v1)

# unit-test the action logic in-process with the SDK test harness (no gRPC needed)
$ go test ./plugins/greet/...
```

```go
// main_test.go — exercise Execute directly via the SDK harness.
func TestSayShouts(t *testing.T) {
	res, err := sdk.TestExecute(greetPlugin{}, sdk.ExecuteRequest{
		Action: "say",
		Inputs: map[string]sdk.Value{"to": sdk.StringValue("world"), "shout": sdk.BoolValue(true)},
	})
	if err != nil || !res.Success {
		t.Fatalf("execute failed: %v (%s)", err, res.Error)
	}
	if got, _ := res.Outputs["message"].AsString(); got != "HELLO, WORLD!" {
		t.Fatalf("got %q", got)
	}
}
```

---

## (b) `git` — actions + completions plugin

A richer plugin: multiple actions (`clone`, `changelog`) plus **dynamic completions** so the LSP/shell can
suggest branch names for the `ref` argument. This shows the `Completer` interface in action.

### Layout

```
plugins/git/
├── go.mod
├── main.go
├── actions.go
├── completions.go
└── plugin.yaml
```

### `plugin.yaml`

```yaml
name: git
version: 1.2.0
api_version: plugin/1.0
description: Git operations for FlowDSL.
actions:
  - name: clone
    version: v1
    inputs:
      url: { type: string, required: true }
      ref: { type: string, required: false, description: "Branch/tag/sha (completable)." }
      dir: { type: string, required: false }
    outputs:
      sha: { type: string }
  - name: changelog
    version: v1
    inputs:
      since: { type: string, required: true }
      to:    { type: string, required: true }
    outputs:
      markdown: { type: string }
```

### `main.go`

```go
package main

import (
	"context"
	sdk "github.com/conduit-io/conduit/sdk/plugin"
)

// gitPlugin satisfies sdk.Action AND sdk.Completer, so Serve registers both the
// ActionRunner.Execute and ActionRunner.Complete RPCs.
type gitPlugin struct{}

func (gitPlugin) Health(context.Context) error { return nil }

func (gitPlugin) Describe(context.Context) (sdk.Manifest, error) {
	return sdk.Manifest{
		Name: "git", Version: "1.2.0", APIVersion: "plugin/1.0",
		Actions: []sdk.ActionSpec{
			{Name: "clone", Version: "v1", Inputs: map[string]sdk.ParamSpec{
				"url": {Type: "string", Required: true},
				"ref": {Type: "string"},
				"dir": {Type: "string"},
			}, Outputs: map[string]sdk.ParamSpec{"sha": {Type: "string"}}},
			{Name: "changelog", Version: "v1", Inputs: map[string]sdk.ParamSpec{
				"since": {Type: "string", Required: true},
				"to":    {Type: "string", Required: true},
			}, Outputs: map[string]sdk.ParamSpec{"markdown": {Type: "string"}}},
		},
	}, nil
}

func main() { sdk.Serve(gitPlugin{}) }
```

### `actions.go`

```go
package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	sdk "github.com/conduit-io/conduit/sdk/plugin"
)

func (g gitPlugin) Execute(ctx context.Context, req sdk.ExecuteRequest, emit func(string)) (sdk.ExecuteResult, error) {
	switch req.Action {
	case "clone":
		return g.clone(ctx, req, emit)
	case "changelog":
		return g.changelog(ctx, req, emit)
	default:
		return sdk.ExecuteResult{}, fmt.Errorf("unknown action %q", req.Action)
	}
}

func (gitPlugin) clone(ctx context.Context, req sdk.ExecuteRequest, emit func(string)) (sdk.ExecuteResult, error) {
	url, _ := req.Inputs["url"].AsString()
	dir, _ := req.Inputs["dir"].AsString()
	ref, _ := req.Inputs["ref"].AsString()
	if dir == "" {
		dir = "repo"
	}
	// ctx cancellation aborts the child process (see 40 §cancelation).
	if out, err := runGit(ctx, req.Workdir, emit, "clone", url, dir); err != nil {
		return sdk.ExecuteResult{Success: false, Error: out, Transient: isTransient(out)}, nil
	}
	if ref != "" {
		if out, err := runGit(ctx, dir, emit, "checkout", ref); err != nil {
			return sdk.ExecuteResult{Success: false, Error: out}, nil
		}
	}
	sha, _ := runGit(ctx, dir, nil, "rev-parse", "HEAD")
	return sdk.ExecuteResult{Success: true, Outputs: map[string]sdk.Value{
		"sha": sdk.StringValue(strings.TrimSpace(sha)),
	}}, nil
}

func (gitPlugin) changelog(ctx context.Context, req sdk.ExecuteRequest, emit func(string)) (sdk.ExecuteResult, error) {
	since, _ := req.Inputs["since"].AsString()
	to, _ := req.Inputs["to"].AsString()
	rng := fmt.Sprintf("%s..%s", since, to)
	out, err := runGit(ctx, req.Workdir, nil, "log", "--pretty=- %s (%h)", rng)
	if err != nil {
		return sdk.ExecuteResult{Success: false, Error: out}, nil
	}
	md := "## Changes\n\n" + out
	return sdk.ExecuteResult{Success: true, Outputs: map[string]sdk.Value{
		"markdown": sdk.StringValue(md),
	}}, nil
}

func runGit(ctx context.Context, dir string, emit func(string), args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if emit != nil {
		emit(string(out))
	}
	return string(out), err
}

func isTransient(out string) bool {
	return strings.Contains(out, "Could not resolve host") || strings.Contains(out, "timed out")
}
```

### `completions.go`

```go
package main

import (
	"context"
	"os/exec"
	"strings"

	sdk "github.com/conduit-io/conduit/sdk/plugin"
)

// Complete is invoked by the host's completion engine (see 50) when the user is
// editing a `with { ref = <cursor> }` argument for git/clone. It streams branch/tag
// candidates. This satisfies sdk.Completer.
func (gitPlugin) Complete(ctx context.Context, action, arg, prefix string) ([]sdk.Completion, error) {
	if action != "clone" || arg != "ref" {
		return nil, nil
	}
	out, err := exec.CommandContext(ctx, "git", "for-each-ref", "--format=%(refname:short)").Output()
	if err != nil {
		return nil, nil // no completions on error; never fail the editor
	}
	var items []sdk.Completion
	for _, ref := range strings.Fields(string(out)) {
		if strings.HasPrefix(ref, prefix) {
			items = append(items, sdk.Completion{Value: ref, Description: "git ref"})
		}
	}
	return items, nil
}
```

### Reference it from a `.flow`

```flow
flow "1.0"
workflow build {
  task src {
    step { uses: git/clone@v1 with { url = "https://github.com/acme/app", ref = "main" } }
    output sha: string = "${{ steps[0].outputs.sha }}"
  }
}
```

When editing `ref = "…"` in an editor, the LSP calls the plugin's `Complete` and offers live branch names — the
same `__complete` path documented in [93 §4](93-sequence-diagrams.md#4-shell-dynamic-completion-via-__complete).

### Build / install / test

```console
$ cd plugins/git && go build -o ../../bin/conduit-plugin-git .
$ conduit plugin install ./bin/conduit-plugin-git
installed git@1.2.0 (actions: clone@v1, changelog@v1; completions: clone.ref)

# verify handshake + Describe end-to-end (spawns the real subprocess over gRPC)
$ conduit plugin verify git
✔ handshake ok (protocol 1)   ✔ Describe ok (2 actions)   ✔ Health ok

$ go test ./plugins/git/...   # unit tests for clone/changelog/completion parsing
```

---

## (c) `vault` — secret-provider plugin

A secret-provider plugin resolves `ref("vault://…")` references declared in a `.flow`'s `secret {}` block. It
implements `SecretProvider.Resolve`; the returned bytes are marked secret and redacted everywhere downstream
(see [61 — Secrets Management](61-secrets-management.md)).

### Layout

```
plugins/vault/
├── go.mod
├── main.go
└── plugin.yaml
```

### `plugin.yaml`

```yaml
name: vault
version: 0.9.0
api_version: plugin/1.0
description: Resolves vault://kv/<path>#<field> secret references.
secret_schemes: ["vault"]
```

### `main.go`

```go
package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	vaultapi "github.com/hashicorp/vault/api"
	sdk "github.com/conduit-io/conduit/sdk/plugin"
)

// vaultPlugin satisfies sdk.SecretProvider (Describe + Health + Resolve).
type vaultPlugin struct{ client *vaultapi.Client }

func (vaultPlugin) Describe(context.Context) (sdk.Manifest, error) {
	return sdk.Manifest{
		Name: "vault", Version: "0.9.0", APIVersion: "plugin/1.0",
		SecretSchemes: []string{"vault"},
	}, nil
}

func (p vaultPlugin) Health(ctx context.Context) error {
	h, err := p.client.Sys().HealthWithContext(ctx)
	if err != nil {
		return err
	}
	if h.Sealed {
		return fmt.Errorf("vault is sealed")
	}
	return nil
}

// Resolve parses vault://kv/<path>#<field>, reads the KV secret, and returns the field.
func (p vaultPlugin) Resolve(ctx context.Context, raw string) ([]byte, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "vault" {
		return nil, fmt.Errorf("not a vault ref: %q", raw)
	}
	mount, path, _ := strings.Cut(strings.TrimPrefix(u.Path, "/"), "/") // "kv", "ci/registry"
	field := u.Fragment                                                 // "token"
	sec, err := p.client.KVv2(mount).Get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("vault read %s/%s: %w", mount, path, err)
	}
	v, ok := sec.Data[field]
	if !ok {
		return nil, fmt.Errorf("field %q not found at %s/%s", field, mount, path)
	}
	return []byte(fmt.Sprint(v)), nil
}

func main() {
	cfg := vaultapi.DefaultConfig() // reads VAULT_ADDR / VAULT_TOKEN
	client, err := vaultapi.NewClient(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "vault init:", err)
		os.Exit(1)
	}
	sdk.Serve(vaultPlugin{client: client})
}
```

### Reference it from a `.flow`

```flow
flow "1.0"
workflow deploy {
  secret {
    # The `vault://` scheme is routed to this plugin's Resolve.
    REGISTRY_TOKEN = ref("vault://kv/ci/registry#token")
  }
  task push {
    step { uses: oci/push@v2 with { image = "app:1.0", auth = "${{ secrets.REGISTRY_TOKEN }}" } }
  }
}
```

The value never appears in logs, `--output json`, or crash dumps — the SDK wraps it in a redacting type at the
gRPC boundary. See the redaction sequence in [93 §6](93-sequence-diagrams.md#6-secret-resolution--redaction).

### Build / install / test

```console
$ cd plugins/vault && go build -o ../../bin/conduit-plugin-vault .
$ conduit plugin install ./bin/conduit-plugin-vault
installed vault@0.9.0 (secret schemes: vault)

# smoke test resolution against a dev vault
$ VAULT_ADDR=http://127.0.0.1:8200 VAULT_TOKEN=root \
    conduit secret resolve 'vault://kv/ci/registry#token'
«redacted» (12 bytes) — resolved by vault@0.9.0

$ go test ./plugins/vault/...   # unit-test URL parsing & field extraction with a mock KV
```

---

## Registering a plugin with Conduit (`conduit.yaml`)

Installed plugins are discovered from the local registry, but a project can pin exact versions and sources:

```yaml
# conduit.yaml
plugins:
  - name: greet
    version: 1.0.0
    source: file://./bin/conduit-plugin-greet
  - name: git
    version: 1.2.0
    source: oci://ghcr.io/conduit-io/plugin-git:1.2.0
  - name: vault
    version: 0.9.0
    source: oci://ghcr.io/conduit-io/plugin-vault:0.9.0
    config:
      addr: ${VAULT_ADDR}      # host env expansion at load time
```

Version pinning and source resolution rules are specified in [40 §Registry & pinning](40-plugin-architecture.md);
config precedence for the `config:` block follows [60 — Configuration](60-configuration.md).

---

## Cross-references

- Handshake, lifecycle, capability model, registry → [40 — Plugin Architecture](40-plugin-architecture.md)
- SDK helper types (`Value`, `TestExecute`, `Serve`) → [41 — Extension SDK](41-extension-sdk.md)
- Completion contribution path → [50 — Completion Engine](50-completion-engine.md)
- Secret provider & redaction → [61 — Secrets Management](61-secrets-management.md)
- How `uses:` resolves to these plugins → [20 — DSL Grammar §7.4](20-dsl-grammar.md), [91 — Sample DSL Files](91-sample-dsl-files.md)
- Plugin & agent sequence diagrams → [93 — Sequence Diagrams](93-sequence-diagrams.md)
