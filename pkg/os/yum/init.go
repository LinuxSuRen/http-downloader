package yum

import "github.com/linuxsuren/http-downloader/pkg/os/core"

// SetInstallerRegistry sets the installer of registry
func SetInstallerRegistry(registry core.InstallerRegistry) {
	registry.Registry("docker", &dockerInstallerInCentOS{})
	registry.Registry("conntrack", &conntrackInstallerInCentOS{})
	registry.Registry("socat", &socatInstallerInCentOS{})
	registry.Registry("vim", &vimInstallerInCentOS{})
	registry.Registry("golang", &golangInstallerInCentOS{})
	registry.Registry("git", &gitInstallerInCentOS{})
	registry.Registry("kubectl", &kubectlInstallerInCentOS{})
}
