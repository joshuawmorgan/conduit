## conduit completion bash

Generate the autocompletion script for bash

### Synopsis

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(conduit completion bash)

To load completions for every new session, execute once:

#### Linux:

	conduit completion bash > /etc/bash_completion.d/conduit

#### macOS:

	conduit completion bash > $(brew --prefix)/etc/bash_completion.d/conduit

You will need to start a new shell for this setup to take effect.


```
conduit completion bash
```

### Options

```
  -h, --help              help for bash
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

