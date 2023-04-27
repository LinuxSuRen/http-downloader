package apt

import (
	"fmt"

	fakeruntime "github.com/linuxsuren/go-fake-runtime"
)

// gitInstallerInUbuntu is the installer of git in CentOS
type gitInstallerInUbuntu struct {
	Execer fakeruntime.Execer
}

// Available check if support current platform
func (d *gitInstallerInUbuntu) Available() (ok bool) {
	if d.Execer.OS() == "linux" {
		_, err := d.Execer.LookPath("apt-get")
		ok = err == nil
	}
	return
}

// Install installs the git
func (d *gitInstallerInUbuntu) Install() (err error) {
	if err = d.Execer.RunCommand("apt-get", "update", "-y"); err == nil {
		err = d.Execer.RunCommand("apt-get", "install", "-y",
			"git")
	}
	return
}

// Uninstall uninstalls the git
func (d *gitInstallerInUbuntu) Uninstall() (err error) {
	err = d.Execer.RunCommand("apt-get", "remove", "-y", "git")
	return
}

// WaitForStart waits for the service be started
func (d *gitInstallerInUbuntu) WaitForStart() (ok bool, err error) {
	ok = true
	return
}

// Start starts the git service
func (d *gitInstallerInUbuntu) Start() error {
	fmt.Println("not supported yet")
	return nil
}

// Stop stops the git service
func (d *gitInstallerInUbuntu) Stop() error {
	fmt.Println("not supported yet")
	return nil
}
