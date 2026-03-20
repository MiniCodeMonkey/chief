package prd

import (
	"errors"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// WatcherEvent represents a file change event.
type WatcherEvent struct {
	PRD   *PRD
	Error error
}

// Watcher watches a prd.json file for changes and sends events.
type Watcher struct {
	path     string
	watcher  *fsnotify.Watcher
	events   chan WatcherEvent
	done     chan struct{}
	mu       sync.Mutex
	running  bool
	lastPRD  *PRD
}

// NewWatcher creates a new Watcher for the given PRD file path.
func NewWatcher(path string) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		path:    path,
		watcher: fsWatcher,
		events:  make(chan WatcherEvent, 10),
		done:    make(chan struct{}),
	}

	return w, nil
}

// Start begins watching the PRD file for changes.
func (w *Watcher) Start() error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return errors.New("watcher already running")
	}
	w.running = true
	w.mu.Unlock()

	// Load the initial PRD
	prd, err := LoadPRD(w.path)
	if err != nil {
		// Don't fail startup, just send error event
		w.events <- WatcherEvent{Error: err}
	} else {
		w.lastPRD = prd
	}

	// Add the file to the watcher
	if err := w.watcher.Add(w.path); err != nil {
		return err
	}

	// Start the event processing goroutine
	go w.processEvents()

	return nil
}

// Stop stops watching the PRD file.
func (w *Watcher) Stop() {
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

// Events returns the channel for receiving PRD change events.
func (w *Watcher) Events() <-chan WatcherEvent {
	return w.events
}

// processEvents processes filesystem events and loads the PRD when it changes.
func (w *Watcher) processEvents() {
	for {
		select {
		case <-w.done:
			close(w.events)
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// Only react to write and create events
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				w.handleFileChange()
			}

			// Handle file removal - try to re-watch
			if event.Op&fsnotify.Remove != 0 {
				w.events <- WatcherEvent{Error: errors.New("prd.json was removed")}
				// Try to re-add the watch (file might be re-created)
				_ = w.watcher.Add(w.path)
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			w.events <- WatcherEvent{Error: err}
		}
	}
}

// handleFileChange loads the PRD and sends an event if it changed.
func (w *Watcher) handleFileChange() {
	prd, err := LoadPRD(w.path)
	if err != nil {
		w.events <- WatcherEvent{Error: err}
		return
	}

	// Check if any story status changed
	if w.hasStatusChanged(prd) {
		w.lastPRD = prd
		w.events <- WatcherEvent{PRD: prd}
	}
}

// hasStatusChanged returns true if any story's inProgress or passes field changed.
func (w *Watcher) hasStatusChanged(newPRD *PRD) bool {
	if w.lastPRD == nil {
		return true
	}

	// If number of stories changed, treat as changed
	if len(w.lastPRD.UserStories) != len(newPRD.UserStories) {
		return true
	}

	// Build a map of old stories by ID for comparison
	oldStories := make(map[string]*UserStory)
	for i := range w.lastPRD.UserStories {
		oldStories[w.lastPRD.UserStories[i].ID] = &w.lastPRD.UserStories[i]
	}

	// Check each new story for status changes
	for i := range newPRD.UserStories {
		newStory := &newPRD.UserStories[i]
		oldStory, exists := oldStories[newStory.ID]

		if !exists {
			// New story added
			return true
		}

		// Check if status fields changed
		if oldStory.Passes != newStory.Passes || oldStory.InProgress != newStory.InProgress {
			return true
		}
	}

	return false
}
