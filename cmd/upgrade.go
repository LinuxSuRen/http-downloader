package cmd

import (
	"context"
	"fmt"

	fakeruntime "github.com/linuxsuren/go-fake-runtime"
	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/spf13/cobra"
)

func newUpgradeCmd(ctx context.Context) (cmd *cobra.Command) {
	opt := &upgradeOption{
		installOption: &installOption{
			downloadOption: newDownloadOption(ctx),
			execer:         &fakeruntime.DefaultExecer{},
		},
	}
	cmd = &cobra.Command{
		Use:     "upgrade",
		Aliases: []string{"up"},
		Short:   "Upgrade the tools which are installed by hd",
		Example: "hd upgrade\nhd upgrade jcli",
		RunE:    opt.runE,
		GroupID: coreGroup.ID,
	}
	return
}

type upgradeOption struct {
	*installOption
}

func (o *upgradeOption) runE(cmd *cobra.Command, args []string) (err error) {
	tools, err := installer.LoadInstalledPackages()
	if err != nil {
		return
	}

	if len(tools) == 0 {
		cmd.Println("no installed tools, install one via: hd install <name>")
		return
	}

	found := 0
	for i := range tools {
		tool := tools[i]

		if len(args) > 0 && !matchName(args, tool.Name) {
			continue
		}
		found++

		if tool.Native {
			cmd.Printf("skip the native package [%s], please upgrade it by yourself\n", tool.Name)
			continue
		}

		if tool.Org == "" || tool.Repo == "" {
			cmd.Printf("skip [%s], no source information found\n", tool.Name)
			continue
		}

		cmd.Printf("upgrading %s/%s\n", tool.Org, tool.Repo)
		source := fmt.Sprintf("%s/%s", tool.Org, tool.Repo)
		if err = o.upgradeOne(cmd, source); err != nil {
			cmd.Printf("failed to upgrade %s, error: %v\n", tool.Name, err)
			err = nil // continue with the rest tools
		}
		o.Output = ""
	}

	if found == 0 && len(args) > 0 {
		err = fmt.Errorf("cannot find any installed tools by: %v", args)
	}
	return
}

func (o *upgradeOption) upgradeOne(cmd *cobra.Command, source string) (err error) {
	inner := &installOption{
		downloadOption: o.downloadOption,
		execer:         o.execer,
		fromSource:     o.fromSource,
		fromBranch:     o.fromBranch,
		target:         o.target,
		goget:          o.goget,
		force:          true, // always install the latest version while upgrading
		CleanPackage:   true,
		Download:       true,
	}
	if err = inner.preRunE(cmd, []string{source}); err != nil {
		return
	}
	return inner.install(cmd, []string{source})
}

func matchName(args []string, name string) (matched bool) {
	for _, arg := range args {
		if arg == name {
			return true
		}
	}
	return
}
