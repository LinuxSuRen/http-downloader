package cmd

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
)

// openInTerminal launches a tool in a new terminal window, so that both CLI
// and TUI tools can be used directly from the web console. The caller is
// responsible for checking that the binary exists.
func openInTerminal(binPath string) (err error) {
	cmd, err := terminalCommand(binPath)
	if err != nil {
		return err
	}

	// the new process must not inherit the server's standard pipes,
	// otherwise the tool and the HTTP server keep each other alive
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	return cmd.Start()
}

// terminalCommand builds the command which opens a new terminal window
// running the given binary
func terminalCommand(binPath string) (cmd *exec.Cmd, err error) {
	dir := filepath.Dir(binPath)
	switch runtime.GOOS {
	case "windows":
		// an empty title is required when the path is quoted, /D sets the
		// working directory of the new window and cmd /k keeps the window
		// open after the tool exits
		cmd = exec.Command("cmd", "/c", "start", "", "/D", dir, "cmd", "/k", binPath)
	case "darwin":
		cmd = exec.Command("open", "-a", "Terminal", binPath)
	case "linux":
		cmd, err = linuxTerminalCommand(binPath)
	default:
		err = fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	return
}

// linuxTerminalCommand finds an installed terminal emulator and wraps the
// binary into its run-a-command invocation
func linuxTerminalCommand(binPath string) (cmd *exec.Cmd, err error) {
	for _, terminal := range linuxTerminals {
		if _, lookupErr := exec.LookPath(terminal.bin); lookupErr == nil {
			args := append(append([]string{}, terminal.args...), binPath)
			return exec.Command(terminal.bin, args...), nil
		}
	}
	return nil, fmt.Errorf("no supported terminal emulator found, please run %s manually", binPath)
}

// linuxTerminals holds the known terminal emulators together with the flag
// they take to run a command
var linuxTerminals = []struct {
	bin  string
	args []string
}{
	{"x-terminal-emulator", []string{"-e"}},
	{"gnome-terminal", []string{"--"}},
	{"konsole", []string{"-e"}},
	{"xfce4-terminal", []string{"-x"}},
	{"mate-terminal", []string{"-x"}},
	{"xterm", []string{"-e"}},
}
