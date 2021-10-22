package cmd

import (
	"context"
	extpkg "github.com/linuxsuren/cobra-extension/pkg"
	extver "github.com/linuxsuren/cobra-extension/version"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"runtime"
)

// NewRoot returns the root command
func NewRoot(cxt context.Context) (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "hd",
		Short: "HTTP download tool",
	}

	if err := loadConfig(); err != nil {
		panic(err)
	}

	cmd.AddCommand(
		newGetCmd(cxt), newInstallCmd(cxt), newFetchCmd(cxt), newSearchCmd(cxt), newTestCmd(),
		extver.NewVersionCmd("linuxsuren", "http-downloader", "hd", nil),
		extpkg.NewCompletionCmd(cmd))
	return
}

func loadConfig() (err error) {
	viper.SetConfigName("hd")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("$HOME/.config")
	viper.AddConfigPath(".")
	if err = viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error if desired
			err = nil
		}
	}
	viper.SetDefault("provider", ProviderGitHub)
	viper.SetDefault("fetch", true)
	viper.SetDefault("thread", runtime.NumCPU()/2)
	viper.SetDefault("goget", false)
	return
}
