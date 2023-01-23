package apt

import (
	"fmt"
	"runtime"

	"github.com/linuxsuren/http-downloader/pkg/exec"
)

// vimInstallerInUbuntu is the installer of vim in CentOS
type vimInstallerInUbuntu struct {
	Execer exec.Execer
}

// Available check if support current platform
func (d *vimInstallerInUbuntu) Available() (ok bool) {
	if runtime.GOOS == "linux" {
		_, err := d.Execer.LookPath("apt-get")
		ok = err == nil
	}
	return
}

// Install installs the vim
func (d *vimInstallerInUbuntu) Install() (err error) {
	if err = exec.RunCommand("apt-get", "update", "-y"); err != nil {
		return
	}
	if err = exec.RunCommand("apt-get", "install", "-y",
		"vim"); err != nil {
		return
	}
	return
}

// Uninstall uninstalls the vim
func (d *vimInstallerInUbuntu) Uninstall() (err error) {
	err = exec.RunCommand("apt-get", "remove", "-y",
		"vim")
	return
}

// WaitForStart waits for the service be started
func (d *vimInstallerInUbuntu) WaitForStart() (ok bool, err error) {
	ok = true
	return
}

// Start starts the vim service
func (d *vimInstallerInUbuntu) Start() error {
	fmt.Println("not supported yet")
	return nil
}

// Stop stops the vim service
func (d *vimInstallerInUbuntu) Stop() error {
	fmt.Println("not supported yet")
	return nil
}
