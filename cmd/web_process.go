package cmd

import (
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	processRunning = "running"
	processExited  = "exited"
	processFailed  = "failed"
	processStopped = "stopped"

	// processOutputLimit is the number of output lines kept per process,
	// the older lines are dropped
	processOutputLimit = 500
)

// managedProcess is a tool which runs as a child process of the server.
// Its output is captured and can be followed through the UI, no terminal
// window pops up.
type managedProcess struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Binary    string    `json:"binary"`
	StartedAt time.Time `json:"started_at"`
	Status    string    `json:"status"`
	// ExitHint tells how the process ended
	ExitHint string `json:"exit_hint,omitempty"`

	mu      sync.Mutex
	lines   []string
	pending string
	cmd     *exec.Cmd
	// stopping marks a stop request, so the exit is reported as stopped
	// instead of failed
	stopping bool
}

// appendLine adds one output line to the ring buffer
func (p *managedProcess) appendLine(line string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lines = append(p.lines, line)
	if len(p.lines) > processOutputLimit {
		p.lines = p.lines[len(p.lines)-processOutputLimit:]
	}
}

// snapshotLines returns the captured output lines
func (p *managedProcess) snapshotLines() (lines []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string{}, p.lines...)
}

// setStatus updates the status of the process
func (p *managedProcess) setStatus(status, hint string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Status = status
	p.ExitHint = hint
}

// view returns the API view of the process
func (p *managedProcess) view() managedProcess {
	return managedProcess{
		ID:        p.ID,
		Name:      p.Name,
		Binary:    p.Binary,
		StartedAt: p.StartedAt,
		Status:    p.Status,
		ExitHint:  p.ExitHint,
	}
}

// processOutput splits the output of a process into lines, the incomplete
// tail is buffered until its line ending arrives
type processOutput struct {
	proc *managedProcess
}

// Write implements io.Writer, it must be cheap: the process must never
// block on a slow web page
func (w processOutput) Write(chunk []byte) (n int, err error) {
	w.proc.mu.Lock()
	w.proc.pending += strings.ReplaceAll(string(chunk), "\r\n", "\n")
	if idx := strings.LastIndex(w.proc.pending, "\n"); idx >= 0 {
		complete := w.proc.pending[:idx]
		w.proc.pending = w.proc.pending[idx+1:]
		w.proc.mu.Unlock()

		for _, line := range strings.Split(complete, "\n") {
			if line != "" {
				w.proc.appendLine(line)
			}
		}
		return len(chunk), nil
	}
	w.proc.mu.Unlock()
	return len(chunk), nil
}

// processManager keeps the processes started from the web console
type processManager struct {
	mu     sync.Mutex
	nextID int
	items  map[int]*managedProcess
}

func newProcessManager() *processManager {
	return &processManager{items: map[int]*managedProcess{}}
}

// start launches a binary as a child process of the server and captures
// its output
func (m *processManager) start(name, binPath string, args []string) (proc *managedProcess, err error) {
	cmd := exec.Command(binPath, args...)
	// the child must not inherit the pipes of the HTTP server
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	m.mu.Lock()
	m.nextID++
	proc = &managedProcess{
		ID:        m.nextID,
		Name:      name,
		Binary:    binPath,
		StartedAt: time.Now(),
		Status:    processRunning,
		cmd:       cmd,
	}
	m.items[proc.ID] = proc
	m.mu.Unlock()

	output := processOutput{proc: proc}
	cmd.Stdout = output
	cmd.Stderr = output

	if err = cmd.Start(); err != nil {
		proc.setStatus(processFailed, err.Error())
		return proc, err
	}

	go func() {
		waitErr := cmd.Wait()
		proc.mu.Lock()
		flush := proc.pending
		proc.pending = ""
		stopping := proc.stopping
		proc.mu.Unlock()
		if flush != "" {
			proc.appendLine(flush)
		}

		switch {
		case stopping:
			proc.setStatus(processStopped, "stopped from the console")
		case waitErr != nil:
			proc.setStatus(processFailed, waitErr.Error())
		default:
			proc.setStatus(processExited, "")
		}
	}()
	return proc, nil
}

// startTool starts an installed tool without any argument
func (m *processManager) startTool(name, binPath string) (*managedProcess, error) {
	return m.start(name, binPath, nil)
}

// get returns a process by its id
func (m *processManager) get(id int) *managedProcess {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.items[id]
}

// list returns the processes, newest first
func (m *processManager) list() (result []*managedProcess) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, proc := range m.items {
		result = append(result, proc)
	}
	for i := range result {
		for j := i + 1; j < len(result); j++ {
			if result[j].ID > result[i].ID {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return
}

// stop terminates a running process and keeps its record
func (m *processManager) stop(id int) (err error) {
	proc := m.get(id)
	if proc == nil {
		return fmt.Errorf("process %d does not exist", id)
	}

	proc.mu.Lock()
	status := proc.Status
	cmd := proc.cmd
	if status == processRunning {
		proc.stopping = true
	}
	proc.mu.Unlock()
	if status != processRunning || cmd == nil || cmd.Process == nil {
		return fmt.Errorf("process %d is not running", id)
	}

	if runtime.GOOS == "windows" {
		// the tool may have children, take the whole tree down
		// #nosec G204 -- the arguments are the process id of the record
		return exec.Command("taskkill", "/PID", fmt.Sprint(cmd.Process.Pid), "/T", "/F").Run()
	}
	return cmd.Process.Kill()
}

// ensure the process output writer is an io.Writer
var _ io.Writer = processOutput{}
