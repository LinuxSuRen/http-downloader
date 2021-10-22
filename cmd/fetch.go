package cmd

import (
	"context"
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/spf13/cobra"
	"os"
)

func newFetchCmd(context.Context) (cmd *cobra.Command) {
	opt := &fetchOption{}
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

func (o *fetchOption) preRunE(_ *cobra.Command, _ []string) (err error) {
	switch o.Provider {
	case ProviderGitHub:
		o.Provider = installer.ConfigGitHub
	case ProviderGitee:
		o.Provider = "https://gitee.com/LinuxSuRen/hd-home"
	case "":
		err = fmt.Errorf("--provider cannot be empty")
		return
	}

	if o.reset {
		var configDir string
		if configDir, err = installer.GetConfigDir(); err == nil {
			if err = os.RemoveAll(configDir); err != nil {
				err = fmt.Errorf("failed to remove directory: %s, error %v", configDir, err)
				return
			}
		} else {
			err = fmt.Errorf("failed to get config directory, error %v", err)
			return
		}
	}
	return
}

func (o *fetchOption) runE(cmd *cobra.Command, _ []string) (err error) {
	return installer.FetchLatestRepo(o.Provider, o.branch, cmd.OutOrStdout())
}

type fetchOption struct {
	searchOption

	branch string
	reset  bool
}
