package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/conduit-io/conduit/internal/state"
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "runs",
		Short:       "List recent workflow runs from the state store",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"ai.sideEffect": "none", "ai.idempotent": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := state.NewFile(g.cfg.StateDir)
			if err != nil {
				return err
			}
			runs, err := store.List()
			if err != nil {
				return err
			}
			if g.output == "json" {
				return writeJSON(runs)
			}
			if len(runs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no runs recorded")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "RUN\tWORKFLOW\tSTATUS\tSTARTED\tDURATION")
			for _, r := range runs {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
					r.ID, r.Workflow, r.Status, r.StartedAt.Format("2006-01-02 15:04:05"), r.Duration().Round(1e6))
			}
			tw.Flush()
			return nil
		},
	}
	return cmd
}
