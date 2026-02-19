package cmd

import (
	"fmt"
	"log"

	"github.com/minicodemonkey/chief/internal/update"
)

// UpdateOptions holds configuration for the update command.
type UpdateOptions struct {
	Version     string // Current version (from build ldflags)
	ReleasesURL string // Override GitHub API URL (for testing)
}

// RunUpdate downloads and installs the latest version of Chief.
func RunUpdate(opts UpdateOptions) error {
	fmt.Println("Checking for updates...")

	// First check if an update is available
	result, err := update.CheckForUpdate(opts.Version, update.Options{
		ReleasesURL: opts.ReleasesURL,
	})
	if err != nil {
		return fmt.Errorf("checking for updates: %w", err)
	}

	if !result.UpdateAvailable {
		fmt.Printf("Already on latest version (v%s).\n", result.CurrentVersion)
		return nil
	}

	fmt.Printf("Downloading v%s (you have v%s)...\n", result.LatestVersion, result.CurrentVersion)

	// Perform the update
	if _, err := update.PerformUpdate(opts.Version, update.Options{
		ReleasesURL: opts.ReleasesURL,
	}); err != nil {
		return err
	}

	fmt.Printf("Updated to v%s. Restart 'chief serve' to apply.\n", result.LatestVersion)
	return nil
}

// CheckVersionOnStartup performs a non-blocking version check and prints a message if an update is available.
// This is called on startup for interactive CLI commands.
func CheckVersionOnStartup(version string) {
	go func() {
		result, err := update.CheckForUpdate(version, update.Options{})
		if err != nil {
			// Silently fail — version check is best-effort
			return
		}
		if result.UpdateAvailable {
			fmt.Printf("Chief v%s available (you have v%s). Run 'chief update' to upgrade.\n",
				result.LatestVersion, result.CurrentVersion)
		}
	}()
}

// CheckVersionForServe performs a version check and returns the result for use by the serve command.
func CheckVersionForServe(version, releasesURL string) *update.CheckResult {
	result, err := update.CheckForUpdate(version, update.Options{
		ReleasesURL: releasesURL,
	})
	if err != nil {
		log.Printf("Version check failed: %v", err)
		return nil
	}
	return result
}
