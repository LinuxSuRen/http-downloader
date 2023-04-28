package yum

import (
	"fmt"

	fakeruntime "github.com/linuxsuren/go-fake-runtime"
)

// conntrackInstallerInCentOS is the installer of Docker in CentOS
type conntrackInstallerInCentOS struct {
	Execer fakeruntime.Execer
}

// Available check if support current platform
func (d *conntrackInstallerInCentOS) Available() (ok bool) {
	if d.Execer.OS() == "linux" {
		_, err := d.Execer.LookPath("yum")
		ok = err == nil
	}
	return
}

// Install installs the Docker
func (d *conntrackInstallerInCentOS) Install() (err error) {
	if err = d.Execer.RunCommand("yum", "install", "-y",
		"conntrack"); err != nil {
		return
	}
	return
}

// Uninstall uninstalls the Docker
func (d *conntrackInstallerInCentOS) Uninstall() (err error) {
	err = d.Execer.RunCommand("yum", "remove", "-y",
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
