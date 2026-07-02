// Package cli assembles the Conduit command tree on top of Cobra.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/conduit-io/conduit/internal/buildinfo"
	"github.com/conduit-io/conduit/internal/config"
)

// globalOpts holds persistent flag values shared by subcommands.
type globalOpts struct {
	configPath string
	output     string
	noInput    bool
	verbosity  int
	cfg        *config.Config
}

var g = &globalOpts{}

// NewRootCmd builds the root command tree.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "conduit",
		Short: "Conduit — enterprise CLI & workflow automation platform",
		Long: `Conduit runs FlowDSL (*.flow) workflows as DAGs, with a plugin
architecture, CEL expressions, shell completion and IDE integration.

Author workflows in FlowDSL, validate and format them, visualize the DAG,
then execute with 'conduit run'. See 'conduit <command> --help' for details.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       buildinfo.Version,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(g.configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			if cmd.Flags().Changed("output") {
				cfg.Output = g.output
			} else if g.output == "" {
				g.output = cfg.Output
			}
			g.cfg = cfg
			return nil
		},
	}

	pf := root.PersistentFlags()
	pf.StringVar(&g.configPath, "config", "conduit.yaml", "path to config file")
	pf.StringVarP(&g.output, "output", "o", "", "output format: table|json|yaml")
	pf.BoolVar(&g.noInput, "no-input", false, "never prompt; fail instead (for CI/agents)")
	pf.CountVarP(&g.verbosity, "verbose", "v", "increase verbosity (-v, -vv)")

	root.SetVersionTemplate("{{.Version}}\n")

	root.AddCommand(
		newVersionCmd(),
		newRunCmd(),
		newValidateCmd(),
		newFmtCmd(),
		newGraphCmd(),
		newLintCmd(),
		newInitCmd(),
		newListCmd(),
		newDocsCmd(),
		newMetaCmd(),
		newPluginCmd(),
	)
	return root
}

// Execute runs the root command and returns a process exit code.
func Execute() int {
	root := NewRootCmd()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return exitCodeFor(err)
	}
	return 0
}
