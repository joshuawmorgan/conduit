package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/conduit-io/conduit/internal/flow/parser"
)

func newDocsCmd() *cobra.Command {
	var outDir string
	cmd := &cobra.Command{
		Use:         "docs",
		Short:       "Generate CLI reference documentation",
		Long:        "Generate Markdown docs for every command plus the FlowDSL grammar reference.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"ai.sideEffect": "writes-files", "ai.idempotent": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return err
			}
			root := cmd.Root()
			root.DisableAutoGenTag = true
			if err := doc.GenMarkdownTree(root, outDir); err != nil {
				return err
			}
			// Grammar reference.
			grammar := "# FlowDSL Grammar\n\n```\n" + strings.TrimSpace(parser.Grammar()) + "\n```\n"
			if err := os.WriteFile(filepath.Join(outDir, "flowdsl-grammar.md"), []byte(grammar), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "documentation written to %s\n", outDir)
			return nil
		},
	}
	cmd.Flags().StringVarP(&outDir, "out", "O", "docs/cli", "output directory")
	return cmd
}
