package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/conduit-io/conduit/internal/dag"
	"github.com/conduit-io/conduit/internal/flow/diag"
)

func newValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "validate <file.flow>",
		Short:             "Parse and semantically validate a FlowDSL file",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: flowFileCompletion,
		Annotations:       map[string]string{"ai.sideEffect": "none", "ai.idempotent": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			file, d, err := parseFlowFile(args[0])
			if err != nil {
				return err
			}
			// Also validate each workflow builds a valid DAG.
			for _, wf := range file.Workflows() {
				if _, derr := dag.Build(wf); derr != nil {
					d.Errorf("FLOW-E031", wf.Pos, "%v", derr)
				}
			}
			d.Sort()

			if g.output == "json" {
				return writeJSON(map[string]any{
					"file":        args[0],
					"ok":          !d.HasErrors(),
					"errorCount":  d.ErrorCount(),
					"diagnostics": d.Items,
				})
			}
			printDiagnostics(d)
			if d.HasErrors() {
				return withExit(exitDSL, fmt.Errorf("validation failed: %d error(s)", d.ErrorCount()))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: OK (%d workflow(s), %d warning(s))\n",
				args[0], len(file.Workflows()), warnCount(d))
			return nil
		},
	}
	return cmd
}

func warnCount(d *diag.Diagnostics) int {
	n := 0
	for _, it := range d.Items {
		if it.Severity == diag.Warning {
			n++
		}
	}
	return n
}
