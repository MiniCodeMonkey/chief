package prd

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// ProgressEntry represents progress notes for a single story from a single session.
type ProgressEntry struct {
	StoryID string
	Date    string
	Content string // raw markdown body (bullet lines)
}

// ProgressPath returns the progress.md path for a given prd.json path.
func ProgressPath(prdPath string) string {
	return filepath.Join(filepath.Dir(prdPath), "progress.md")
}

var storyHeaderRegex = regexp.MustCompile(`^## (\d{4}-\d{2}-\d{2}) - (.+)$`)

// ParseProgress reads and parses a progress.md file.
// Returns a map of story ID -> list of progress entries (one per session/date).
func ParseProgress(path string) (map[string][]ProgressEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	result := make(map[string][]ProgressEntry)
	var current *ProgressEntry
	var lines []string

	flush := func() {
		if current != nil && len(lines) > 0 {
			current.Content = strings.Join(lines, "\n")
			result[current.StoryID] = append(result[current.StoryID], *current)
		}
		current = nil
		lines = nil
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		// Check for section separator
		if strings.TrimSpace(line) == "---" {
			flush()
			continue
		}

		// Check for story header
		if matches := storyHeaderRegex.FindStringSubmatch(line); matches != nil {
			flush()
			current = &ProgressEntry{
				Date:    matches[1],
				StoryID: matches[2],
			}
			continue
		}

		// Collect lines within a story section
		if current != nil {
			lines = append(lines, line)
		}
	}

	// Flush the last entry
	flush()

	if err := scanner.Err(); err != nil {
		return result, err
	}
	return result, nil
}

// ProgressWatcher watches progress.md for changes and sends parsed entries.
type ProgressWatcher struct {
	dir     string
	watcher *fsnotify.Watcher
	events  chan map[string][]ProgressEntry
	done    chan struct{}
	mu      sync.Mutex
	running bool
	closed  bool
}

// NewProgressWatcher creates a new watcher for progress.md in the same
// directory as the given prd.json path.
func NewProgressWatcher(prdPath string) (*ProgressWatcher, error) {
	dir := filepath.Dir(prdPath)
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &ProgressWatcher{
		dir:     dir,
		watcher: fsWatcher,
		events:  make(chan map[string][]ProgressEntry, 10),
		done:    make(chan struct{}),
	}, nil
}

// Start begins watching for progress.md changes.
func (w *ProgressWatcher) Start() error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return nil
	}
	w.running = true
	w.mu.Unlock()

	// Watch the directory so we catch creates and writes
	if err := w.watcher.Add(w.dir); err != nil {
		return err
	}

	go w.processEvents()
	return nil
}

// Stop stops watching and releases resources held by the underlying fsnotify
// watcher. Safe to call before Start and idempotent across repeated calls.
func (w *ProgressWatcher) Stop() {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	w.closed = true
	wasRunning := w.running
	w.running = false
	w.mu.Unlock()

	if wasRunning {
		close(w.done)
	}
	w.watcher.Close()
}

// Events returns the channel for receiving parsed progress data.
func (w *ProgressWatcher) Events() <-chan map[string][]ProgressEntry {
	return w.events
}

// processEvents listens for filesystem events and re-parses progress.md on change.
func (w *ProgressWatcher) processEvents() {
	progressPath := filepath.Join(w.dir, "progress.md")
	for {
		select {
		case <-w.done:
			close(w.events)
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				if filepath.Base(event.Name) == "progress.md" {
					entries, err := ParseProgress(progressPath)
					if err == nil && entries != nil {
						w.events <- entries
					}
				}
			}

		case _, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
		}
	}
}
