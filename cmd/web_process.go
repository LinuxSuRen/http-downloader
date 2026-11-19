package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/linuxsuren/http-downloader/pkg/installer"
)

const (
	procRunning = "running"
	procStopped = "stopped"
	procExited  = "exited"

	// maxProcLogs bounds the output tail kept per managed process
	maxProcLogs = 400
	// probeAttempts and probeInterval define the window in which the web
	// console looks for an HTTP endpoint of a started tool
	probeAttempts = 24
	probeInterval = 500 * time.Millisecond
	procStopWait  = 3 * time.Second
	probeHTTPWait = 2 * time.Second
	procLogBufLen = 64 * 1024
)

// procState is the API view of a process managed by the web console
type procState struct {
	Name      string `json:"name"`
	PID       int    `json:"pid,omitempty"`
	Status    string `json:"status"`
	URL       string `json:"url,omitempty"`
	StartedAt string `json:"started_at,omitempty"`
	Binary    string `json:"binary,omitempty"`
	ExitCode  *int   `json:"exit_code,omitempty"`
}

// managedProc is a tool process started from the web console: it carries
// the latest state and a tail of the process output
type managedProc struct {
	mu    sync.Mutex
	state procState
	cmd   *exec.Cmd
	logs  []string
	stop  bool
	done  chan struct{}
}

func (p *managedProc) appendLog(line string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.logs) >= maxProcLogs {
		p.logs = p.logs[1:]
	}
	p.logs = append(p.logs, line)
}

// snapshot returns the current state
func (p *managedProc) snapshot() procState {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}

// tailLogs returns a copy of the output tail
func (p *managedProc) tailLogs() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string{}, p.logs...)
}

// status returns the current status value
func (p *managedProc) status() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state.Status
}

// isRunning tells whether the process is still alive
func (p *managedProc) isRunning() bool {
	return p.status() == procRunning
}

// setURL records the detected HTTP endpoint when the process still runs
func (p *managedProc) setURL(url string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.state.Status == procRunning {
		p.state.URL = url
	}
}

// GET /api/processes returns the processes started by the web console
// GET /api/processes/{name}/logs returns the state and the output tail
func (s *webServer) handleProcesses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "only GET is supported")
		return
	}
	// the exact path lists the processes, anything below addresses one
	rest := strings.TrimPrefix(r.URL.Path, "/api/processes")
	if rest == "" || rest == "/" {
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": s.listProcesses()})
		return
	}
	rest = strings.TrimPrefix(rest, "/")

	name, withLogs := rest, false
	if idx := strings.Index(rest, "/"); idx >= 0 {
		name, withLogs = rest[:idx], strings.HasSuffix(rest, "/logs")
	}
	proc := s.findProc(name)
	if proc == nil || !withLogs {
		writeError(w, http.StatusNotFound, "PROCESS_NOT_FOUND", "process does not exist")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": proc.snapshot(), "logs": proc.tailLogs(),
	})
}

// listProcesses returns the states of the running processes, stopped and
// exited ones are not retained: the section of the console shows what is
// alive, the record of a tool is replaced on its next start
func (s *webServer) listProcesses() (states []procState) {
	s.procMu.Lock()
	procs := make([]*managedProc, 0, len(s.procs))
	for _, proc := range s.procs {
		if proc.isRunning() {
			procs = append(procs, proc)
		}
	}
	s.procMu.Unlock()

	states = make([]procState, 0, len(procs))
	for i := range procs {
		states = append(states, procs[i].snapshot())
	}
	sort.Slice(states, func(i, j int) bool { return states[i].Name < states[j].Name })
	return
}

// findProc returns the managed process of a tool, nil when absent
func (s *webServer) findProc(name string) *managedProc {
	s.procMu.Lock()
	defer s.procMu.Unlock()
	return s.procs[name]
}

// runningProc returns the managed process when the tool is still running
func (s *webServer) runningProc(name string) *managedProc {
	proc := s.findProc(name)
	if proc == nil || !proc.isRunning() {
		return nil
	}
	return proc
}

// startTool launches an installed tool as a managed background process,
// no extra terminal window is opened. The launch command of the tool
// record provides the arguments. The output is captured and an HTTP
// endpoint of the tool is detected automatically.
func (s *webServer) startTool(w http.ResponseWriter, name string) {
	tool, _ := installer.FindInstalledPackage(name)
	if tool == nil {
		writeError(w, http.StatusNotFound, "TOOL_NOT_FOUND",
			fmt.Sprintf("tool %s is not installed yet", name))
		return
	}
	if proc := s.runningProc(name); proc != nil {
		writeError(w, http.StatusConflict, "ALREADY_RUNNING",
			fmt.Sprintf("tool %s is already running, stop it first", name))
		return
	}

	binPath := s.findToolBinary(tool)
	if binPath == "" {
		writeError(w, http.StatusNotFound, "BINARY_NOT_FOUND",
			fmt.Sprintf("cannot find the binary of %s, please reinstall it", name))
		return
	}

	args, err := splitCommand(tool.LaunchCommand, name)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_COMMAND",
			fmt.Sprintf("the launch command of %s has unbalanced quotes", name))
		return
	}

	// #nosec G702 -- the path points to an existing binary of the local
	// install record, the console binds to 127.0.0.1 only
	cmd := s.buildCommand(binPath, args)
	cmd.Dir = filepath.Dir(binPath)
	prepareProcess(cmd)

	reader, writer, pipeErr := os.Pipe()
	if pipeErr != nil {
		writeError(w, http.StatusInternalServerError, "START_FAILED",
			fmt.Sprintf("failed to start %s, please try again", name))
		return
	}
	cmd.Stdout = writer
	cmd.Stderr = writer

	if err := cmd.Start(); err != nil {
		_ = reader.Close()
		_ = writer.Close()
		// #nosec G706 -- the error only reaches the server side log
		log.Printf("cannot start %s: %v", name, err)
		writeError(w, http.StatusInternalServerError, "START_FAILED",
			fmt.Sprintf("failed to start %s, please check whether it is a runnable program", name))
		return
	}
	// the parent copy must go away, the child keeps its own descriptor
	// and the output reader sees EOF once the tool exits
	_ = writer.Close()

	proc := &managedProc{
		cmd:  cmd,
		done: make(chan struct{}),
		state: procState{
			Name:      name,
			Status:    procRunning,
			StartedAt: time.Now().UTC().Format(time.RFC3339),
			Binary:    binPath,
			PID:       cmd.Process.Pid,
		},
	}

	s.procMu.Lock()
	s.procs[name] = proc
	s.procMu.Unlock()

	outputDone := make(chan struct{})
	go func() {
		defer close(outputDone)
		scanProcessOutput(reader, proc.appendLog)
	}()
	go func() {
		defer close(proc.done)
		waitErr := cmd.Wait()
		// the output drain finishes after the tool exits, only then the
		// final state is set so the tail is complete
		<-outputDone
		_ = reader.Close()

		exitCode := 0
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}
		pid := proc.snapshot().PID
		proc.mu.Lock()
		if proc.state.Status == procRunning {
			if proc.stop {
				proc.state.Status = procStopped
			} else {
				proc.state.Status = procExited
			}
		}
		proc.state.ExitCode = &exitCode
		proc.mu.Unlock()
		if waitErr != nil && !proc.stop {
			// #nosec G706 -- the error only reaches the server side log
			log.Printf("process %s (pid %d) exited: %v", name, pid, waitErr)
		}
	}()
	go s.probeProcess(proc)

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": proc.snapshot()})
}

// splitCommand splits a launch command into the arguments of the tool
// binary, single and double quoted segments keep their spaces
// errUnbalancedQuotes marks a launch command with an open quote
var errUnbalancedQuotes = errors.New("unbalanced quotes")

// splitCommand splits a launch command into the arguments of the tool
// binary, single and double quoted segments keep their spaces. A command
// copied from a doc often starts with the tool name itself, that prefix
// is dropped because the binary always comes from the install record.
func splitCommand(command, toolName string) (args []string, err error) {
	var current strings.Builder
	var quote rune
	flush := func() {
		if current.Len() > 0 {
			args = append(args, current.String())
			current.Reset()
		}
	}
	for _, ch := range command {
		switch {
		case quote != 0:
			if ch == quote {
				quote = 0
			} else {
				current.WriteRune(ch)
			}
		case ch == '\'' || ch == '"':
			quote = ch
		case ch == ' ' || ch == '\t':
			flush()
		default:
			current.WriteRune(ch)
		}
	}
	flush()
	if quote != 0 {
		return nil, errUnbalancedQuotes
	}
	if len(args) > 0 && toolName != "" {
		if first := args[0]; first == toolName || strings.HasSuffix(first, "/"+toolName) {
			args = args[1:]
		}
	}
	return args, nil
}

// scanProcessOutput splits the process output into lines and appends them
// to the process log tail
func scanProcessOutput(reader io.Reader, appendLog func(string)) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), procLogBufLen)
	for scanner.Scan() {
		if line := strings.TrimRight(scanner.Text(), "\r"); line != "" {
			appendLog(line)
		}
	}
}

// setLaunchCommand stores the launch command of a tool in its install
// record, the web console passes it as arguments whenever the tool starts
func (s *webServer) setLaunchCommand(w http.ResponseWriter, r *http.Request, name string) {
	var req struct {
		Command string `json:"command"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST",
			"please provide the launch command")
		return
	}
	command := strings.TrimSpace(req.Command)
	if _, err := splitCommand(command, name); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_COMMAND",
			"the launch command has unbalanced quotes")
		return
	}

	tool, _ := installer.FindInstalledPackage(name)
	if tool == nil {
		writeError(w, http.StatusNotFound, "TOOL_NOT_FOUND",
			fmt.Sprintf("tool %s is not installed yet", name))
		return
	}
	tool.LaunchCommand = command
	if err := installer.RecordInstalledPackage(*tool); err != nil {
		// #nosec G706 -- the error only reaches the server side log
		log.Printf("cannot save the launch command of %s: %v", name, err)
		writeError(w, http.StatusInternalServerError, "SAVE_FAILED",
			"failed to save the launch command, please try again")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": map[string]string{"name": name, "launch_command": command},
	})
}

// stopTool stops a managed process, stopping a tool which is not running
// anymore is a no-op and answers the current state
func (s *webServer) stopTool(w http.ResponseWriter, name string) {
	proc := s.findProc(name)
	if proc == nil {
		writeError(w, http.StatusNotFound, "PROCESS_NOT_FOUND",
			fmt.Sprintf("tool %s was not started from the console", name))
		return
	}

	proc.mu.Lock()
	cmd := proc.cmd
	running := proc.state.Status == procRunning
	proc.stop = true
	proc.mu.Unlock()

	if running && cmd != nil && cmd.Process != nil {
		if err := stopProcess(cmd, proc.done); err != nil {
			// #nosec G706 -- the error only reaches the server side log
			log.Printf("cannot stop %s: %v", name, err)
			writeError(w, http.StatusInternalServerError, "STOP_FAILED",
				fmt.Sprintf("failed to stop %s, please try again", name))
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": proc.snapshot()})
}

// probeProcess watches a started process for a while and records the URL
// of its HTTP endpoint once one responds
func (s *webServer) probeProcess(proc *managedProc) {
	for i := 0; i < probeAttempts; i++ {
		select {
		case <-proc.done:
			return
		case <-time.After(probeInterval):
		}
		if !proc.isRunning() {
			return
		}
		if url, ok := s.probeWeb(proc.snapshot().PID); ok {
			proc.setURL(url)
			return
		}
	}
}

// detectWebURL checks whether the process with the given pid listens on a
// port which answers an HTTP request, the address of the first responding
// endpoint wins
func (s *webServer) detectWebURL(pid int) (string, bool) {
	for _, port := range listeningPorts(pid) {
		if port <= 0 || port == s.selfPort {
			continue
		}
		url := fmt.Sprintf("http://localhost:%d", port)
		resp, err := probeHTTPClient.Get(url)
		if err != nil {
			continue
		}
		_ = resp.Body.Close()
		return url, true
	}
	return "", false
}

// probeHTTPClient is shared by the endpoint probes, a short timeout keeps
// the detection loop responsive
var probeHTTPClient = &http.Client{Timeout: probeHTTPWait}

// listeningPorts returns the TCP ports the process listens on, best
// effort per platform
func listeningPorts(pid int) (ports []int) {
	switch runtime.GOOS {
	case "windows":
		ports = parseNetstatPorts(netstatOutput(pid), pid)
	default:
		if out, err := lsofOutput(pid); err == nil {
			ports = parseLsofPorts(out)
		} else if runtime.GOOS == "linux" {
			ports = procNetPorts(pid)
		}
	}

	seen := make(map[int]bool, len(ports))
	deduped := make([]int, 0, len(ports))
	for _, port := range ports {
		if port <= 0 || seen[port] {
			continue
		}
		seen[port] = true
		deduped = append(deduped, port)
	}
	sort.Ints(deduped)
	return deduped
}

// lsofOutput asks lsof for the listening sockets of a process, lsof ships
// with macOS and is part of the common linux desktops
func lsofOutput(pid int) (string, error) {
	// #nosec G702 -- the pid of a process which was started from the
	// local web console
	out, err := exec.Command("lsof", "-nP", "-a", "-p", strconv.Itoa(pid),
		"-iTCP", "-sTCP:LISTEN", "-Fn").Output()
	return string(out), err
}

// parseLsofPorts parses the -Fn output of lsof, the address lines look
// like n127.0.0.1:8080 or n*:8080
func parseLsofPorts(out string) (ports []int) {
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "n") || !strings.Contains(line, ":") {
			continue
		}
		port, err := strconv.Atoi(line[strings.LastIndex(line, ":")+1:])
		if err == nil {
			ports = append(ports, port)
		}
	}
	return
}

// netstatOutput asks the Windows netstat for the listening TCP sockets
func netstatOutput(pid int) string {
	// #nosec G702 -- the pid of a process which was started from the
	// local web console
	out, _ := exec.Command("netstat", "-ano", "-p", "tcp").Output()
	return string(out)
}

// parseNetstatPorts parses the output of 'netstat -ano -p tcp' and keeps
// the listening ports of the given pid
func parseNetstatPorts(out string, pid int) (ports []int) {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 5 || !strings.HasPrefix(fields[0], "TCP") ||
			fields[len(fields)-2] != "LISTENING" {
			continue
		}
		owner, err := strconv.Atoi(fields[len(fields)-1])
		if err != nil || owner != pid {
			continue
		}
		local := fields[1]
		port, err := strconv.Atoi(local[strings.LastIndex(local, ":")+1:])
		if err == nil {
			ports = append(ports, port)
		}
	}
	return
}

// procNetPorts reads the listening ports of a linux process from procfs,
// it is the fallback when lsof is not installed: the socket inodes of the
// process are matched against the LISTEN entries of the kernel tables
func procNetPorts(pid int) (ports []int) {
	inodes := make(map[string]bool)
	fdDir := fmt.Sprintf("/proc/%d/fd", pid)
	entries, err := os.ReadDir(fdDir)
	if err != nil {
		return nil
	}
	for i := range entries {
		link, linkErr := os.Readlink(fdDir + "/" + entries[i].Name())
		if linkErr == nil && strings.HasPrefix(link, "socket:[") {
			inodes[strings.TrimSuffix(strings.TrimPrefix(link, "socket:["), "]")] = true
		}
	}

	for _, table := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		data, err := os.ReadFile(table) // #nosec G304 -- a fixed kernel path
		if err != nil {
			continue
		}
		ports = append(ports, parseProcNetTable(string(data), inodes)...)
	}
	return
}

// parseProcNetTable extracts the LISTEN ports of the given socket inodes
// from a /proc/net/tcp style table
func parseProcNetTable(table string, inodes map[string]bool) (ports []int) {
	for _, line := range strings.Split(table, "\n") {
		fields := strings.Fields(line)
		// sl local rem st ... inode, 0A is the LISTEN state
		if len(fields) < 10 || fields[3] != "0A" || !inodes[fields[9]] {
			continue
		}
		local := fields[1]
		port, err := strconv.ParseInt(local[strings.LastIndex(local, ":")+1:], 16, 32)
		if err == nil {
			ports = append(ports, int(port))
		}
	}
	return
}
