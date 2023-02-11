package os

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/linuxsuren/http-downloader/pkg/exec"
	"github.com/linuxsuren/http-downloader/pkg/os/apt"
	"github.com/linuxsuren/http-downloader/pkg/os/core"
	"github.com/linuxsuren/http-downloader/pkg/os/dnf"
	"github.com/linuxsuren/http-downloader/pkg/os/docker"
	"github.com/linuxsuren/http-downloader/pkg/os/yum"
	"github.com/mitchellh/go-homedir"
)

// DefaultInstallerRegistry is the default installer registry
type DefaultInstallerRegistry struct {
	installerMap map[string][]core.Installer
}

var defaultInstallerRegistry *DefaultInstallerRegistry

func init() {
	defaultExecer := exec.DefaultExecer{}
	defaultInstallerRegistry = &DefaultInstallerRegistry{
		installerMap: map[string][]core.Installer{},
	}
	yum.SetInstallerRegistry(defaultInstallerRegistry, defaultExecer)
	apt.SetInstallerRegistry(defaultInstallerRegistry, defaultExecer)
	docker.SetInstallerRegistry(defaultInstallerRegistry, defaultExecer)
	dnf.SetInstallerRegistry(defaultInstallerRegistry, defaultExecer)

	var userHome string
	var err error
	if userHome, err = homedir.Dir(); err == nil {
		configDir := path.Join(userHome, "/.config/hd-home")
		if err = GenericInstallerRegistry(filepath.Join(configDir, "config/generic.yaml"), defaultInstallerRegistry); err != nil {
			fmt.Println(err)
		}
	}
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
		for _, item := range installers {
			if item.Available() {
				return true
			}
		}
	}
	return false
}

// SearchPackages searches the packages by keyword
func SearchPackages(keyword string) (pkgs []string) {
	for key, pms := range defaultInstallerRegistry.installerMap {
		if strings.Contains(key, keyword) {
			var available bool
			for _, pm := range pms {
				if pm.Available() {
					available = true
					break
				}
			}

			if available {
				pkgs = append(pkgs, key)
			}
		}
	}
	return
}

// InstallWithProxy installs a package with name
func InstallWithProxy(name string, proxy map[string]string) (err error) {
	var installer core.Installer
	if installers, ok := GetInstallers(name); ok {
		for _, installer = range installers {
			if installer.Available() {
				if proxyAble, ok := installer.(core.ProxyAble); ok {
					proxyAble.SetURLReplace(proxy)
				}

				err = installer.Install()
				break
			}
		}
	}

	if installer != nil && err == nil {
		if err = installer.Start(); err != nil {
			err = fmt.Errorf("failed to start service %s, error: %v", name, err)
			return
		}

		var ok bool
		if ok, err = installer.WaitForStart(); err != nil {
			err = fmt.Errorf("failed to check the service status of %s, error: %v", name, err)
		} else if !ok {
			err = fmt.Errorf("%s was not started yet, please check it manually", name)
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
