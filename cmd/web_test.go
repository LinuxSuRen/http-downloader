package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/mitchellh/go-homedir"
	"github.com/stretchr/testify/assert"
)

func setupWebServer(t *testing.T) (*httptest.Server, *webServer) {
	tmpHome := t.TempDir()
	oldCache := homedir.DisableCache
	homedir.DisableCache = true
	t.Setenv("HOME", tmpHome)
	t.Cleanup(func() { homedir.DisableCache = oldCache })

	assert.NoError(t, installer.RecordInstalledPackage(installer.InstalledPackage{
		Name:            "fake-cli",
		Org:             "fake-org",
		Repo:            "fake-cli",
		Version:         "v1.0.0",
		Binaries:        []string{filepath.Join(tmpHome, "bin", "fake-cli")},
		TargetDirectory: filepath.Join(tmpHome, "bin"),
		InstalledAt:     "2026-01-01T00:00:00Z",
	}))
	assert.NoError(t, os.MkdirAll(filepath.Join(tmpHome, "bin"), 0750))
	assert.NoError(t, os.WriteFile(filepath.Join(tmpHome, "bin", "fake-cli"), []byte("bin"), 0750))

	server := newWebServer(context.Background())
	httpServer := httptest.NewServer(server.routes())
	t.Cleanup(httpServer.Close)
	return httpServer, server
}

// seedPkgConfig writes a package config into the local hd-home cache
func seedPkgConfig(t *testing.T, org, repo, content string) {
	t.Helper()
	dir := filepath.Join(homedirHome(t), ".config", "hd-home", "config", org)
	assert.NoError(t, os.MkdirAll(dir, 0750))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, repo+".yml"), []byte(content), 0644))
}

func TestWebProxy(t *testing.T) {
	httpServer, _ := setupWebServer(t)
	client := httpServer.Client()

	// seed the proxy candidates of the local hd-home cache
	assert.NoError(t, os.MkdirAll(filepath.Join(homedirHome(t), ".config", "hd-home"), 0750))
	assert.NoError(t, os.WriteFile(filepath.Join(homedirHome(t), ".config", "hd-home", "proxy.yaml"),
		[]byte("servers:\n- fake-proxy.com\n"), 0644))

	resp, err := client.Get(httpServer.URL + "/api/proxy")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	result := struct {
		Data proxyInfo `json:"data"`
	}{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, []string{"fake-proxy.com"}, result.Data.Servers)

	// method check
	resp, err = client.Post(httpServer.URL+"/api/proxy", "application/json", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

func TestWebAPI(t *testing.T) {
	httpServer, _ := setupWebServer(t)
	client := httpServer.Client()

	// index page
	resp, err := client.Get(httpServer.URL + "/")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))

	// logo of the project
	resp, err = client.Get(httpServer.URL + "/logo.svg")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "image/svg+xml", resp.Header.Get("Content-Type"))
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "<svg")

	// unknown page
	resp, err = client.Get(httpServer.URL + "/nope")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// list tools
	resp, err = client.Get(httpServer.URL + "/api/tools")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	listResult := struct {
		Data []toolItem `json:"data"`
	}{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&listResult))
	assert.Len(t, listResult.Data, 1)
	assert.Equal(t, "fake-cli", listResult.Data[0].Name)
	assert.Equal(t, "v1.0.0", listResult.Data[0].Version)
	assert.True(t, listResult.Data[0].Upgradable)

	// install without a name
	resp, err = client.Post(httpServer.URL+"/api/tools", "application/json",
		strings.NewReader("{}"))
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// upgrade a missing tool
	resp, err = client.Post(httpServer.URL+"/api/tools/none/upgrade", "application/json", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// query a missing job
	resp, err = client.Get(httpServer.URL + "/api/jobs/999")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// remove the tool with its binaries
	req, _ := http.NewRequest(http.MethodDelete, httpServer.URL+"/api/tools/fake-cli", nil)
	resp, err = client.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_, statErr := os.Stat(filepath.Join(homedirHome(t), "bin", "fake-cli"))
	assert.True(t, os.IsNotExist(statErr))

	tools, err := installer.LoadInstalledPackages()
	assert.NoError(t, err)
	assert.Empty(t, tools)
}

func TestWebSearch(t *testing.T) {
	httpServer, server := setupWebServer(t)
	client := httpServer.Client()

	// missing keyword
	resp, err := client.Get(httpServer.URL + "/api/search")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// search against a fake local config cache
	cacheDir := filepath.Join(homedirHome(t), ".config", "hd-home", "config", "fake-org")
	assert.NoError(t, os.MkdirAll(cacheDir, 0750))
	assert.NoError(t, os.WriteFile(filepath.Join(cacheDir, "fake-cli.yml"),
		[]byte("name: fake-cli\n"), 0644))

	// the repo sources of the platform join the answer, their hits
	// address as source/name like on the command line
	server.searchRepos = func(ctx context.Context, configDir, keyword string) []installer.RepoPackage {
		return []installer.RepoPackage{{Source: "chocolatey", Name: "fake-cli", Version: "1.0.0"}}
	}

	resp, err = client.Get(httpServer.URL + "/api/search?q=fake-cli")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	searchResult := struct {
		Data []string `json:"data"`
	}{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&searchResult))
	assert.Contains(t, searchResult.Data, "fake-org/fake-cli")
	assert.Contains(t, searchResult.Data, "chocolatey/fake-cli")
}

func TestWebJobLogsSSE(t *testing.T) {
	httpServer, _ := setupWebServer(t)
	client := httpServer.Client()

	server := newWebServer(context.Background())
	job := &webJob{ID: 1, Action: "install", Tool: "fake-cli", Status: jobRunning}
	job.appendLog("downloading fake-cli")
	server.jobs[1] = job
	sseServer := httptest.NewServer(server.routes())
	t.Cleanup(sseServer.Close)

	// consume the SSE stream after the job is finished
	go func() {
		job.appendLog("installing fake-cli")
		time.Sleep(200 * time.Millisecond)
		job.mu.Lock()
		job.Status = jobSuccess
		job.finished = true
		job.mu.Unlock()
	}()

	resp, err := client.Get(sseServer.URL + "/api/jobs/1/logs")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	deadline := time.Now().Add(5 * time.Second)
	events := ""
	buf := make([]byte, 4096)
	for time.Now().Before(deadline) {
		n, readErr := resp.Body.Read(buf)
		events += string(buf[:n])
		if strings.Contains(events, `"type":"done"`) || readErr != nil {
			break
		}
	}
	assert.Contains(t, events, `"type":"log"`)
	assert.Contains(t, events, "downloading fake-cli")
	assert.Contains(t, events, `"type":"done"`)
	assert.Contains(t, events, `"status":"success"`)

	// job status query
	_ = httpServer
}

func TestWebCategoriesAndPackages(t *testing.T) {
	httpServer, _ := setupWebServer(t)
	client := httpServer.Client()

	seedPkgConfig(t, "fake-org", "fake-cli", `name: fake-cli
categories:
  - ci
  - build
`)
	seedPkgConfig(t, "fake-org", "fake-web", `name: fake-web
homepage: https://example.com/fake-web
categories:
  - web
`)

	// categories are sorted and deduplicated
	resp, err := client.Get(httpServer.URL + "/api/categories")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	categories := struct {
		Data []string `json:"data"`
	}{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&categories))
	assert.Equal(t, []string{"build", "ci", "web"}, categories.Data)

	// the unfiltered view lists everything of the cache, the installed
	// record of fake-cli is part of the cache, so no extra item appears
	resp, err = client.Get(httpServer.URL + "/api/packages")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	packages := struct {
		Data []packageItem `json:"data"`
	}{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&packages))
	assert.Len(t, packages.Data, 2)
	assert.Equal(t, "fake-cli", packages.Data[0].Repo)
	assert.Equal(t, "fake-cli", packages.Data[0].InstalledName)
	assert.Equal(t, "2026-01-01T00:00:00Z", packages.Data[0].InstalledAt)

	// an installed tool which is not part of the cache is merged into the
	// unfiltered view, so installed and not-installed share one list
	assert.NoError(t, installer.RecordInstalledPackage(installer.InstalledPackage{
		Name:    "lonely-cli",
		Org:     "ghost-org",
		Repo:    "lonely-cli",
		Version: "v0.2.0",
	}))
	resp, err = client.Get(httpServer.URL + "/api/packages")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	packages = struct {
		Data []packageItem `json:"data"`
	}{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&packages))
	assert.Len(t, packages.Data, 3)
	assert.Equal(t, "ghost-org/lonely-cli", packages.Data[2].Org+"/"+packages.Data[2].Repo)
	assert.True(t, packages.Data[2].Installed)
	assert.Equal(t, "lonely-cli", packages.Data[2].Name)
	assert.Equal(t, "v0.2.0", packages.Data[2].Version)

	// packages of a category carry the installed state, the record-only
	// tool does not match the category and is not merged in
	resp, err = client.Get(httpServer.URL + "/api/packages?category=ci")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	packages = struct {
		Data []packageItem `json:"data"`
	}{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&packages))
	assert.Len(t, packages.Data, 1)
	assert.Equal(t, "fake-cli", packages.Data[0].Name)
	assert.Equal(t, "fake-org", packages.Data[0].Org)
	assert.Equal(t, []string{"ci", "build"}, packages.Data[0].Categories)
	assert.True(t, packages.Data[0].Installed)
	assert.Equal(t, "v1.0.0", packages.Data[0].Version)
	assert.False(t, packages.Data[0].Web)

	// a web package has a homepage and nothing to install
	resp, err = client.Get(httpServer.URL + "/api/packages?category=web")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	packages = struct {
		Data []packageItem `json:"data"`
	}{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&packages))
	assert.Len(t, packages.Data, 1)
	assert.Equal(t, "fake-web", packages.Data[0].Name)
	assert.Equal(t, "https://example.com/fake-web", packages.Data[0].Homepage)
	assert.True(t, packages.Data[0].Web)
	assert.False(t, packages.Data[0].Installed)

	// keyword search covers both and stays sorted
	resp, err = client.Get(httpServer.URL + "/api/packages?q=fake")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	packages = struct {
		Data []packageItem `json:"data"`
	}{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&packages))
	assert.Len(t, packages.Data, 2)
	assert.Equal(t, "fake-cli", packages.Data[0].Repo)
	assert.Equal(t, "fake-web", packages.Data[1].Repo)
}

func TestWebPackagesRepoSources(t *testing.T) {
	httpServer, server := setupWebServer(t)
	client := httpServer.Client()

	searched := ""
	server.searchRepos = func(ctx context.Context, configDir, keyword string) []installer.RepoPackage {
		searched = keyword
		return []installer.RepoPackage{
			{Source: "chocolatey", Name: "7zip", Version: "24.8.0", Description: "file archiver"},
			{Source: "chocolatey", Name: "curl", Version: "8.23.0", Description: "transfer data"},
		}
	}

	// the local hd-home cache exists, so the handler never reaches the
	// network for the registry part
	assert.NoError(t, os.MkdirAll(
		filepath.Join(homedirHome(t), ".config", "hd-home", "config"), 0750))

	// an installed repo source package is matched by its source/name
	// record and reports the installed version
	assert.NoError(t, installer.RecordInstalledPackage(installer.InstalledPackage{
		Name:        "curl",
		Version:     "8.9.0",
		Source:      "chocolatey",
		Native:      true,
		InstalledAt: "2026-02-01T00:00:00Z",
	}))

	resp, err := client.Get(httpServer.URL + "/api/packages?q=7zip")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	packages := struct {
		Data []packageItem `json:"data"`
	}{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&packages))
	assert.Equal(t, "7zip", searched)
	assert.Len(t, packages.Data, 2)

	sevenZip := packages.Data[0] // chocolatey/7zip sorts before chocolatey/curl
	assert.True(t, sevenZip.RepoPkg)
	assert.Equal(t, "chocolatey", sevenZip.Org)
	assert.Equal(t, "7zip", sevenZip.Repo)
	assert.Equal(t, "7zip", sevenZip.Name)
	assert.Equal(t, "24.8.0", sevenZip.Version)
	assert.Equal(t, "file archiver", sevenZip.Description)
	assert.False(t, sevenZip.Installed)

	curl := packages.Data[1]
	assert.True(t, curl.RepoPkg)
	assert.True(t, curl.Installed)
	assert.Equal(t, "8.9.0", curl.Version)
	assert.Equal(t, "curl", curl.InstalledName)
	assert.Equal(t, "2026-02-01T00:00:00Z", curl.InstalledAt)
	assert.True(t, curl.Native)

	// without a keyword the registry list stays to itself, the repo
	// sources only answer keyword searches
	server.searchRepos = func(ctx context.Context, configDir, keyword string) []installer.RepoPackage {
		t.Error("repo sources must stay out of the unfiltered list")
		return nil
	}
	resp, err = client.Get(httpServer.URL + "/api/packages")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	packages = struct {
		Data []packageItem `json:"data"`
	}{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&packages))
	for i := range packages.Data {
		assert.False(t, packages.Data[i].RepoPkg)
	}
}

func TestWebPackageVersions(t *testing.T) {
	httpServer, server := setupWebServer(t)
	server.listVersions = func(org, repo string, count int) ([]versionTag, error) {
		assert.Equal(t, "fake-org", org)
		assert.Equal(t, "fake-cli", repo)
		assert.Equal(t, 20, count)
		return []versionTag{{Tag: "v1.1.0"}, {Tag: "v1.0.0"}}, nil
	}
	client := httpServer.Client()

	resp, err := client.Get(httpServer.URL + "/api/packages/fake-org/fake-cli/versions")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	versions := struct {
		Data []versionTag `json:"data"`
	}{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&versions))
	assert.Equal(t, []versionTag{{Tag: "v1.1.0"}, {Tag: "v1.0.0"}}, versions.Data)

	// an excessive count is clamped
	server.listVersions = func(org, repo string, count int) ([]versionTag, error) {
		assert.Equal(t, 20, count)
		return nil, nil
	}
	resp, err = client.Get(httpServer.URL + "/api/packages/fake-org/fake-cli/versions?count=500")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// unknown sub paths are rejected
	resp, err = client.Get(httpServer.URL + "/api/packages/fake-org/fake-cli/other")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// a GitHub failure becomes a friendly error
	server.listVersions = func(org, repo string, count int) ([]versionTag, error) {
		return nil, errors.New("rate limit")
	}
	resp, err = client.Get(httpServer.URL + "/api/packages/fake-org/fake-cli/versions")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestWebProcessLifecycle(t *testing.T) {
	httpServer, server := setupWebServer(t)
	server.buildCommand = func(binPath string, args []string) *exec.Cmd {
		cmd := exec.Command(binPath, append([]string{"-test.run=TestHelperProcess"}, args...)...)
		cmd.Env = append(os.Environ(), processHelperEnv+"=1", processHelperMode+"="+processHelperModeHang)
		return cmd
	}
	server.probeWeb = func(pid int) (string, bool) {
		return "http://localhost:19800", true
	}
	// point the installed record to the test binary, the helper process
	// inside it acts as the tool
	assert.NoError(t, installer.RecordInstalledPackage(installer.InstalledPackage{
		Name:            "fake-cli",
		Org:             "fake-org",
		Repo:            "fake-cli",
		Version:         "v1.0.0",
		Binaries:        []string{os.Args[0]},
		TargetDirectory: filepath.Join(homedirHome(t), "bin"),
	}))
	client := httpServer.Client()

	// nothing runs yet
	resp, err := client.Get(httpServer.URL + "/api/processes")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	initial := struct {
		Data []procState `json:"data"`
	}{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&initial))
	assert.Empty(t, initial.Data)

	// start the tool as a managed process
	resp, err = client.Post(httpServer.URL+"/api/tools/fake-cli/start", "application/json", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	started := struct {
		Data procState `json:"data"`
	}{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&started))
	assert.Equal(t, "fake-cli", started.Data.Name)
	assert.Equal(t, procRunning, started.Data.Status)
	assert.Greater(t, started.Data.PID, 0)

	// a running tool cannot start twice
	resp, err = client.Post(httpServer.URL+"/api/tools/fake-cli/start", "application/json", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)

	// the HTTP endpoint of the tool is detected automatically
	var detected procState
	assert.Eventually(t, func() bool {
		resp, err := client.Get(httpServer.URL + "/api/processes")
		assert.NoError(t, err)
		list := struct {
			Data []procState `json:"data"`
		}{}
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&list))
		if len(list.Data) == 1 && list.Data[0].URL != "" {
			detected = list.Data[0]
			return true
		}
		return false
	}, 5*time.Second, 200*time.Millisecond)
	assert.Equal(t, "http://localhost:19800", detected.URL)

	// the process output is captured
	resp, err = client.Get(httpServer.URL + "/api/processes/fake-cli/logs")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	logs := struct {
		Logs []string `json:"logs"`
	}{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&logs))
	assert.Contains(t, strings.Join(logs.Logs, "\n"), "fake tool is running")

	// stop the tool
	resp, err = client.Post(httpServer.URL+"/api/tools/fake-cli/stop", "application/json", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Eventually(t, func() bool {
		proc := server.findProc("fake-cli")
		return proc != nil && proc.status() == procStopped
	}, 5*time.Second, 200*time.Millisecond)
	assert.Equal(t, procStopped, server.findProc("fake-cli").snapshot().Status)

	// stopped processes are not retained in the list
	assert.Eventually(t, func() bool {
		resp, err := client.Get(httpServer.URL + "/api/processes")
		assert.NoError(t, err)
		list := struct {
			Data []procState `json:"data"`
		}{}
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&list))
		return len(list.Data) == 0
	}, 5*time.Second, 200*time.Millisecond)

	// stopping again is a no-op which answers the current state
	resp, err = client.Post(httpServer.URL+"/api/tools/fake-cli/stop", "application/json", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// unknown tools and missing binaries
	resp, err = client.Post(httpServer.URL+"/api/tools/none/start", "application/json", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp, err = client.Post(httpServer.URL+"/api/tools/none/stop", "application/json", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.NoError(t, installer.RecordInstalledPackage(installer.InstalledPackage{
		Name:     "ghost-cli",
		Org:      "fake-org",
		Repo:     "ghost-cli",
		Binaries: []string{filepath.Join(homedirHome(t), "bin", "ghost-cli")},
	}))
	resp, err = client.Post(httpServer.URL+"/api/tools/ghost-cli/start", "application/json", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// start is POST only
	resp, err = client.Get(httpServer.URL + "/api/tools/fake-cli/start")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// the log endpoint of an unknown process
	resp, err = client.Get(httpServer.URL + "/api/processes/missing/logs")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestWebProcessExit(t *testing.T) {
	httpServer, server := setupWebServer(t)
	server.buildCommand = func(binPath string, args []string) *exec.Cmd {
		cmd := exec.Command(binPath, append([]string{"-test.run=TestHelperProcess"}, args...)...)
		cmd.Env = append(os.Environ(), processHelperEnv+"=1", processHelperMode+"="+processHelperModeFail)
		return cmd
	}
	server.probeWeb = func(pid int) (string, bool) { return "", false }
	assert.NoError(t, installer.RecordInstalledPackage(installer.InstalledPackage{
		Name:     "fake-cli",
		Org:      "fake-org",
		Repo:     "fake-cli",
		Binaries: []string{os.Args[0]},
	}))
	client := httpServer.Client()

	resp, err := client.Post(httpServer.URL+"/api/tools/fake-cli/start", "application/json", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// a tool which exits by itself is reported with its exit code
	assert.Eventually(t, func() bool {
		proc := server.findProc("fake-cli")
		if proc == nil || proc.status() != procExited {
			return false
		}
		state := proc.snapshot()
		return state.ExitCode != nil && *state.ExitCode == 3
	}, 5*time.Second, 200*time.Millisecond)

	// exited processes are not retained in the list either
	resp, err = client.Get(httpServer.URL + "/api/processes")
	assert.NoError(t, err)
	final := struct {
		Data []procState `json:"data"`
	}{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&final))
	assert.Empty(t, final.Data)
}

func TestWebLaunchCommand(t *testing.T) {
	httpServer, server := setupWebServer(t)
	var gotArgs []string
	server.buildCommand = func(binPath string, args []string) *exec.Cmd {
		gotArgs = args
		cmd := exec.Command(binPath, append([]string{"-test.run=TestHelperProcess"}, args...)...)
		cmd.Env = append(os.Environ(), processHelperEnv+"=1", processHelperMode+"="+processHelperModeFail)
		return cmd
	}
	server.probeWeb = func(pid int) (string, bool) { return "", false }
	assert.NoError(t, installer.RecordInstalledPackage(installer.InstalledPackage{
		Name:     "fake-cli",
		Org:      "fake-org",
		Repo:     "fake-cli",
		Binaries: []string{os.Args[0]},
	}))
	seedPkgConfig(t, "fake-org", "fake-cli", "name: fake-cli\n")
	client := httpServer.Client()

	put := func(body string) *http.Response {
		req, _ := http.NewRequest(http.MethodPut, httpServer.URL+"/api/tools/fake-cli/command",
			strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		assert.NoError(t, err)
		return resp
	}

	// save a launch command, the leading tool name stays as typed
	resp := put(`{"command":"fake-cli server --port 8080"}`)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	// the tool list and the package list carry the command
	resp, err := client.Get(httpServer.URL + "/api/tools")
	assert.NoError(t, err)
	tools := struct {
		Data []toolItem `json:"data"`
	}{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&tools))
	assert.Len(t, tools.Data, 1)
	assert.Equal(t, "fake-cli server --port 8080", tools.Data[0].LaunchCommand)

	resp, err = client.Get(httpServer.URL + "/api/packages")
	assert.NoError(t, err)
	packages := struct {
		Data []packageItem `json:"data"`
	}{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&packages))
	assert.Len(t, packages.Data, 1)
	assert.Equal(t, "fake-cli server --port 8080", packages.Data[0].LaunchCommand)

	// unbalanced quotes and a broken body are rejected
	resp = put(`{"command":"a 'unclosed"}`)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	_ = resp.Body.Close()
	resp = put(`not-json`)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	_ = resp.Body.Close()

	// unknown tool
	req, _ := http.NewRequest(http.MethodPut, httpServer.URL+"/api/tools/none/command",
		strings.NewReader(`{"command":"x"}`))
	resp, err = client.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	_ = resp.Body.Close()

	// starting the tool passes the command, the tool name prefix is dropped
	resp, err = client.Post(httpServer.URL+"/api/tools/fake-cli/start", "application/json", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
	assert.Equal(t, []string{"server", "--port", "8080"}, gotArgs)
	assert.Eventually(t, func() bool {
		proc := server.findProc("fake-cli")
		return proc != nil && proc.status() == procExited
	}, 5*time.Second, 200*time.Millisecond)

	// clearing the command persists too
	resp = put(`{"command":""}`)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
	tool, _ := installer.FindInstalledPackage("fake-cli")
	assert.Equal(t, "", tool.LaunchCommand)
}

func TestFindToolBinary(t *testing.T) {
	server := newWebServer(context.Background())

	// the recorded binary path wins
	bin := filepath.Join(t.TempDir(), "fake-cli")
	assert.NoError(t, os.WriteFile(bin, []byte("bin"), 0750))
	found := server.findToolBinary(&installer.InstalledPackage{Name: "fake-cli", Binaries: []string{bin}})
	assert.Equal(t, bin, found)

	// nothing matches an empty record
	assert.Equal(t, "", server.findToolBinary(&installer.InstalledPackage{Name: "hd-no-such-tool"}))

	// a fallback install lands in the working directory, the console
	// finds it there no matter which platform runs
	workDir := t.TempDir()
	t.Chdir(workDir)
	name := "cwd-tool"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	assert.NoError(t, os.WriteFile(filepath.Join(workDir, name), []byte("bin"), 0750))
	found = server.findToolBinary(&installer.InstalledPackage{Name: "cwd-tool"})
	assert.Equal(t, name, filepath.Base(found))

	// older records carry no Windows executable suffix, the lookup
	// still finds the file next to the recorded directory
	if runtime.GOOS == "windows" {
		dir := t.TempDir()
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "legacy.exe"), []byte("bin"), 0750))
		found = server.findToolBinary(&installer.InstalledPackage{Name: "legacy", TargetDirectory: dir})
		assert.Equal(t, filepath.Join(dir, "legacy.exe"), found)
	}
}

// TestHelperProcess is not a real test, it is the tool process of the web
// console tests, see TestWebProcessLifecycle
func TestHelperProcess(t *testing.T) {
	if os.Getenv(processHelperEnv) != "1" {
		return
	}
	switch os.Getenv(processHelperMode) {
	case processHelperModeHang:
		fmt.Println("fake tool is running")
		time.Sleep(time.Minute)
	case processHelperModeFail:
		os.Exit(3)
	}
	os.Exit(0)
}

func homedirHome(t *testing.T) string {
	home, err := homedir.Dir()
	assert.NoError(t, err)
	return home
}
