## conduit completion fish

Generate the autocompletion script for fish

### Synopsis

Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

	conduit completion fish | source

To load completions for every new session, execute once:

	conduit completion fish > ~/.config/fish/completions/conduit.fish

You will need to start a new shell for this setup to take effect.


```
conduit completion fish [flags]
```

### Options

```
  -h, --help              help for fish
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
      --config string   path to config file (default "conduit.yaml")
      --no-input        never prompt; fail instead (for CI/agents)
  -o, --output string   output format: table|json|yaml
  -v, --verbose count   increase verbosity (-v, -vv)
```

### SEE ALSO

* [conduit completion](conduit_completion.md)	 - Generate the autocompletion script for the specified shell

