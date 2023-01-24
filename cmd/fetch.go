package cmd

import (
	"context"
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg"
	"os"

	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/spf13/cobra"
)

func newFetchCmd(context.Context) (cmd *cobra.Command) {
	opt := &fetchOption{
		fetcher: &installer.DefaultFetcher{},
	}
	cmd = &cobra.Command{
		Use:     "fetch",
		Short:   "Fetch the latest hd config",
		PreRunE: opt.preRunE,
		RunE:    opt.runE,
	}

	flags := cmd.Flags()
	opt.addFlags(flags)
	flags.StringVarP(&opt.branch, "branch", "b", installer.ConfigBranch,
		"The branch of git repository (not support currently)")
	flags.BoolVarP(&opt.reset, "reset", "", false,
		"If you want to reset the hd-config which means delete and clone it again")

	_ = cmd.RegisterFlagCompletionFunc("provider", ArrayCompletion(ProviderGitHub, ProviderGitee))
	return
}

func (o *fetchOption) preRunE(c *cobra.Command, _ []string) (err error) {
	if c.Context() != nil {
		o.fetcher.SetContext(c.Context())
	}
	if o.reset {
		var configDir string
		if configDir, err = o.fetcher.GetConfigDir(); err == nil {
			err = os.RemoveAll(configDir)
			err = pkg.ErrorWrap(err, "failed to remove directory: %s, error %v", configDir, err)
		} else {
			err = fmt.Errorf("failed to get config directory, error %v", err)
		}
	}
	return
}

func (o *fetchOption) runE(cmd *cobra.Command, _ []string) (err error) {
	return o.fetcher.FetchLatestRepo(o.Provider, o.branch, cmd.OutOrStdout())
}

type fetchOption struct {
	searchOption

	branch  string
	reset   bool
	fetcher installer.Fetcher
}
