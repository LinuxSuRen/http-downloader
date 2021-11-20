package brew

import "github.com/linuxsuren/http-downloader/pkg/os/core"

// SetInstallerRegistry sets the installer of registry
func SetInstallerRegistry(registry core.InstallerRegistry) {
	registry.Registry("vim", &vimInstallerInMacOS{})
	registry.Registry("bash-completion", &vimInstallerInMacOS{})
	registry.Registry("asciinema", &asciinemaInstallerInMacOS{})
	registry.Registry("ffmpge", &ffmpegInstallerInMacOS{})
}
