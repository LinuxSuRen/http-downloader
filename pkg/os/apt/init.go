package apt

import "github.com/linuxsuren/http-downloader/pkg/os/core"

func SetInstallerRegistry(registry core.InstallerRegistry) {
	registry.Registry("docker", &DockerInstallerInUbuntu{})
}
