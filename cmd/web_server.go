package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	fakeruntime "github.com/linuxsuren/go-fake-runtime"
	"github.com/linuxsuren/http-downloader/pkg/common"
	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/spf13/cobra"
)

const (
	jobRunning = "running"
	jobSuccess = "success"
	jobFailed  = "failed"
)

type webJob struct {
	ID      int    `json:"id"`
	Action  string `json:"action"` // install or upgrade
	Tool    string `json:"tool"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`

	mu       sync.Mutex
	logs     []string
	finished bool
}

func (j *webJob) appendLog(line string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.logs = append(j.logs, line)
}

// snapshot returns the current logs and whether the job is finished
func (j *webJob) snapshot() (logs []string, finished bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return append([]string{}, j.logs...), j.finished
}

type webServer struct {
	ctx    context.Context
	execer fakeruntime.Execer

	mu     sync.Mutex
	nextID int
	jobs   map[int]*webJob
	// only one job runs at a time: its output is captured globally
	jobLock chan struct{}
}

func newWebServer(ctx context.Context) *webServer {
	return &webServer{
		ctx:     ctx,
		execer:  fakeruntime.NewDefaultExecer(),
		jobs:    map[int]*webJob{},
		jobLock: make(chan struct{}, 1),
	}
}

func (s *webServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/tools", s.handleTools)
	mux.HandleFunc("/api/tools/", s.handleTool)
	mux.HandleFunc("/api/search", s.handleSearch)
	mux.HandleFunc("/api/fetch", s.handleFetch)
	mux.HandleFunc("/api/jobs/", s.handleJob)
	return mux
}

// handleIndex serves the embedded single page UI
func (s *webServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "page not found")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(webIndexHTML)
}

// GET /api/tools returns the installed tools
// POST /api/tools {"name":"jcli","force":true} starts an install job
func (s *webServer) handleTools(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": s.loadTools()})
	case http.MethodPost:
		var req struct {
			Name  string `json:"name"`
			Force bool   `json:"force"`
		}
		if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.Name) == "" {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "please provide the tool name")
			return
		}
		job := s.startJob("install", req.Name, func(stub *cobra.Command) error {
			return s.installTool(stub, req.Name, req.Force)
		})
		writeJSON(w, http.StatusAccepted, job)
	default:
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "only GET and POST are supported")
	}
}

// GET /api/search?q=keyword searches the installable packages from hd-home
func (s *webServer) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "only GET is supported")
		return
	}
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	if keyword == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "please provide the search keyword")
		return
	}

	fetcher := &installer.DefaultFetcher{}
	fetcher.SetContext(s.ctx)
	configDir, err := fetcher.GetConfigDir()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SEARCH_FAILED", "failed to search packages")
		return
	}
	if exists, _ := common.PathExists(configDir); !exists {
		// no local cache yet, fetch it once so that the search works
		if err = fetcher.FetchLatestRepo("", "", io.Discard); err != nil {
			log.Printf("cannot fetch the hd-home config: %v", err)
			writeError(w, http.StatusInternalServerError, "SEARCH_FAILED",
				"failed to load the package index, please try again later")
			return
		}
	}

	result := installer.FindByKeyword(keyword, configDir)
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": result})
}

// POST /api/fetch updates the local hd-home config cache as a background job
func (s *webServer) handleFetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "only POST is supported")
		return
	}
	job := s.startJob("fetch", "hd-home", func(stub *cobra.Command) error {
		fetcher := &installer.DefaultFetcher{}
		fetcher.SetContext(s.ctx)
		// the progress goes to the captured output so that it streams
		// into the log panel
		err := fetcher.FetchLatestRepo("", "", os.Stdout)
		if err == nil {
			return nil
		}

		// the local cache may be corrupt (e.g. a malformed shallow file),
		// reset it and clone again, the same as 'hd fetch --reset'
		log.Printf("fetch failed (%v), reset the local config and retry", err)
		if configDir, dirErr := fetcher.GetConfigDir(); dirErr == nil {
			if rmErr := os.RemoveAll(configDir); rmErr != nil {
				return rmErr
			}
		}
		return fetcher.FetchLatestRepo("", "", os.Stdout)
	})
	writeJSON(w, http.StatusAccepted, job)
}

// DELETE /api/tools/{name} removes a tool
// POST /api/tools/{name}/upgrade starts an upgrade job
func (s *webServer) handleTool(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/tools/")
	name, action := rest, ""
	if idx := strings.Index(rest, "/"); idx >= 0 {
		name, action = rest[:idx], rest[idx+1:]
	}
	if name == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid tool name")
		return
	}

	switch {
	case r.Method == http.MethodDelete:
		if err := s.removeTool(name); err != nil {
			writeError(w, http.StatusInternalServerError, "REMOVE_FAILED",
				fmt.Sprintf("failed to remove %s", name))
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": s.loadTools()})
	case r.Method == http.MethodPost && action == "upgrade":
		tool, _ := installer.FindInstalledPackage(name)
		if tool == nil {
			writeError(w, http.StatusNotFound, "TOOL_NOT_FOUND",
				fmt.Sprintf("tool %s is not installed yet", name))
			return
		}
		job := s.startJob("upgrade", name, func(stub *cobra.Command) error {
			return s.upgradeTool(stub, tool.Org, tool.Repo)
		})
		writeJSON(w, http.StatusAccepted, job)
	default:
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "unsupported action")
	}
}

// GET /api/jobs/{id} returns the job status
// GET /api/jobs/{id}/logs streams the job logs via SSE
func (s *webServer) handleJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "only GET is supported")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
	if path == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "job id is required")
		return
	}

	idPart := path
	sse := false
	if idx := strings.Index(path, "/"); idx >= 0 {
		idPart, sse = path[:idx], strings.HasSuffix(path, "/logs")
	}
	id, err := strconv.Atoi(idPart)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid job id")
		return
	}

	s.mu.Lock()
	job := s.jobs[id]
	s.mu.Unlock()
	if job == nil {
		writeError(w, http.StatusNotFound, "JOB_NOT_FOUND", "job does not exist")
		return
	}

	if !sse {
		logs, _ := job.snapshot()
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": job, "logs": logs})
		return
	}
	s.streamJobLogs(w, r, job)
}

// streamJobLogs replays the buffered logs first, then streams the new ones
// until the job is done. The done event carries the final status.
func (s *webServer) streamJobLogs(w http.ResponseWriter, r *http.Request, job *webJob) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "STREAM_UNSUPPORTED", "streaming is not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	send := func(payload interface{}) bool {
		data, err := json.Marshal(payload)
		if err != nil {
			return true
		}
		if _, err = fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	sent := 0
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	for {
		logs, finished := job.snapshot()
		for ; sent < len(logs); sent++ {
			if !send(map[string]string{"type": "log", "line": logs[sent]}) {
				return
			}
		}
		if finished {
			_ = send(map[string]string{"type": "done", "status": job.Status, "message": job.Message})
			return
		}
		select {
		case <-ticker.C:
		case <-r.Context().Done():
			return
		}
	}
}

// startJob runs fn in the background with the process output captured into
// the job logs. Only one job runs at a time.
func (s *webServer) startJob(action, name string, fn func(stub *cobra.Command) error) *webJob {
	s.mu.Lock()
	s.nextID++
	job := &webJob{ID: s.nextID, Action: action, Tool: name, Status: jobRunning}
	s.jobs[job.ID] = job
	s.mu.Unlock()

	go func() {
		s.jobLock <- struct{}{}
		defer func() { <-s.jobLock }()

		err := withCapturedOutput(job, func() error {
			stub := &cobra.Command{Use: "hd"}
			// the fetch flow reads the context from the command, a nil one
			// makes go-git panic
			stub.SetContext(context.Background())
			stub.SetOut(log.Writer())
			stub.SetErr(log.Writer())
			return fn(stub)
		})

		job.mu.Lock()
		if err != nil {
			job.Status = jobFailed
			job.Message = fmt.Sprintf("failed to %s %s", action, name)
			log.Printf("job %d failed: %v", job.ID, err)
		} else {
			job.Status = jobSuccess
		}
		job.finished = true
		job.mu.Unlock()
	}()
	return job
}

// withCapturedOutput redirects os.Stdout and the standard logger into the
// job logs while fn is running, so that the install/upgrade progress can be
// streamed to the web page.
func withCapturedOutput(job *webJob, fn func() error) (err error) {
	reader, writer, pipeErr := os.Pipe()
	if pipeErr != nil {
		return fn()
	}

	originStdout := os.Stdout
	originLogOut := log.Writer()
	os.Stdout = writer
	log.SetOutput(writer)

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, readErr := reader.Read(buf)
			if n > 0 {
				for _, line := range strings.Split(strings.TrimRight(string(buf[:n]), "\n"), "\n") {
					if line != "" {
						job.appendLog(line)
					}
				}
			}
			if readErr != nil {
				return
			}
		}
	}()

	defer func() {
		os.Stdout = originStdout
		log.SetOutput(originLogOut)
		_ = writer.Close()
		<-done
	}()
	err = fn()
	return
}

// loadTools returns the installed tools as the API view
func (s *webServer) loadTools() (tools []toolItem) {
	packages, err := installer.LoadInstalledPackages()
	if err != nil {
		log.Printf("cannot load the installed tools: %v", err)
		return
	}

	tools = make([]toolItem, 0, len(packages))
	for i := range packages {
		pkg := packages[i]
		tools = append(tools, toolItem{
			Name:        pkg.Name,
			Version:     pkg.Version,
			Org:         pkg.Org,
			Repo:        pkg.Repo,
			InstalledAt: pkg.InstalledAt,
			Directory:   pkg.TargetDirectory,
			Upgradable:  pkg.Org != "" && pkg.Repo != "" && !pkg.Native,
		})
	}
	return
}

// newInstallOption builds an installOption with the same defaults as the
// install command flags, which are not applied when there is no cobra flag set
func (s *webServer) newInstallOption() *installOption {
	opt := &installOption{
		downloadOption: newDownloadOption(s.ctx),
		execer:         s.execer,
	}
	opt.OS = runtime.GOOS
	opt.Arch = runtime.GOARCH
	opt.Timeout = 15 * time.Minute
	opt.MaxAttempts = 10
	opt.ShowProgress = false
	opt.Download = true
	opt.CleanPackage = true
	// -1 means keeping the system default file permission, zero would
	// make the downloaded package unreadable
	opt.Mod = -1
	return opt
}

// installTool installs a tool by its name, the same way as hd install
func (s *webServer) installTool(stub *cobra.Command, name string, force bool) (err error) {
	opt := s.newInstallOption()
	opt.force = force
	if err = opt.preRunE(stub, []string{name}); err != nil {
		return
	}
	return opt.install(stub, []string{name})
}

// upgradeTool upgrades a tool from its source repository
func (s *webServer) upgradeTool(stub *cobra.Command, org, repo string) error {
	up := &upgradeOption{
		installOption: s.newInstallOption(),
	}
	return up.upgradeOne(stub, fmt.Sprintf("%s/%s", org, repo))
}

// removeTool removes a tool the same way as hd remove
func (s *webServer) removeTool(name string) (err error) {
	opt := &removeOption{execer: s.execer}
	stub := &cobra.Command{Use: "hd"}
	stub.SetOut(io.Discard)
	stub.SetErr(io.Discard)
	return opt.runE(stub, []string{name})
}
