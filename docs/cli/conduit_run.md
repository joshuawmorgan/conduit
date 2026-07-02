## conduit run

Execute a workflow from a FlowDSL file

```
conduit run <file.flow> [flags]
```

### Examples

```
  conduit run ci.flow
  conduit run ci.flow --workflow deploy --param branch=main
  conduit run ci.flow --dry-run
  conduit run ci.flow --output json
```

### Options

```
  -c, --concurrency int     max concurrent tasks (0 = auto)
      --dry-run             plan and print actions without executing
  -h, --help                help for run
  -p, --param stringArray   set a parameter (key=value); repeatable
  -w, --workflow string     workflow name (if the file has more than one)
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

