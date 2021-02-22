package cmd

import (
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg/installer"
	"context"
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
	flags.StringVarP(&opt.provider, "provider", "p", "github",
		"The provider of hd-home repository. You can pass it a name (github, gitee) or a public git repository URI. Please use option --reset=true if you want to change the provider.")
	flags.StringVarP(&opt.branch, "branch", "b", installer.ConfigBranch,
		"The branch of git repository (not support currently)")
	flags.BoolVarP(&opt.reset, "reset", "", false,
		"If you want to reset the hd-config which means delete and clone it again")
	return
}

func (o *fetchOption) preRunE(_ *cobra.Command, _ []string) (err error) {
	switch o.provider {
	case "github":
		o.provider = installer.ConfigGitHub
	case "gitee":
		o.provider = "https://gitee.com/LinuxSuRen/hd-home"
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
	return installer.FetchLatestRepo(o.provider, o.branch, cmd.OutOrStdout())
}

type fetchOption struct {
	provider string
	branch   string
	reset    bool
}
