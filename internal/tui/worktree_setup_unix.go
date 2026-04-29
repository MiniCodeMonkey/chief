//go:build !windows

package tui

import (
	"os/exec"
	"syscall"
)

// configureProcessGroupKill puts the child into its own process group and
// installs a Cancel hook that SIGKILLs the entire group when the context is
// cancelled (e.g. on timeout). Without this, scripts that fork additional
// processes (npm install -> node, etc.) leak when only the immediate sh is
// killed.
func configureProcessGroupKill(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
