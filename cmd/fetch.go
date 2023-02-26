package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/linuxsuren/http-downloader/pkg"
	"github.com/linuxsuren/http-downloader/pkg/log"

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
		GroupID: configGroup.ID,
	}

	flags := cmd.Flags()
	opt.addFlags(flags)
	flags.StringVarP(&opt.branch, "branch", "b", installer.ConfigBranch,
		"The branch of git repository (not support currently)")
	flags.BoolVarP(&opt.reset, "reset", "", false,
		"If you want to reset the hd-config which means delete and clone it again")
	flags.IntVarP(&opt.retry, "retry", "", 6, "Retry times due to timeout error")
	flags.DurationVarP(&opt.timeout, "timeout", "", time.Second*10, "Timeout of fetching")
	return
}

func (o *fetchOption) setTimeout(c *cobra.Command) {
	if c.Context() != nil {
		var ctx context.Context
		ctx, o.cancel = context.WithTimeout(c.Context(), o.timeout)
		o.fetcher.SetContext(ctx)
	}
}

func (o *fetchOption) preRunE(c *cobra.Command, _ []string) (err error) {
	o.setTimeout(c)
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

func (o *fetchOption) runE(c *cobra.Command, _ []string) (err error) {
	logger := log.GetLoggerFromContextOrDefault(c)

	var i int
	for i = 0; i < o.retry; i++ {
		err = o.fetcher.FetchLatestRepo(o.Provider, o.branch, c.OutOrStdout())
		if err == nil || (!strings.Contains(err.Error(), "context deadline exceeded") &&
			!strings.Contains(err.Error(), "i/o timeout")) {
			break
		}
		o.setTimeout(c)
		logger.Print(".")
	}
	if i >= 1 {
		logger.Println()
	}
	return
}

type fetchOption struct {
	searchOption

	branch  string
	reset   bool
	fetcher installer.Fetcher
	cancel  context.CancelFunc
	retry   int
	timeout time.Duration
}
