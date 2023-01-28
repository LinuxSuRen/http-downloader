package installer

import (
	"context"
	"fmt"
	"github.com/go-git/go-git/v5/plumbing/object"
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
	SetContext(ctx context.Context)
}

// DefaultFetcher is the default fetcher which fetches the config files from a git repository
type DefaultFetcher struct {
	ctx context.Context
}

// GetConfigDir returns the directory of the config
func (f *DefaultFetcher) GetConfigDir() (configDir string, err error) {
	var userHome string
	if userHome, err = homedir.Dir(); err == nil {
		configDir = path.Join(userHome, "/.config/hd-home")
	}
	return
}

// SetContext sets the context of the fetch
func (f *DefaultFetcher) SetContext(ctx context.Context) {
	f.ctx = ctx
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

			var gitConfig *config.Config
			if gitConfig, err = repo.Config(); err != nil {
				return
			}
			if gitConfig.Branches == nil {
				gitConfig.Branches = map[string]*config.Branch{
					branch: {
						Name:   branch,
						Remote: remoteName,
						Merge:  plumbing.NewBranchReferenceName(branch),
					},
				}
			}

			var branchObj *config.Branch
			if branchObj, err = repo.Branch(branch); err != nil && err != git.ErrBranchNotFound {
				return
			}

			if branchObj != nil {
				branchObj.Remote = remoteName
				gitConfig.Branches[branch] = branchObj
			}
			if err = repo.SetConfig(gitConfig); err != nil {
				return
			}

			var head *plumbing.Reference
			if wd, err = repo.Worktree(); err == nil {
				if err = makeSureRemote(remoteName, repoAddr, repo); err != nil {
					err = fmt.Errorf("cannot add remote: %s, address: %s, error: %v", remoteName, repoAddr, err)
					return
				}

				if err = repo.FetchContext(f.ctx, &git.FetchOptions{
					RemoteName: remoteName,
					Progress:   progress,
					Force:      true,
				}); err != nil && err != git.NoErrAlreadyUpToDate {
					err = fmt.Errorf("failed to fetch '%s', error: %v", remoteName, err)
					return
				}

				if head, err = repo.Reference(plumbing.NewRemoteReferenceName(remoteName, branch), true); err != nil {
					return
				}
				// avoid force push from remote
				if err = wd.Reset(&git.ResetOptions{
					Commit: head.Hash(),
					Mode:   git.HardReset,
				}); err != nil {
					err = fmt.Errorf("unable to reset to '%s'", head.Hash().String())
					return
				}

				if err = wd.Checkout(&git.CheckoutOptions{
					Branch: plumbing.NewBranchReferenceName(branch),
					Create: true,
					Force:  true,
					//Keep:   true,
				}); err != nil && !strings.Contains(err.Error(), "already exists") {
					err = fmt.Errorf("unable to checkout git branch: %s, error: %v", branch, err)
					return
				}

				if err = wd.PullContext(f.ctx, &git.PullOptions{
					RemoteName:    remoteName,
					ReferenceName: plumbing.NewBranchReferenceName(branch),
					Progress:      progress,
					Force:         true,
				}); err != nil && err != git.NoErrAlreadyUpToDate {
					err = fmt.Errorf("failed to pull git repository '%s', error: %v", repo, err)
					return
				}
				err = nil
			}

			if head, err = repo.Head(); err == nil {
				var log object.CommitIter
				if log, err = repo.Log(&git.LogOptions{From: head.Hash()}); err == nil {
					var next *object.Commit
					for next, err = log.Next(); err == nil; next, err = log.Next() {
						if !strings.HasPrefix(next.Message, "Auto commit by bot, ci skip") {
							_, _ = fmt.Fprintln(progress, "Last updated", next.Author.When)
							_, _ = fmt.Fprintln(progress, strings.TrimSpace(next.Message))
							break
						}
					}
				}
			}
		} else {
			err = fmt.Errorf("failed to open git local repository, error: %v", err)
		}
	} else {
		_, _ = fmt.Fprintf(progress, "no local config exist, try to clone it\n")

		if _, err = git.PlainCloneContext(f.ctx, configDir, false, &git.CloneOptions{
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

// FetchLatestRepo is a fake method
func (f *FakeFetcher) FetchLatestRepo(provider string, branch string,
	progress io.Writer) (err error) {
	err = f.FetchLatestRepoErr
	return
}

// SetContext is a fake method
func (f *FakeFetcher) SetContext(ctx context.Context) {}
