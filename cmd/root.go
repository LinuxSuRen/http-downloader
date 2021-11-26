package cmd

import (
	"context"
	"fmt"
	extpkg "github.com/linuxsuren/cobra-extension/pkg"
	extver "github.com/linuxsuren/cobra-extension/version"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
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
		newGetCmd(cxt), newInstallCmd(cxt), newFetchCmd(cxt), newSearchCmd(cxt), newTestCmd(), newSetupCommand(),
		extver.NewVersionCmd("linuxsuren", "http-downloader", "hd", nil),
		extpkg.NewCompletionCmd(cmd))
	return
}

func loadConfig() (err error) {
	viper.SetConfigName("hd")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("$HOME/.config")
	if err = viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error if desired
			err = nil
		} else {
			err = fmt.Errorf("failed to load config: %s, error: %v", os.ExpandEnv("$HOME/.config/hd.yaml"), err)
		}
	}
	viper.SetDefault("provider", ProviderGitHub)
	viper.SetDefault("fetch", true)
	viper.SetDefault("thread", runtime.NumCPU()/2)
	viper.SetDefault("goget", false)
	return
}
