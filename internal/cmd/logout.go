package cmd

import (
	"errors"
	"fmt"

	"github.com/minicodemonkey/chief/internal/auth"
)

// LogoutOptions contains configuration for the logout command.
type LogoutOptions struct {
	BaseURL string // Override base URL (for testing)
}

// RunLogout performs the device logout flow.
func RunLogout(opts LogoutOptions) error {
	// Load credentials to get device name and access token
	creds, err := auth.LoadCredentials()
	if err != nil {
		if errors.Is(err, auth.ErrNotLoggedIn) {
			fmt.Println("Not logged in.")
			return nil
		}
		return err
	}

	deviceName := creds.DeviceName

	// Call revocation endpoint
	if err := auth.RevokeDevice(creds.AccessToken, opts.BaseURL); err != nil {
		fmt.Printf("Warning: could not deauthorize device server-side: %v\n", err)
		fmt.Println("Local credentials will still be removed.")
	}

	// Delete local credentials
	if err := auth.DeleteCredentials(); err != nil {
		return fmt.Errorf("removing local credentials: %w", err)
	}

	fmt.Printf("Logged out. Device %q has been deauthorized.\n", deviceName)
	return nil
}
