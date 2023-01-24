package cmd

import (
	"context"
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	sysos "os"
)

func newSearchCmd(context.Context) (cmd *cobra.Command) {
	opt := &searchOption{
		fetcher: &installer.DefaultFetcher{},
	}

	cmd = &cobra.Command{
		Use:     "search",
		Aliases: []string{"s", "find", "f"},
		Short:   "Search packages from the hd config repo",
		Args:    cobra.MinimumNArgs(1),
		RunE:    opt.runE,
	}
	opt.addFlags(cmd.Flags())
	return
}

type searchOption struct {
	Fetch       bool
	Provider    string
	ProxyGitHub string
	fetcher     installer.Fetcher
}

func (s *searchOption) addFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&s.Fetch, "fetch", "", viper.GetBool("fetch"),
		"If fetch the latest config from https://github.com/LinuxSuRen/hd-home")
	flags.StringVarP(&s.Provider, "provider", "", viper.GetString("provider"), "The file provider")
	flags.StringVarP(&s.ProxyGitHub, "proxy-github", "", viper.GetString("proxy-github"),
		`The proxy address of github.com, the proxy address will be the prefix of the final address.
Available proxy: gh.api.99988866.xyz, ghproxy.com
Thanks to https://github.com/hunshcn/gh-proxy`)
}

func (s *searchOption) runE(_ *cobra.Command, args []string) (err error) {
	err = search(args[0], s.Fetch, s.fetcher)
	return
}

func search(keyword string, fetch bool, fetcher installer.Fetcher) (err error) {
	if fetch {
		if err = fetcher.FetchLatestRepo("", "", sysos.Stdout); err != nil {
			return
		}
	}

	var configDir string
	if configDir, err = fetcher.GetConfigDir(); err != nil {
		return
	}

	result := installer.FindByKeyword(keyword, configDir)
	for _, item := range result {
		fmt.Println(item)
	}
	return
}
