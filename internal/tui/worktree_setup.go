package tui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// runSetupCommand executes setupCmd via `sh -c` in dir. When timeout > 0 the
// command (and any process group it spawns on Unix) is killed once the
// deadline is exceeded; in that case the returned error is prefixed with
// timeoutLabel so users see their original config string (e.g. "5m") rather
// than Go's normalized form ("5m0s").
//
// On non-Unix platforms (Windows) only the immediate sh process is killed,
// which is the same behaviour exec.CommandContext already provides.
func runSetupCommand(setupCmd, dir string, timeout time.Duration, timeoutLabel string) error {
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", setupCmd)
	cmd.Dir = dir
	configureProcessGroupKill(cmd)
	// Don't let a stuck child (uninterruptible I/O, NFS, etc.) hang the TUI
	// after SIGKILL. exec.Cmd.WaitDelay forces Wait to return shortly after
	// the deadline even if the process hasn't actually exited yet.
	cmd.WaitDelay = 5 * time.Second

	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			label := timeoutLabel
			if label == "" {
				label = timeout.String()
			}
			return fmt.Errorf("setup command timed out after %s\n%s", label, strings.TrimSpace(string(out)))
		}
		return fmt.Errorf("%s\n%s", err.Error(), strings.TrimSpace(string(out)))
	}
	return nil
}
