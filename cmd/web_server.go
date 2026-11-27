package cmd

import (
	"context"
	"encoding/json"
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

	fakeruntime "github.com/linuxsuren/go-fake-runtime"
	hdpkg "github.com/linuxsuren/http-downloader/pkg"
	"github.com/linuxsuren/http-downloader/pkg/common"
	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/linuxsuren/http-downloader/pkg/npmregistry"
	"github.com/linuxsuren/http-downloader/pkg/remote"
	"github.com/linuxsuren/http-downloader/pkg/source"
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

	// procMu guards procs, the map of the tool processes which were
	// started from the web console
	procMu sync.Mutex
	procs  map[string]*managedProc
	// selfPort carries the port of the console itself, the web service
	// detection must not report it as the endpoint of a tool
	selfPort int

	// overridable behaviors, they talk to GitHub or to the desktop, which
	// is neither available nor desired in the unit tests
	listVersions func(org, repo string, count int) ([]versionTag, error)
	probeWeb     func(pid int) (url string, ok bool)
	buildCommand func(binPath string, args []string) *exec.Cmd
	// searchNPM is replaceable so the web tests do not need network access
	searchNPM func(ctx context.Context, keyword string, size int) ([]npmregistry.Package, error)
	// runJob is replaceable so the install API can be tested without
	// performing a real download or npm install
	runJob func(action, name string, fn func(stub *cobra.Command) error) *webJob
}

func newWebServer(ctx context.Context) *webServer {
	server := &webServer{
		ctx:     ctx,
		execer:  fakeruntime.NewDefaultExecer(),
		jobs:    map[int]*webJob{},
		jobLock: make(chan struct{}, 1),
		procs:   map[string]*managedProc{},
	}
	server.listVersions = listGitHubVersions
	server.probeWeb = server.detectWebURL
	server.searchNPM = func(ctx context.Context, keyword string, size int) ([]npmregistry.Package, error) {
		return npmregistry.Search(ctx, keyword, size, nil, npmregistry.DefaultBaseURL)
	}
	server.buildCommand = func(binPath string, args []string) *exec.Cmd {
		// #nosec G702 -- the path points to an existing binary of the
		// local install record, the console binds to 127.0.0.1 only
		return exec.Command(binPath, args...)
	}
	return server
}

func (s *webServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/logo.svg", s.handleLogo)
	mux.HandleFunc("/api/tools", s.handleTools)
	mux.HandleFunc("/api/tools/", s.handleTool)
	mux.HandleFunc("/api/search", s.handleSearch)
	mux.HandleFunc("/api/fetch", s.handleFetch)
	mux.HandleFunc("/api/jobs/", s.handleJob)
	mux.HandleFunc("/api/categories", s.handleCategories)
	mux.HandleFunc("/api/packages", s.handlePackages)
	mux.HandleFunc("/api/packages/", s.handlePackageVersions)
	mux.HandleFunc("/api/proxy", s.handleProxy)
	mux.HandleFunc("/api/settings", s.handleSettings)
	mux.HandleFunc("/api/processes", s.handleProcesses)
	mux.HandleFunc("/api/processes/", s.handleProcesses)
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
// POST /api/tools {"name":"jcli","version":"v1.2.3","proxy":"gh.api...","force":true,"source":"hd-home"}
// starts an install job, the optional version pins the release tag, so it
// also covers upgrades and downgrades, the optional proxy downloads the
// release through a GitHub acceleration proxy. source selects hd-home, npm
// or a native package, ssh_password can be supplied for a one-off SSH target.
func (s *webServer) handleTools(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": s.loadTools()})
	case http.MethodPost:
		var req struct {
			Name        string `json:"name"`
			Version     string `json:"version"`
			Proxy       string `json:"proxy"`
			Force       bool   `json:"force"`
			Source      string `json:"source"`
			SSHPassword string `json:"ssh_password"`
		}
		if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.Name) == "" {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "please provide the tool name")
			return
		}
		sourceName := strings.TrimSpace(req.Source)
		if sourceName == "" {
			sourceName = source.HDHome
		}
		if !source.Known(sourceName) || sourceName == source.All {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid package source")
			return
		}
		version := strings.TrimSpace(req.Version)
		proxy := strings.TrimSpace(req.Proxy)
		target := strings.TrimSpace(req.Name)
		if version != "" && sourceName == source.HDHome && !strings.Contains(target, "@") {
			target = fmt.Sprintf("%s@%s", target, version)
		}
		job := s.startJob("install", req.Name, func(stub *cobra.Command) error {
			return s.installTool(stub, target, version, proxy, req.Force, sourceName, req.SSHPassword)
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

// settingsInfo is the API view of the default install settings. The SSH
// password is never returned, only a flag telling whether one is configured.
type settingsInfo struct {
	InstallDir  string `json:"install_dir"`
	Mode        string `json:"mode"`
	SSHHost     string `json:"ssh_host"`
	SSHPort     int    `json:"ssh_port"`
	SSHUser     string `json:"ssh_user"`
	SSHKey      string `json:"ssh_key"`
	SSHOS       string `json:"ssh_os"`
	SSHArch     string `json:"ssh_arch"`
	HasPassword bool   `json:"has_password"`
}

func currentSettings() settingsInfo {
	cfg := loadRemoteConfig()
	return settingsInfo{
		InstallDir:  configuredInstallDir(),
		Mode:        cfg.Mode,
		SSHHost:     cfg.Host,
		SSHPort:     cfg.Port,
		SSHUser:     cfg.User,
		SSHKey:      cfg.KeyFile,
		SSHOS:       cfg.OS,
		SSHArch:     cfg.Arch,
		HasPassword: cfg.Password != "",
	}
}

// GET /api/settings returns the install defaults, PUT updates hd.yaml
func (s *webServer) handleSettings(w http.ResponseWriter, r *http.Request) {
	v := viper.GetViper()
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": currentSettings()})
	case http.MethodPut:
		var req struct {
			InstallDir  *string `json:"install_dir"`
			Mode        *string `json:"mode"`
			SSHHost     *string `json:"ssh_host"`
			SSHPort     *int    `json:"ssh_port"`
			SSHUser     *string `json:"ssh_user"`
			SSHPassword *string `json:"ssh_password"`
			SSHKey      *string `json:"ssh_key"`
			SSHOS       *string `json:"ssh_os"`
			SSHArch     *string `json:"ssh_arch"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "cannot parse the settings")
			return
		}
		if req.InstallDir != nil {
			v.Set(keyInstallDir, strings.TrimSpace(*req.InstallDir))
		}
		cfg := loadRemoteConfig()
		if req.Mode != nil {
			cfg.Mode = strings.TrimSpace(*req.Mode)
		}
		if req.SSHHost != nil {
			cfg.Host = strings.TrimSpace(*req.SSHHost)
		}
		if req.SSHPort != nil {
			cfg.Port = *req.SSHPort
		}
		if req.SSHUser != nil {
			cfg.User = strings.TrimSpace(*req.SSHUser)
		}
		if req.SSHPassword != nil {
			cfg.Password = *req.SSHPassword
		}
		if req.SSHKey != nil {
			cfg.KeyFile = strings.TrimSpace(*req.SSHKey)
		}
		if req.SSHOS != nil {
			cfg.OS = strings.TrimSpace(*req.SSHOS)
		}
		if req.SSHArch != nil {
			cfg.Arch = strings.TrimSpace(*req.SSHArch)
		}
		cfg = cfg.Normalize()
		if cfg.Mode != remote.ModeLocal && cfg.Mode != remote.ModeSSH {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid install mode")
			return
		}
		if cfg.Mode == remote.ModeSSH {
			if err := cfg.Validate(); err != nil {
				writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
				return
			}
		}
		saveRemoteConfig(cfg)
		path, err := writeHDConfig(v)
		if err != nil {
			log.Printf("cannot save the settings: %v", err)
			writeError(w, http.StatusInternalServerError, "SETTINGS_FAILED", "failed to save the settings")
			return
		}
		log.Println("settings saved into", path)
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": currentSettings()})
	default:
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "only GET and PUT are supported")
	}
}

// GET /api/search?q=keyword&source=hd-home searches the installable packages
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
	sourceName := strings.TrimSpace(r.URL.Query().Get("source"))
	if sourceName == "" {
		sourceName = source.All
	}
	if !source.Known(sourceName) {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid package source")
		return
	}

	configDir := ""
	if sourceName == source.All || sourceName == source.HDHome {
		var ok bool
		if configDir, ok = s.ensureConfigCache(); !ok {
			writeError(w, http.StatusInternalServerError, "SEARCH_FAILED",
				"failed to load the package index, please try again later")
			return
		}
	}

	result := installer.SearchSources(keyword, configDir, sourceName)
	if sourceName == source.All || sourceName == source.NPM {
		packages, err := s.searchNPM(r.Context(), keyword, 20)
		if err != nil {
			log.Printf("cannot search npm: %v", err)
		}
		result = append(result, npmResults(packages)...)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": result})
}

func npmResults(packages []npmregistry.Package) (result []source.Result) {
	for i := range packages {
		result = append(result, source.Result{
			Source:      source.NPM,
			Name:        packages[i].Name,
			Version:     packages[i].Version,
			Description: packages[i].Description,
			Homepage:    packages[i].Homepage,
		})
	}
	return
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

// packageItem is the API view of an installable package
type packageItem struct {
	Org        string   `json:"org"`
	Repo       string   `json:"repo"`
	Name       string   `json:"name"`
	Source     string   `json:"source"`
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
	// LaunchCommand is the argument string the web console passes to the
	// binary when the tool starts
	LaunchCommand string `json:"launch_command,omitempty"`
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
	sourceName := strings.TrimSpace(r.URL.Query().Get("source"))
	if sourceName == "" {
		sourceName = source.All
	}
	if !source.Known(sourceName) {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid package source")
		return
	}

	includeHDHome := sourceName == source.All || sourceName == source.HDHome
	configDir := ""
	if includeHDHome {
		var ok bool
		if configDir, ok = s.ensureConfigCache(); !ok {
			writeError(w, http.StatusInternalServerError, "PACKAGES_FAILED",
				"failed to load the package index, please try again later")
			return
		}
	}

	var orgRepos []string
	switch {
	case category != "":
		packages := installer.FindPackagesByCategory(category)
		orgRepos = make([]string, 0, len(packages))
		for i := range packages {
			orgRepos = append(orgRepos, packages[i].Org+"/"+packages[i].Repo)
		}
	case keyword != "" && includeHDHome:
		orgRepos = installer.FindByKeyword(keyword, configDir)
	case keyword == "" && includeHDHome:
		orgRepos = installer.FindAllPackages(configDir)
	}

	records := loadInstalledRecords()
	installed := installedIndex(records)
	installedByName := make(map[string]*installer.InstalledPackage, len(records))
	for i := range records {
		if records[i].Source == source.NPM || records[i].Source == source.Native {
			installedByName[records[i].Name] = &records[i]
		}
	}

	seen := make(map[string]bool, len(orgRepos))
	items := make([]packageItem, 0, len(orgRepos)+len(records))
	for _, orgRepo := range orgRepos {
		org, repo, valid := splitOrgRepo(orgRepo)
		if !valid {
			continue
		}

		item := packageItem{Org: org, Repo: repo, Name: repo, Source: source.HDHome}
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
			applyRecord(&item, record)
		}
		seen[orgRepo] = true
		items = append(items, item)
	}

	// the unfiltered view also carries the installed tools which are not
	// part of the hd-home cache, so nothing installed is lost from the
	// merged list
	if category == "" && keyword == "" && sourceName == source.All {
		for i := range records {
			record := records[i]
			if record.Org != "" && record.Repo != "" && seen[record.Org+"/"+record.Repo] {
				continue
			}

			items = append(items, packageItem{
				Org:           record.Org,
				Repo:          record.Repo,
				Name:          record.Name,
				Source:        installer.SourceOfInstalled(record.Source),
				Installed:     true,
				Version:       record.Version,
				InstalledName: record.Name,
				InstalledAt:   record.InstalledAt,
				Native:        record.Native,
				LaunchCommand: record.LaunchCommand,
			})
		}
	}

	// npm has a huge catalog, only query it when there is a keyword to
	// search for
	if keyword != "" && (sourceName == source.NPM || sourceName == source.All) {
		packages, err := s.searchNPM(r.Context(), keyword, 20)
		if err != nil {
			log.Printf("cannot search npm: %v", err)
		}
		for _, result := range npmResults(packages) {
			item := packageItem{
				Name:    result.Name,
				Source:  source.NPM,
				Version: result.Version,
			}
			if result.Homepage != "" {
				item.Homepage = result.Homepage
			} else {
				item.Homepage = "https://www.npmjs.com/package/" + result.Name
			}
			if record := installedByName[result.Name]; record != nil {
				applyRecord(&item, record)
			}
			items = append(items, item)
		}
	}

	if sourceName == source.Native || (sourceName == source.All && keyword != "") {
		for _, result := range installer.SearchSources(keyword, configDir, source.Native) {
			item := packageItem{Name: result.Name, Source: source.Native, Native: true}
			if record := installedByName[result.Name]; record != nil {
				applyRecord(&item, record)
			}
			items = append(items, item)
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Source != items[j].Source {
			return items[i].Source < items[j].Source
		}
		return items[i].Org+"/"+items[i].Repo+items[i].Name <
			items[j].Org+"/"+items[j].Repo+items[j].Name
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items})
}

func applyRecord(item *packageItem, record *installer.InstalledPackage) {
	item.Installed = true
	item.Version = record.Version
	item.InstalledName = record.Name
	item.InstalledAt = record.InstalledAt
	item.LaunchCommand = record.LaunchCommand
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
// POST /api/tools/{name}/start starts the tool as a managed process
// POST /api/tools/{name}/stop stops the managed process of the tool
// PUT /api/tools/{name}/command saves the launch command of the tool
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
			if tool.Source == source.NPM {
				return s.installTool(stub, tool.Name, "", proxy, true, source.NPM, "")
			}
			return s.upgradeTool(stub, tool.Org, tool.Repo, proxy)
		})
		writeJSON(w, http.StatusAccepted, job)
	case r.Method == http.MethodPost && action == "start":
		s.startTool(w, name)
	case r.Method == http.MethodPost && action == "stop":
		s.stopTool(w, name)
	case r.Method == http.MethodPut && action == "command":
		s.setLaunchCommand(w, r, name)
	default:
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "unsupported action")
	}
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
	if s.runJob != nil {
		return s.runJob(action, name, fn)
	}
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
			Name:          pkg.Name,
			Version:       pkg.Version,
			Org:           pkg.Org,
			Repo:          pkg.Repo,
			Source:        installer.SourceOfInstalled(pkg.Source),
			Mode:          pkg.Mode,
			Host:          pkg.Host,
			InstalledAt:   pkg.InstalledAt,
			Directory:     pkg.TargetDirectory,
			Upgradable:    (pkg.Org != "" && pkg.Repo != "" || pkg.Source == source.NPM) && !pkg.Native,
			LaunchCommand: pkg.LaunchCommand,
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
	opt.sourceName = source.HDHome
	opt.target = configuredInstallDir()
	opt.targetConfig = loadRemoteConfig()
	// -1 means keeping the system default file permission, zero would
	// make the downloaded package unreadable
	opt.Mod = -1
	return opt
}

// installTool installs a tool by its name, the same way as hd install.
// The version is optional, when set it pins the release tag which covers
// upgrades and downgrades. The proxy is optional, when set the release is
// downloaded through the GitHub acceleration proxy. source selects the
// package registry and ssh_password is only used for a one-off SSH target.
func (s *webServer) installTool(stub *cobra.Command, name, version, proxy string, force bool,
	sourceName, sshPassword string) (err error) {
	opt := s.newInstallOption()
	opt.force = force
	opt.ProxyGitHub = proxy
	opt.sourceName = sourceName
	if sshPassword != "" && opt.targetConfig.Mode == remote.ModeSSH {
		opt.targetConfig.Password = sshPassword
	}
	if err = opt.preRunE(stub, []string{name}); err != nil {
		return
	}
	if err = opt.install(stub, []string{name}); err != nil {
		return
	}
	if version != "" && sourceName == source.HDHome {
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
	tool, _ := installer.FindInstalledPackage(name)
	if tool != nil && tool.Source == source.NPM {
		if err = s.uninstallNPM(tool); err != nil {
			return
		}
	}
	opt := &removeOption{execer: s.execer}
	stub := &cobra.Command{Use: "hd"}
	stub.SetOut(io.Discard)
	stub.SetErr(io.Discard)
	return opt.runE(stub, []string{name})
}

func (s *webServer) uninstallNPM(tool *installer.InstalledPackage) error {
	return uninstallNPMPackage(tool, s.execer)
}
