package apt

import (
	"fmt"

	fakeruntime "github.com/linuxsuren/go-fake-runtime"
)

// conntrackInstallerInUbuntu is the installer of Conntrack in CentOS
type conntrackInstallerInUbuntu struct {
	Execer fakeruntime.Execer
}

// Available check if support current platform
func (d *conntrackInstallerInUbuntu) Available() (ok bool) {
	if d.Execer.OS() == "linux" {
		_, err := d.Execer.LookPath("apt-get")
		ok = err == nil
	}
	return
}

// Install installs the Conntrack
func (d *conntrackInstallerInUbuntu) Install() (err error) {
	if err = d.Execer.RunCommand("apt-get", "update", "-y"); err != nil {
		return
	}
	if err = d.Execer.RunCommand("apt-get", "install", "-y",
		"conntrack"); err != nil {
		return
	}
	return
}

// Uninstall uninstalls the Conntrack
func (d *conntrackInstallerInUbuntu) Uninstall() (err error) {
	err = d.Execer.RunCommand("apt-get", "remove", "-y",
		"conntrack")
	return
}

// WaitForStart waits for the service be started
func (d *conntrackInstallerInUbuntu) WaitForStart() (ok bool, err error) {
	ok = true
	return
}

// Start starts the Conntrack service
func (d *conntrackInstallerInUbuntu) Start() error {
	fmt.Println("not supported yet")
	return nil
}

// Stop stops the Conntrack service
func (d *conntrackInstallerInUbuntu) Stop() error {
	fmt.Println("not supported yet")
	return nil
}
