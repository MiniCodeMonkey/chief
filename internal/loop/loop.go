// Package loop provides the core agent loop that orchestrates Gemini Code
// to implement user stories. It includes the main Loop struct for single
// PRD execution, Manager for parallel PRD execution, and Parser for
// processing Gemini's stream-json output.
package loop

import (
	"bufio"
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
	// Build Gemini command with stream-json for real-time event parsing
	l.mu.Lock()
	l.geminiCmd = exec.CommandContext(ctx, "gemini", gemini.BuildStreamArgs(l.prompt, "", true)...)
	l.geminiCmd.Dir = l.effectiveWorkDir()
	l.lastOutputTime = time.Now()
	l.mu.Unlock()

	stdout, err := l.geminiCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := l.geminiCmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := l.geminiCmd.Start(); err != nil {
		return fmt.Errorf("failed to start Gemini: %w", err)
	}

	// Start inactivity-based watchdog (kills process if no output for watchdogTimeout)
	var watchdogFired atomic.Bool
	watchdogDone := make(chan struct{})
	if l.watchdogTimeout > 0 {
		go l.runWatchdog(l.watchdogTimeout, watchdogDone, &watchdogFired)
	}

	// Drain stderr in background, log it, and capture last meaningful lines
	var stderrLines []string
	var stderrMu sync.Mutex
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 256*1024), 256*1024)
		for scanner.Scan() {
			line := scanner.Text()
			l.logLine("[stderr] " + line)
			// Emit stderr as events so TUI can show them
			l.mu.Lock()
			iter := l.iteration
			l.mu.Unlock()
			l.events <- Event{Type: EventStderr, Iteration: iter, Text: line}
			// Keep last 10 lines for error context
			stderrMu.Lock()
			stderrLines = append(stderrLines, line)
			if len(stderrLines) > 10 {
				stderrLines = stderrLines[len(stderrLines)-10:]
			}
			stderrMu.Unlock()
		}
	}()

	// Parse stdout stream-json lines in real-time
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()

		// Reset inactivity watchdog on every line of output
		l.mu.Lock()
		l.lastOutputTime = time.Now()
		l.mu.Unlock()

		l.logLine(line)

		event := ParseLine(line)
		if event != nil {
			l.mu.Lock()
			event.Iteration = l.iteration
			l.mu.Unlock()
			l.events <- *event
		}
	}

	waitErr := l.geminiCmd.Wait()
	close(watchdogDone) // Stop watchdog
	<-stderrDone        // Ensure all stderr is captured before using it

	l.mu.Lock()
	l.geminiCmd = nil
	stopped := l.stopped
	l.mu.Unlock()

	if waitErr != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if watchdogFired.Load() {
			return fmt.Errorf("watchdog timeout: no output for %s", l.watchdogTimeout)
		}
		if stopped {
			return nil
		}
		// Build error with stderr context
		stderrMu.Lock()
		stderrContext := filterStderrForError(stderrLines)
		stderrMu.Unlock()
		if stderrContext != "" {
			return fmt.Errorf("Gemini failed: %s", stderrContext)
		}
		return fmt.Errorf("Gemini exited with error: %w", waitErr)
	}

	return nil
}

// filterStderrForError extracts the most useful error info from stderr lines.
func filterStderrForError(lines []string) string {
	// Look for API errors, stack traces, or meaningful messages
	// Skip noise like "YOLO mode" and "Server ... supports"
	var useful []string
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") ||
			strings.Contains(lower, "apikey") ||
			strings.Contains(lower, "unavailable") ||
			strings.Contains(lower, "forbidden") ||
			strings.Contains(lower, "unauthorized") ||
			strings.Contains(lower, "quota") ||
			strings.Contains(lower, "rate limit") ||
			strings.Contains(lower, "timeout") ||
			strings.Contains(lower, "status:") {
			useful = append(useful, line)
		}
	}
	if len(useful) > 0 {
		return strings.Join(useful, "\n")
	}
	// If no specific error found, return last few lines
	if len(lines) > 3 {
		lines = lines[len(lines)-3:]
	}
	return strings.Join(lines, "\n")
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
		iter := l.iteration
		l.mu.Unlock()

		// Log raw output
		l.logLine(line)

		// Parse the line and emit event if valid
		if event := ParseLine(line); event != nil {
			event.Iteration = iter
			l.events <- *event
		} else {
			// If not parsed as a semantic event, emit as raw stdout event
			l.events <- Event{
				Type:      EventStdout,
				Iteration: iter,
				Text:      line,
			}
		}
	}
}

// logStream logs a stream with a prefix and emits stderr events.
func (l *Loop) logStream(r io.Reader, prefix string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		l.logLine(prefix + line)

		// Emit stderr events
		l.mu.Lock()
		iter := l.iteration
		l.mu.Unlock()
		l.events <- Event{
			Type:      EventStderr,
			Iteration: iter,
			Text:      line,
		}
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
