package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/minicodemonkey/chief/internal/ws"
)

// handleGetDiffs handles a get_diffs request from the browser.
// Unlike get_diff, this does not require prd_id and returns parsed per-file diffs.
func handleGetDiffs(sender messageSender, finder projectFinder, msg ws.Message) {
	var req ws.GetDiffsMessage
	if err := json.Unmarshal(msg.Raw, &req); err != nil {
		log.Printf("Error parsing get_diffs message: %v", err)
		return
	}

	project, found := finder.FindProject(req.Project)
	if !found {
		sendError(sender, ws.ErrCodeProjectNotFound,
			fmt.Sprintf("Project %q not found", req.Project), msg.ID)
		return
	}

	diffText, _, err := getStoryDiff(project.Path, req.StoryID)
	if err != nil {
		sendError(sender, ws.ErrCodeFilesystemError,
			fmt.Sprintf("Failed to get diff for story %q: %v", req.StoryID, err), msg.ID)
		return
	}

	files := parseDiffFiles(diffText)

	resp := ws.DiffsResponseMessage{
		Type: ws.TypeDiffsResponse,
		Payload: ws.DiffsResponsePayload{
			Project: req.Project,
			StoryID: req.StoryID,
			Files:   files,
		},
	}
	if err := sender.Send(resp); err != nil {
		log.Printf("Error sending diffs_response: %v", err)
	}
}

// parseDiffFiles splits a unified diff into per-file details.
func parseDiffFiles(diffText string) []ws.DiffFileDetail {
	if diffText == "" {
		return []ws.DiffFileDetail{}
	}

	// Split on "diff --git" boundaries
	chunks := strings.Split(diffText, "diff --git ")
	var files []ws.DiffFileDetail

	for _, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}

		// Extract filename from first line: "a/path b/path"
		firstLine := chunk
		if idx := strings.IndexByte(chunk, '\n'); idx != -1 {
			firstLine = chunk[:idx]
		}

		filename := ""
		if parts := strings.SplitN(firstLine, " b/", 2); len(parts) == 2 {
			filename = parts[1]
		}

		// Count additions and deletions
		additions, deletions := 0, 0
		for _, line := range strings.Split(chunk, "\n") {
			if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
				additions++
			} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
				deletions++
			}
		}

		files = append(files, ws.DiffFileDetail{
			Filename:  filename,
			Additions: additions,
			Deletions: deletions,
			Patch:     "diff --git " + chunk,
		})
	}

	if files == nil {
		files = []ws.DiffFileDetail{}
	}
	return files
}

// handleGetDiff handles a get_diff request.
func handleGetDiff(sender messageSender, finder projectFinder, msg ws.Message) {
	var req ws.GetDiffMessage
	if err := json.Unmarshal(msg.Raw, &req); err != nil {
		log.Printf("Error parsing get_diff message: %v", err)
		return
	}

	project, found := finder.FindProject(req.Project)
	if !found {
		sendError(sender, ws.ErrCodeProjectNotFound,
			fmt.Sprintf("Project %q not found", req.Project), msg.ID)
		return
	}

	prdDir := filepath.Join(project.Path, ".chief", "prds", req.PRDID)
	if _, err := os.Stat(prdDir); os.IsNotExist(err) {
		sendError(sender, ws.ErrCodePRDNotFound,
			fmt.Sprintf("PRD %q not found in project %q", req.PRDID, req.Project), msg.ID)
		return
	}

	diffText, files, err := getStoryDiff(project.Path, req.StoryID)
	if err != nil {
		sendError(sender, ws.ErrCodeFilesystemError,
			fmt.Sprintf("Failed to get diff for story %q: %v", req.StoryID, err), msg.ID)
		return
	}

	sendDiffMessage(sender, req.Project, req.PRDID, req.StoryID, files, diffText)
}

// getStoryDiff returns the diff and list of changed files for a story's commit(s).
// It finds commits matching the story ID pattern "feat: <story-id> -" in the commit message.
func getStoryDiff(repoDir, storyID string) (string, []string, error) {
	// Find commit hash(es) for this story by searching commit messages
	commitHash, err := findStoryCommit(repoDir, storyID)
	if err != nil {
		return "", nil, err
	}
	if commitHash == "" {
		return "", nil, fmt.Errorf("no commit found for story %s", storyID)
	}

	// Get the unified diff for the commit
	diffText, err := getCommitDiff(repoDir, commitHash)
	if err != nil {
		return "", nil, fmt.Errorf("getting diff: %w", err)
	}

	// Get the list of changed files
	files, err := getCommitFiles(repoDir, commitHash)
	if err != nil {
		return "", nil, fmt.Errorf("getting changed files: %w", err)
	}

	return diffText, files, nil
}

// findStoryCommit finds the most recent commit hash matching a story ID.
// It searches for commits with messages matching "feat: <storyID> -" or
// containing the story ID.
func findStoryCommit(repoDir, storyID string) (string, error) {
	// Search for commits with messages containing the story ID
	cmd := exec.Command("git", "log", "--format=%H", "--grep", storyID, "-1")
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("searching git log: %w", err)
	}

	hash := strings.TrimSpace(string(output))
	return hash, nil
}

// getCommitDiff returns the unified diff for a specific commit.
func getCommitDiff(repoDir, commitHash string) (string, error) {
	cmd := exec.Command("git", "show", "--format=", "--patch", commitHash)
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// getCommitFiles returns the list of files changed in a specific commit.
func getCommitFiles(repoDir, commitHash string) ([]string, error) {
	cmd := exec.Command("git", "show", "--format=", "--name-only", commitHash)
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	raw := strings.TrimSpace(string(output))
	if raw == "" {
		return []string{}, nil
	}

	files := strings.Split(raw, "\n")
	return files, nil
}

// sendDiffMessage sends a diff message.
func sendDiffMessage(sender messageSender, project, prdID, storyID string, files []string, diffText string) {
	if sender == nil {
		return
	}

	if files == nil {
		files = []string{}
	}

	envelope := ws.NewMessage(ws.TypeDiff)
	msg := ws.DiffMessage{
		Type:      envelope.Type,
		ID:        envelope.ID,
		Timestamp: envelope.Timestamp,
		Project:   project,
		PRDID:     prdID,
		StoryID:   storyID,
		Files:     files,
		DiffText:  diffText,
	}
	if err := sender.Send(msg); err != nil {
		log.Printf("Error sending diff: %v", err)
	}
}
