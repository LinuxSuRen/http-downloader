package dnf

import (
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"github.com/linuxsuren/http-downloader/pkg/os/core"
)

// SetInstallerRegistry sets the installer of registry
func SetInstallerRegistry(registry core.InstallerRegistry, defaultExecer exec.Execer) {
	registry.Registry("docker", &dockerInstallerInFedora{Execer: defaultExecer})
}
