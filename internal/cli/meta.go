package cli

import (
	"github.com/spf13/cobra"

	"github.com/conduit-io/conduit/internal/buildinfo"
	"github.com/conduit-io/conduit/internal/meta"
)

func newMetaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "meta",
		Short: "Introspect Conduit's machine-readable metadata",
		Long:  "Emit the command catalog consumed by docs generation, completion and AI agents.",
	}
	dump := &cobra.Command{
		Use:         "dump",
		Short:       "Print the full command metadata catalog as JSON",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"ai.sideEffect": "none", "ai.idempotent": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			cat := meta.FromCobra(cmd.Root(), buildinfo.Version)
			return writeJSON(cat)
		},
	}
	cmd.AddCommand(dump)
	return cmd
}
