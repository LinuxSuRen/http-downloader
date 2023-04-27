package dnf

import (
	fakeruntime "github.com/linuxsuren/go-fake-runtime"
	"github.com/linuxsuren/http-downloader/pkg/os/core"
)

// SetInstallerRegistry sets the installer of registry
func SetInstallerRegistry(registry core.InstallerRegistry, defaultExecer fakeruntime.Execer) {
	registry.Registry("docker", &dockerInstallerInFedora{Execer: defaultExecer})
}
