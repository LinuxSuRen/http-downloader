package apt

import (
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"github.com/linuxsuren/http-downloader/pkg/os/core"
)

// SetInstallerRegistry sets the installer of registry
func SetInstallerRegistry(registry core.InstallerRegistry, defaultExecer exec.Execer) {
	registry.Registry("docker", &dockerInstallerInUbuntu{Execer: defaultExecer})
	registry.Registry("conntrack", &conntrackInstallerInUbuntu{Execer: defaultExecer})
	registry.Registry("socat", &socatInstallerInUbuntu{Execer: defaultExecer})
	registry.Registry("vim", &vimInstallerInUbuntu{Execer: defaultExecer})
	registry.Registry("golang", &golangInstallerInUbuntu{Execer: defaultExecer})
	registry.Registry("git", &gitInstallerInUbuntu{Execer: defaultExecer})
	registry.Registry("bash-completion", &bashCompletionInstallerInUbuntu{Execer: defaultExecer})
	registry.Registry("asciinema", &asciinemaInstallerInUbuntu{Execer: defaultExecer})
	registry.Registry("ffmpge", &ffmpegInstallerInUbuntu{Execer: defaultExecer})
}
