package installer

import (
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/linuxsuren/http-downloader/pkg/common"
	"github.com/mitchellh/go-homedir"
)

const (
	// ConfigGitHub is the default git repository URI
	ConfigGitHub = "https://github.com/LinuxSuRen/hd-home"
	// ConfigBranch is the default branch name of hd-home git repository
	ConfigBranch = "master"
)

var configRepos = map[string]string{
	"github": ConfigGitHub,
	"gitee":  "https://gitee.com/LinuxSuRen/hd-home",
}

// Fetcher is the interface of a fetcher which responses to fetch config files
type Fetcher interface {
	GetConfigDir() (configDir string, err error)
	FetchLatestRepo(provider string, branch string,
		progress io.Writer) (err error)
}

// DefaultFetcher is the default fetcher which fetches the config files from a git repository
type DefaultFetcher struct {
}

// GetConfigDir returns the directory of the config
func (f *DefaultFetcher) GetConfigDir() (configDir string, err error) {
	var userHome string
	if userHome, err = homedir.Dir(); err == nil {
		configDir = path.Join(userHome, "/.config/hd-home")
	}
	return
}

// FetchLatestRepo fetches the hd-home as the config
func (f *DefaultFetcher) FetchLatestRepo(provider string, branch string,
	progress io.Writer) (err error) {
	repoAddr, ok := configRepos[provider]
	if !ok {
		if provider != "" {
			fmt.Printf("not support '%s', use 'github' instead\n", provider)
		}
		repoAddr = ConfigGitHub
	}

	if branch == "" {
		branch = ConfigBranch
	}

	remoteName := "origin"
	if repoAddr != ConfigGitHub {
		remoteName = provider
	}

	var configDir string
	if configDir, err = f.GetConfigDir(); err != nil {
		return
	}

	if ok, _ := common.PathExists(configDir); ok {
		var repo *git.Repository
		if repo, err = git.PlainOpen(configDir); err == nil {
			var wd *git.Worktree

			if wd, err = repo.Worktree(); err == nil {
				if err = makeSureRemote(remoteName, repoAddr, repo); err != nil {
					err = fmt.Errorf("cannot add remote: %s, address: %s, error: %v", remoteName, repoAddr, err)
					return
				}

				if err = repo.Fetch(&git.FetchOptions{
					RemoteName: remoteName,
					Progress:   progress,
					Force:      true,
				}); err != nil && err != git.NoErrAlreadyUpToDate {
					err = fmt.Errorf("failed to fetch '%s', error: %v", remoteName, err)
					return
				}

				head, _ := repo.Head()
				if head != nil {
					// avoid force push from remote
					if err = wd.Reset(&git.ResetOptions{
						Commit: head.Hash(),
						Mode:   git.HardReset,
					}); err != nil {
						err = fmt.Errorf("unable to reset to '%s'", head.Hash().String())
						return
					}
				}

				if err = wd.Checkout(&git.CheckoutOptions{
					Branch: plumbing.NewRemoteReferenceName(remoteName, branch),
					Create: false,
					Keep:   true,
				}); err != nil {
					err = fmt.Errorf("unable to checkout git branch: %s, error: %v", branch, err)
					return
				}

				if err = wd.Pull(&git.PullOptions{
					RemoteName: remoteName,
					Progress:   progress,
					Force:      true,
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
		_, _ = fmt.Fprintf(progress, "no local config exist, try to clone it\n")

		if _, err = git.PlainClone(configDir, false, &git.CloneOptions{
			RemoteName: remoteName,
			URL:        repoAddr,
			Progress:   progress,
		}); err != nil {
			err = fmt.Errorf("failed to clone git repository '%s' into '%s', error: %v", repoAddr, configDir, err)
		}
	}

	if err != nil && strings.Contains(err.Error(), "exit status 128") {
		// target directory was created accidentally, remove it then try again
		_ = os.RemoveAll(configDir)
		return f.FetchLatestRepo(repoAddr, branch, progress)
	}
	return
}

func makeSureRemote(name, repoAddr string, repo *git.Repository) (err error) {
	if _, err = repo.Remote(name); err != nil {
		_, err = repo.CreateRemote(&config.RemoteConfig{
			Name: name,
			URLs: []string{repoAddr},
		})
	}
	return
}

// FakeFetcher is a fake fetch. We expect to use it for unit test cases.
type FakeFetcher struct {
	ConfigDir          string
	GetConfigDirErr    error
	FetchLatestRepoErr error
}

// GetConfigDir is a fake method
func (f *FakeFetcher) GetConfigDir() (configDir string, err error) {
	configDir = f.ConfigDir
	err = f.GetConfigDirErr
	return
}

// FetchLatestRepo is fake method
func (f *FakeFetcher) FetchLatestRepo(provider string, branch string,
	progress io.Writer) (err error) {
	err = f.FetchLatestRepoErr
	return
}
