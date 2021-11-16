package cmd

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
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

	err = viper.SafeWriteConfigAs(os.ExpandEnv("$HOME/.config/hd.yaml"))
	return
}
