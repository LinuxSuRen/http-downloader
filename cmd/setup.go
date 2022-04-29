package cmd

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"path"
)

func newSetupCommand() (cmd *cobra.Command) {
	opt := &setupOption{}
	cmd = &cobra.Command{
		Use:   "setup",
		Short: "Init the configuration of hd",
		RunE:  opt.runE,
	}
	return
}

type setupOption struct {
}

func (o *setupOption) runE(cmd *cobra.Command, args []string) (err error) {
	selector := &survey.Select{
		Message: "Select proxy-github",
		Options: []string{"gh.api.99988866.xyz", "ghproxy.com", "mirror.ghproxy.com", ""},
	}

	var choose string
	if err = survey.AskOne(selector, &choose); err == nil {
		viper.Set("proxy-github", choose)
	} else {
		return
	}

	configDir := os.ExpandEnv("$HOME/.config")
	if err = os.MkdirAll(configDir, 0755); err != nil {
		err = fmt.Errorf("failed to create directory: %s, error: %v", configDir, err)
		return
	}

	err = viper.SafeWriteConfigAs(path.Join(configDir, "hd.yaml"))
	return
}
