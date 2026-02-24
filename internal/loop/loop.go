// Package loop provides the core agent loop that orchestrates Claude Code
// to implement user stories. It includes the main Loop struct for single
// PRD execution, Manager for parallel PRD execution, and Parser for
// processing Claude's stream-json output.
package loop

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/minicodemonkey/chief/embed"
	"github.com/minicodemonkey/chief/internal/prd"
)

// RetryConfig configures automatic retry behavior on Claude crashes.
type RetryConfig struct {
	MaxRetries  int           // Maximum number of retry attempts (default: 3)
	RetryDelays []time.Duration // Delays between retries (default: 0s, 5s, 15s)
	Enabled     bool          // Whether retry is enabled (default: true)
}

// DefaultRetryConfig returns the default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:  3,
		RetryDelays: []time.Duration{0, 5 * time.Second, 15 * time.Second},
		Enabled:     true,
	}
}

// Loop manages the core agent loop that invokes Claude repeatedly until all stories are complete.
type Loop struct {
	prdPath     string
	workDir     string
	prompt      string
	maxIter     int
	iteration   int
	events      chan Event
	claudeCmd   *exec.Cmd
	logFile     *os.File
	mu          sync.Mutex
	stopped     bool
	paused      bool
	retryConfig RetryConfig
}

// NewLoop creates a new Loop instance.
func NewLoop(prdPath, prompt string, maxIter int) *Loop {
	return &Loop{
		prdPath:     prdPath,
		prompt:      prompt,
		maxIter:     maxIter,
		events:      make(chan Event, 100),
		retryConfig: DefaultRetryConfig(),
	}
}

// NewLoopWithWorkDir creates a new Loop instance with a configurable working directory.
// When workDir is empty, defaults to the project root for backward compatibility.
func NewLoopWithWorkDir(prdPath, workDir string, prompt string, maxIter int) *Loop {
	return &Loop{
		prdPath:     prdPath,
		workDir:     workDir,
		prompt:      prompt,
		maxIter:     maxIter,
		events:      make(chan Event, 100),
		retryConfig: DefaultRetryConfig(),
	}
}

// NewLoopWithEmbeddedPrompt creates a new Loop instance using the embedded agent prompt.
// The PRD path placeholder in the prompt is automatically substituted.
func NewLoopWithEmbeddedPrompt(prdPath string, maxIter int) *Loop {
	prompt := embed.GetPrompt(prdPath, prd.ProgressPath(prdPath))
	return NewLoop(prdPath, prompt, maxIter)
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
	logPath := filepath.Join(prdDir, "claude.log")
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
				Text:       fmt.Sprintf("Claude crashed, retrying (%d/%d)...", attempt, config.MaxRetries),
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

// runIteration spawns Claude and processes its output.
func (l *Loop) runIteration(ctx context.Context) error {
	// Build Claude command with required flags
	l.mu.Lock()
	l.claudeCmd = exec.CommandContext(ctx, "claude",
		"--dangerously-skip-permissions",
		"-p", l.prompt,
		"--output-format", "stream-json",
		"--verbose",
	)
	// Set working directory: use workDir if configured, otherwise default to PRD directory
	l.claudeCmd.Dir = l.effectiveWorkDir()
	l.mu.Unlock()

	// Create pipes for stdout and stderr
	stdout, err := l.claudeCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := l.claudeCmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := l.claudeCmd.Start(); err != nil {
		return fmt.Errorf("failed to start Claude: %w", err)
	}

	// Process stdout in a separate goroutine
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		l.processOutput(stdout)
	}()

	// Log stderr to the log file
	go func() {
		defer wg.Done()
		l.logStream(stderr, "[stderr] ")
	}()

	// Wait for output processing to complete
	wg.Wait()

	// Wait for the command to finish
	if err := l.claudeCmd.Wait(); err != nil {
		// If the context was cancelled, don't treat it as an error
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Check if we were stopped intentionally
		l.mu.Lock()
		stopped := l.stopped
		l.mu.Unlock()
		if stopped {
			return nil
		}
		return fmt.Errorf("Claude exited with error: %w", err)
	}

	l.mu.Lock()
	l.claudeCmd = nil
	l.mu.Unlock()

	return nil
}

// processOutput reads stdout line by line, logs it, and parses events.
func (l *Loop) processOutput(r io.Reader) {
	scanner := bufio.NewScanner(r)
	// Increase buffer size for long lines (Claude can output large JSON)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

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

// Stop terminates the current Claude process and stops the loop.
func (l *Loop) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.stopped = true

	if l.claudeCmd != nil && l.claudeCmd.Process != nil {
		// Kill the process
		l.claudeCmd.Process.Kill()
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

// effectiveWorkDir returns the working directory to use for Claude.
// If workDir is set, it is used directly. Otherwise, defaults to the PRD directory.
func (l *Loop) effectiveWorkDir() string {
	if l.workDir != "" {
		return l.workDir
	}
	return filepath.Dir(l.prdPath)
}

// IsRunning returns whether a Claude process is currently running.
func (l *Loop) IsRunning() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.claudeCmd != nil && l.claudeCmd.Process != nil
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
