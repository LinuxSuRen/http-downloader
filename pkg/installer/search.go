package installer

import (
	"path"
	"path/filepath"
	"strings"

	hdpkgos "github.com/linuxsuren/http-downloader/pkg/os"
	"github.com/linuxsuren/http-downloader/pkg/source"
)

// SearchSources searches the local hd-home cache and the native package
// managers, tagging each result with its source. npm results come from the
// network enabled npmregistry client and are added by the callers.
func SearchSources(keyword, configDir, sourceFilter string) (result []source.Result) {
	includeHDHome := sourceFilter == "" || sourceFilter == source.All || sourceFilter == source.HDHome
	includeNative := sourceFilter == "" || sourceFilter == source.All || sourceFilter == source.Native

	if includeHDHome {
		files, _ := filepath.Glob(path.Join(configDir, "config/**/*.yml"))
		for _, metaFile := range files {
			fileName := filepath.Base(metaFile)
			org := filepath.Base(filepath.Dir(metaFile))
			repo := strings.TrimSuffix(fileName, path.Ext(metaFile))

			if !strings.Contains(repo, keyword) && !hasKeyword(metaFile, keyword) {
				continue
			}
			item := source.Result{
				Source: source.HDHome,
				Org:    org,
				Repo:   repo,
				Name:   repo,
			}
			if cfg := getHDConfig(configDir, path.Join(org, repo)); cfg != nil {
				if cfg.Name != "" {
					item.Name = cfg.Name
				}
				item.Homepage = cfg.Homepage
				item.Version = cfg.LatestVersion
				item.Description = cfg.Homepage
			}
			result = append(result, item)
		}
	}

	if includeNative {
		for _, name := range hdpkgos.SearchPackages(keyword) {
			result = append(result, source.Result{
				Source: source.Native,
				Name:   name,
			})
		}
	}
	return
}

// SourceOfInstalled tells where an installed tool came from, older records
// without a source are hd-home installs
func SourceOfInstalled(src string) string {
	if src == "" {
		return source.HDHome
	}
	return src
}
