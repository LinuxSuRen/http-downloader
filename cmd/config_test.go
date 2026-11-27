package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/linuxsuren/http-downloader/pkg/remote"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func resetViper(t *testing.T) {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
}

func TestPlatformInstallDir(t *testing.T) {
	assert.Equal(t, installer.UnixBinDir, platformInstallDir("linux", ""))
	assert.Equal(t, installer.UnixBinDir, platformInstallDir("darwin", ""))
	assert.Equal(t, installer.WindowsBinDir, platformInstallDir("windows", ""))
	assert.Equal(t, "/usr/bin", platformInstallDir("windows", "/usr/bin/bash"))
	assert.NotContains(t, platformInstallDir("windows", ""), "/usr")
}

func TestEffectiveInstallDir(t *testing.T) {
	resetViper(t)
	assert.Equal(t, platformInstallDir(runtime.GOOS, os.Getenv("SHELL")), effectiveInstallDir())

	viper.Set(keyInstallDir, filepath.Join("opt", "hd", "bin"))
	assert.Equal(t, filepath.Join("opt", "hd", "bin"), configuredInstallDir())
	assert.Equal(t, filepath.Join("opt", "hd", "bin"), effectiveInstallDir())

	viper.Set(keyInstallDir, "  ")
	assert.Empty(t, configuredInstallDir())
}

func TestRemoteConfigRoundTrip(t *testing.T) {
	resetViper(t)
	cfg := remote.Config{
		Mode:     remote.ModeSSH,
		Host:     "example.com",
		Port:     2222,
		User:     "hd",
		Password: "secret",
		KeyFile:  "/home/hd/.ssh/id_rsa",
		Insecure: true,
		OS:       "linux",
		Arch:     "arm64",
	}
	saveRemoteConfig(cfg)

	got := loadRemoteConfig()
	assert.Equal(t, cfg.Mode, got.Mode)
	assert.Equal(t, cfg.Host, got.Host)
	assert.Equal(t, cfg.Port, got.Port)
	assert.Equal(t, cfg.User, got.User)
	assert.Equal(t, cfg.Password, got.Password)
	assert.Equal(t, cfg.KeyFile, got.KeyFile)
	assert.True(t, got.Insecure)
	assert.Equal(t, cfg.OS, got.OS)
	assert.Equal(t, cfg.Arch, got.Arch)
}

func TestLoadRemoteConfigDefaults(t *testing.T) {
	v := viper.New()
	v.SetDefault(keySSHPort, 22)
	v.SetDefault(keySSHUser, "root")
	cfg := loadRemoteConfigFrom(v)
	assert.Equal(t, remote.ModeLocal, cfg.Mode)
	assert.Equal(t, 22, cfg.Port)
	assert.Equal(t, "root", cfg.User)
	assert.Equal(t, "linux", cfg.OS)
	assert.Equal(t, "amd64", cfg.Arch)
}

func TestParsePort(t *testing.T) {
	port, err := parsePort("")
	assert.NoError(t, err)
	assert.Zero(t, port)

	port, err = parsePort(" 2222 ")
	assert.NoError(t, err)
	assert.Equal(t, 2222, port)

	for _, value := range []string{"abc", "0", "-1", "65536"} {
		_, err = parsePort(value)
		assert.Error(t, err, value)
	}
}

func TestWriteHDConfig(t *testing.T) {
	home := t.TempDir()
	oldCache := homedir.DisableCache
	homedir.DisableCache = true
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Cleanup(func() { homedir.DisableCache = oldCache })

	v := viper.New()
	v.Set(keyInstallDir, "D:\\hd\\bin")
	path, err := writeHDConfig(v)
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".config", "hd.yaml"), path)

	data, err := os.ReadFile(path)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "install-dir")
}
