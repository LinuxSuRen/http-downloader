package apt

import "github.com/linuxsuren/http-downloader/pkg/os/core"

// SetInstallerRegistry sets the installer of registry
func SetInstallerRegistry(registry core.InstallerRegistry) {
	registry.Registry("docker", &DockerInstallerInUbuntu{})
}
