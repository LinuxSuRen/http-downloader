package cmd

import (
	"fmt"
	"os"
	"path"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newSetupCommand(v *viper.Viper, stdio terminal.Stdio) (cmd *cobra.Command) {
	opt := &setupOption{
		stdio: stdio,
		v:     v,
	}
	cmd = &cobra.Command{
		Use:   "setup",
		Short: "Init the configuration of hd",
		RunE:  opt.runE,
	}
	return
}

type setupOption struct {
	stdio terminal.Stdio
	v     *viper.Viper
}

func (o *setupOption) runE(cmd *cobra.Command, args []string) (err error) {
	var (
		proxyGitHub string
		provider    string
	)

	if proxyGitHub, err = selectFromList([]string{"ghproxy.com", "gh.api.99988866.xyz", "mirror.ghproxy.com", ""},
		o.v.GetString("proxy-github"),
		"Select proxy-github", o.stdio); err == nil {
		o.v.Set("proxy-github", proxyGitHub)
	} else {
		return
	}

	if provider, err = selectFromList([]string{"github", "gitee"}, o.v.GetString("provider"),
		"Select provider", o.stdio); err == nil {
		o.v.Set("provider", provider)
	} else {
		return
	}

	configDir := os.ExpandEnv("$HOME/.config")
	if err = os.MkdirAll(configDir, 0750); err != nil {
		err = fmt.Errorf("failed to create directory: %s, error: %v", configDir, err)
		return
	}

	err = o.v.WriteConfigAs(path.Join(configDir, "hd.yaml"))
	return
}

func selectFromList(items []string, defaultItem, title string, stdio terminal.Stdio) (val string, err error) {
	selector := &survey.Select{
		Message: title,
		Options: items,
		Default: defaultItem,
	}
	err = survey.AskOne(selector, &val, survey.WithStdio(stdio.In, stdio.Out, stdio.Err))
	return
}
