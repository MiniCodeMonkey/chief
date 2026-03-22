package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/minicodemonkey/chief/internal/auth"
	"github.com/minicodemonkey/chief/internal/config"
)

// LoginOptions contains configuration for the login command.
type LoginOptions struct {
	URL          string        // Uplink server URL (overrides config)
	PollInterval time.Duration // Poll interval (default: 5s)
}

// RunLogin authenticates this machine with the Uplink server using the device flow.
func RunLogin(opts LoginOptions) error {
	// Resolve uplink URL: flag > config > default
	uplinkURL := opts.URL
	if uplinkURL == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("cannot determine working directory: %w", err)
		}
		cfg, err := config.Load(cwd)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		uplinkURL = cfg.Uplink.URL
	}

	// Get a device name from hostname
	deviceName, err := os.Hostname()
	if err != nil {
		deviceName = "unknown"
	}

	// Start the device flow
	flow := auth.NewDeviceFlow(uplinkURL)

	codeResp, err := flow.RequestCode(deviceName)
	if err != nil {
		return fmt.Errorf("failed to start device flow: %w", err)
	}

	// Print instructions for the user
	fmt.Println()
	fmt.Println("To authenticate, visit:")
	fmt.Printf("  %s\n", codeResp.VerifyURL)
	fmt.Println()
	fmt.Printf("Enter code: %s\n", codeResp.UserCode)
	fmt.Println()
	fmt.Println("Waiting for approval...")

	// Poll for approval
	pollInterval := opts.PollInterval
	if pollInterval == 0 {
		pollInterval = 5 * time.Second
	}
	for {
		time.Sleep(pollInterval)

		pollResp, err := flow.PollForToken(codeResp.DeviceCode)
		if err != nil {
			return fmt.Errorf("polling failed: %w", err)
		}

		switch pollResp.Result {
		case auth.PollSuccess:
			// Save credentials
			creds := &auth.Credentials{
				AccessToken:  pollResp.Token.AccessToken,
				RefreshToken: pollResp.Token.RefreshToken,
				Expiry:       time.Now().Add(time.Duration(pollResp.Token.ExpiresIn) * time.Second),
				DeviceName:   deviceName,
				DeviceID:     pollResp.Token.DeviceID,
				UplinkURL:    uplinkURL,
			}

			if err := auth.SaveCredentials(creds); err != nil {
				return fmt.Errorf("failed to save credentials: %w", err)
			}

			fmt.Println()
			fmt.Printf("Logged in successfully! Device ID: %s\n", creds.DeviceID)
			return nil

		case auth.PollDenied:
			return fmt.Errorf("authorization denied")

		case auth.PollPending:
			// Continue polling
		}
	}
}
