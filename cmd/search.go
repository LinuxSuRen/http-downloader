package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/linuxsuren/http-downloader/pkg/log"
	"github.com/linuxsuren/http-downloader/pkg/npmregistry"
	"github.com/linuxsuren/http-downloader/pkg/source"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func newSearchCmd(context.Context) (cmd *cobra.Command) {
	opt := &searchOption{
		fetcher: &installer.DefaultFetcher{},
		Source:  source.All,
	}

	cmd = &cobra.Command{
		Use:     "search",
		Aliases: []string{"s", "find", "f"},
		Short:   "Search packages from hd-home, npm and native package managers",
		Args:    cobra.MinimumNArgs(1),
		RunE:    opt.runE,
		GroupID: configGroup.ID,
	}
	opt.addFlags(cmd.Flags())
	opt.addSourceFlag(cmd.Flags())
	return
}

type searchOption struct {
	Fetch       bool
	Provider    string
	ProxyGitHub string
	Source      string
	fetcher     installer.Fetcher
}

func (s *searchOption) addFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&s.Fetch, "fetch", "", viper.GetBool("fetch"),
		"If fetch the latest config from https://github.com/LinuxSuRen/hd-home")
	flags.StringVarP(&s.Provider, "provider", "", viper.GetString("provider"), "The file provider")
	flags.StringVarP(&s.ProxyGitHub, "proxy-github", "", viper.GetString("proxy-github"),
		`The proxy address of github.com, the proxy address will be the prefix of the final address.
Submit the new proxy server in https://github.com/LinuxSuRen/hd-home`)
}

// addSourceFlag adds the source filter only to the commands which search
// packages. It is not part of addFlags because downloadOption embeds
// searchOption and shares that method with get and install.
func (s *searchOption) addSourceFlag(flags *pflag.FlagSet) {
	flags.StringVarP(&s.Source, "source", "", source.All,
		"Limit the search to one source: hd-home, npm, native or all")
}

func (s *searchOption) runE(c *cobra.Command, args []string) (err error) {
	logger := log.GetLoggerFromContextOrDefault(c)
	sourceName := s.Source
	if !source.Known(sourceName) {
		return fmt.Errorf("unknown package source: %s", sourceName)
	}
	return searchFrom(c.Context(), args[0], s.Fetch, sourceName, s.fetcher, c.OutOrStdout(), logger)
}

// search keeps the hd-home only behavior for callers which do not provide a
// source filter
func search(keyword string, fetch bool, fetcher installer.Fetcher, writer io.Writer, logger *log.LevelLog) (err error) {
	return searchFrom(context.Background(), keyword, fetch, source.HDHome, fetcher, writer, logger)
}

func searchFrom(ctx context.Context, keyword string, fetch bool, sourceName string,
	fetcher installer.Fetcher, writer io.Writer, logger *log.LevelLog) (err error) {
	if fetch {
		if err = fetcher.FetchLatestRepo("", "", writer); err != nil {
			return
		}
	}

	var configDir string
	if configDir, err = fetcher.GetConfigDir(); err != nil {
		return
	}

	logger.Info("start to search in:", configDir)
	results := installer.SearchSources(keyword, configDir, sourceName)
	if sourceName == source.All || sourceName == source.NPM {
		packages, npmErr := npmregistry.Search(ctx, keyword, 20,
			&http.Client{Timeout: 20 * time.Second}, "")
		if npmErr != nil {
			// npm is an extra source, a registry failure must not hide the
			// results of the other sources
			logger.Info("cannot search npm:", npmErr.Error())
		}
		for i := range packages {
			results = append(results, source.Result{
				Source:      source.NPM,
				Name:        packages[i].Name,
				Version:     packages[i].Version,
				Description: packages[i].Description,
				Homepage:    packages[i].Homepage,
			})
		}
	}

	for _, item := range results {
		fmt.Fprintf(writer, "%s | %s\n", item.Source, item.ID())
	}
	return
}
