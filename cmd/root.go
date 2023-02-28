package cmd

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/linuxsuren/http-downloader/pkg/log"

	"github.com/AlecAivazis/survey/v2/terminal"
	extver "github.com/linuxsuren/cobra-extension/version"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var coreGroup *cobra.Group
var configGroup *cobra.Group

func init() {
	coreGroup = &cobra.Group{ID: "core", Title: "Core"}
	configGroup = &cobra.Group{ID: "conig", Title: "Config"}
}

// NewRoot returns the root command
func NewRoot(cxt context.Context) (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "hd",
		Short: "HTTP download tool",
	}
	cmd.AddGroup(coreGroup, configGroup)

	if err := loadConfig(); err != nil {
		panic(err)
	}

	v := viper.GetViper()
	stdio := terminal.Stdio{
		Out: os.Stdout,
		In:  os.Stdin,
		Err: os.Stderr,
	}

	cxt = context.WithValue(cxt, log.LoggerContextKey, log.GetLogger())
	cmd.AddCommand(
		newGetCmd(cxt), newInstallCmd(cxt), newFetchCmd(cxt), newSearchCmd(cxt), newSetupCommand(v, stdio),
		extver.NewVersionCmd("linuxsuren", "http-downloader", "hd", nil))

	for _, c := range cmd.Commands() {
		registerFlagCompletionFunc(c, "provider", ArrayCompletion(ProviderGitHub, ProviderGitee))
		registerFlagCompletionFunc(c, "proxy-github", ArrayCompletion("gh.api.99988866.xyz",
			"ghproxy.com", "mirror.ghproxy.com"))
		registerFlagCompletionFunc(c, "os", ArrayCompletion("window", "linux", "darwin"))
		registerFlagCompletionFunc(c, "arch", ArrayCompletion("amd64", "arm64"))
		registerFlagCompletionFunc(c, "format", ArrayCompletion("tar.gz", "zip", "msi"))
	}
	return
}

func registerFlagCompletionFunc(cmd *cobra.Command, flag string, completionFunc CompletionFunc) {
	if p := cmd.Flag(flag); p != nil {
		if err := cmd.RegisterFlagCompletionFunc(flag, completionFunc); err != nil {
			cmd.Println(err)
		}
	}
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
	viper.SetDefault("fetch", false)
	viper.SetDefault("goget", false)
	viper.SetDefault("no-proxy", false)

	thread := runtime.NumCPU()
	if thread > 4 {
		thread = thread / 2
	}
	viper.SetDefault("thread", thread)
	return
}
