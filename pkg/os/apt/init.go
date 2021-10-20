package apt

import "github.com/linuxsuren/http-downloader/pkg/os/core"

// SetInstallerRegistry sets the installer of registry
func SetInstallerRegistry(registry core.InstallerRegistry) {
	registry.Registry("docker", &dockerInstallerInUbuntu{})
	registry.Registry("conntrack", &conntrackInstallerInUbuntu{})
	registry.Registry("socat", &socatInstallerInUbuntu{})
	registry.Registry("vim", &vimInstallerInUbuntu{})
	registry.Registry("golang", &golangInstallerInUbuntu{})
	registry.Registry("git", &gitInstallerInUbuntu{})
	registry.Registry("kubectl", &kubectlInstallerInUbuntu{})
	registry.Registry("bash-completion", &bashCompletionInstallerInUbuntu{})
	registry.Registry("asciinema", &asciinemaInstallerInUbuntu{})
}
