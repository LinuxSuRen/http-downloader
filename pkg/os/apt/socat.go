package apt

import (
	"fmt"

	fakeruntime "github.com/linuxsuren/go-fake-runtime"
)

// socatInstallerInUbuntu is the installer of socat in CentOS
type socatInstallerInUbuntu struct {
	Execer fakeruntime.Execer
}

// Available check if support current platform
func (d *socatInstallerInUbuntu) Available() (ok bool) {
	if d.Execer.OS() == "linux" {
		_, err := d.Execer.LookPath("apt-get")
		ok = err == nil
	}
	return
}

// Install installs the socat
func (d *socatInstallerInUbuntu) Install() (err error) {
	if err = d.Execer.RunCommand("apt-get", "update", "-y"); err != nil {
		return
	}
	if err = d.Execer.RunCommand("apt-get", "install", "-y",
		"socat"); err != nil {
		return
	}
	return
}

// Uninstall uninstalls the socat
func (d *socatInstallerInUbuntu) Uninstall() (err error) {
	err = d.Execer.RunCommand("apt-get", "remove", "-y",
		"socat")
	return
}

// WaitForStart waits for the service be started
func (d *socatInstallerInUbuntu) WaitForStart() (ok bool, err error) {
	ok = true
	return
}

// Start starts the socat service
func (d *socatInstallerInUbuntu) Start() error {
	fmt.Println("not supported yet")
	return nil
}

// Stop stops the socat service
func (d *socatInstallerInUbuntu) Stop() error {
	fmt.Println("not supported yet")
	return nil
}
