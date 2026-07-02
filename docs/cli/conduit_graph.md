## conduit graph

Render a workflow's task DAG

```
conduit graph <file.flow> [flags]
```

### Examples

```
  conduit graph ci.flow                 # mermaid (default)
  conduit graph ci.flow --format dot
  conduit graph ci.flow --output json
```

### Options

```
  -f, --format string     graph format: mermaid|dot (default "mermaid")
  -h, --help              help for graph
  -w, --workflow string   workflow name
```

### Options inherited from parent commands

```
      --config string   path to config file (default "conduit.yaml")
      --no-input        never prompt; fail instead (for CI/agents)
  -o, --output string   output format: table|json|yaml
  -v, --verbose count   increase verbosity (-v, -vv)
```

### SEE ALSO

* [conduit](conduit.md)	 - Conduit — enterprise CLI & workflow automation platform

