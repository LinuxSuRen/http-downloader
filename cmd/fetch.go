package cmd

import (
	"os"
	"path"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
)

func newFetchCmd() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "fetch",
		Short: "Fetch the latest hd config",
		RunE: func(_ *cobra.Command, _ []string) (err error) {
			return fetchHomeConfig()
		},
	}
	return
}

func getConfigDir() (configDir string, err error) {
	var userHome string
	if userHome, err = homedir.Dir(); err == nil {
		configDir = path.Join(userHome, "/.config/hd-home")
	}
	return
}

func fetchHomeConfig() (err error) {
	return fetchLatestRepo("https://github.com/LinuxSuRen/hd-home", "master")
}

func fetchLatestRepo(repo string, branch string) (err error) {
	var configDir string
	if configDir, err = getConfigDir(); err != nil {
		return
	}

	if ok, _ := pathExists(configDir); ok {
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
