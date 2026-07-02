## conduit completion zsh

Generate the autocompletion script for zsh

### Synopsis

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <(conduit completion zsh)

To load completions for every new session, execute once:

#### Linux:

	conduit completion zsh > "${fpath[1]}/_conduit"

#### macOS:

	conduit completion zsh > $(brew --prefix)/share/zsh/site-functions/_conduit

You will need to start a new shell for this setup to take effect.


```
conduit completion zsh [flags]
```

### Options

```
  -h, --help              help for zsh
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

