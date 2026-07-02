package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	pluginmgr "github.com/conduit-io/conduit/internal/plugin"
)

func newPluginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage and inspect Conduit plugins",
	}
	cmd.AddCommand(newPluginListCmd(), newPluginDescribeCmd())
	return cmd
}

func newPluginListCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List discovered plugins",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"ai.sideEffect": "none", "ai.idempotent": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			pm := pluginmgr.New(g.cfg.PluginDirs)
			defer pm.Close()
			names := pm.Discovered()
			if g.output == "json" {
				return writeJSON(map[string]any{"dirs": g.cfg.PluginDirs, "plugins": names})
			}
			if len(names) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no plugins found in %v\n", g.cfg.PluginDirs)
				return nil
			}
			for _, n := range names {
				fmt.Fprintln(cmd.OutOrStdout(), n)
			}
			return nil
		},
	}
}

func newPluginDescribeCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "describe <name>",
		Short:       "Show a plugin's provided actions",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"ai.sideEffect": "launches-plugin", "ai.idempotent": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			pm := pluginmgr.New(g.cfg.PluginDirs)
			defer pm.Close()
			desc, err := pm.Describe(args[0])
			if err != nil {
				return withExit(exitError_, err)
			}
			if g.output == "json" {
				return writeJSON(desc)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", desc.Name, desc.Version)
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			for _, a := range desc.Actions {
				fmt.Fprintf(tw, "  %s\n", a)
			}
			tw.Flush()
			return nil
		},
	}
}
