package dnf

import (
	"fmt"
	"strings"
	"time"

	fakeruntime "github.com/linuxsuren/go-fake-runtime"
)

// dockerInstallerInFedora is the installer of Docker in Fedora
type dockerInstallerInFedora struct {
	Execer fakeruntime.Execer
	count  int
}

// Available check if support current platform
func (d *dockerInstallerInFedora) Available() (ok bool) {
	if d.Execer.OS() == "linux" {
		_, err := d.Execer.LookPath("dnf")
		ok = err == nil
	}
	return
}

// Install installs the Docker
func (d *dockerInstallerInFedora) Install() (err error) {
	if err = d.Execer.RunCommand("dnf", "-y", "install", "dnf-plugins-core"); err != nil {
		return
	}

	if err = d.Execer.RunCommand("dnf", "config-manager",
		"--add-repo", "https://download.docker.com/linux/fedora/docker-ce.repo"); err != nil {
		return
	}

	err = d.Execer.RunCommand("dnf", "install", "docker-ce", "docker-ce-cli", "containerd.io", "docker-compose-plugin")
	return
}

// Uninstall uninstalls the Docker
func (d *dockerInstallerInFedora) Uninstall() (err error) {
	err = d.Execer.RunCommand("dnf", "remove", "docker",
		"docker-client", "docker-client-latest", "docker-common", "docker-latest",
		"docker-latest-logrotate",
		"docker-logrotate",
		"docker-selinux",
		"docker-engine-selinux",
		"docker-engine")
	return
}

// WaitForStart waits for the service be started
func (d *dockerInstallerInFedora) WaitForStart() (ok bool, err error) {
	var result string
	if result, err = d.Execer.RunCommandAndReturn("systemctl", "", "status", "docker"); err != nil {
		return
	} else if strings.Contains(result, "Unit docker.service could not be found") {
		err = fmt.Errorf("unit docker.service could not be found")
	} else if strings.Contains(result, "Active: active") {
		ok = true
	} else {
		if d.count > 0 {
			fmt.Println("waiting for Docker service start")
		} else if d.count > 9 {
			return
		}

		d.count++
		time.Sleep(time.Second * 1)
		return d.WaitForStart()
	}
	return
}

// Start starts the Docker service
func (d *dockerInstallerInFedora) Start() (err error) {
	err = d.Execer.RunCommand("systemctl", "start", "docker")
	return
}

// Stop stops the Docker service
func (d *dockerInstallerInFedora) Stop() (err error) {
	err = d.Execer.RunCommand("systemctl", "stop", "docker")
	return
}
