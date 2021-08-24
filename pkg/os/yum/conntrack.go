package yum

import (
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"runtime"
)

// conntrackInstallerInCentOS is the installer of Docker in CentOS
type conntrackInstallerInCentOS struct {
	count int
}

// Available check if support current platform
func (d *conntrackInstallerInCentOS) Available() (ok bool) {
	if runtime.GOOS == "linux" {
		_, err := exec.LookPath("yum")
		ok = err == nil
	}
	return
}

// Install installs the Docker
func (d *conntrackInstallerInCentOS) Install() (err error) {
	if err = exec.RunCommand("yum", "install", "-y",
		"conntrack"); err != nil {
		return
	}
	return
}

// Uninstall uninstalls the Docker
func (d *conntrackInstallerInCentOS) Uninstall() (err error) {
	err = exec.RunCommand("yum", "remove", "-y",
		"conntrack")
	return
}

// WaitForStart waits for the service be started
func (d *conntrackInstallerInCentOS) WaitForStart() (ok bool, err error) {
	ok = true
	return
}

// Start starts the Docker service
func (d *conntrackInstallerInCentOS) Start() error {
	fmt.Println("not supported yet")
	return nil
}

// Stop stops the Docker service
func (d *conntrackInstallerInCentOS) Stop() error {
	fmt.Println("not supported yet")
	return nil
}
