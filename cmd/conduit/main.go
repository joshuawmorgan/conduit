// Command conduit is the Conduit CLI & automation platform entry point.
package main

import (
	"os"

	"github.com/conduit-io/conduit/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
