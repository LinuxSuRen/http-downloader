package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/linuxsuren/http-downloader/pkg/npmregistry"
	"github.com/linuxsuren/http-downloader/pkg/remote"
	"github.com/linuxsuren/http-downloader/pkg/source"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
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
	// the npm registry is an external service, keep the tests offline by
	// default and let specific tests override this stub
	server.searchNPM = func(context.Context, string, int) ([]npmregistry.Package, error) {
		return nil, nil
	}
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
	httpServer, _ := setupWebServer(t)
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

	resp, err = client.Get(httpServer.URL + "/api/search?q=fake-cli")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	searchResult := struct {
		Data []source.Result `json:"data"`
	}{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&searchResult))
	assert.Contains(t, searchResult.Data, source.Result{
		Source: source.HDHome,
		Org:    "fake-org",
		Repo:   "fake-cli",
		Name:   "fake-cli",
	})
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

func TestWebSettings(t *testing.T) {
	resetViper(t)
	httpServer, _ := setupWebServer(t)
	client := httpServer.Client()

	// the defaults are platform aware and always include the install mode
	resp, err := client.Get(httpServer.URL + "/api/settings")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	settings := struct {
		Data settingsInfo `json:"data"`
	}{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&settings))
	assert.Empty(t, settings.Data.InstallDir)
	assert.Equal(t, remote.ModeLocal, settings.Data.Mode)
	assert.Equal(t, 22, settings.Data.SSHPort)

	// update the shared config through the web console
	body := `{"install_dir":"D:\\hd\\bin","mode":"ssh","ssh_host":"example.com",` +
		`"ssh_port":2222,"ssh_user":"hd","ssh_password":"secret",` +
		`"ssh_os":"linux","ssh_arch":"arm64"}`
	req, err := http.NewRequest(http.MethodPut, httpServer.URL+"/api/settings", strings.NewReader(body))
	assert.NoError(t, err)
	resp, err = client.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	resp, err = client.Get(httpServer.URL + "/api/settings")
	assert.NoError(t, err)
	settings = struct {
		Data settingsInfo `json:"data"`
	}{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&settings))
	assert.Equal(t, `D:\hd\bin`, settings.Data.InstallDir)
	assert.Equal(t, remote.ModeSSH, settings.Data.Mode)
	assert.Equal(t, "example.com", settings.Data.SSHHost)
	assert.Equal(t, 2222, settings.Data.SSHPort)
	assert.True(t, settings.Data.HasPassword)
	// passwords are never returned by the API
	data, err := os.ReadFile(filepath.Join(homedirHome(t), ".config", "hd.yaml"))
	assert.NoError(t, err)
	assert.Contains(t, string(data), "install-dir")
}

func TestWebSettingsRejectsInvalidMode(t *testing.T) {
	resetViper(t)
	httpServer, _ := setupWebServer(t)
	client := httpServer.Client()

	req, _ := http.NewRequest(http.MethodPut, httpServer.URL+"/api/settings",
		strings.NewReader(`{"mode":"telnet"}`))
	resp, err := client.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestWebSearchSourceLabels(t *testing.T) {
	httpServer, server := setupWebServer(t)
	server.searchNPM = func(_ context.Context, keyword string, size int) ([]npmregistry.Package, error) {
		assert.Equal(t, "fake", keyword)
		return []npmregistry.Package{{
			Name:        "fake-npm-pkg",
			Version:     "1.2.3",
			Description: "fake npm package",
		}}, nil
	}
	client := httpServer.Client()

	// all sources mixes hd-home configs with npm packages
	cacheDir := filepath.Join(homedirHome(t), ".config", "hd-home", "config", "fake-org")
	assert.NoError(t, os.MkdirAll(cacheDir, 0750))
	assert.NoError(t, os.WriteFile(filepath.Join(cacheDir, "fake-cli.yml"),
		[]byte("name: fake-cli\n"), 0644))

	resp, err := client.Get(httpServer.URL + "/api/search?q=fake")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	result := struct {
		Data []source.Result `json:"data"`
	}{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Contains(t, result.Data, source.Result{
		Source: source.HDHome, Org: "fake-org", Repo: "fake-cli", Name: "fake-cli",
	})
	assert.Contains(t, result.Data, source.Result{
		Source: source.NPM, Name: "fake-npm-pkg", Version: "1.2.3", Description: "fake npm package",
	})

	// the source filter is honoured
	resp, err = client.Get(httpServer.URL + "/api/search?q=fake&source=npm")
	assert.NoError(t, err)
	result = struct {
		Data []source.Result `json:"data"`
	}{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Len(t, result.Data, 1)
	assert.Equal(t, source.NPM, result.Data[0].Source)

	resp, err = client.Get(httpServer.URL + "/api/search?q=fake&source=unknown")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestWebPackagesSourceFilter(t *testing.T) {
	httpServer, server := setupWebServer(t)
	server.searchNPM = func(context.Context, string, int) ([]npmregistry.Package, error) {
		return []npmregistry.Package{{Name: "fake-npm-pkg", Version: "1.0.0"}}, nil
	}
	client := httpServer.Client()

	resp, err := client.Get(httpServer.URL + "/api/packages?source=npm&q=fake")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	packages := struct {
		Data []packageItem `json:"data"`
	}{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&packages))
	assert.Len(t, packages.Data, 1)
	assert.Equal(t, source.NPM, packages.Data[0].Source)
	assert.Equal(t, "fake-npm-pkg", packages.Data[0].Name)
}

func TestWebToolsInstallSource(t *testing.T) {
	resetViper(t)
	httpServer, server := setupWebServer(t)
	client := httpServer.Client()

	// an invalid source is rejected before a job is started
	resp, err := client.Post(httpServer.URL+"/api/tools", "application/json",
		strings.NewReader(`{"name":"fake","source":"unknown"}`))
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// npm packages are accepted and labelled in the job, stub the executor
	// so no real npm command runs
	server.runJob = func(action, name string, _ func(*cobra.Command) error) *webJob {
		return &webJob{ID: 1, Action: action, Tool: name, Status: jobSuccess}
	}
	resp, err = client.Post(httpServer.URL+"/api/tools", "application/json",
		strings.NewReader(`{"name":"chalk","source":"npm"}`))
	assert.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	job := webJob{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&job))
	assert.Equal(t, "install", job.Action)
	assert.Equal(t, "chalk", job.Tool)
}

func busyPortError() error {
	return &net.OpError{Op: "listen", Net: "tcp", Err: syscall.EADDRINUSE}
}

func TestListenWithDrift(t *testing.T) {
	free, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	t.Cleanup(func() { _ = free.Close() })

	var requested []string
	opt := &webOption{
		Listen:            "127.0.0.1",
		Port:              39000,
		PortDrift:         true,
		PortDriftAttempts: 4,
		listen: func(network, address string) (net.Listener, error) {
			assert.Equal(t, "tcp", network)
			requested = append(requested, address)
			if len(requested) < 3 {
				return nil, busyPortError()
			}
			return free, nil
		},
	}
	listener, port, err := opt.listenWithDrift()
	assert.NoError(t, err)
	assert.Equal(t, 39002, port)
	assert.Equal(t, free, listener)
	assert.Equal(t, []string{
		"127.0.0.1:39000", "127.0.0.1:39001", "127.0.0.1:39002",
	}, requested)
}

func TestListenWithDriftDisabled(t *testing.T) {
	calls := 0
	opt := &webOption{
		Listen: "127.0.0.1",
		Port:   39000,
		listen: func(string, string) (net.Listener, error) {
			calls++
			return nil, busyPortError()
		},
	}
	_, _, err := opt.listenWithDrift()
	assert.ErrorIs(t, err, syscall.EADDRINUSE)
	assert.Equal(t, 1, calls, "a disabled drift must not try other ports")
}

func TestListenWithDriftErrors(t *testing.T) {
	t.Run("unrelated errors are not retried", func(t *testing.T) {
		boom := errors.New("boom")
		calls := 0
		opt := &webOption{
			Listen:    "127.0.0.1",
			Port:      39000,
			PortDrift: true,
			listen: func(string, string) (net.Listener, error) {
				calls++
				return nil, boom
			},
		}
		_, _, err := opt.listenWithDrift()
		assert.ErrorIs(t, err, boom)
		assert.Equal(t, 1, calls)
	})

	t.Run("all ports busy", func(t *testing.T) {
		opt := &webOption{
			Listen:            "127.0.0.1",
			Port:              39000,
			PortDrift:         true,
			PortDriftAttempts: 2,
			listen: func(string, string) (net.Listener, error) {
				return nil, busyPortError()
			},
		}
		_, _, err := opt.listenWithDrift()
		assert.ErrorContains(t, err, "no available port in range 39000-39001")
	})

	t.Run("invalid port", func(t *testing.T) {
		opt := &webOption{Listen: "127.0.0.1", Port: 0}
		_, _, err := opt.listenWithDrift()
		assert.ErrorContains(t, err, "invalid port")
	})

	t.Run("invalid attempt count falls back to a single try", func(t *testing.T) {
		calls := 0
		opt := &webOption{
			Listen:            "127.0.0.1",
			Port:              39000,
			PortDrift:         true,
			PortDriftAttempts: -1,
			listen: func(string, string) (net.Listener, error) {
				calls++
				return nil, busyPortError()
			},
		}
		_, _, err := opt.listenWithDrift()
		assert.Error(t, err)
		assert.Equal(t, 1, calls)
	})
}

func TestWebCmdPortDrift(t *testing.T) {
	free, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	t.Cleanup(func() { _ = free.Close() })

	server := newWebServer(context.Background())
	var served *http.Server
	opt := &webOption{
		Listen:            "127.0.0.1",
		Port:              39000,
		PortDrift:         true,
		PortDriftAttempts: 2,
		server:            server,
		listen: func(_, address string) (net.Listener, error) {
			if address == "127.0.0.1:39000" {
				return nil, busyPortError()
			}
			return free, nil
		},
		serve: func(s *http.Server, _ net.Listener) error {
			served = s
			return nil
		},
	}
	cmd := &cobra.Command{}
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	assert.NoError(t, opt.runE(cmd, nil))
	assert.Equal(t, 39001, server.selfPort, "the console must not detect itself as a tool")
	assert.Contains(t, buf.String(), "port 39000 is in use, drifting to 39001")
	assert.Contains(t, buf.String(), "web server is running at http://")
	assert.NotNil(t, served)
	assert.NotZero(t, served.ReadHeaderTimeout)
}

func TestNewWebCmdPortDriftFlags(t *testing.T) {
	cmd := newWebCmd(context.Background())
	for _, name := range []string{"port-drift", "port-drift-attempts"} {
		assert.NotNil(t, cmd.Flags().Lookup(name), name)
	}
	assert.Equal(t, "true", cmd.Flags().Lookup("port-drift").DefValue)
	assert.Equal(t, "100", cmd.Flags().Lookup("port-drift-attempts").DefValue)
}
