//go:build windows

package cmd

import (
	"fmt"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

// prepareProcess puts the tool process into its own process group
func prepareProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// stopProcess kills the whole process tree with taskkill, /T covers the
// children of the tool
func stopProcess(cmd *exec.Cmd, done <-chan struct{}) error {
	pid := strconv.Itoa(cmd.Process.Pid)
	// #nosec G702 -- the pid of a process which was started from the
	// local web console
	kill := exec.Command("taskkill", "/T", "/F", "/PID", pid)
	if err := kill.Run(); err != nil {
		return fmt.Errorf("taskkill %s: %w", pid, err)
	}
	select {
	case <-done:
		return nil
	case <-time.After(procStopWait):
		return fmt.Errorf("process %s did not exit in time", pid)
	}
}
