// Package meta defines the command metadata model. It is the single source of
// truth projected into help text, docs generation, shell completion and the
// machine-readable catalog consumed by AI agents (`conduit meta dump`).
package meta

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Flag describes a single command flag.
type Flag struct {
	Name       string `json:"name"`
	Shorthand  string `json:"shorthand,omitempty"`
	Type       string `json:"type"`
	Default    string `json:"default,omitempty"`
	Usage      string `json:"usage"`
	Persistent bool   `json:"persistent"`
}

// Command is the metadata for one command node.
type Command struct {
	Name        string            `json:"name"`
	Path        string            `json:"path"`
	Aliases     []string          `json:"aliases,omitempty"`
	Summary     string            `json:"summary"`
	Description string            `json:"description,omitempty"`
	Example     string            `json:"example,omitempty"`
	Args        string            `json:"args,omitempty"`
	Flags       []Flag            `json:"flags,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Hidden      bool              `json:"hidden,omitempty"`
	Subcommands []*Command        `json:"subcommands,omitempty"`
}

// Catalog is the root metadata document.
type Catalog struct {
	Tool    string   `json:"tool"`
	Version string   `json:"version"`
	Root    *Command `json:"root"`
}

// FromCobra builds a metadata catalog by walking a Cobra command tree.
func FromCobra(root *cobra.Command, version string) *Catalog {
	return &Catalog{Tool: root.Name(), Version: version, Root: convert(root)}
}

func convert(c *cobra.Command) *Command {
	m := &Command{
		Name:        c.Name(),
		Path:        c.CommandPath(),
		Aliases:     c.Aliases,
		Summary:     c.Short,
		Description: c.Long,
		Example:     c.Example,
		Args:        c.Use,
		Annotations: c.Annotations,
		Hidden:      c.Hidden,
	}
	c.LocalFlags().VisitAll(func(f *pflag.Flag) {
		m.Flags = append(m.Flags, Flag{
			Name: f.Name, Shorthand: f.Shorthand, Type: f.Value.Type(),
			Default: f.DefValue, Usage: f.Usage,
		})
	})
	c.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		m.Flags = append(m.Flags, Flag{
			Name: f.Name, Shorthand: f.Shorthand, Type: f.Value.Type(),
			Default: f.DefValue, Usage: f.Usage, Persistent: true,
		})
	})
	for _, sub := range c.Commands() {
		if sub.Name() == "help" {
			continue
		}
		m.Subcommands = append(m.Subcommands, convert(sub))
	}
	return m
}
