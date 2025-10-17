package cli

import (
	"net/http"

	"github.com/spf13/cobra"
)

// Args contains the configuration for a new IncusOS CLI instance.
type Args struct {
	DefaultListFormat string
	SupportsTarget    bool
	DoHTTP            func(remoteName string, req *http.Request) (*http.Response, error)
}

// NewCommand returns a new cobra Command suitable for inclusion by downstreams.
func NewCommand(args *Args) *cobra.Command {
	cmd := cmdAdminOS{
		args: args,
	}

	return cmd.command()
}
