package cmd

import (
	"context"
	extpkg "github.com/linuxsuren/cobra-extension/pkg"
	extver "github.com/linuxsuren/cobra-extension/version"
	"github.com/spf13/cobra"
)

// NewRoot returns the root command
func NewRoot(cxt context.Context) (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "hd",
		Short: "HTTP download tool",
	}

	cmd.AddCommand(
		newGetCmd(cxt), newInstallCmd(cxt), newFetchCmd(cxt), newSearchCmd(cxt), newTestCmd(),
		extver.NewVersionCmd("linuxsuren", "http-downloader", "hd", nil),
		extpkg.NewCompletionCmd(cmd))
	return
}
