package cmd

import (
	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/spf13/cobra"
	"runtime"
)

type initOption struct {
	require, optional, fetch bool

	// inner fields
	requireTools, optionalTools map[string]string
}

// NewInitCommand returns a command for init
func NewInitCommand(requireTools, optionalTools map[string]string) (cmd *cobra.Command) {
	opt := &initOption{
		requireTools:  requireTools,
		optionalTools: optionalTools,
	}

	cmd = &cobra.Command{
		Use:   "init",
		Short: "Init your command",
		RunE:  opt.runE,
	}

	flags := cmd.Flags()
	flags.BoolVarP(&opt.require, "require", "r", true,
		"Indicate if you want to install required tools")
	flags.BoolVarP(&opt.optional, "optional", "o", false,
		"Indicate if you want to install optional tools")
	flags.BoolVarP(&opt.fetch, "fetch", "", true,
		"Indicate if fetch the latest config of tools")
	return
}

func (o *initOption) runE(_ *cobra.Command, _ []string) (err error) {
	is := installer.Installer{
		Provider: "github",
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Fetch:    o.fetch,
	}
	if o.require {
		err = is.CheckDepAndInstall(o.requireTools)
	}
	if err == nil && o.optional {
		err = is.CheckDepAndInstall(o.optionalTools)
	}
	return
}
