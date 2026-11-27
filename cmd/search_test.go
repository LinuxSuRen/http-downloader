package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path"
	"testing"

	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/linuxsuren/http-downloader/pkg/log"
	"github.com/linuxsuren/http-downloader/pkg/source"
	"github.com/stretchr/testify/assert"
)

func Test_search(t *testing.T) {
	logger := log.GetLogger()

	buf := &bytes.Buffer{}
	err := search("keyword", true, &installer.FakeFetcher{}, buf, logger)
	assert.Nil(t, err)
	assert.Empty(t, buf.String())

	// expect an error with GetConfigDir
	err = search("", true, &installer.FakeFetcher{GetConfigDirErr: errors.New("fake")}, buf, logger)
	assert.NotNil(t, err)

	// expect an error with FetchLatestRepo
	err = search("", true, &installer.FakeFetcher{FetchLatestRepoErr: errors.New("fake")}, buf, logger)
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

	err = search("repo", true, &installer.FakeFetcher{ConfigDir: tempDir}, buf, logger)
	assert.Nil(t, err)
}

func TestSearchFromLabelsHDHome(t *testing.T) {
	tempDir := t.TempDir()
	orgDir := path.Join(tempDir, "config", "fake-org")
	assert.NoError(t, os.MkdirAll(orgDir, 0750))
	assert.NoError(t, os.WriteFile(path.Join(orgDir, "fake-cli.yml"),
		[]byte("name: fake-cli\n"), 0644))

	buf := &bytes.Buffer{}
	err := searchFrom(context.Background(), "fake-cli", false, source.HDHome,
		&installer.FakeFetcher{ConfigDir: tempDir}, buf, log.GetLogger())
	assert.NoError(t, err)
	assert.Equal(t, "hd-home | fake-org/fake-cli\n", buf.String())
}

func TestSearchFromRejectsUnknownSource(t *testing.T) {
	cmd := newSearchCmd(context.Background())
	opt := &searchOption{Source: "unknown", fetcher: &installer.FakeFetcher{}}
	err := opt.runE(cmd, []string{"fake"})
	assert.ErrorContains(t, err, "unknown package source")
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
