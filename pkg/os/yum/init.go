package yum

import (
	fakeruntime "github.com/linuxsuren/go-fake-runtime"
	"github.com/linuxsuren/http-downloader/pkg/os/core"
)

// SetInstallerRegistry sets the installer of registry
func SetInstallerRegistry(registry core.InstallerRegistry, defaultExecer fakeruntime.Execer) {
	registry.Registry("docker", &dockerInstallerInCentOS{Execer: defaultExecer})
	registry.Registry("conntrack", &conntrackInstallerInCentOS{Execer: defaultExecer})
	registry.Registry("socat", &socatInstallerInCentOS{Execer: defaultExecer})
	registry.Registry("vim", &vimInstallerInCentOS{Execer: defaultExecer})
	registry.Registry("golang", &golangInstallerInCentOS{Execer: defaultExecer})
	registry.Registry("git", &gitInstallerInCentOS{Execer: defaultExecer})
	registry.Registry("bash-completion", &bashCompletionInstallerInCentOS{Execer: defaultExecer})
}
