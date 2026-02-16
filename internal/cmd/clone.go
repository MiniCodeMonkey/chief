package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/minicodemonkey/chief/internal/workspace"
	"github.com/minicodemonkey/chief/internal/ws"
)

// handleCloneRepo handles a clone_repo request.
func handleCloneRepo(sender messageSender, scanner *workspace.Scanner, msg ws.Message) {
	var req ws.CloneRepoMessage
	if err := json.Unmarshal(msg.Raw, &req); err != nil {
		log.Printf("Error parsing clone_repo message: %v", err)
		return
	}

	if req.URL == "" {
		sendError(sender, ws.ErrCodeCloneFailed, "URL is required", msg.ID)
		return
	}

	workspaceDir := scanner.WorkspacePath()

	// Determine target directory name
	dirName := req.DirectoryName
	if dirName == "" {
		dirName = inferDirName(req.URL)
	}

	targetDir := filepath.Join(workspaceDir, dirName)

	// Check if target already exists
	if _, err := os.Stat(targetDir); err == nil {
		sendError(sender, ws.ErrCodeCloneFailed,
			fmt.Sprintf("Directory %q already exists in workspace", dirName), msg.ID)
		return
	}

	// Run clone in a goroutine so we don't block the message loop
	go runClone(sender, scanner, req.URL, dirName, workspaceDir)
}

// inferDirName extracts a directory name from a git URL.
func inferDirName(url string) string {
	// Remove trailing .git
	url = strings.TrimSuffix(url, ".git")
	// Remove trailing slash
	url = strings.TrimRight(url, "/")
	// Get the last path component
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		name := parts[len(parts)-1]
		// Also handle ssh-style urls like git@github.com:user/repo
		if idx := strings.LastIndex(name, ":"); idx >= 0 {
			name = name[idx+1:]
		}
		if name != "" {
			return name
		}
	}
	return "cloned-repo"
}

// percentPattern matches git clone progress percentages.
var percentPattern = regexp.MustCompile(`(\d+)%`)

// runClone executes the git clone and streams progress messages.
func runClone(sender messageSender, scanner *workspace.Scanner, url, dirName, workspaceDir string) {
	cmd := exec.Command("git", "clone", "--progress", url, dirName)
	cmd.Dir = workspaceDir

	// Git clone writes progress to stderr
	stderr, err := cmd.StderrPipe()
	if err != nil {
		sendCloneComplete(sender, url, "", false, fmt.Sprintf("Failed to set up clone: %v", err))
		return
	}

	if err := cmd.Start(); err != nil {
		sendCloneComplete(sender, url, "", false, fmt.Sprintf("Failed to start clone: %v", err))
		return
	}

	// Stream progress from stderr
	stderrScanner := bufio.NewScanner(stderr)
	stderrScanner.Split(scanGitProgress)
	for stderrScanner.Scan() {
		line := strings.TrimSpace(stderrScanner.Text())
		if line == "" {
			continue
		}

		percent := 0
		if matches := percentPattern.FindStringSubmatch(line); len(matches) > 1 {
			percent, _ = strconv.Atoi(matches[1])
		}

		sendCloneProgress(sender, url, line, percent)
	}

	if err := cmd.Wait(); err != nil {
		sendCloneComplete(sender, url, "", false, fmt.Sprintf("Clone failed: %v", err))
		return
	}

	// Trigger a rescan so the new project appears immediately
	scanner.ScanAndUpdate()

	sendCloneComplete(sender, url, dirName, true, "")
}

// scanGitProgress is a bufio.SplitFunc that splits on \r or \n,
// since git clone uses \r for progress updates.
func scanGitProgress(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	// Find the first \r or \n
	for i, b := range data {
		if b == '\r' || b == '\n' {
			return i + 1, data[:i], nil
		}
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// sendCloneProgress sends a clone_progress message.
func sendCloneProgress(sender messageSender, url, progressText string, percent int) {
	if sender == nil {
		return
	}
	envelope := ws.NewMessage(ws.TypeCloneProgress)
	msg := ws.CloneProgressMessage{
		Type:         envelope.Type,
		ID:           envelope.ID,
		Timestamp:    envelope.Timestamp,
		URL:          url,
		ProgressText: progressText,
		Percent:      percent,
	}
	if err := sender.Send(msg); err != nil {
		log.Printf("Error sending clone_progress: %v", err)
	}
}

// sendCloneComplete sends a clone_complete message.
func sendCloneComplete(sender messageSender, url, project string, success bool, errMsg string) {
	if sender == nil {
		return
	}
	envelope := ws.NewMessage(ws.TypeCloneComplete)
	msg := ws.CloneCompleteMessage{
		Type:      envelope.Type,
		ID:        envelope.ID,
		Timestamp: envelope.Timestamp,
		URL:       url,
		Success:   success,
		Error:     errMsg,
		Project:   project,
	}
	if err := sender.Send(msg); err != nil {
		log.Printf("Error sending clone_complete: %v", err)
	}
}

// handleCreateProject handles a create_project request.
func handleCreateProject(sender messageSender, scanner *workspace.Scanner, msg ws.Message) {
	var req ws.CreateProjectMessage
	if err := json.Unmarshal(msg.Raw, &req); err != nil {
		log.Printf("Error parsing create_project message: %v", err)
		return
	}

	if req.Name == "" {
		sendError(sender, ws.ErrCodeFilesystemError, "Project name is required", msg.ID)
		return
	}

	workspaceDir := scanner.WorkspacePath()
	projectDir := filepath.Join(workspaceDir, req.Name)

	// Check if directory already exists
	if _, err := os.Stat(projectDir); err == nil {
		sendError(sender, ws.ErrCodeFilesystemError,
			fmt.Sprintf("Directory %q already exists", req.Name), msg.ID)
		return
	}

	// Create the directory
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		sendError(sender, ws.ErrCodeFilesystemError,
			fmt.Sprintf("Failed to create directory: %v", err), msg.ID)
		return
	}

	// Optionally run git init
	if req.GitInit {
		cmd := exec.Command("git", "init", projectDir)
		if out, err := cmd.CombinedOutput(); err != nil {
			sendError(sender, ws.ErrCodeFilesystemError,
				fmt.Sprintf("git init failed: %v\n%s", err, strings.TrimSpace(string(out))), msg.ID)
			return
		}
	}

	// Trigger rescan so new project appears immediately
	scanner.ScanAndUpdate()

	// Send updated project_state if git init was done (it's a discoverable project)
	if req.GitInit {
		project, found := scanner.FindProject(req.Name)
		if found {
			envelope := ws.NewMessage(ws.TypeProjectState)
			psMsg := ws.ProjectStateMessage{
				Type:      envelope.Type,
				ID:        envelope.ID,
				Timestamp: envelope.Timestamp,
				Project:   project,
			}
			if err := sender.Send(psMsg); err != nil {
				log.Printf("Error sending project_state: %v", err)
			}
			return
		}
	}

	// Send a simple project_list update for non-git projects
	envelope := ws.NewMessage(ws.TypeProjectList)
	plMsg := ws.ProjectListMessage{
		Type:      envelope.Type,
		ID:        envelope.ID,
		Timestamp: envelope.Timestamp,
		Projects:  scanner.Projects(),
	}
	if err := sender.Send(plMsg); err != nil {
		log.Printf("Error sending project_list: %v", err)
	}
}
