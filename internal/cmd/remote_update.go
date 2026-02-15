package cmd

import (
	"fmt"
	"log"
	"strings"

	"github.com/minicodemonkey/chief/internal/update"
	"github.com/minicodemonkey/chief/internal/ws"
)

// handleTriggerUpdate handles a trigger_update request from the web app.
// It downloads and installs the latest binary, sends confirmation over WebSocket,
// and returns true if the process should exit (so systemd Restart=always picks up the new binary).
func handleTriggerUpdate(client *ws.Client, msg ws.Message, version, releasesURL string) bool {
	log.Println("Received trigger_update request")

	// Check for update
	result, err := update.CheckForUpdate(version, update.Options{
		ReleasesURL: releasesURL,
	})
	if err != nil {
		sendError(client, ws.ErrCodeUpdateFailed,
			fmt.Sprintf("checking for updates: %v", err), msg.ID)
		return false
	}

	if !result.UpdateAvailable {
		// Already on latest — send informational message (not an error)
		envelope := ws.NewMessage(ws.TypeUpdateAvailable)
		infoMsg := ws.UpdateAvailableMessage{
			Type:           envelope.Type,
			ID:             envelope.ID,
			Timestamp:      envelope.Timestamp,
			CurrentVersion: result.CurrentVersion,
			LatestVersion:  result.LatestVersion,
		}
		if err := client.Send(infoMsg); err != nil {
			log.Printf("Error sending update_available: %v", err)
		}
		log.Printf("Already on latest version (v%s)", result.CurrentVersion)
		return false
	}

	// Perform the update
	log.Printf("Downloading v%s (current: v%s)...", result.LatestVersion, result.CurrentVersion)
	_, err = update.PerformUpdate(version, update.Options{
		ReleasesURL: releasesURL,
	})
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "Permission denied") {
			sendError(client, ws.ErrCodeUpdateFailed,
				"Permission denied. The chief binary is not writable. Ensure the service user has write permissions to the binary path.", msg.ID)
		} else {
			sendError(client, ws.ErrCodeUpdateFailed,
				fmt.Sprintf("update failed: %v", err), msg.ID)
		}
		return false
	}

	// Send confirmation before exiting
	log.Printf("Updated to v%s. Exiting for restart.", result.LatestVersion)
	envelope := ws.NewMessage(ws.TypeUpdateAvailable)
	confirmMsg := ws.UpdateAvailableMessage{
		Type:           envelope.Type,
		ID:             envelope.ID,
		Timestamp:      envelope.Timestamp,
		CurrentVersion: result.CurrentVersion,
		LatestVersion:  result.LatestVersion,
	}
	if err := client.Send(confirmMsg); err != nil {
		log.Printf("Error sending update confirmation: %v", err)
	}

	return true
}
