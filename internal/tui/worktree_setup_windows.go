//go:build windows

package tui

import "os/exec"

// configureProcessGroupKill is a no-op on Windows: there is no portable
// process-group concept for `sh -c` scripts here, and the worktree setup path
// is Unix-shell-only in practice.
func configureProcessGroupKill(cmd *exec.Cmd) {}
