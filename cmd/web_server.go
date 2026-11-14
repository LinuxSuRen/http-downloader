package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	fakeruntime "github.com/linuxsuren/go-fake-runtime"
	hdpkg "github.com/linuxsuren/http-downloader/pkg"
	"github.com/linuxsuren/http-downloader/pkg/common"
	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	jobRunning = "running"
	jobSuccess = "success"
	jobFailed  = "failed"

	// jobHistory is the number of finished jobs kept for the task list
	jobHistory = 20
)

// webJob is a background task of the console, its progress is reported as
// numbers instead of log lines
type webJob struct {
	ID      int    `json:"id"`
	Action  string `json:"action"` // install, upgrade or fetch
	Tool    string `json:"tool"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	// Phase tells what the job is doing, e.g. resolving or downloading
	Phase string `json:"phase,omitempty"`
	// Percent is 0 to 100, a negative value means the progress is unknown
	// and the UI shows an indeterminate indicator
	Percent int    `json:"percent"`
	Detail  string `json:"detail,omitempty"`

	mu       sync.Mutex
	received int64
	total    int64
	finished bool
}

// setPhase switches the phase of the job
func (j *webJob) setPhase(phase string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Phase = phase
}

// addBytes accumulates the transferred bytes and derives the percent,
// a non positive total keeps the progress indeterminate
func (j *webJob) addBytes(delta, total int64) {
	if delta <= 0 {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	j.received += delta
	if total > 0 {
		j.total = total
	}
	if j.total > 0 {
		j.Percent = int(j.received * 100 / j.total)
		if j.Percent > 100 {
			j.Percent = 100
		}
		j.Detail = fmt.Sprintf("%s / %s", humanBytes(float64(j.received)), humanBytes(float64(j.total)))
	} else {
		j.Percent = -1
		j.Detail = humanBytes(float64(j.received))
	}
	if j.Phase == "" {
		j.Phase = "downloading"
	}
}

// setProgress sets an explicit percent, used by the fetch which parses the
// progress of the underlying git transfer
func (j *webJob) setProgress(percent int, detail string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Percent = percent
	j.Detail = detail
}

// markDownloading switches the job into the downloading phase once
func (j *webJob) markDownloading() {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.Phase != "downloading" {
		j.Phase = "downloading"
	}
}

// snapshot returns the current progress and whether the job is finished
func (j *webJob) snapshot() (phase string, percent int, detail string, finished bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.Phase, j.Percent, j.Detail, j.finished
}

// humanBytes formats a byte count in a compact human readable form
func humanBytes(bytes float64) string {
	units := []string{"B", "KiB", "MiB", "GiB"}
	for _, unit := range units {
		if bytes < 1024 || unit == "GiB" {
			if unit == "B" {
				return fmt.Sprintf("%.0f %s", bytes, unit)
			}
			return fmt.Sprintf("%.1f %s", bytes, unit)
		}
		bytes /= 1024
	}
	return fmt.Sprintf("%.1f GiB", bytes)
}

type webServer struct {
	ctx    context.Context
	execer fakeruntime.Execer

	mu     sync.Mutex
	nextID int
	jobs   map[int]*webJob
	// only one job runs at a time, they share the target directories and
	// the download working directory
	jobLock chan struct{}

	// procs manages the tools started as child processes of the server
	procs *processManager

	// overridable behaviors, they talk to GitHub or start desktop
	// processes, which is neither available nor desired in the unit tests
	listVersions func(org, repo string, count int) ([]versionTag, error)
	startTool    func(name, binPath string) (*managedProcess, error)
}

func newWebServer(ctx context.Context) *webServer {
	server := &webServer{
		ctx:     ctx,
		execer:  fakeruntime.NewDefaultExecer(),
		jobs:    map[int]*webJob{},
		jobLock: make(chan struct{}, 1),
		procs:   newProcessManager(),
	}
	server.listVersions = listGitHubVersions
	server.startTool = server.procs.startTool
	return server
}

func (s *webServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/tools", s.handleTools)
	mux.HandleFunc("/api/tools/", s.handleTool)
	mux.HandleFunc("/api/search", s.handleSearch)
	mux.HandleFunc("/api/fetch", s.handleFetch)
	mux.HandleFunc("/api/jobs", s.handleJobList)
	mux.HandleFunc("/api/jobs/", s.handleJob)
	mux.HandleFunc("/api/categories", s.handleCategories)
	mux.HandleFunc("/api/packages", s.handlePackages)
	mux.HandleFunc("/api/packages/", s.handlePackageVersions)
	mux.HandleFunc("/api/proxy", s.handleProxy)
	mux.HandleFunc("/api/processes", s.handleProcesses)
	mux.HandleFunc("/api/processes/", s.handleProcess)
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
// POST /api/tools {"name":"jcli","version":"v1.2.3","proxy":"gh.api...","force":true}
// starts an install job, the optional version pins the release tag, so it
// also covers upgrades and downgrades, the optional proxy downloads the
// release through a GitHub acceleration proxy
func (s *webServer) handleTools(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": s.loadTools()})
	case http.MethodPost:
		var req struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			Proxy   string `json:"proxy"`
			Force   bool   `json:"force"`
		}
		if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.Name) == "" {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "please provide the tool name")
			return
		}
		version := strings.TrimSpace(req.Version)
		proxy := strings.TrimSpace(req.Proxy)
		target := strings.TrimSpace(req.Name)
		if version != "" && !strings.Contains(target, "@") {
			target = fmt.Sprintf("%s@%s", target, version)
		}
		job := s.startJob("install", req.Name, func(stub *cobra.Command, job *webJob) error {
			return s.installTool(stub, target, version, proxy, req.Force, job)
		})
		writeJSON(w, http.StatusAccepted, job)
	default:
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "only GET and POST are supported")
	}
}

// proxyInfo is the API view of the GitHub acceleration proxy settings
type proxyInfo struct {
	// Current is the proxy configured in the hd config file, empty means
	// downloading directly from GitHub
	Current string   `json:"current"`
	Servers []string `json:"servers,omitempty"`
}

// GET /api/proxy returns the GitHub acceleration proxy candidates: the
// configured one plus the servers maintained in hd-home
func (s *webServer) handleProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "only GET is supported")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": proxyInfo{
			Current: strings.TrimSpace(viper.GetString("proxy-github")),
			Servers: installer.GetProxyServers(),
		},
	})
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

	configDir, ok := s.ensureConfigCache()
	if !ok {
		writeError(w, http.StatusInternalServerError, "SEARCH_FAILED",
			"failed to load the package index, please try again later")
		return
	}

	result := installer.FindByKeyword(keyword, configDir)
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": result})
}

// ensureConfigCache returns the local hd-home config directory, it fetches
// the repository once when there is no cache yet
func (s *webServer) ensureConfigCache() (configDir string, ok bool) {
	fetcher := &installer.DefaultFetcher{}
	fetcher.SetContext(s.ctx)
	var err error
	if configDir, err = fetcher.GetConfigDir(); err != nil {
		return "", false
	}
	if exists, _ := common.PathExists(configDir); exists {
		return configDir, true
	}
	if err = fetcher.FetchLatestRepo("", "", io.Discard); err != nil {
		log.Printf("cannot fetch the hd-home config: %v", err)
		return "", false
	}
	return configDir, true
}

// GET /api/categories returns the package categories of the local hd-home cache
func (s *webServer) handleCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "only GET is supported")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": installer.FindCategories()})
}

// packageItem is the API view of an installable package from hd-home
type packageItem struct {
	Org        string   `json:"org"`
	Repo       string   `json:"repo"`
	Name       string   `json:"name"`
	Categories []string `json:"categories,omitempty"`
	Homepage   string   `json:"homepage,omitempty"`
	// Web marks a package which has nothing to install, the UI opens its
	// homepage instead
	Web bool `json:"web"`
	// InstalledName is the name of the installed record, it is the name
	// expected by the open, upgrade and remove actions
	InstalledName string `json:"installed_name,omitempty"`
	Installed     bool   `json:"installed"`
	Version       string `json:"version,omitempty"`
	InstalledAt   string `json:"installed_at,omitempty"`
	Native        bool   `json:"native,omitempty"`
}

// GET /api/packages lists the tools of the local hd-home cache together
// with their installed state. Without a category and a keyword every
// package is listed and the installed tools which are not part of the
// cache are merged in, so installed and not-installed tools share one
// view. A category or a keyword filters the cache packages only.
func (s *webServer) handlePackages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "only GET is supported")
		return
	}
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))

	configDir, ok := s.ensureConfigCache()
	if !ok {
		writeError(w, http.StatusInternalServerError, "PACKAGES_FAILED",
			"failed to load the package index, please try again later")
		return
	}

	var orgRepos []string
	switch {
	case category != "":
		packages := installer.FindPackagesByCategory(category)
		orgRepos = make([]string, 0, len(packages))
		for i := range packages {
			orgRepos = append(orgRepos, packages[i].Org+"/"+packages[i].Repo)
		}
	case keyword != "":
		orgRepos = installer.FindByKeyword(keyword, configDir)
	default:
		orgRepos = installer.FindAllPackages(configDir)
	}

	records := loadInstalledRecords()
	installed := installedIndex(records)
	seen := make(map[string]bool, len(orgRepos))
	items := make([]packageItem, 0, len(orgRepos)+len(records))
	for _, orgRepo := range orgRepos {
		org, repo, valid := splitOrgRepo(orgRepo)
		if !valid {
			continue
		}

		item := packageItem{Org: org, Repo: repo, Name: repo}
		if config := installer.LoadPackageConfig(org, repo); config != nil {
			if config.Name != "" {
				item.Name = config.Name
			}
			item.Categories = config.Categories
			item.Homepage = config.Homepage
			item.Web = config.Homepage != "" && config.Filename == "" &&
				config.Binary == "" && config.TargetBinary == "" && !config.FromSource
		}
		if record := installed[orgRepo]; record != nil {
			item.Installed = true
			item.Version = record.Version
			item.InstalledName = record.Name
			item.InstalledAt = record.InstalledAt
		}
		seen[orgRepo] = true
		items = append(items, item)
	}

	// the unfiltered view also carries the installed tools which are not
	// part of the hd-home cache, so nothing installed is lost from the
	// merged list
	if category == "" && keyword == "" {
		for i := range records {
			record := records[i]
			if record.Org != "" && record.Repo != "" && seen[record.Org+"/"+record.Repo] {
				continue
			}

			items = append(items, packageItem{
				Org:           record.Org,
				Repo:          record.Repo,
				Name:          record.Name,
				Installed:     true,
				Version:       record.Version,
				InstalledName: record.Name,
				InstalledAt:   record.InstalledAt,
				Native:        record.Native,
			})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Org+"/"+items[i].Repo < items[j].Org+"/"+items[j].Repo
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items})
}

// versionTag is the API view of a release tag
type versionTag struct {
	Tag string `json:"tag"`
}

// GET /api/packages/{org}/{repo}/versions returns the recent release tags
// of a package, the first one is the latest
func (s *webServer) handlePackageVersions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "only GET is supported")
		return
	}

	rest := strings.TrimPrefix(r.URL.Path, "/api/packages/")
	if !strings.HasSuffix(rest, "/versions") {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "page not found")
		return
	}
	org, repo, valid := splitOrgRepo(strings.TrimSuffix(rest, "/versions"))
	if !valid || org == "" || repo == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid package path")
		return
	}

	count, err := strconv.Atoi(r.URL.Query().Get("count"))
	if err != nil || count <= 0 || count > 50 {
		count = 20
	}
	tags, err := s.listVersions(org, repo, count)
	if err != nil {
		// #nosec G706 -- the values come from the local request path, they
		// only reach the server side log
		log.Printf("cannot list the versions of %s/%s: %v", org, repo, err)
		writeError(w, http.StatusInternalServerError, "VERSIONS_FAILED",
			"failed to query the versions, please try again later")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": tags})
}

// listGitHubVersions returns the recent release tags of a GitHub repository
func listGitHubVersions(org, repo string, count int) (tags []versionTag, err error) {
	client := hdpkg.ReleaseClient{Org: org, Repo: repo}
	client.Init()

	var releases []hdpkg.ReleaseAsset
	if releases, err = client.ListReleases(org, repo, count); err == nil {
		tags = make([]versionTag, 0, len(releases))
		for i := range releases {
			if releases[i].TagName != "" {
				tags = append(tags, versionTag{Tag: releases[i].TagName})
			}
		}
	}
	return
}

// splitOrgRepo splits an org/repo string
func splitOrgRepo(orgRepo string) (org, repo string, valid bool) {
	items := strings.Split(strings.Trim(orgRepo, "/"), "/")
	if len(items) != 2 || items[0] == "" || items[1] == "" {
		return "", "", false
	}
	return items[0], items[1], true
}

// loadInstalledRecords loads the installed tools, a broken record file
// yields an empty list
func loadInstalledRecords() (records []installer.InstalledPackage) {
	if loaded, err := installer.LoadInstalledPackages(); err == nil {
		records = loaded
	}
	return
}

// installedIndex indexes the installed tools by their org/repo
func installedIndex(records []installer.InstalledPackage) (index map[string]*installer.InstalledPackage) {
	index = make(map[string]*installer.InstalledPackage, len(records))
	for i := range records {
		if records[i].Org != "" && records[i].Repo != "" {
			index[records[i].Org+"/"+records[i].Repo] = &records[i]
		}
	}
	return
}

// fetchProgress parses the progress text of the underlying git transfer
// and reports the percent to the job
type fetchProgress struct {
	job *webJob
}

// gitProgress matches lines like "Receiving objects:  45% (123/273)"
var gitProgress = regexp.MustCompile(`(\d+)% \((\d+)/(\d+)\)`)

// Write implements io.Writer, unknown text is dropped
func (p fetchProgress) Write(chunk []byte) (n int, err error) {
	for _, line := range strings.Split(string(chunk), "\n") {
		matches := gitProgress.FindAllStringSubmatch(line, -1)
		if len(matches) == 0 {
			continue
		}
		// git sends the counting and the compressing phases first, the
		// last match of the last line wins
		m := matches[len(matches)-1]
		if percent, err := strconv.Atoi(m[1]); err == nil {
			p.job.setProgress(percent, fmt.Sprintf("%s / %s objects", m[2], m[3]))
		}
	}
	return len(chunk), nil
}

// POST /api/fetch updates the local hd-home config cache as a background job
func (s *webServer) handleFetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "only POST is supported")
		return
	}
	job := s.startJob("fetch", "hd-home", func(stub *cobra.Command, job *webJob) error {
		job.setPhase("updating index")
		fetcher := &installer.DefaultFetcher{}
		fetcher.SetContext(s.ctx)
		// the git transfer progress is parsed into the job percent
		progress := fetchProgress{job: job}
		err := fetcher.FetchLatestRepo("", "", progress)
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
		return fetcher.FetchLatestRepo("", "", progress)
	})
	writeJSON(w, http.StatusAccepted, job)
}

// DELETE /api/tools/{name} removes a tool
// POST /api/tools/{name}/upgrade starts an upgrade job
// POST /api/tools/{name}/open starts the tool as a child process of the
// server, its output is visible in the UI
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
		var req struct {
			Proxy string `json:"proxy"`
		}
		if r.Body != nil {
			// the body is optional, an empty or broken one keeps the
			// direct download
			_ = decodeJSON(r, &req)
		}
		proxy := strings.TrimSpace(req.Proxy)
		job := s.startJob("upgrade", name, func(stub *cobra.Command, job *webJob) error {
			return s.upgradeTool(stub, tool.Org, tool.Repo, proxy, job)
		})
		writeJSON(w, http.StatusAccepted, job)
	case r.Method == http.MethodPost && action == "open":
		s.openTool(w, name)
	default:
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "unsupported action")
	}
}

// openTool starts an installed tool as a child process of the server, the
// output of the process is captured and can be followed in the UI
func (s *webServer) openTool(w http.ResponseWriter, name string) {
	tool, _ := installer.FindInstalledPackage(name)
	if tool == nil {
		writeError(w, http.StatusNotFound, "TOOL_NOT_FOUND",
			fmt.Sprintf("tool %s is not installed yet", name))
		return
	}

	binPath := s.findToolBinary(tool)
	if binPath == "" {
		writeError(w, http.StatusNotFound, "BINARY_NOT_FOUND",
			fmt.Sprintf("cannot find the binary of %s, please reinstall it", name))
		return
	}
	proc, err := s.startTool(name, binPath)
	if err != nil {
		// #nosec G706 -- the values come from the local install record, they
		// only reach the server side log
		log.Printf("cannot launch %s: %v", name, err)
		writeError(w, http.StatusInternalServerError, "LAUNCH_FAILED",
			fmt.Sprintf("failed to launch %s", name))
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": proc.view()})
}

// findToolBinary returns an existing binary path of an installed tool
func (s *webServer) findToolBinary(tool *installer.InstalledPackage) (binPath string) {
	for _, binary := range tool.Binaries {
		if binary == "" {
			continue
		}
		if exists, _ := common.PathExists(binary); exists {
			return binary
		}
	}
	if tool.TargetDirectory != "" {
		candidate := filepath.Join(tool.TargetDirectory, tool.Name)
		if exists, _ := common.PathExists(candidate); exists {
			return candidate
		}
	}
	if found, err := s.execer.LookPath(tool.Name); err == nil && found != "" {
		return found
	}
	return
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
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": job})
		return
	}
	s.streamJobEvents(w, r, job)
}

// handleJobList returns the known jobs, newest first, so the task list
// survives a page reload
// GET /api/jobs
func (s *webServer) handleJobList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "only GET is supported")
		return
	}

	s.mu.Lock()
	jobs := make([]*webJob, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	s.mu.Unlock()
	for i := range jobs {
		for j := i + 1; j < len(jobs); j++ {
			if jobs[j].ID > jobs[i].ID {
				jobs[i], jobs[j] = jobs[j], jobs[i]
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": jobs})
}

// streamJobEvents streams the progress of a job until it is done. The done
// event carries the final status.
func (s *webServer) streamJobEvents(w http.ResponseWriter, r *http.Request, job *webJob) {
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

	sentPhase, sentPercent, sentDetail := "", -2, ""
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	for {
		phase, percent, detail, finished := job.snapshot()
		if phase != sentPhase || percent != sentPercent || detail != sentDetail {
			if !send(map[string]interface{}{
				"type": "progress", "phase": phase, "percent": percent, "detail": detail,
			}) {
				return
			}
			sentPhase, sentPercent, sentDetail = phase, percent, detail
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

// startJob runs fn in the background and reports its progress through the
// job. Only one job runs at a time, they share the download directory.
func (s *webServer) startJob(action, name string, fn func(stub *cobra.Command, job *webJob) error) *webJob {
	s.mu.Lock()
	s.nextID++
	job := &webJob{ID: s.nextID, Action: action, Tool: name, Status: jobRunning, Percent: -1}
	s.jobs[job.ID] = job
	s.pruneJobsLocked()
	s.mu.Unlock()

	go func() {
		s.jobLock <- struct{}{}
		defer func() { <-s.jobLock }()

		stub := &cobra.Command{Use: "hd"}
		// the fetch flow reads the context from the command, a nil one
		// makes go-git panic
		stub.SetContext(context.Background())
		stub.SetOut(log.Writer())
		stub.SetErr(log.Writer())
		err := fn(stub, job)

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

// pruneJobsLocked drops the oldest finished jobs, the caller holds the lock
func (s *webServer) pruneJobsLocked() {
	finished := make([]int, 0, len(s.jobs))
	for id, job := range s.jobs {
		job.mu.Lock()
		done := job.finished
		job.mu.Unlock()
		if done {
			finished = append(finished, id)
		}
	}
	if len(finished) <= jobHistory {
		return
	}
	for i := range finished {
		for j := i + 1; j < len(finished); j++ {
			if finished[j] < finished[i] {
				finished[i], finished[j] = finished[j], finished[i]
			}
		}
	}
	for _, id := range finished[:len(finished)-jobHistory] {
		delete(s.jobs, id)
	}
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

// installTool installs a tool by its name, the same way as hd install.
// The version is optional, when set it pins the release tag which covers
// upgrades and downgrades. The proxy is optional, when set the release is
// downloaded through the GitHub acceleration proxy. The job receives the
// download progress.
func (s *webServer) installTool(stub *cobra.Command, name, version, proxy string, force bool, job *webJob) (err error) {
	opt := s.newInstallOption()
	opt.force = force
	opt.ProxyGitHub = proxy
	if job != nil {
		job.setPhase("resolving")
		opt.onDownloadProgress = func(delta, total int64) {
			job.markDownloading()
			job.addBytes(delta, total)
		}
	}
	if err = opt.preRunE(stub, []string{name}); err != nil {
		return
	}
	if err = opt.install(stub, []string{name}); err != nil {
		return
	}
	if version != "" {
		// the config cache may carry a different latest version than the
		// pinned one, keep the installed record in sync with the request
		if tool, _ := installer.FindInstalledPackage(opt.tool); tool != nil {
			tool.Version = version
			_ = installer.RecordInstalledPackage(*tool)
		}
	}
	return
}

// upgradeTool upgrades a tool from its source repository, the proxy is
// optional and works the same as in installTool
func (s *webServer) upgradeTool(stub *cobra.Command, org, repo, proxy string, job *webJob) error {
	up := &upgradeOption{
		installOption: s.newInstallOption(),
	}
	up.ProxyGitHub = proxy
	if job != nil {
		job.setPhase("resolving")
		up.onDownloadProgress = func(delta, total int64) {
			job.markDownloading()
			job.addBytes(delta, total)
		}
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

// GET /api/processes returns the processes started from the console,
// newest first
func (s *webServer) handleProcesses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "only GET is supported")
		return
	}
	items := s.procs.list()
	views := make([]managedProcess, 0, len(items))
	for _, proc := range items {
		views = append(views, proc.view())
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": views})
}

// GET /api/processes/{id} returns the process with its captured output
// GET /api/processes/{id}/logs streams the output via SSE
// DELETE /api/processes/{id} stops a running process
func (s *webServer) handleProcess(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/processes/")
	if path == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "process id is required")
		return
	}

	idPart := path
	sse := false
	if idx := strings.Index(path, "/"); idx >= 0 {
		idPart, sse = path[:idx], strings.HasSuffix(path, "/logs")
	}
	id, err := strconv.Atoi(idPart)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid process id")
		return
	}

	proc := s.procs.get(id)
	if proc == nil {
		writeError(w, http.StatusNotFound, "PROCESS_NOT_FOUND", "process does not exist")
		return
	}

	switch {
	case r.Method == http.MethodGet && sse:
		s.streamProcessOutput(w, r, proc)
	case r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": proc.view(), "output": proc.snapshotLines(),
		})
	case r.Method == http.MethodDelete:
		if err = s.procs.stop(id); err != nil {
			writeError(w, http.StatusConflict, "STOP_FAILED",
				fmt.Sprintf("failed to stop process %d", id))
			return
		}
		// the exit is reported by the process waiter as stopped
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": proc.view()})
	default:
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED",
			"only GET and DELETE are supported")
	}
}

// streamProcessOutput replays the captured output first, then streams the
// new lines until the process ends
func (s *webServer) streamProcessOutput(w http.ResponseWriter, r *http.Request, proc *managedProcess) {
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
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		lines := proc.snapshotLines()
		for ; sent < len(lines); sent++ {
			if !send(map[string]string{"type": "log", "line": lines[sent]}) {
				return
			}
		}

		proc.mu.Lock()
		status := proc.Status
		proc.mu.Unlock()
		if status != processRunning && sent >= len(proc.snapshotLines()) {
			_ = send(map[string]string{"type": "done", "status": status, "message": proc.ExitHint})
			return
		}
		select {
		case <-ticker.C:
		case <-r.Context().Done():
			return
		}
	}
}
