package installer

import (
	"github.com/go-git/go-git/v5"
	"github.com/linuxsuren/http-downloader/pkg/common"
	"github.com/mitchellh/go-homedir"
	"os"
	"path"
	"strings"
)

// FetchConfig fetches the hd-home as the config
func FetchConfig() (err error) {
	return fetchLatestRepo("https://github.com/LinuxSuRen/hd-home", "master")
}

// GetConfigDir returns the directory of the config
func GetConfigDir() (configDir string, err error) {
	var userHome string
	if userHome, err = homedir.Dir(); err == nil {
		configDir = path.Join(userHome, "/.config/hd-home")
	}
	return
}

func fetchLatestRepo(repo string, branch string) (err error) {
	var configDir string
	if configDir, err = GetConfigDir(); err != nil {
		return
	}

	if ok, _ := common.PathExists(configDir); ok {
		var repo *git.Repository
		if repo, err = git.PlainOpen(""); err == nil {
			var wd *git.Worktree
			if wd, err = repo.Worktree(); err == nil {
				if err = wd.Pull(&git.PullOptions{
					RemoteName: "origin",
					Progress:   os.Stdout,
					Force:      true,
				}); err != nil && err != git.NoErrAlreadyUpToDate {
					return
				}
				err = nil
			}
		}
	} else {
		_, err = git.PlainClone(configDir, false, &git.CloneOptions{
			URL:      repo,
			Progress: os.Stdout,
		})
	}

	if err != nil && strings.Contains(err.Error(), "exit status 128") {
		// target directory was created accidentally, remove it then try again
		_ = os.RemoveAll(configDir)
		return fetchLatestRepo(repo, branch)
	}
	return
}
