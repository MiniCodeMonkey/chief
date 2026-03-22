package commands

import (
	"github.com/minicodemonkey/chief/internal/cmd"
	"github.com/spf13/cobra"
)

var flagLoginURL string

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with the Uplink server",
	Long:  "Start the device authorization flow to authenticate this machine with the Uplink server.",
	Args:  cobra.NoArgs,
	RunE: func(c *cobra.Command, args []string) error {
		return cmd.RunLogin(cmd.LoginOptions{
			URL: flagLoginURL,
		})
	},
}

func init() {
	loginCmd.Flags().StringVar(&flagLoginURL, "url", "", "Uplink server URL (overrides config)")
	rootCmd.AddCommand(loginCmd)
}
