package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/conduit-io/conduit/internal/buildinfo"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Annotations: map[string]string{
			"ai.idempotent": "true",
			"ai.sideEffect": "none",
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if g.output == "json" {
				return writeJSON(buildinfo.Map())
			}
			fmt.Fprintln(cmd.OutOrStdout(), buildinfo.String())
			return nil
		},
	}
}
