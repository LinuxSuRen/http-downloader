package cmd

import (
	"context"
	"os"

	fakeruntime "github.com/linuxsuren/go-fake-runtime"
	"github.com/linuxsuren/http-downloader/pkg/installer"
	npminstaller "github.com/linuxsuren/http-downloader/pkg/os/npm"
	"github.com/linuxsuren/http-downloader/pkg/remote"
)

// uninstallNPMPackage removes a globally installed npm package from the
// target recorded when it was installed
func uninstallNPMPackage(tool *installer.InstalledPackage, execer fakeruntime.Execer) error {
	if tool.Mode == remote.ModeSSH {
		cfg := loadRemoteConfig()
		exec, err := remote.NewSSHExecutor(context.Background(), cfg)
		if err != nil {
			return err
		}
		defer exec.Close()
		return exec.Run("npm uninstall -g "+remote.Quote(tool.Name), nil, os.Stdout, os.Stderr)
	}
	uninstaller := &npminstaller.CommonInstaller{Name: tool.Name, Execer: execer}
	return uninstaller.Uninstall()
}
