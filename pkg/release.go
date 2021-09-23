package pkg

import (
	"context"
	"github.com/google/go-github/v29/github"
)

// ReleaseClient is the client of jcli github
type ReleaseClient struct {
	Client *github.Client
	Org    string
	Repo   string

	ctx context.Context
}

// ReleaseAsset is the asset from GitHub release
type ReleaseAsset struct {
	TagName string
	Body    string
}

// Init init the GitHub client
func (g *ReleaseClient) Init() {
	g.Client = github.NewClient(nil)
	g.ctx = context.TODO()
}

// ListReleases returns the release list
func (g *ReleaseClient) ListReleases(owner, repo string, count int) (list []ReleaseAsset, err error) {
	opt := &github.ListOptions{
		PerPage: count,
	}

	var releaseList []*github.RepositoryRelease
	if releaseList, _, err = g.Client.Repositories.ListReleases(g.ctx, owner, repo, opt); err == nil {
		for i := range releaseList {
			list = append(list, ReleaseAsset{
				TagName: releaseList[i].GetTagName(),
				Body:    releaseList[i].GetBody(),
			})
		}
	}
	return
}

// GetLatestJCLIAsset returns the latest jcli asset
// deprecated, please use GetLatestAsset instead
func (g *ReleaseClient) GetLatestJCLIAsset() (*ReleaseAsset, error) {
	return g.GetLatestReleaseAsset(g.Org, g.Repo)
}

// GetLatestAsset returns the latest release asset which can accept preRelease or not
func (g *ReleaseClient) GetLatestAsset(acceptPreRelease bool) (*ReleaseAsset, error) {
	if acceptPreRelease {
		return g.GetLatestPreReleaseAsset(g.Org, g.Repo)
	}
	return g.GetLatestReleaseAsset(g.Org, g.Repo)
}

// GetLatestPreReleaseAsset returns the release asset that could be preRelease
func (g *ReleaseClient) GetLatestPreReleaseAsset(owner, repo string) (ra *ReleaseAsset, err error) {
	ctx := context.Background()

	var list []*github.RepositoryRelease
	if list, _, err = g.Client.Repositories.ListReleases(ctx, owner, repo, &github.ListOptions{
		Page:    1,
		PerPage: 5,
	}); err == nil {
		ra = &ReleaseAsset{
			TagName: list[0].GetTagName(),
			Body:    list[0].GetBody(),
		}
	}
	return
}

// GetLatestReleaseAsset returns the latest release asset
func (g *ReleaseClient) GetLatestReleaseAsset(owner, repo string) (ra *ReleaseAsset, err error) {
	ctx := context.Background()

	var release *github.RepositoryRelease
	if release, _, err = g.Client.Repositories.GetLatestRelease(ctx, owner, repo); err == nil {
		ra = &ReleaseAsset{
			TagName: release.GetTagName(),
			Body:    release.GetBody(),
		}
	}
	return
}

// GetJCLIAsset returns the asset from a tag name
func (g *ReleaseClient) GetJCLIAsset(tagName string) (*ReleaseAsset, error) {
	return g.GetReleaseAssetByTagName(g.Org, g.Repo, tagName)
}

// GetReleaseAssetByTagName returns the release asset by tag name
func (g *ReleaseClient) GetReleaseAssetByTagName(owner, repo, tagName string) (ra *ReleaseAsset, err error) {
	var list []ReleaseAsset
	if list, err = g.ListReleases(owner, repo, 99999); err == nil {
		for _, item := range list {
			if item.TagName == tagName {
				ra = &ReleaseAsset{
					TagName: item.TagName,
					Body:    item.Body,
				}
				break
			}
		}
	}
	return
}
