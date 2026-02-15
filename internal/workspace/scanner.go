// Package workspace provides workspace directory scanning for discovering
// git repositories and tracking their state.
package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/minicodemonkey/chief/internal/ws"
)

// ScanInterval is how often the scanner re-scans the workspace.
const ScanInterval = 60 * time.Second

// Scanner discovers and tracks git repositories in a workspace directory.
type Scanner struct {
	workspace string
	client    *ws.Client
	interval  time.Duration

	mu       sync.RWMutex
	projects []ws.ProjectSummary
}

// New creates a new Scanner for the given workspace directory.
func New(workspace string, client *ws.Client) *Scanner {
	return &Scanner{
		workspace: workspace,
		client:    client,
		interval:  ScanInterval,
	}
}

// Projects returns the current list of discovered projects.
func (s *Scanner) Projects() []ws.ProjectSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]ws.ProjectSummary, len(s.projects))
	copy(result, s.projects)
	return result
}

// Scan performs a single scan of the workspace directory and returns discovered projects.
func (s *Scanner) Scan() []ws.ProjectSummary {
	entries, err := os.ReadDir(s.workspace)
	if err != nil {
		log.Printf("Warning: failed to read workspace directory: %v", err)
		return nil
	}

	var projects []ws.ProjectSummary
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirPath := filepath.Join(s.workspace, entry.Name())

		// Check for .git/ directory
		gitDir := filepath.Join(dirPath, ".git")
		info, err := os.Stat(gitDir)
		if err != nil {
			if os.IsPermission(err) {
				log.Printf("Warning: permission denied accessing %s, skipping", dirPath)
			}
			continue
		}
		// .git can be a directory (normal repo) or a file (worktree)
		if !info.IsDir() {
			// .git file means it's a worktree link, still a valid git repo
			_ = info
		}

		project := scanProject(dirPath, entry.Name())
		projects = append(projects, project)
	}

	return projects
}

// ScanAndUpdate performs a scan and updates the stored project list.
// Returns true if the project list changed.
func (s *Scanner) ScanAndUpdate() bool {
	newProjects := s.Scan()

	s.mu.Lock()
	defer s.mu.Unlock()

	if projectsEqual(s.projects, newProjects) {
		return false
	}

	s.projects = newProjects
	return true
}

// Run starts the periodic scanning loop. It performs an initial scan immediately,
// then re-scans at the configured interval. It sends project_list updates over
// WebSocket when projects change.
func (s *Scanner) Run(ctx context.Context) {
	// Initial scan
	if s.ScanAndUpdate() {
		s.sendProjectList()
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.ScanAndUpdate() {
				log.Println("Workspace projects changed, sending update")
				s.sendProjectList()
			}
		}
	}
}

// sendProjectList sends a project_list message over WebSocket.
func (s *Scanner) sendProjectList() {
	if s.client == nil {
		return
	}

	s.mu.RLock()
	projects := make([]ws.ProjectSummary, len(s.projects))
	copy(projects, s.projects)
	s.mu.RUnlock()

	msg := ws.NewMessage(ws.TypeProjectList)
	plMsg := ws.ProjectListMessage{
		Type:      msg.Type,
		ID:        msg.ID,
		Timestamp: msg.Timestamp,
		Projects:  projects,
	}

	if err := s.client.Send(plMsg); err != nil {
		log.Printf("Error sending project_list: %v", err)
	}
}

// scanProject gathers information about a single project directory.
func scanProject(dirPath, name string) ws.ProjectSummary {
	project := ws.ProjectSummary{
		Name: name,
		Path: dirPath,
	}

	// Check for .chief/ directory
	chiefDir := filepath.Join(dirPath, ".chief")
	if info, err := os.Stat(chiefDir); err == nil && info.IsDir() {
		project.HasChief = true
	}

	// Get git branch
	branch, err := gitCurrentBranch(dirPath)
	if err == nil {
		project.Branch = branch
	}

	// Get last commit info
	commit, err := gitLastCommit(dirPath)
	if err == nil {
		project.Commit = commit
	}

	// Get PRD list if .chief/ exists
	if project.HasChief {
		project.PRDs = scanPRDs(dirPath)
	}

	return project
}

// gitCurrentBranch returns the current branch for a git repo.
func gitCurrentBranch(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// gitLastCommit returns the last commit info for a git repo.
func gitLastCommit(dir string) (ws.CommitInfo, error) {
	// Use git log with a specific format to get hash, message, author, timestamp
	cmd := exec.Command("git", "log", "-1", "--format=%H%n%s%n%an%n%aI")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return ws.CommitInfo{}, err
	}

	lines := strings.SplitN(strings.TrimSpace(string(output)), "\n", 4)
	if len(lines) < 4 {
		return ws.CommitInfo{}, fmt.Errorf("unexpected git log output")
	}

	return ws.CommitInfo{
		Hash:      lines[0],
		Message:   lines[1],
		Author:    lines[2],
		Timestamp: lines[3],
	}, nil
}

// scanPRDs discovers PRDs in a project's .chief/prds/ directory.
func scanPRDs(dirPath string) []ws.PRDInfo {
	prdsDir := filepath.Join(dirPath, ".chief", "prds")
	entries, err := os.ReadDir(prdsDir)
	if err != nil {
		return nil
	}

	var prds []ws.PRDInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		prdJSON := filepath.Join(prdsDir, entry.Name(), "prd.json")
		data, err := os.ReadFile(prdJSON)
		if err != nil {
			continue
		}

		var prdData struct {
			Project     string `json:"project"`
			UserStories []struct {
				ID     string `json:"id"`
				Passes bool   `json:"passes"`
			} `json:"userStories"`
		}
		if err := json.Unmarshal(data, &prdData); err != nil {
			continue
		}

		total := len(prdData.UserStories)
		passed := 0
		for _, s := range prdData.UserStories {
			if s.Passes {
				passed++
			}
		}

		status := fmt.Sprintf("%d/%d", passed, total)

		prds = append(prds, ws.PRDInfo{
			ID:               entry.Name(),
			Name:             prdData.Project,
			StoryCount:       total,
			CompletionStatus: status,
		})
	}

	return prds
}

// projectsEqual compares two project lists for equality.
func projectsEqual(a, b []ws.ProjectSummary) bool {
	if len(a) != len(b) {
		return false
	}

	// Build maps for comparison
	aMap := make(map[string]ws.ProjectSummary, len(a))
	for _, p := range a {
		aMap[p.Name] = p
	}

	for _, pb := range b {
		pa, ok := aMap[pb.Name]
		if !ok {
			return false
		}
		if pa.Path != pb.Path ||
			pa.HasChief != pb.HasChief ||
			pa.Branch != pb.Branch ||
			pa.Commit.Hash != pb.Commit.Hash ||
			len(pa.PRDs) != len(pb.PRDs) {
			return false
		}
		// Compare PRDs
		for i := range pa.PRDs {
			if pa.PRDs[i].ID != pb.PRDs[i].ID ||
				pa.PRDs[i].StoryCount != pb.PRDs[i].StoryCount ||
				pa.PRDs[i].CompletionStatus != pb.PRDs[i].CompletionStatus {
				return false
			}
		}
	}

	return true
}
