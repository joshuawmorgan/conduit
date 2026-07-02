package cli

import (
	"github.com/spf13/cobra"
)

// flowFileCompletion offers *.flow files as positional-argument completions.
func flowFileCompletion(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return []string{"flow"}, cobra.ShellCompDirectiveFilterFileExt
}

// workflowCompletion completes --workflow values by parsing the referenced file.
func workflowCompletion(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	file, _, err := parseFlowFile(args[0])
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var names []string
	for _, w := range file.Workflows() {
		names = append(names, w.NameStr())
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}
