package cmd

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/linuxsuren/http-downloader/pkg/log"
	"github.com/linuxsuren/http-downloader/pkg/remote"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newSetupCommand(v *viper.Viper, stdio terminal.Stdio) (cmd *cobra.Command) {
	opt := &setupOption{
		stdio: stdio,
		v:     v,
	}
	cmd = &cobra.Command{
		Use:     "setup",
		Short:   "Init the configuration of hd",
		RunE:    opt.runE,
		GroupID: configGroup.ID,
	}
	flags := cmd.Flags()
	flags.StringVarP(&opt.proxy, "proxy", "p", "", "The proxy of GitHub")
	flags.StringVarP(&opt.provider, "provider", "", "", "The provider of hd configuration")
	flags.StringVar(&opt.installDir, keyInstallDir, "", "The default installation directory")
	flags.StringVar(&opt.mode, keyInstallMode, "", "The install mode: local or ssh")
	flags.StringVar(&opt.sshHost, keySSHHost, "", "The SSH host of the target server")
	flags.IntVar(&opt.sshPort, keySSHPort, 0, "The SSH port of the target server")
	flags.StringVar(&opt.sshUser, keySSHUser, "", "The SSH user of the target server")
	flags.StringVar(&opt.sshPassword, keySSHPassword, "", "The SSH password, an alternative to a private key")
	flags.StringVar(&opt.sshKey, keySSHKeyFile, "", "The SSH private key file")
	flags.BoolVar(&opt.sshInsecure, keySSHInsecure, false, "Skip the SSH host key verification")
	flags.StringVar(&opt.sshOS, keySSHOS, "", "The OS of the SSH target, linux by default")
	flags.StringVar(&opt.sshArch, keySSHArch, "", "The architecture of the SSH target, amd64 by default")
	return
}

type setupOption struct {
	stdio terminal.Stdio
	v     *viper.Viper

	proxy       string
	provider    string
	installDir  string
	mode        string
	sshHost     string
	sshPort     int
	sshUser     string
	sshPassword string
	sshKey      string
	sshInsecure bool
	sshOS       string
	sshArch     string
}

func (o *setupOption) runE(cmd *cobra.Command, args []string) (err error) {
	logger := log.GetLoggerFromContextOrDefault(cmd)
	proxyServers := []string{""}
	proxyServers = append(proxyServers, installer.GetProxyServers()...)

	if o.proxy == "" {
		if o.proxy, err = selectFromList(proxyServers,
			o.v.GetString("proxy-github"),
			"Select proxy-github", o.stdio); err != nil {
			return
		}
	}
	o.v.Set("proxy-github", o.proxy)

	if o.provider == "" {
		if o.provider, err = selectFromList([]string{"github", "gitee"}, o.v.GetString("provider"),
			"Select provider", o.stdio); err != nil {
			return
		}
	}
	o.v.Set("provider", o.provider)

	// Only the values explicitly given on the command line are changed,
	// an interactive setup without these flags keeps its previous settings.
	flags := cmd.Flags()
	if flags.Changed(keyInstallDir) {
		o.v.Set(keyInstallDir, strings.TrimSpace(o.installDir))
	}

	cfg := loadRemoteConfigFrom(o.v)
	if flags.Changed(keyInstallMode) {
		if o.mode != remote.ModeLocal && o.mode != remote.ModeSSH {
			err = fmt.Errorf("invalid install mode: %s, expected local or ssh", o.mode)
			return
		}
		cfg.Mode = o.mode
	}
	if flags.Changed(keySSHHost) {
		cfg.Host = strings.TrimSpace(o.sshHost)
	}
	if flags.Changed(keySSHPort) {
		cfg.Port = o.sshPort
	}
	if flags.Changed(keySSHUser) {
		cfg.User = strings.TrimSpace(o.sshUser)
	}
	if flags.Changed(keySSHPassword) {
		cfg.Password = o.sshPassword
	}
	if flags.Changed(keySSHKeyFile) {
		cfg.KeyFile = strings.TrimSpace(o.sshKey)
	}
	if flags.Changed(keySSHInsecure) {
		cfg.Insecure = o.sshInsecure
	}
	if flags.Changed(keySSHOS) {
		cfg.OS = strings.TrimSpace(o.sshOS)
	}
	if flags.Changed(keySSHArch) {
		cfg.Arch = strings.TrimSpace(o.sshArch)
	}
	if cfg.Mode == remote.ModeSSH {
		if err = cfg.Validate(); err != nil {
			return
		}
	}
	saveRemoteConfigTo(o.v, cfg)

	var configPath string
	if configPath, err = writeHDConfig(o.v); err != nil {
		return
	}
	logger.Info("write config into:", configPath)
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
