package commands

import (
	"github.com/minicodemonkey/chief/internal/cmd"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the persistent daemon",
	Long:  "Runs a persistent daemon that discovers projects, connects to Uplink, and handles remote commands.",
	Args:  cobra.NoArgs,
	RunE: func(c *cobra.Command, args []string) error {
		return cmd.RunServe(cmd.ServeOptions{})
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
