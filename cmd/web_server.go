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

	// overridable behaviors, they talk to GitHub or to the desktop, which
	// is neither available nor desired in the unit tests
	listVersions func(org, repo string, count int) ([]versionTag, error)
	launchTool   func(binPath string) error
}

func newWebServer(ctx context.Context) *webServer {
	server := &webServer{
		ctx:     ctx,
		execer:  fakeruntime.NewDefaultExecer(),
		jobs:    map[int]*webJob{},
		jobLock: make(chan struct{}, 1),
	}
	server.listVersions = listGitHubVersions
	server.launchTool = openInTerminal
	return server
}

func (s *webServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/tools", s.handleTools)
	mux.HandleFunc("/api/tools/", s.handleTool)
	mux.HandleFunc("/api/search", s.handleSearch)
	mux.HandleFunc("/api/fetch", s.handleFetch)
	mux.HandleFunc("/api/jobs/", s.handleJob)
	mux.HandleFunc("/api/categories", s.handleCategories)
	mux.HandleFunc("/api/packages", s.handlePackages)
	mux.HandleFunc("/api/packages/", s.handlePackageVersions)
	mux.HandleFunc("/api/proxy", s.handleProxy)
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
		job := s.startJob("install", req.Name, func(stub *cobra.Command) error {
			return s.installTool(stub, target, version, proxy, req.Force)
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
// The list is paginated (page starts at 1) and sorted server side, name
// ascending by default: ?page=1&page_size=48&sort=name&order=asc
func (s *webServer) handlePackages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "only GET is supported")
		return
	}
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	sortField := strings.TrimSpace(r.URL.Query().Get("sort"))
	sortOrder := strings.TrimSpace(r.URL.Query().Get("order"))

	page := parsePositiveInt(r.URL.Query().Get("page"), 1)
	pageSize := parsePositiveInt(r.URL.Query().Get("page_size"), defaultPackagePageSize)
	if pageSize > maxPackagePageSize {
		pageSize = maxPackagePageSize
	}

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

	// the whole list is sorted before it is sliced into the requested page
	sortPackages(items, sortField, sortOrder)

	installedTotal := 0
	for i := range items {
		if items[i].Installed {
			installedTotal++
		}
	}
	total := len(items)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":            items[start:end],
		"page":            page,
		"page_size":       pageSize,
		"total":           total,
		"installed_total": installedTotal,
	})
}

// packagePageLimits are the defaults and the upper bound of the package
// list pagination
const (
	defaultPackagePageSize = 48
	maxPackagePageSize     = 200
)

// sortPackages orders the list in place, name is the default sort field
// and asc the default order, the org/repo is the stable tie breaker
func sortPackages(items []packageItem, field, order string) {
	desc := order == "desc"
	less := func(i, j int) bool {
		var a, b string
		switch field {
		case "org":
			a = items[i].Org + "/" + items[i].Repo
			b = items[j].Org + "/" + items[j].Repo
		default:
			a = strings.ToLower(items[i].Name)
			b = strings.ToLower(items[j].Name)
		}
		if a == b {
			// keep a stable order for the tools sharing one name
			a = items[i].Org + "/" + items[i].Repo
			b = items[j].Org + "/" + items[j].Repo
		}
		if desc {
			return a > b
		}
		return a < b
	}
	sort.Slice(items, less)
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

// parsePositiveInt reads a positive integer query parameter, it falls back
// to the given default when the value is missing or invalid
func parsePositiveInt(value string, fallback int) int {
	if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
		return parsed
	}
	return fallback
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
// POST /api/tools/{name}/open launches the tool in a new terminal window
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
		job := s.startJob("upgrade", name, func(stub *cobra.Command) error {
			return s.upgradeTool(stub, tool.Org, tool.Repo, proxy)
		})
		writeJSON(w, http.StatusAccepted, job)
	case r.Method == http.MethodPost && action == "open":
		s.openTool(w, name)
	default:
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "unsupported action")
	}
}

// openTool launches an installed tool in a new terminal window
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
	if err := s.launchTool(binPath); err != nil {
		// #nosec G706 -- the values come from the local install record, they
		// only reach the server side log
		log.Printf("cannot launch %s: %v", name, err)
		writeError(w, http.StatusInternalServerError, "LAUNCH_FAILED",
			fmt.Sprintf("failed to launch %s in a terminal window", name))
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": map[string]string{"name": name, "binary": binPath},
	})
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

// installTool installs a tool by its name, the same way as hd install.
// The version is optional, when set it pins the release tag which covers
// upgrades and downgrades. The proxy is optional, when set the release is
// downloaded through the GitHub acceleration proxy.
func (s *webServer) installTool(stub *cobra.Command, name, version, proxy string, force bool) (err error) {
	opt := s.newInstallOption()
	opt.force = force
	opt.ProxyGitHub = proxy
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
func (s *webServer) upgradeTool(stub *cobra.Command, org, repo, proxy string) error {
	up := &upgradeOption{
		installOption: s.newInstallOption(),
	}
	up.ProxyGitHub = proxy
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
