package commands

import (
	"fmt"

	"github.com/minicodemonkey/chief/internal/cmd"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the persistent daemon",
	Long:  "Runs a persistent daemon that discovers projects, connects to Uplink, and handles remote commands.",
	Args:  cobra.NoArgs,
	RunE: func(c *cobra.Command, args []string) error {
		provider, err := resolveProvider()
		if err != nil {
			fmt.Printf("  Warning: agent provider not available, runs disabled: %v\n", err)
		}
		return cmd.RunServe(cmd.ServeOptions{
			Provider: provider,
		})
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
