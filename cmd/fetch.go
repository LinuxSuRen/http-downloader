package cmd

import (
	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/spf13/cobra"
)

func newFetchCmd() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "fetch",
		Short: "Fetch the latest hd config",
		RunE: func(_ *cobra.Command, _ []string) (err error) {
			return installer.FetchConfig()
		},
	}
	return
}
