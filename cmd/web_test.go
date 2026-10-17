package cmd

import (
	"context"
	"encoding/json"
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

func setupWebServer(t *testing.T) *httptest.Server {
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
	return httpServer
}

func TestWebAPI(t *testing.T) {
	httpServer := setupWebServer(t)
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

func TestWebJobLogsSSE(t *testing.T) {
	httpServer := setupWebServer(t)
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

func homedirHome(t *testing.T) string {
	home, err := homedir.Dir()
	assert.NoError(t, err)
	return home
}
