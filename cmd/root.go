package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/linuxsuren/http-downloader/pkg/log"
	"github.com/mitchellh/go-homedir"

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

	v := viper.GetViper()
	if err := loadConfig(v); err != nil {
		panic(err)
	}

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

func loadConfig(v *viper.Viper) (err error) {
	var userHome string
	if userHome, err = homedir.Dir(); err != nil {
		return
	}

	configDir := filepath.Join(userHome, ".config")
	v.SetConfigName("hd")
	v.SetConfigType("yaml")
	v.AddConfigPath(configDir)
	if err = v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error if desired
			err = nil
		} else {
			err = fmt.Errorf("failed to load config: %s, error: %v", os.ExpandEnv("$HOME/.config/hd.yaml"), err)
		}
	}
	v.SetDefault("provider", ProviderGitHub)
	v.SetDefault("fetch", false)
	v.SetDefault("goget", false)
	v.SetDefault("no-proxy", false)

	thread := runtime.NumCPU()
	if thread > 4 {
		thread = thread / 2
	}
	v.SetDefault("thread", thread)
	return
}
