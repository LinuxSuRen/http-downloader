package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestWebAPI(t *testing.T) {
	httpServer, _ := setupWebServer(t)
	client := httpServer.Client()

	// index page
	resp, err := client.Get(httpServer.URL + "/")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))

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
		Data []string `json:"data"`
	}{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&searchResult))
	assert.Contains(t, searchResult.Data, "fake-org/fake-cli")
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

	// missing both the category and the keyword
	resp, err = client.Get(httpServer.URL + "/api/packages")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// packages of a category carry the installed state
	resp, err = client.Get(httpServer.URL + "/api/packages?category=ci")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	packages := struct {
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

func TestWebOpenTool(t *testing.T) {
	httpServer, server := setupWebServer(t)
	launched := ""
	server.launchTool = func(binPath string) error {
		launched = binPath
		return nil
	}
	client := httpServer.Client()

	// open the installed tool with its binary
	resp, err := client.Post(httpServer.URL+"/api/tools/fake-cli/open", "application/json", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	opened := struct {
		Data struct {
			Name   string `json:"name"`
			Binary string `json:"binary"`
		} `json:"data"`
	}{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&opened))
	assert.Equal(t, "fake-cli", opened.Data.Name)
	assert.Equal(t, filepath.Join(homedirHome(t), "bin", "fake-cli"), opened.Data.Binary)
	assert.Equal(t, opened.Data.Binary, launched)

	// open a tool which is not installed
	resp, err = client.Post(httpServer.URL+"/api/tools/none/open", "application/json", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// open an installed tool whose binary is gone
	assert.NoError(t, installer.RecordInstalledPackage(installer.InstalledPackage{
		Name:            "ghost-cli",
		Org:             "fake-org",
		Repo:            "ghost-cli",
		Binaries:        []string{filepath.Join(homedirHome(t), "bin", "ghost-cli")},
		TargetDirectory: filepath.Join(homedirHome(t), "bin"),
	}))
	resp, err = client.Post(httpServer.URL+"/api/tools/ghost-cli/open", "application/json", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// open is POST only
	resp, err = client.Get(httpServer.URL + "/api/tools/fake-cli/open")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func homedirHome(t *testing.T) string {
	home, err := homedir.Dir()
	assert.NoError(t, err)
	return home
}
