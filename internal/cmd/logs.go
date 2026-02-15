package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/minicodemonkey/chief/internal/ws"
)

// storyLogger manages per-story log files during Ralph loop runs.
type storyLogger struct {
	mu      sync.Mutex
	logDir  string          // .chief/prds/<id>/logs/
	files   map[string]*os.File // story_id -> open file
}

// newStoryLogger creates a story logger for a given PRD.
// It creates the logs directory and removes any previous log files (V1 simplicity).
func newStoryLogger(prdPath string) (*storyLogger, error) {
	prdDir := filepath.Dir(prdPath)
	logDir := filepath.Join(prdDir, "logs")

	// Remove previous logs (overwrite on new run)
	os.RemoveAll(logDir)

	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating log directory: %w", err)
	}

	return &storyLogger{
		logDir: logDir,
		files:  make(map[string]*os.File),
	}, nil
}

// WriteLog writes a line to the log file for the given story.
func (sl *storyLogger) WriteLog(storyID, line string) {
	if storyID == "" {
		return
	}

	sl.mu.Lock()
	defer sl.mu.Unlock()

	f, ok := sl.files[storyID]
	if !ok {
		var err error
		logPath := filepath.Join(sl.logDir, storyID+".log")
		f, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			log.Printf("Error opening story log file %s: %v", logPath, err)
			return
		}
		sl.files[storyID] = f
	}

	f.WriteString(line + "\n")
}

// Close closes all open log files.
func (sl *storyLogger) Close() {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	for _, f := range sl.files {
		f.Close()
	}
	sl.files = make(map[string]*os.File)
}

// handleGetLogs handles a get_logs request.
func handleGetLogs(client *ws.Client, finder projectFinder, msg ws.Message) {
	var req ws.GetLogsMessage
	if err := json.Unmarshal(msg.Raw, &req); err != nil {
		log.Printf("Error parsing get_logs message: %v", err)
		return
	}

	project, found := finder.FindProject(req.Project)
	if !found {
		sendError(client, ws.ErrCodeProjectNotFound,
			fmt.Sprintf("Project %q not found", req.Project), msg.ID)
		return
	}

	prdDir := filepath.Join(project.Path, ".chief", "prds", req.PRDID)
	if _, err := os.Stat(prdDir); os.IsNotExist(err) {
		sendError(client, ws.ErrCodePRDNotFound,
			fmt.Sprintf("PRD %q not found in project %q", req.PRDID, req.Project), msg.ID)
		return
	}

	logDir := filepath.Join(prdDir, "logs")

	// If story_id is provided, return that specific story's logs
	if req.StoryID != "" {
		lines, err := readLogFile(filepath.Join(logDir, req.StoryID+".log"), req.Lines)
		if err != nil {
			sendError(client, ws.ErrCodeFilesystemError,
				fmt.Sprintf("Failed to read logs for story %q: %v", req.StoryID, err), msg.ID)
			return
		}

		sendLogLines(client, req.Project, req.PRDID, req.StoryID, lines)
		return
	}

	// If story_id is omitted, return the most recent log activity for the PRD
	storyID, lines, err := readMostRecentLog(logDir, req.Lines)
	if err != nil {
		sendError(client, ws.ErrCodeFilesystemError,
			fmt.Sprintf("Failed to read logs: %v", err), msg.ID)
		return
	}

	sendLogLines(client, req.Project, req.PRDID, storyID, lines)
}

// readLogFile reads lines from a log file. If maxLines is 0, reads all lines.
func readLogFile(path string, maxLines int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	// Increase buffer size for long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}

	return lines, scanner.Err()
}

// readMostRecentLog finds the most recently modified log file and reads it.
func readMostRecentLog(logDir string, maxLines int) (string, []string, error) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", []string{}, nil
		}
		return "", nil, err
	}

	// Find the most recently modified .log file
	var mostRecent string
	var mostRecentTime int64

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().UnixNano() > mostRecentTime {
			mostRecentTime = info.ModTime().UnixNano()
			mostRecent = entry.Name()
		}
	}

	if mostRecent == "" {
		return "", []string{}, nil
	}

	storyID := strings.TrimSuffix(mostRecent, ".log")
	lines, err := readLogFile(filepath.Join(logDir, mostRecent), maxLines)
	return storyID, lines, err
}

// sendLogLines sends a log_lines message over WebSocket.
func sendLogLines(client *ws.Client, project, prdID, storyID string, lines []string) {
	if lines == nil {
		lines = []string{}
	}

	envelope := ws.NewMessage(ws.TypeLogLines)
	msg := ws.LogLinesMessage{
		Type:      envelope.Type,
		ID:        envelope.ID,
		Timestamp: envelope.Timestamp,
		Project:   project,
		PRDID:     prdID,
		StoryID:   storyID,
		Lines:     lines,
		Level:     "info",
	}
	if err := client.Send(msg); err != nil {
		log.Printf("Error sending log_lines: %v", err)
	}
}
