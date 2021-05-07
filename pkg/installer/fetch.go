package installer

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/linuxsuren/http-downloader/pkg/common"
	"github.com/mitchellh/go-homedir"
	"io"
	"os"
	"path"
	"strings"
)

const (
	// ConfigGitHub is the default git repository URI
	ConfigGitHub = "https://github.com/LinuxSuRen/hd-home"
	// ConfigBranch is the default branch name of hd-home git repository
	ConfigBranch = "master"
)

// FetchConfig fetches the hd-home as the config
func FetchConfig() (err error) {
	return FetchLatestRepo(ConfigGitHub, ConfigBranch, os.Stdout)
}

// GetConfigDir returns the directory of the config
func GetConfigDir() (configDir string, err error) {
	var userHome string
	if userHome, err = homedir.Dir(); err == nil {
		configDir = path.Join(userHome, "/.config/hd-home")
	}
	return
}

// FetchLatestRepo fetches the hd-home as the config
func FetchLatestRepo(repoAddr string, branch string, progress io.Writer) (err error) {
	var configDir string
	if configDir, err = GetConfigDir(); err != nil {
		return
	}

	if ok, _ := common.PathExists(configDir); ok {
		var repo *git.Repository
		if repo, err = git.PlainOpen(configDir); err == nil {
			var wd *git.Worktree

			if wd, err = repo.Worktree(); err == nil {
				if err = wd.Pull(&git.PullOptions{
					Progress: progress,
					Force:    true,
				}); err != nil && err != git.NoErrAlreadyUpToDate {
					err = fmt.Errorf("failed to pull git repository '%s', error: %v", repo, err)
					return
				}
				err = nil
			}
		} else {
			err = fmt.Errorf("failed to open git local repository, error: %v", err)
		}
	} else {
		if _, err = git.PlainClone(configDir, false, &git.CloneOptions{
			URL:      repoAddr,
			Progress: progress,
		}); err != nil {
			err = fmt.Errorf("failed to clone git repository '%s' into '%s', error: %v", repoAddr, configDir, err)
		}
	}

	if err != nil && strings.Contains(err.Error(), "exit status 128") {
		// target directory was created accidentally, remove it then try again
		_ = os.RemoveAll(configDir)
		return FetchLatestRepo(repoAddr, branch, progress)
	}
	return
}
