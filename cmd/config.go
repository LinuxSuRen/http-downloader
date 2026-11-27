package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/linuxsuren/http-downloader/pkg/remote"
	"github.com/spf13/viper"
)

// Configuration keys of hd.yaml. The install directory is empty by default
// which means the platform specific directory is used.
const (
	keyInstallDir  = "install-dir"
	keyInstallMode = "install-mode"
	keySSHHost     = "ssh-host"
	keySSHPort     = "ssh-port"
	keySSHUser     = "ssh-user"
	keySSHPassword = "ssh-password"
	keySSHKeyFile  = "ssh-key"
	keySSHInsecure = "ssh-insecure"
	keySSHOS       = "ssh-os"
	keySSHArch     = "ssh-arch"
)

// platformInstallDir returns the default directory for a platform. Git Bash
// on Windows uses the Unix layout of its emulation layer.
func platformInstallDir(goos, shell string) string {
	if strings.EqualFold(goos, "windows") && shell == "/usr/bin/bash" {
		return "/usr/bin"
	}
	return installer.DefaultInstallDir(goos)
}

// configuredInstallDir returns the directory saved in hd.yaml, if any
func configuredInstallDir() string {
	return strings.TrimSpace(viper.GetString(keyInstallDir))
}

// effectiveInstallDir is the default used by the install command and the web
// console when neither a CLI flag nor a per-tool config defines a directory
func effectiveInstallDir() string {
	if dir := configuredInstallDir(); dir != "" {
		return dir
	}
	return platformInstallDir(runtime.GOOS, os.Getenv("SHELL"))
}

// loadRemoteConfig reads the install mode and SSH settings from hd.yaml
func loadRemoteConfig() remote.Config {
	return loadRemoteConfigFrom(viper.GetViper())
}

// loadRemoteConfigFrom is the same as loadRemoteConfig for a custom viper,
// which the setup command uses in its tests
func loadRemoteConfigFrom(v *viper.Viper) remote.Config {
	cfg := remote.Config{
		Mode:     strings.TrimSpace(v.GetString(keyInstallMode)),
		Host:     strings.TrimSpace(v.GetString(keySSHHost)),
		Port:     v.GetInt(keySSHPort),
		User:     strings.TrimSpace(v.GetString(keySSHUser)),
		Password: v.GetString(keySSHPassword),
		KeyFile:  strings.TrimSpace(v.GetString(keySSHKeyFile)),
		Insecure: v.GetBool(keySSHInsecure),
		OS:       strings.TrimSpace(v.GetString(keySSHOS)),
		Arch:     strings.TrimSpace(v.GetString(keySSHArch)),
	}
	if cfg.Mode == "" {
		cfg.Mode = remote.ModeLocal
	}
	return cfg.Normalize()
}

// saveRemoteConfig persists the install mode and SSH settings
func saveRemoteConfig(cfg remote.Config) {
	saveRemoteConfigTo(viper.GetViper(), cfg)
}

// saveRemoteConfigTo writes the settings into the given viper instance
func saveRemoteConfigTo(v *viper.Viper, cfg remote.Config) {
	cfg = cfg.Normalize()
	v.Set(keyInstallMode, cfg.Mode)
	v.Set(keySSHHost, cfg.Host)
	v.Set(keySSHPort, cfg.Port)
	v.Set(keySSHUser, cfg.User)
	v.Set(keySSHKeyFile, cfg.KeyFile)
	v.Set(keySSHInsecure, cfg.Insecure)
	v.Set(keySSHOS, cfg.OS)
	v.Set(keySSHArch, cfg.Arch)
	if cfg.Password != "" {
		v.Set(keySSHPassword, cfg.Password)
	}
}

// writeHDConfig saves the current settings to ~/.config/hd.yaml. Both the
// setup command and the web console use it so they share one config file.
func writeHDConfig(v *viper.Viper) (configPath string, err error) {
	var home string
	if home, err = (&installer.DefaultFetcher{}).GetHomeDir(); err != nil {
		return
	}
	configDir := filepath.Join(home, ".config")
	if err = os.MkdirAll(configDir, 0750); err != nil {
		err = fmt.Errorf("failed to create directory: %s, error: %v", configDir, err)
		return
	}
	configPath = filepath.Join(configDir, "hd.yaml")
	err = v.WriteConfigAs(configPath)
	return
}

// parsePort converts a port from a flag or a web request, zero keeps the
// SSH default
func parsePort(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid SSH port: %s", value)
	}
	return port, nil
}
