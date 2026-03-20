package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/minicodemonkey/chief/internal/ws"
)

// handleGetPRDs handles a get_prds request.
func handleGetPRDs(sender messageSender, finder projectFinder, msg ws.Message) {
	var req ws.GetPRDsMessage
	if err := json.Unmarshal(msg.Raw, &req); err != nil {
		log.Printf("Error parsing get_prds message: %v", err)
		return
	}

	project, found := finder.FindProject(req.Project)
	if !found {
		sendError(sender, ws.ErrCodeProjectNotFound,
			fmt.Sprintf("Project %q not found", req.Project), msg.ID)
		return
	}

	items := make([]ws.PRDItem, 0, len(project.PRDs))
	for _, prd := range project.PRDs {
		items = append(items, ws.PRDItem{
			ID:         prd.ID,
			Name:       prd.Name,
			StoryCount: prd.StoryCount,
			Status:     mapCompletionStatus(prd.CompletionStatus),
		})
	}

	resp := ws.PRDsResponseMessage{
		Type: ws.TypePRDsResponse,
		Payload: ws.PRDsResponsePayload{
			Project: req.Project,
			PRDs:    items,
		},
	}
	if err := sender.Send(resp); err != nil {
		log.Printf("Error sending prds_response: %v", err)
	}
}

// mapCompletionStatus converts a "passed/total" completion status to a
// browser-friendly status string: "draft", "active", or "done".
func mapCompletionStatus(status string) string {
	parts := strings.SplitN(status, "/", 2)
	if len(parts) != 2 {
		return "draft"
	}
	passed, total := parts[0], parts[1]
	if total == "0" {
		return "draft"
	}
	if passed == total {
		return "done"
	}
	return "active"
}
