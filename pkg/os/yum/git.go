package yum

import (
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"runtime"
)

// gitInstallerInCentOS is the installer of git in CentOS
type gitInstallerInCentOS struct {
	count int
}

// Available check if support current platform
func (d *gitInstallerInCentOS) Available() (ok bool) {
	if runtime.GOOS == "linux" {
		_, err := exec.LookPath("yum")
		ok = err == nil
	}
	return
}

// Install installs the git
func (d *gitInstallerInCentOS) Install() (err error) {
	if err = exec.RunCommand("yum", "install", "-y",
		"git"); err != nil {
		return
	}
	return
}

// Uninstall uninstalls the git
func (d *gitInstallerInCentOS) Uninstall() (err error) {
	err = exec.RunCommand("yum", "remove", "-y",
		"git")
	return
}

// WaitForStart waits for the service be started
func (d *gitInstallerInCentOS) WaitForStart() (ok bool, err error) {
	ok = true
	return
}

// Start starts the git service
func (d *gitInstallerInCentOS) Start() error {
	fmt.Println("not supported yet")
	return nil
}

// Stop stops the git service
func (d *gitInstallerInCentOS) Stop() error {
	fmt.Println("not supported yet")
	return nil
}
