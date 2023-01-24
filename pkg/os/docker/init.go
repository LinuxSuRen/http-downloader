package docker

import (
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"github.com/linuxsuren/http-downloader/pkg/os/core"
)

// SetInstallerRegistry sets the installer of registry
func SetInstallerRegistry(registry core.InstallerRegistry, execer exec.Execer) {
	registry.Registry("bitbucket", &bitbucket{Execer: execer})
}
