package yum

import (
	"fmt"
	"runtime"

	"github.com/linuxsuren/http-downloader/pkg/exec"
)

// vimInstallerInCentOS is the installer of vim in CentOS
type vimInstallerInCentOS struct {
	Execer exec.Execer
}

// Available check if support current platform
func (d *vimInstallerInCentOS) Available() (ok bool) {
	if runtime.GOOS == "linux" {
		_, err := d.Execer.LookPath("yum")
		ok = err == nil
	}
	return
}

// Install installs the vim
func (d *vimInstallerInCentOS) Install() (err error) {
	if err = exec.RunCommand("yum", "install", "-y",
		"vim"); err != nil {
		return
	}
	return
}

// Uninstall uninstalls the vim
func (d *vimInstallerInCentOS) Uninstall() (err error) {
	err = exec.RunCommand("yum", "remove", "-y",
		"vim")
	return
}

// WaitForStart waits for the service be started
func (d *vimInstallerInCentOS) WaitForStart() (ok bool, err error) {
	ok = true
	return
}

// Start starts the vim service
func (d *vimInstallerInCentOS) Start() error {
	fmt.Println("not supported yet")
	return nil
}

// Stop stops the vim service
func (d *vimInstallerInCentOS) Stop() error {
	fmt.Println("not supported yet")
	return nil
}
