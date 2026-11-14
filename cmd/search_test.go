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
	hdos "github.com/linuxsuren/http-downloader/pkg/os"
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
	}, {
		name: "repos",
	}, {
		name: "managers",
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

func TestFormatRepoPackage(t *testing.T) {
	for i, tt := range []struct {
		pkg    hdos.RepoPackage
		expect string
	}{{
		pkg:    hdos.RepoPackage{Manager: "apt", Name: "jenkins"},
		expect: "apt:jenkins",
	}, {
		pkg:    hdos.RepoPackage{Manager: "winget", Name: "Git.Git", Version: "2.47.0"},
		expect: "winget:Git.Git (2.47.0)",
	}, {
		pkg:    hdos.RepoPackage{Manager: "apt", Name: "jenkins", Description: "Jenkins CI"},
		expect: "apt:jenkins - Jenkins CI",
	}, {
		pkg:    hdos.RepoPackage{Manager: "choco", Name: "git", Version: "2.47", Description: "Git tool"},
		expect: "choco:git (2.47) - Git tool",
	}} {
		t.Run(tt.expect, func(t *testing.T) {
			assert.Equal(t, tt.expect, FormatRepoPackage(tt.pkg), "failed with case [%d]", i)
		})
	}
}

func TestSearchRepos(t *testing.T) {
	buf := &bytes.Buffer{}
	opt := &searchOption{
		Repos: true,
		repoSearch: func(keyword string, managers []string) []hdos.RepoPackage {
			assert.Equal(t, "jenkins", keyword)
			assert.Equal(t, []string{"apt", "winget"}, managers)
			return []hdos.RepoPackage{{
				Manager:     "apt",
				Name:        "jenkins",
				Version:     "2.426",
				Description: "Jenkins automation server",
			}}
		},
		Managers: []string{"apt", "winget"},
	}
	opt.searchRepos("jenkins", buf)
	assert.Equal(t, "apt:jenkins (2.426) - Jenkins automation server\n", buf.String())

	// without the repos flag nothing is searched
	empty := &bytes.Buffer{}
	(&searchOption{}).searchRepos("jenkins", empty)
	assert.Empty(t, empty.String())
}
