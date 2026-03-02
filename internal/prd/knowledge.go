package prd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// CriteriaResult holds the verification result for a single acceptance criterion.
type CriteriaResult struct {
	Criterion string `json:"criterion"`
	Passed    bool   `json:"passed"`
	Evidence  string `json:"evidence"`
}

// CompletedStoryRecord holds structured data about a completed story.
type CompletedStoryRecord struct {
	FilesChanged    []string          `json:"filesChanged"`
	Approach        string            `json:"approach"`
	Learnings       []string          `json:"learnings"`
	CriteriaResults []CriteriaResult  `json:"criteriaResults,omitempty"`
}

// Knowledge represents the structured knowledge base (knowledge.json).
type Knowledge struct {
	Patterns         []string                        `json:"patterns"`
	CompletedStories map[string]CompletedStoryRecord `json:"completedStories"`
}

// KnowledgePath returns the knowledge.json path for a given prd.json path.
func KnowledgePath(prdPath string) string {
	return filepath.Join(filepath.Dir(prdPath), "knowledge.json")
}

// LoadKnowledge reads and parses a knowledge.json file.
// Returns an empty Knowledge struct if the file does not exist.
func LoadKnowledge(path string) (*Knowledge, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Knowledge{
				Patterns:         []string{},
				CompletedStories: map[string]CompletedStoryRecord{},
			}, nil
		}
		return nil, fmt.Errorf("failed to read knowledge file: %w", err)
	}

	var k Knowledge
	if err := json.Unmarshal(data, &k); err != nil {
		return nil, fmt.Errorf("failed to parse knowledge JSON: %w", err)
	}

	// Ensure maps/slices are initialized
	if k.Patterns == nil {
		k.Patterns = []string{}
	}
	if k.CompletedStories == nil {
		k.CompletedStories = map[string]CompletedStoryRecord{}
	}

	return &k, nil
}

// SaveKnowledge writes the knowledge struct to a JSON file.
func SaveKnowledge(path string, k *Knowledge) error {
	data, err := json.MarshalIndent(k, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal knowledge: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write knowledge file: %w", err)
	}

	return nil
}

// KnowledgeWatcher watches knowledge.json for changes and sends parsed data.
type KnowledgeWatcher struct {
	dir     string
	watcher *fsnotify.Watcher
	events  chan *Knowledge
	done    chan struct{}
	mu      sync.Mutex
	running bool
}

// NewKnowledgeWatcher creates a new watcher for knowledge.json in the same
// directory as the given prd.json path.
func NewKnowledgeWatcher(prdPath string) (*KnowledgeWatcher, error) {
	dir := filepath.Dir(prdPath)
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &KnowledgeWatcher{
		dir:     dir,
		watcher: fsWatcher,
		events:  make(chan *Knowledge, 10),
		done:    make(chan struct{}),
	}, nil
}

// Start begins watching for knowledge.json changes.
func (w *KnowledgeWatcher) Start() error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return nil
	}
	w.running = true
	w.mu.Unlock()

	if err := w.watcher.Add(w.dir); err != nil {
		return err
	}

	go w.processEvents()
	return nil
}

// Stop stops watching.
func (w *KnowledgeWatcher) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.running = false
	w.mu.Unlock()

	close(w.done)
	w.watcher.Close()
}

// Events returns the channel for receiving parsed knowledge data.
func (w *KnowledgeWatcher) Events() <-chan *Knowledge {
	return w.events
}

// processEvents listens for filesystem events and re-parses knowledge.json on change.
func (w *KnowledgeWatcher) processEvents() {
	knowledgePath := filepath.Join(w.dir, "knowledge.json")
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
				if filepath.Base(event.Name) == "knowledge.json" {
					k, err := LoadKnowledge(knowledgePath)
					if err == nil && k != nil {
						w.events <- k
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
