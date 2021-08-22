package os

import (
	"github.com/linuxsuren/http-downloader/pkg/os/apt"
	"github.com/linuxsuren/http-downloader/pkg/os/core"
	"github.com/linuxsuren/http-downloader/pkg/os/yum"
)

// DefaultInstallerRegistry is the default installer registry
type DefaultInstallerRegistry struct {
	installerMap map[string][]core.Installer
}

var defaultInstallerRegistry *DefaultInstallerRegistry

func init() {
	defaultInstallerRegistry = &DefaultInstallerRegistry{
		installerMap: map[string][]core.Installer{},
	}
	yum.SetInstallerRegistry(defaultInstallerRegistry)
	apt.SetInstallerRegistry(defaultInstallerRegistry)
}

// Registry registries a DockerInstaller
func (r *DefaultInstallerRegistry) Registry(name string, installer core.Installer) {
	_, ok := r.installerMap[name]
	if ok {
		r.installerMap[name] = append(r.installerMap[name], installer)
	} else {
		r.installerMap[name] = []core.Installer{installer}
	}
}

// GetInstallers returns all the installers belong to a package
func GetInstallers(name string) (installers []core.Installer, ok bool) {
	installers, ok = defaultInstallerRegistry.installerMap[name]
	return
}

// HasPackage finds if the target package installer exist
func HasPackage(name string) bool {
	if installers, ok := GetInstallers(name); ok {
		for _, installer := range installers {
			if installer.Available() {
				return true
			}
		}
	}
	return false
}

// Install installs a package with name
func Install(name string) (err error) {
	if installers, ok := GetInstallers(name); ok {
		for _, installer := range installers {
			if installer.Available() {
				err = installer.Install()
				break
			}
		}
	}
	return
}

// Uninstall uninstalls a package with name
func Uninstall(name string) (err error) {
	if installers, ok := GetInstallers(name); ok {
		for _, installer := range installers {
			if installer.Available() {
				err = installer.Uninstall()
				break
			}
		}
	}
	return
}
