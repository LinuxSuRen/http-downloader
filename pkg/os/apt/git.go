package apt

import (
	"fmt"
	"runtime"

	"github.com/linuxsuren/http-downloader/pkg/exec"
)

// gitInstallerInUbuntu is the installer of git in CentOS
type gitInstallerInUbuntu struct {
	Execer exec.Execer
}

// Available check if support current platform
func (d *gitInstallerInUbuntu) Available() (ok bool) {
	if runtime.GOOS == "linux" {
		_, err := d.Execer.LookPath("apt-get")
		ok = err == nil
	}
	return
}

// Install installs the git
func (d *gitInstallerInUbuntu) Install() (err error) {
	if err = exec.RunCommand("apt-get", "update", "-y"); err != nil {
		return
	}
	if err = exec.RunCommand("apt-get", "install", "-y",
		"git"); err != nil {
		return
	}
	return
}

// Uninstall uninstalls the git
func (d *gitInstallerInUbuntu) Uninstall() (err error) {
	err = exec.RunCommand("apt-get", "remove", "-y",
		"git")
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
