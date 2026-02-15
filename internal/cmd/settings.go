package cmd

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/minicodemonkey/chief/internal/config"
	"github.com/minicodemonkey/chief/internal/ws"
)

// handleGetSettings handles a get_settings request.
func handleGetSettings(client *ws.Client, finder projectFinder, msg ws.Message) {
	var req ws.GetSettingsMessage
	if err := json.Unmarshal(msg.Raw, &req); err != nil {
		log.Printf("Error parsing get_settings message: %v", err)
		return
	}

	project, found := finder.FindProject(req.Project)
	if !found {
		sendError(client, ws.ErrCodeProjectNotFound,
			fmt.Sprintf("Project %q not found", req.Project), msg.ID)
		return
	}

	cfg, err := config.Load(project.Path)
	if err != nil {
		sendError(client, ws.ErrCodeFilesystemError,
			fmt.Sprintf("Failed to load settings: %v", err), msg.ID)
		return
	}

	envelope := ws.NewMessage(ws.TypeSettings)
	settingsMsg := ws.SettingsMessage{
		Type:          envelope.Type,
		ID:            envelope.ID,
		Timestamp:     envelope.Timestamp,
		Project:       req.Project,
		MaxIterations: cfg.EffectiveMaxIterations(),
		AutoCommit:    cfg.EffectiveAutoCommit(),
		CommitPrefix:  cfg.CommitPrefix,
		ClaudeModel:   cfg.ClaudeModel,
		TestCommand:   cfg.TestCommand,
	}
	if err := client.Send(settingsMsg); err != nil {
		log.Printf("Error sending settings: %v", err)
	}
}

// handleUpdateSettings handles an update_settings request.
func handleUpdateSettings(client *ws.Client, finder projectFinder, msg ws.Message) {
	var req ws.UpdateSettingsMessage
	if err := json.Unmarshal(msg.Raw, &req); err != nil {
		log.Printf("Error parsing update_settings message: %v", err)
		return
	}

	project, found := finder.FindProject(req.Project)
	if !found {
		sendError(client, ws.ErrCodeProjectNotFound,
			fmt.Sprintf("Project %q not found", req.Project), msg.ID)
		return
	}

	cfg, err := config.Load(project.Path)
	if err != nil {
		sendError(client, ws.ErrCodeFilesystemError,
			fmt.Sprintf("Failed to load settings: %v", err), msg.ID)
		return
	}

	// Merge provided fields
	if req.MaxIterations != nil {
		if *req.MaxIterations < 1 {
			sendError(client, ws.ErrCodeFilesystemError,
				"max_iterations must be at least 1", msg.ID)
			return
		}
		cfg.MaxIterations = *req.MaxIterations
	}
	if req.AutoCommit != nil {
		cfg.AutoCommit = req.AutoCommit
	}
	if req.CommitPrefix != nil {
		cfg.CommitPrefix = *req.CommitPrefix
	}
	if req.ClaudeModel != nil {
		cfg.ClaudeModel = *req.ClaudeModel
	}
	if req.TestCommand != nil {
		cfg.TestCommand = *req.TestCommand
	}

	if err := config.Save(project.Path, cfg); err != nil {
		sendError(client, ws.ErrCodeFilesystemError,
			fmt.Sprintf("Failed to save settings: %v", err), msg.ID)
		return
	}

	// Echo back full updated settings
	envelope := ws.NewMessage(ws.TypeSettings)
	settingsMsg := ws.SettingsMessage{
		Type:          envelope.Type,
		ID:            envelope.ID,
		Timestamp:     envelope.Timestamp,
		Project:       req.Project,
		MaxIterations: cfg.EffectiveMaxIterations(),
		AutoCommit:    cfg.EffectiveAutoCommit(),
		CommitPrefix:  cfg.CommitPrefix,
		ClaudeModel:   cfg.ClaudeModel,
		TestCommand:   cfg.TestCommand,
	}
	if err := client.Send(settingsMsg); err != nil {
		log.Printf("Error sending settings: %v", err)
	}
}
