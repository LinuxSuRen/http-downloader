package cmd

import (
	extver "github.com/linuxsuren/cobra-extension/version"
	"github.com/spf13/cobra"
)

// NewRoot returns the root command
func NewRoot() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "hd",
		Short: "HTTP download tool",
	}

	cmd.AddCommand(
		NewGetCmd(), NewInstallCmd(),
		extver.NewVersionCmd("linuxsuren", "http-downloader", "hd", nil))
	return
}
