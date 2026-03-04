// Package loop provides the core agent loop that orchestrates Gemini Code
// to implement user stories. It includes the main Loop struct for single
// PRD execution, Manager for parallel PRD execution, and Parser for
// processing Gemini's stream-json output.
package loop

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lvcoi/melliza/embed"
	"github.com/lvcoi/melliza/internal/gemini"
	"github.com/lvcoi/melliza/internal/prd"
)

// RetryConfig configures automatic retry behavior on Gemini crashes.
type RetryConfig struct {
	MaxRetries  int             // Maximum number of retry attempts (default: 3)
	RetryDelays []time.Duration // Delays between retries (default: 0s, 5s, 15s)
	Enabled     bool            // Whether retry is enabled (default: true)
}

// DefaultWatchdogTimeout is the default duration of silence before the watchdog kills a hung process.
const DefaultWatchdogTimeout = 5 * time.Minute

// DefaultRetryConfig returns the default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:  3,
		RetryDelays: []time.Duration{0, 5 * time.Second, 15 * time.Second},
		Enabled:     true,
	}
}

// Loop manages the core agent loop that invokes Gemini repeatedly until all stories are complete.
type Loop struct {
	prdPath         string
	workDir         string
	prompt          string
	buildPrompt     func() (string, error) // optional: rebuild prompt each iteration
	maxIter         int
	iteration       int
	events          chan Event
	geminiCmd       *exec.Cmd
	logFile         *os.File
	mu              sync.Mutex
	stopped         bool
	paused          bool
	retryConfig     RetryConfig
	lastOutputTime  time.Time
	watchdogTimeout time.Duration
}

// NewLoop creates a new Loop instance.
func NewLoop(prdPath, prompt string, maxIter int) *Loop {
	return &Loop{
		prdPath:         prdPath,
		prompt:          prompt,
		maxIter:         maxIter,
		events:          make(chan Event, 100),
		retryConfig:     DefaultRetryConfig(),
		watchdogTimeout: DefaultWatchdogTimeout,
	}
}

// NewLoopWithWorkDir creates a new Loop instance with a configurable working directory.
// When workDir is empty, defaults to the project root for backward compatibility.
func NewLoopWithWorkDir(prdPath, workDir string, prompt string, maxIter int) *Loop {
	return &Loop{
		prdPath:         prdPath,
		workDir:         workDir,
		prompt:          prompt,
		maxIter:         maxIter,
		events:          make(chan Event, 100),
		retryConfig:     DefaultRetryConfig(),
		watchdogTimeout: DefaultWatchdogTimeout,
	}
}

// NewLoopWithEmbeddedPrompt creates a new Loop instance using the embedded agent prompt.
// The prompt is rebuilt on each iteration to inline the current story context.
func NewLoopWithEmbeddedPrompt(prdPath string, maxIter int) *Loop {
	l := NewLoop(prdPath, "", maxIter)
	l.buildPrompt = promptBuilderForPRD(prdPath)
	return l
}

// promptBuilderForPRD returns a function that loads the PRD and builds a prompt
// with the next story inlined. This is called before each iteration so that
// newly completed stories are skipped.
func promptBuilderForPRD(prdPath string) func() (string, error) {
	return func() (string, error) {
		p, err := prd.LoadPRD(prdPath)
		if err != nil {
			return "", fmt.Errorf("failed to load PRD for prompt: %w", err)
		}

		story := p.NextStory()
		if story == nil {
			return "", fmt.Errorf("all stories are complete")
		}

		storyCtx := p.NextStoryContext()

		return embed.GetPrompt(prdPath, prd.ProgressPath(prdPath), *storyCtx, story.ID, story.Title), nil
	}
}

// Events returns the channel for receiving events from the loop.
func (l *Loop) Events() <-chan Event {
	return l.events
}

// Iteration returns the current iteration number.
func (l *Loop) Iteration() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.iteration
}

// Run executes the agent loop until completion or max iterations.
func (l *Loop) Run(ctx context.Context) error {
	// Open log file in PRD directory
	prdDir := filepath.Dir(l.prdPath)
	logPath := filepath.Join(prdDir, "gemini.log")
	var err error
	l.logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer l.logFile.Close()
	defer close(l.events)

	for {
		l.mu.Lock()
		if l.stopped {
			l.mu.Unlock()
			return nil
		}
		if l.paused {
			l.mu.Unlock()
			return nil
		}
		l.iteration++
		currentIter := l.iteration
		l.mu.Unlock()

		// Check if max iterations reached
		if currentIter > l.maxIter {
			l.events <- Event{
				Type:      EventMaxIterationsReached,
				Iteration: currentIter - 1,
			}
			return nil
		}

		// Rebuild prompt if builder is set (inlines the current story each iteration)
		if l.buildPrompt != nil {
			prompt, err := l.buildPrompt()
			if err != nil {
				l.events <- Event{
					Type:      EventComplete,
					Iteration: currentIter,
				}
				return nil
			}
			l.mu.Lock()
			l.prompt = prompt
			l.mu.Unlock()
		}

		// Send iteration start event
		l.events <- Event{
			Type:      EventIterationStart,
			Iteration: currentIter,
		}

		// Run a single iteration with retry logic
		if err := l.runIterationWithRetry(ctx); err != nil {
			l.events <- Event{
				Type: EventError,
				Err:  err,
			}
			return err
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check prd.json for completion
		p, err := prd.LoadPRD(l.prdPath)
		if err != nil {
			l.events <- Event{
				Type: EventError,
				Err:  fmt.Errorf("failed to load PRD: %w", err),
			}
			return err
		}

		if p.AllComplete() {
			l.events <- Event{
				Type:      EventComplete,
				Iteration: currentIter,
			}
			return nil
		}

		// Check pause flag after iteration (loop stops after current iteration completes)
		l.mu.Lock()
		if l.paused {
			l.mu.Unlock()
			return nil
		}
		l.mu.Unlock()
	}
}

// runIterationWithRetry wraps runIteration with retry logic for crash recovery.
func (l *Loop) runIterationWithRetry(ctx context.Context) error {
	l.mu.Lock()
	config := l.retryConfig
	l.mu.Unlock()

	var lastErr error
	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Check if retry is enabled (except for first attempt)
		if attempt > 0 {
			if !config.Enabled {
				return lastErr
			}

			// Get delay for this retry
			delayIdx := attempt - 1
			if delayIdx >= len(config.RetryDelays) {
				delayIdx = len(config.RetryDelays) - 1
			}
			delay := config.RetryDelays[delayIdx]

			// Emit retry event
			l.mu.Lock()
			iter := l.iteration
			l.mu.Unlock()
			l.events <- Event{
				Type:       EventRetrying,
				Iteration:  iter,
				RetryCount: attempt,
				RetryMax:   config.MaxRetries,
				Text:       fmt.Sprintf("Gemini crashed, retrying (%d/%d)...", attempt, config.MaxRetries),
			}

			// Wait before retry
			if delay > 0 {
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}

		// Check if stopped during delay
		l.mu.Lock()
		if l.stopped {
			l.mu.Unlock()
			return nil
		}
		l.mu.Unlock()

		// Run the iteration
		err := l.runIteration(ctx)
		if err == nil {
			return nil // Success
		}

		// Check if this is a context cancellation (don't retry)
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Check if stopped intentionally
		l.mu.Lock()
		stopped := l.stopped
		l.mu.Unlock()
		if stopped {
			return nil
		}

		lastErr = err
	}

	return fmt.Errorf("max retries (%d) exceeded: %w", config.MaxRetries, lastErr)
}

// runIteration spawns Gemini and processes its output.
func (l *Loop) runIteration(ctx context.Context) error {
	if err := gemini.EnsureAuth(); err != nil {
		return err
	}

	runCtx := ctx
	var cancel context.CancelFunc
	if l.watchdogTimeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, l.watchdogTimeout)
		defer cancel()
	}

	// Build Gemini command with required flags
	l.mu.Lock()
	l.geminiCmd = exec.CommandContext(runCtx, "gemini", gemini.BuildHeadlessArgs(l.prompt, "", true)...)
	// Set working directory: use workDir if configured, otherwise default to PRD directory
	l.geminiCmd.Dir = l.effectiveWorkDir()
	l.mu.Unlock()

	var stdout, stderr bytes.Buffer
	l.geminiCmd.Stdout = &stdout
	l.geminiCmd.Stderr = &stderr

	if err := l.geminiCmd.Run(); err != nil {
		// If the context was cancelled, don't treat it as an error
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if runCtx.Err() == context.DeadlineExceeded {
			l.mu.Lock()
			iter := l.iteration
			l.mu.Unlock()
			l.events <- Event{Type: EventWatchdogTimeout, Iteration: iter, Text: fmt.Sprintf("No completion before timeout (%s)", l.watchdogTimeout)}
			return fmt.Errorf("watchdog timeout: command exceeded %s", l.watchdogTimeout)
		}
		// Check if we were stopped intentionally
		l.mu.Lock()
		stopped := l.stopped
		l.mu.Unlock()
		if stopped {
			return nil
		}
		l.logLine(stdout.String())
		if stderr.Len() > 0 {
			l.logLine("[stderr] " + strings.TrimSpace(stderr.String()))
		}
		return fmt.Errorf("Gemini exited with error: %w", err)
	}

	l.logLine(stdout.String())
	if stderr.Len() > 0 {
		l.logLine("[stderr] " + strings.TrimSpace(stderr.String()))
	}

	result, err := gemini.ParseSingleJSONObject(stdout.Bytes())
	if err != nil {
		return err
	}

	if strings.TrimSpace(result.Response) != "" {
		l.emitResponseEvent(result.Response)
	}

	l.mu.Lock()
	l.geminiCmd = nil
	l.mu.Unlock()

	return nil
}

func (l *Loop) emitResponseEvent(text string) {
	event := Event{Type: EventAssistantText, Text: text}
	if strings.Contains(text, "<melliza-complete/>") {
		event.Type = EventComplete
	}
	if storyID := extractStoryID(text, "<ralph-status>", "</ralph-status>"); storyID != "" {
		event.Type = EventStoryStarted
		event.StoryID = storyID
	}
	l.mu.Lock()
	event.Iteration = l.iteration
	l.mu.Unlock()
	l.events <- event
}

// runWatchdog monitors lastOutputTime and kills the process if no output is received
// within the timeout duration. It stops when watchdogDone is closed.
func (l *Loop) runWatchdog(timeout time.Duration, done <-chan struct{}, fired *atomic.Bool) {
	// Check interval scales with timeout: 1/5 of timeout, clamped to [10ms, 10s]
	checkInterval := timeout / 5
	if checkInterval < 10*time.Millisecond {
		checkInterval = 10 * time.Millisecond
	}
	if checkInterval > 10*time.Second {
		checkInterval = 10 * time.Second
	}
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.mu.Lock()
			lastOutput := l.lastOutputTime
			stopped := l.stopped
			l.mu.Unlock()

			if stopped {
				return
			}

			if time.Since(lastOutput) > timeout {
				fired.Store(true)

				// Emit watchdog timeout event
				l.mu.Lock()
				iter := l.iteration
				l.mu.Unlock()
				l.events <- Event{
					Type:      EventWatchdogTimeout,
					Iteration: iter,
					Text:      fmt.Sprintf("No output for %s, killing hung process", timeout),
				}

				// Kill the process
				l.mu.Lock()
				if l.geminiCmd != nil && l.geminiCmd.Process != nil {
					l.geminiCmd.Process.Kill()
				}
				l.mu.Unlock()
				return
			}
		case <-done:
			return
		}
	}
}

// processOutput reads stdout line by line, logs it, and parses events.
func (l *Loop) processOutput(r io.Reader) {
	scanner := bufio.NewScanner(r)
	// Increase buffer size for long lines (Gemini can output large JSON)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		// Update last output time for watchdog
		l.mu.Lock()
		l.lastOutputTime = time.Now()
		l.mu.Unlock()

		// Log raw output
		l.logLine(line)

		// Parse the line and emit event if valid
		if event := ParseLine(line); event != nil {
			l.mu.Lock()
			event.Iteration = l.iteration
			l.mu.Unlock()
			l.events <- *event
		}
	}
}

// logStream logs a stream with a prefix.
func (l *Loop) logStream(r io.Reader, prefix string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		l.logLine(prefix + scanner.Text())
	}
}

// logLine writes a line to the log file.
func (l *Loop) logLine(line string) {
	if l.logFile != nil {
		l.logFile.WriteString(line + "\n")
	}
}

// Stop terminates the current Gemini process and stops the loop.
func (l *Loop) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.stopped = true

	if l.geminiCmd != nil && l.geminiCmd.Process != nil {
		// Kill the process
		l.geminiCmd.Process.Kill()
	}
}

// Pause sets the pause flag. The loop will stop after the current iteration completes.
func (l *Loop) Pause() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.paused = true
}

// Resume clears the pause flag.
func (l *Loop) Resume() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.paused = false
}

// IsPaused returns whether the loop is paused.
func (l *Loop) IsPaused() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.paused
}

// IsStopped returns whether the loop is stopped.
func (l *Loop) IsStopped() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.stopped
}

// effectiveWorkDir returns the working directory to use for Gemini.
// If workDir is set, it is used directly. Otherwise, defaults to the PRD directory.
func (l *Loop) effectiveWorkDir() string {
	if l.workDir != "" {
		return l.workDir
	}
	return filepath.Dir(l.prdPath)
}

// IsRunning returns whether a Gemini process is currently running.
func (l *Loop) IsRunning() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.geminiCmd != nil && l.geminiCmd.Process != nil
}

// SetMaxIterations updates the maximum iterations limit.
func (l *Loop) SetMaxIterations(maxIter int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.maxIter = maxIter
}

// MaxIterations returns the current max iterations limit.
func (l *Loop) MaxIterations() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.maxIter
}

// SetRetryConfig updates the retry configuration.
func (l *Loop) SetRetryConfig(config RetryConfig) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.retryConfig = config
}

// DisableRetry disables automatic retry on crash.
func (l *Loop) DisableRetry() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.retryConfig.Enabled = false
}

// SetWatchdogTimeout sets the watchdog timeout duration.
// Setting timeout to 0 disables the watchdog.
func (l *Loop) SetWatchdogTimeout(timeout time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.watchdogTimeout = timeout
}

// WatchdogTimeout returns the current watchdog timeout duration.
func (l *Loop) WatchdogTimeout() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.watchdogTimeout
}
