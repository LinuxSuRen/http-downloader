package cmd

import (
	"context"
	"fmt"
	"io"

	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/linuxsuren/http-downloader/pkg/log"
	hdos "github.com/linuxsuren/http-downloader/pkg/os"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func newSearchCmd(context.Context) (cmd *cobra.Command) {
	opt := &searchOption{
		fetcher: &installer.DefaultFetcher{},
	}
	opt.repoSearch = func(keyword string, managers []string) []hdos.RepoPackage {
		return (&hdos.RepositorySearcher{}).Search(keyword, managers)
	}

	cmd = &cobra.Command{
		Use:     "search",
		Aliases: []string{"s", "find", "f"},
		Short:   "Search packages from the hd config repo and the system package repositories",
		Args:    cobra.MinimumNArgs(1),
		RunE:    opt.runE,
		GroupID: configGroup.ID,
	}
	opt.addFlags(cmd.Flags())
	return
}

type searchOption struct {
	Fetch       bool
	Provider    string
	ProxyGitHub string
	// Repos indicates searching the system package repositories too, for
	// instance: apt, winget and brew. It reuses the search commands of the
	// package managers which are already installed.
	Repos bool
	// Managers limits the searched system package managers, an empty list
	// searches all the available ones
	Managers []string

	fetcher    installer.Fetcher
	repoSearch func(keyword string, managers []string) []hdos.RepoPackage
}

func (s *searchOption) addFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&s.Fetch, "fetch", "", viper.GetBool("fetch"),
		"If fetch the latest config from https://github.com/LinuxSuRen/hd-home")
	flags.StringVarP(&s.Provider, "provider", "", viper.GetString("provider"), "The file provider")
	flags.StringVarP(&s.ProxyGitHub, "proxy-github", "", viper.GetString("proxy-github"),
		`The proxy address of github.com, the proxy address will be the prefix of the final address.
Submit the new proxy server in https://github.com/LinuxSuRen/hd-home`)
	flags.BoolVarP(&s.Repos, "repos", "", false,
		`Also search the system package repositories, for instance: apt, winget and brew.
The results carry the package manager prefix, for instance: apt:jenkins, which can be
installed via the install command directly`)
	flags.StringSliceVarP(&s.Managers, "managers", "", nil,
		"Limit the searched system package managers, only works together with the flag --repos")
}

func (s *searchOption) runE(c *cobra.Command, args []string) (err error) {
	logger := log.GetLoggerFromContextOrDefault(c)

	err = search(args[0], s.Fetch, s.fetcher, c.OutOrStdout(), logger)
	if err != nil {
		return
	}

	if s.Repos {
		s.searchRepos(args[0], c.OutOrStdout())
	}
	return
}

// searchRepos searches the system package repositories and prints the
// results with their package manager prefixes
func (s *searchOption) searchRepos(keyword string, writer io.Writer) {
	if s.repoSearch == nil {
		return
	}

	for _, pkg := range s.repoSearch(keyword, s.Managers) {
		fmt.Fprintln(writer, FormatRepoPackage(pkg))
	}
}

// FormatRepoPackage formats a repository package into a single line,
// for instance: apt:jenkins (2.426) - Jenkins automation server
func FormatRepoPackage(pkg hdos.RepoPackage) (result string) {
	result = pkg.Ref()
	if pkg.Version != "" {
		result = fmt.Sprintf("%s (%s)", result, pkg.Version)
	}
	if pkg.Description != "" {
		result = fmt.Sprintf("%s - %s", result, pkg.Description)
	}
	return
}

func search(keyword string, fetch bool, fetcher installer.Fetcher, writer io.Writer, logger *log.LevelLog) (err error) {
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
	result := installer.FindByKeyword(keyword, configDir)
	for _, item := range result {
		fmt.Fprintln(writer, item)
	}
	return
}
