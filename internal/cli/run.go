package cli

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/conduit-io/conduit/internal/dag"
	"github.com/conduit-io/conduit/internal/model"
	pluginmgr "github.com/conduit-io/conduit/internal/plugin"
	"github.com/conduit-io/conduit/internal/runtime"
	"github.com/conduit-io/conduit/internal/state"
)

func newRunCmd() *cobra.Command {
	var (
		workflow    string
		params      []string
		dryRun      bool
		concurrency int
	)
	cmd := &cobra.Command{
		Use:   "run <file.flow>",
		Short: "Execute a workflow from a FlowDSL file",
		Args:  cobra.ExactArgs(1),
		Example: `  conduit run ci.flow
  conduit run ci.flow --workflow deploy --param branch=main
  conduit run ci.flow --dry-run
  conduit run ci.flow --output json`,
		Annotations: map[string]string{
			"ai.sideEffect": "executes-shell-and-plugins",
			"ai.idempotent": "false",
		},
		ValidArgsFunction: flowFileCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			file, d, err := parseFlowFile(args[0])
			if err != nil {
				return err
			}
			printDiagnostics(d)
			if d.HasErrors() {
				return withExit(exitDSL, fmt.Errorf("%d semantic error(s); aborting", d.ErrorCount()))
			}
			wf, err := selectWorkflow(file, workflow)
			if err != nil {
				return err
			}
			g2, err := dag.Build(wf)
			if err != nil {
				return withExit(exitDSL, err)
			}
			pmap, err := parseParamFlags(params)
			if err != nil {
				return withExit(exitUsage, err)
			}

			pm := pluginmgr.New(g.cfg.PluginDirs)
			defer pm.Close()

			eng, err := runtime.New(nil)
			if err != nil {
				return err
			}
			conc := concurrency
			if conc == 0 {
				conc = g.cfg.Concurrency
			}
			run, err := eng.Run(context.Background(), g2, wf, runtime.Options{
				Concurrency: conc,
				DryRun:      dryRun,
				Params:      pmap,
				Stdout:      cmd.OutOrStdout(),
				Invoker:     pm,
			})
			if err != nil {
				return err
			}

			// Persist state (best-effort).
			if store, serr := state.NewFile(g.cfg.StateDir); serr == nil {
				_ = store.Save(run)
			}

			if g.output == "json" {
				if err := writeJSON(run); err != nil {
					return err
				}
			} else {
				printRunTable(cmd, run)
			}
			if run.Status == model.StatusFailed {
				return withExit(exitRunFailed, fmt.Errorf("run %s failed", run.ID))
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&workflow, "workflow", "w", "", "workflow name (if the file has more than one)")
	cmd.Flags().StringArrayVarP(&params, "param", "p", nil, "set a parameter (key=value); repeatable")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "plan and print actions without executing")
	cmd.Flags().IntVarP(&concurrency, "concurrency", "c", 0, "max concurrent tasks (0 = auto)")
	_ = cmd.RegisterFlagCompletionFunc("workflow", workflowCompletion)
	return cmd
}

func printRunTable(cmd *cobra.Command, run *model.Run) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "\nRun %s  workflow=%s  status=%s  duration=%s\n",
		run.ID, run.Workflow, run.Status, run.Duration().Round(1e6))
	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "TASK\tSTATUS\tATTEMPTS\tEXIT\tINFO")
	for _, t := range run.Tasks {
		info := t.Error
		if info == "" && t.Status == model.StatusSucceeded {
			info = "ok"
		}
		fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%s\n", t.Name, t.Status, t.Attempts, t.ExitCode, truncate(info, 60))
	}
	tw.Flush()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
