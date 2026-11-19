package cmd

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/linuxsuren/http-downloader/pkg/log"
	"github.com/stretchr/testify/assert"
)

func Test_search(t *testing.T) {
	logger := log.GetLogger()

	buf := &bytes.Buffer{}
	err := search(context.Background(), "keyword", true, &installer.FakeFetcher{}, buf, logger)
	assert.Nil(t, err)
	assert.Empty(t, buf.String())

	// expect an error with GetConfigDir
	err = search(context.Background(), "", true, &installer.FakeFetcher{GetConfigDirErr: errors.New("fake")}, buf, logger)
	assert.NotNil(t, err)

	// expect an error with FetchLatestRepo
	err = search(context.Background(), "", true, &installer.FakeFetcher{FetchLatestRepoErr: errors.New("fake")}, buf, logger)
	assert.NotNil(t, err)

	tempDir, err := os.MkdirTemp("", "config")
	assert.Nil(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	configDir := path.Join(tempDir, "config")
	orgDir := path.Join(configDir, "org")
	err = os.MkdirAll(orgDir, 0755)
	assert.Nil(t, err)
	err = os.WriteFile(path.Join(orgDir, "repo.yml"), []byte("x=x"), os.ModeAppend)
	assert.Nil(t, err)
	err = os.WriteFile(path.Join(orgDir, "fake.yml"), []byte{}, os.ModeAppend)
	assert.Nil(t, err)

	err = search(context.Background(), "repo", true, &installer.FakeFetcher{ConfigDir: tempDir}, buf, logger)
	assert.Nil(t, err)
}

func Test_searchWithRepoSources(t *testing.T) {
	logger := log.GetLogger()
	buf := &bytes.Buffer{}

	// a fake Packages index of an apt style mirror
	gz := &bytes.Buffer{}
	writer := gzip.NewWriter(gz)
	_, _ = writer.Write([]byte(`Package: repo
Version: 1.0.0
Section: web
Description: a fake remote hit
Filename: pool/main/r/repo/repo_1.0.0_amd64.deb
SHA256: fake-sha
`))
	_ = writer.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/Packages.gz") {
			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(gz.Bytes())
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tempDir, err := os.MkdirTemp("", "config")
	assert.Nil(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	// a local package and a repo descriptor side by side
	configDir := path.Join(tempDir, "config")
	orgDir := path.Join(configDir, "org")
	assert.Nil(t, os.MkdirAll(orgDir, 0750))
	assert.Nil(t, os.WriteFile(path.Join(orgDir, "repo.yml"), []byte("x=x"), 0644))
	assert.Nil(t, os.MkdirAll(path.Join(tempDir, "repos"), 0750))
	// the explicit platform list overrides the apt-index default, the
	// flow test runs on every host
	assert.Nil(t, os.WriteFile(path.Join(tempDir, "repos", "debian.yml"),
		[]byte("name: debian-stable\nkind: apt-index\nmirror: "+server.URL+
			"\nsuite: stable\nplatforms: [linux, darwin, windows]\n"), 0644))

	err = search(context.Background(), "repo", false, &installer.FakeFetcher{ConfigDir: tempDir}, buf, logger)
	assert.Nil(t, err)
	output := buf.String()
	assert.Contains(t, output, "org/repo")
	assert.Contains(t, output, "debian-stable/repo\ta fake remote hit")

	// a failing source never hides the local results
	buf.Reset()
	assert.Nil(t, os.WriteFile(path.Join(tempDir, "repos", "debian.yml"),
		[]byte("name: debian-stable\nkind: apt-index\nmirror: http://127.0.0.1:1\nsuite: stable\n"), 0644))
	err = search(context.Background(), "repo", false, &installer.FakeFetcher{ConfigDir: tempDir}, buf, logger)
	assert.Nil(t, err)
	assert.Contains(t, buf.String(), "org/repo")
	assert.NotContains(t, buf.String(), "debian-stable/repo")
}

func Test_newSearchCmd(t *testing.T) {
	cmd := newSearchCmd(context.Background())
	assert.Equal(t, "search", cmd.Name())

	flags := []struct {
		name      string
		shorthand string
	}{{
		name: "fetch",
	}, {
		name: "provider",
	}, {
		name: "proxy-github",
	}}
	for i := range flags {
		tt := flags[i]
		t.Run(tt.name, func(t *testing.T) {
			flag := cmd.Flag(tt.name)
			assert.NotNil(t, flag)
			assert.NotEmpty(t, flag.Usage)
			assert.Equal(t, tt.shorthand, flag.Shorthand)
		})
	}
}
