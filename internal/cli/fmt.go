package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/conduit-io/conduit/internal/flow/format"
	"github.com/conduit-io/conduit/internal/flow/parser"
)

func newFmtCmd() *cobra.Command {
	var write, check bool
	cmd := &cobra.Command{
		Use:               "fmt <file.flow>",
		Short:             "Format a FlowDSL file canonically",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: flowFileCompletion,
		Annotations:       map[string]string{"ai.sideEffect": "writes-file-with---write", "ai.idempotent": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return withExit(exitUsage, err)
			}
			file, perr := parser.ParseBytes(args[0], data)
			if perr != nil {
				return withExit(exitDSL, perr)
			}
			formatted := format.File(file)

			if check {
				if formatted != string(data) {
					return withExit(exitDSL, fmt.Errorf("%s is not formatted", args[0]))
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s: already formatted\n", args[0])
				return nil
			}
			if write {
				if err := os.WriteFile(args[0], []byte(formatted), 0o644); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "formatted %s\n", args[0])
				return nil
			}
			fmt.Fprint(cmd.OutOrStdout(), formatted)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&write, "write", "w", false, "write result back to the file")
	cmd.Flags().BoolVar(&check, "check", false, "exit non-zero if the file is not formatted")
	return cmd
}
