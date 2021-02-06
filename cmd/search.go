package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"path"
	"path/filepath"
	"strings"
)

func newSearchCmd() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "search",
		Short: "Search packages from the hd config repo",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) (err error) {
			err = search(args[0])
			return
		},
	}
	return
}

func search(keyword string) (err error) {
	var configDir string
	if configDir, err = getConfigDir(); err != nil {
		// TODO consider how to deal with the situation when config repo is not exists
		return
	}

	var files []string
	if files, err = filepath.Glob(path.Join(configDir, "config/**/*.yml")); err == nil {
		for _, metaFile := range files {
			ext := path.Ext(metaFile)
			fileName := path.Base(metaFile)
			org := path.Base(path.Dir(metaFile))
			repo := strings.TrimSuffix(fileName, ext)

			if !strings.Contains(repo, keyword) {
				continue
			}

			fmt.Println(path.Join(org, repo))
		}
	}
	return
}
