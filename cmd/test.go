package cmd

import (
	"github.com/linuxsuren/http-downloader/pkg/os"
	"github.com/spf13/cobra"
)

func newTestCmd() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:    "test",
		Hidden: true,
		Args:   cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			install := args[0]
			target := args[1]

			switch install {
			case "install":
				err = os.Install(target)
			case "uninstall":
				err = os.Uninstall(target)
			}
			return
		},
	}
	return
}
