package docker

import (
	fakeruntime "github.com/linuxsuren/go-fake-runtime"
	"github.com/linuxsuren/http-downloader/pkg/os/core"
)

// SetInstallerRegistry sets the installer of registry
func SetInstallerRegistry(registry core.InstallerRegistry, execer fakeruntime.Execer) {
	registry.Registry("bitbucket", &bitbucket{Execer: execer})
}
