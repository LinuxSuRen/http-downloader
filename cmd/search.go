package cmd

import (
	"context"
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	sysos "os"
	"path"
	"path/filepath"
	"strings"
)

func newSearchCmd(context.Context) (cmd *cobra.Command) {
	opt := &searchOption{}

	cmd = &cobra.Command{
		Use:   "search",
		Short: "Search packages from the hd config repo",
		Args:  cobra.MinimumNArgs(1),
		RunE:  opt.runE,
	}
	opt.addFlags(cmd.Flags())
	return
}

type searchOption struct {
	Fetch       bool
	Provider    string
	ProxyGitHub string
}

func (s *searchOption) addFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&s.Fetch, "fetch", "", viper.GetBool("fetch"),
		"If fetch the latest config from https://github.com/LinuxSuRen/hd-home")
	flags.StringVarP(&s.Provider, "provider", "", viper.GetString("provider"), "The file provider")
	flags.StringVarP(&s.ProxyGitHub, "proxy-github", "", viper.GetString("proxy-github"),
		`The proxy address of github.com, the proxy address will be the prefix of the final address.
Available proxy: gh.api.99988866.xyz
Thanks to https://github.com/hunshcn/gh-proxy`)
}

func (s *searchOption) runE(_ *cobra.Command, args []string) (err error) {
	err = search(args[0])
	return
}

func search(keyword string) (err error) {
	if err = installer.FetchLatestRepo("", "", sysos.Stdout); err != nil {
		return
	}

	var configDir string
	if configDir, err = installer.GetConfigDir(); err != nil {
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
