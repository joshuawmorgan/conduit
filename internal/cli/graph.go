package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/conduit-io/conduit/internal/dag"
)

func newGraphCmd() *cobra.Command {
	var workflow, formatFlag string
	cmd := &cobra.Command{
		Use:               "graph <file.flow>",
		Short:             "Render a workflow's task DAG",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: flowFileCompletion,
		Annotations:       map[string]string{"ai.sideEffect": "none", "ai.idempotent": "true"},
		Example: `  conduit graph ci.flow                 # mermaid (default)
  conduit graph ci.flow --format dot
  conduit graph ci.flow --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			file, d, err := parseFlowFile(args[0])
			if err != nil {
				return err
			}
			if d.HasErrors() {
				printDiagnostics(d)
				return withExit(exitDSL, fmt.Errorf("cannot graph: file has errors"))
			}
			wf, err := selectWorkflow(file, workflow)
			if err != nil {
				return err
			}
			gr, err := dag.Build(wf)
			if err != nil {
				return withExit(exitDSL, err)
			}
			out := cmd.OutOrStdout()
			if g.output == "json" {
				layers, _ := gr.Layers()
				order, _ := gr.TopoSort()
				return writeJSON(map[string]any{
					"workflow": gr.Workflow,
					"order":    order,
					"layers":   layers,
					"roots":    gr.Roots(),
				})
			}
			switch formatFlag {
			case "dot":
				fmt.Fprint(out, gr.DOT())
			case "mermaid", "":
				fmt.Fprint(out, gr.Mermaid())
			default:
				return withExit(exitUsage, fmt.Errorf("unknown --format %q (want mermaid|dot)", formatFlag))
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&workflow, "workflow", "w", "", "workflow name")
	cmd.Flags().StringVarP(&formatFlag, "format", "f", "mermaid", "graph format: mermaid|dot")
	_ = cmd.RegisterFlagCompletionFunc("workflow", workflowCompletion)
	_ = cmd.RegisterFlagCompletionFunc("format", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"mermaid", "dot"}, cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}
