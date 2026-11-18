//go:build !windows

package cmd

import (
	"errors"
	"os/exec"
	"syscall"
	"time"
)

// prepareProcess puts the tool process into its own process group, so the
// whole group can be terminated later even when the tool forks children
func prepareProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// stopProcess terminates the process group of the command: a SIGTERM
// first, a SIGKILL when it does not exit in time
func stopProcess(cmd *exec.Cmd, done <-chan struct{}) error {
	pid := cmd.Process.Pid
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	select {
	case <-done:
		return nil
	case <-time.After(procStopWait):
	}
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}
