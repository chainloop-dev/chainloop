package cmd

import (
	"github.com/spf13/cobra"
)

// newTraceCmd creates the trace parent command with all subcommands.
func newTraceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trace",
		Short: "AI coding session tracing and attestation",
	}

	cmd.AddCommand(
		newTraceInitCmd(),
		newTraceRunCmd(),
		newTraceUninstallCmd(),
		newTraceHookCmd(),
	)

	return cmd
}
