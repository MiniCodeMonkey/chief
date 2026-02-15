package workspace

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/minicodemonkey/chief/internal/ws"
)

// InactivityTimeout is how long a project remains active without interaction.
const InactivityTimeout = 10 * time.Minute

// Watcher watches filesystem changes in the workspace using fsnotify.
// It watches the workspace root for new/removed projects and sets up deep
// watchers (.chief/, .git/HEAD) only for active projects.
type Watcher struct {
	workspace string
	scanner   *Scanner
	client    *ws.Client
	watcher   *fsnotify.Watcher

	mu              sync.Mutex
	activeProjects  map[string]*activeProject // project name → state
	inactiveTimeout time.Duration
}

// activeProject tracks an actively-watched project.
type activeProject struct {
	name       string
	path       string
	lastActive time.Time
	watching   bool // whether deep watchers are set up
}

// NewWatcher creates a new Watcher for the given workspace directory.
func NewWatcher(workspace string, scanner *Scanner, client *ws.Client) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &Watcher{
		workspace:       workspace,
		scanner:         scanner,
		client:          client,
		watcher:         fsw,
		activeProjects:  make(map[string]*activeProject),
		inactiveTimeout: InactivityTimeout,
	}, nil
}

// Activate marks a project as active, setting up deep watchers if not already watching.
// Call this when a run is started, a Claude session is opened, or get_project is requested.
func (w *Watcher) Activate(projectName string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	ap, exists := w.activeProjects[projectName]
	if exists {
		ap.lastActive = time.Now()
		log.Printf("[debug] Project %q activity refreshed", projectName)
		return
	}

	// Find the project path from the scanner
	projectPath := ""
	for _, p := range w.scanner.Projects() {
		if p.Name == projectName {
			projectPath = p.Path
			break
		}
	}
	if projectPath == "" {
		log.Printf("[debug] Project %q not found in scanner, cannot activate watcher", projectName)
		return
	}

	ap = &activeProject{
		name:       projectName,
		path:       projectPath,
		lastActive: time.Now(),
	}
	w.activeProjects[projectName] = ap

	w.setupDeepWatchers(ap)
}

// setupDeepWatchers adds fsnotify watches for .chief/ and .git/HEAD for a project.
func (w *Watcher) setupDeepWatchers(ap *activeProject) {
	if ap.watching {
		return
	}

	chiefDir := filepath.Join(ap.path, ".chief")
	prdsDir := filepath.Join(ap.path, ".chief", "prds")
	gitDir := filepath.Join(ap.path, ".git")

	// Watch .chief/ directory
	if err := w.watcher.Add(chiefDir); err != nil {
		log.Printf("[debug] Could not watch %s: %v", chiefDir, err)
	} else {
		log.Printf("[debug] Watching %s for project %q", chiefDir, ap.name)
	}

	// Watch .chief/prds/ directory and each PRD subdirectory
	// fsnotify does not recurse, so we must add each subdirectory explicitly
	if err := w.watcher.Add(prdsDir); err != nil {
		log.Printf("[debug] Could not watch %s: %v", prdsDir, err)
	} else {
		log.Printf("[debug] Watching %s for project %q", prdsDir, ap.name)
		// Also watch each PRD subdirectory (e.g., .chief/prds/feature/)
		entries, err := os.ReadDir(prdsDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					subDir := filepath.Join(prdsDir, entry.Name())
					if err := w.watcher.Add(subDir); err != nil {
						log.Printf("[debug] Could not watch %s: %v", subDir, err)
					} else {
						log.Printf("[debug] Watching %s for project %q", subDir, ap.name)
					}
				}
			}
		}
	}

	// Watch .git/ directory (for HEAD changes = branch switches)
	if err := w.watcher.Add(gitDir); err != nil {
		log.Printf("[debug] Could not watch %s: %v", gitDir, err)
	} else {
		log.Printf("[debug] Watching .git/ for project %q", ap.name)
	}

	ap.watching = true
}

// removeDeepWatchers removes fsnotify watches for a project.
func (w *Watcher) removeDeepWatchers(ap *activeProject) {
	if !ap.watching {
		return
	}

	chiefDir := filepath.Join(ap.path, ".chief")
	prdsDir := filepath.Join(ap.path, ".chief", "prds")
	gitDir := filepath.Join(ap.path, ".git")

	_ = w.watcher.Remove(chiefDir)
	_ = w.watcher.Remove(prdsDir)
	_ = w.watcher.Remove(gitDir)

	ap.watching = false
	log.Printf("[debug] Removed watchers for project %q", ap.name)
}

// Run starts the watcher event loop. It watches the workspace root for project
// additions/removals and handles deep watcher events for active projects.
func (w *Watcher) Run(ctx context.Context) error {
	// Watch workspace root for new/removed project directories
	if err := w.watcher.Add(w.workspace); err != nil {
		return err
	}
	log.Printf("[debug] Watching workspace root: %s", w.workspace)

	// Start inactivity checker
	inactivityTicker := time.NewTicker(1 * time.Minute)
	defer inactivityTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return w.watcher.Close()

		case event, ok := <-w.watcher.Events:
			if !ok {
				return nil
			}
			w.handleEvent(event)

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return nil
			}
			log.Printf("Watcher error: %v", err)

		case <-inactivityTicker.C:
			w.cleanupInactive()
		}
	}
}

// handleEvent processes a single fsnotify event.
func (w *Watcher) handleEvent(event fsnotify.Event) {
	path := event.Name

	// Check if this is a workspace-root-level event (new/removed project)
	if filepath.Dir(path) == w.workspace {
		if event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
			log.Printf("[debug] Workspace root change detected: %s (%s)", filepath.Base(path), event.Op)
			// Trigger a re-scan to detect new/removed projects
			if w.scanner.ScanAndUpdate() {
				w.scanner.sendProjectList()
			}
		}
		return
	}

	// For deep watcher events, find which project this belongs to
	projectName := w.projectForPath(path)
	if projectName == "" {
		return
	}

	// Determine what changed
	rel, err := filepath.Rel(w.workspace, path)
	if err != nil {
		return
	}
	parts := strings.SplitN(rel, string(filepath.Separator), 3)
	if len(parts) < 2 {
		return
	}

	subPath := strings.Join(parts[1:], string(filepath.Separator))

	switch {
	case strings.HasPrefix(subPath, filepath.Join(".chief", "prds")):
		log.Printf("[debug] PRD change detected in project %q: %s", projectName, subPath)
		w.sendProjectState(projectName)

	case subPath == filepath.Join(".git", "HEAD"):
		log.Printf("[debug] Git HEAD change detected in project %q", projectName)
		w.sendProjectState(projectName)

	case strings.HasPrefix(subPath, ".chief"):
		log.Printf("[debug] Chief config change in project %q: %s", projectName, subPath)
		w.sendProjectState(projectName)

	case strings.HasPrefix(subPath, ".git"):
		// Other .git changes (like refs) — check if HEAD changed
		if strings.Contains(subPath, "HEAD") {
			log.Printf("[debug] Git ref change in project %q: %s", projectName, subPath)
			w.sendProjectState(projectName)
		}
	}
}

// projectForPath finds which active project a file path belongs to.
func (w *Watcher) projectForPath(path string) string {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, ap := range w.activeProjects {
		if strings.HasPrefix(path, ap.path+string(filepath.Separator)) || path == ap.path {
			return ap.name
		}
	}
	return ""
}

// sendProjectState re-scans a single project and sends a project_state update.
func (w *Watcher) sendProjectState(projectName string) {
	if w.client == nil {
		return
	}

	// Re-scan the project to get updated state
	w.scanner.ScanAndUpdate()

	// Find the project in the scanner's list
	for _, p := range w.scanner.Projects() {
		if p.Name == projectName {
			msg := ws.NewMessage(ws.TypeProjectState)
			psMsg := ws.ProjectStateMessage{
				Type:      msg.Type,
				ID:        msg.ID,
				Timestamp: msg.Timestamp,
				Project:   p,
			}
			if err := w.client.Send(psMsg); err != nil {
				log.Printf("Error sending project_state for %q: %v", projectName, err)
			}
			return
		}
	}
}

// cleanupInactive removes watchers for projects that have been inactive.
func (w *Watcher) cleanupInactive() {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	for name, ap := range w.activeProjects {
		if now.Sub(ap.lastActive) > w.inactiveTimeout {
			log.Printf("[debug] Project %q inactive for %s, removing watchers", name, w.inactiveTimeout)
			w.removeDeepWatchers(ap)
			delete(w.activeProjects, name)
		}
	}
}

// Close closes the underlying fsnotify watcher.
func (w *Watcher) Close() error {
	return w.watcher.Close()
}
