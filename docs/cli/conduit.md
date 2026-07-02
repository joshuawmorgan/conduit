## conduit

Conduit — enterprise CLI & workflow automation platform

### Synopsis

Conduit runs FlowDSL (*.flow) workflows as DAGs, with a plugin
architecture, CEL expressions, shell completion and IDE integration.

Author workflows in FlowDSL, validate and format them, visualize the DAG,
then execute with 'conduit run'. See 'conduit <command> --help' for details.

### Options

```
      --config string   path to config file (default "conduit.yaml")
  -h, --help            help for conduit
      --no-input        never prompt; fail instead (for CI/agents)
  -o, --output string   output format: table|json|yaml
  -v, --verbose count   increase verbosity (-v, -vv)
```

### SEE ALSO

* [conduit completion](conduit_completion.md)	 - Generate the autocompletion script for the specified shell
* [conduit docs](conduit_docs.md)	 - Generate CLI reference documentation
* [conduit fmt](conduit_fmt.md)	 - Format a FlowDSL file canonically
* [conduit graph](conduit_graph.md)	 - Render a workflow's task DAG
* [conduit init](conduit_init.md)	 - Scaffold a conduit.yaml and an example workflow
* [conduit lint](conduit_lint.md)	 - Report warnings and style issues in a FlowDSL file
* [conduit meta](conduit_meta.md)	 - Introspect Conduit's machine-readable metadata
* [conduit plugin](conduit_plugin.md)	 - Manage and inspect Conduit plugins
* [conduit run](conduit_run.md)	 - Execute a workflow from a FlowDSL file
* [conduit runs](conduit_runs.md)	 - List recent workflow runs from the state store
* [conduit validate](conduit_validate.md)	 - Parse and semantically validate a FlowDSL file
* [conduit version](conduit_version.md)	 - Print version information

