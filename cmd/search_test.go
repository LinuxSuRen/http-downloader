package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path"
	"testing"

	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/stretchr/testify/assert"
)

func Test_search(t *testing.T) {
	buf := &bytes.Buffer{}
	err := search("keyword", true, &installer.FakeFetcher{}, buf)
	assert.Nil(t, err)
	assert.Empty(t, buf.String())

	// expect an error with GetConfigDir
	err = search("", true, &installer.FakeFetcher{GetConfigDirErr: errors.New("fake")}, buf)
	assert.NotNil(t, err)

	// expect an error with FetchLatestRepo
	err = search("", true, &installer.FakeFetcher{FetchLatestRepoErr: errors.New("fake")}, buf)
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

	err = search("repo", true, &installer.FakeFetcher{ConfigDir: tempDir}, buf)
	assert.Nil(t, err)
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
