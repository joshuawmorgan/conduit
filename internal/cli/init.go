package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const exampleFlow = `// Example Conduit workflow. Run it with:  conduit run example.flow
workflow "hello" {
  description: "A minimal example workflow"
  param name: string = "world"

  task greet {
    run: "echo Hello, ${params.name}!"
  }

  task done {
    depends_on: [greet]
    run: "echo workflow complete"
  }
}
`

const exampleConfig = `# Conduit configuration
concurrency: 0          # 0 = number of CPUs
state_dir: .conduit/runs
plugin_dirs:
  - .conduit/plugins
output: table
telemetry: false
log_level: info
`

func newInitCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:         "init [dir]",
		Short:       "Scaffold a conduit.yaml and an example workflow",
		Args:        cobra.MaximumNArgs(1),
		Annotations: map[string]string{"ai.sideEffect": "writes-files", "ai.idempotent": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			files := map[string]string{
				filepath.Join(dir, "conduit.yaml"): exampleConfig,
				filepath.Join(dir, "example.flow"): exampleFlow,
			}
			for path, content := range files {
				if _, err := os.Stat(path); err == nil && !force {
					fmt.Fprintf(cmd.OutOrStdout(), "skip (exists): %s\n", path)
					continue
				}
				if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "created: %s\n", path)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "\nNext:  conduit run example.flow")
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "overwrite existing files")
	return cmd
}
