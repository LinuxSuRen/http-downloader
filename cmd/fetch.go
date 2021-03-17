package cmd

import (
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"os"
	"path"
	"strings"
)

func newFetchCmd() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "fetch",
		Short: "Fetch the latest hd config",
		RunE: func(_ *cobra.Command, _ []string) (err error) {
			return fetchHomeConfig()
		},
	}
	return
}

func getConfigDir() (configDir string, err error) {
	var userHome string
	if userHome, err = homedir.Dir(); err == nil {
		configDir = path.Join(userHome, "/.config/hd-home")
	}
	return
}

func fetchHomeConfig() (err error) {
	var configDir string
	if configDir, err = getConfigDir(); err != nil {
		return
	}

	if ok, _ := pathExists(configDir); ok {
		err = execCommandInDir("git", configDir, "reset", "--hard", "origin/master")
		if err == nil {
			err = execCommandInDir("git", configDir, "pull")
		}
	} else {
		if err = os.MkdirAll(configDir, 0644); err == nil {
			err = execCommand("git", "clone", "https://github.com/LinuxSuRen/hd-home", configDir)
		}
	}

	if err != nil && strings.Contains(err.Error(), "exit status 128") {
		// target directory was created accidentally, remove it then try again
		_ = os.RemoveAll(configDir)
		return fetchHomeConfig()
	}
	return
}
