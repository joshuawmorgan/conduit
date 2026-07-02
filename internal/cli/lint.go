package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newLintCmd() *cobra.Command {
	var strict bool
	cmd := &cobra.Command{
		Use:               "lint <file.flow>",
		Short:             "Report warnings and style issues in a FlowDSL file",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: flowFileCompletion,
		Annotations:       map[string]string{"ai.sideEffect": "none", "ai.idempotent": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			_, d, err := parseFlowFile(args[0])
			if err != nil {
				return err
			}
			if g.output == "json" {
				return writeJSON(map[string]any{"file": args[0], "diagnostics": d.Items})
			}
			printDiagnostics(d)
			if d.Len() == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: clean\n", args[0])
			}
			if d.HasErrors() || (strict && d.Len() > 0) {
				return withExit(exitDSL, fmt.Errorf("lint failed (%d issue(s))", d.Len()))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", false, "treat warnings as failures")
	return cmd
}
