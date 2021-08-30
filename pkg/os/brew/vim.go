package brew

import (
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"runtime"
)

// vimInstallerInMacOS is the installer of vim in CentOS
type vimInstallerInMacOS struct {
	count int
}

// Available check if support current platform
func (d *vimInstallerInMacOS) Available() (ok bool) {
	if runtime.GOOS == "darwin" {
		_, err := exec.LookPath("brew")
		ok = err == nil
	}
	return
}

// Install installs the vim
func (d *vimInstallerInMacOS) Install() (err error) {
	err = exec.RunCommand("brew", "install", "vim")
	return
}

// Uninstall uninstalls the vim
func (d *vimInstallerInMacOS) Uninstall() (err error) {
	err = exec.RunCommand("brew", "remove", "vim")
	return
}

// WaitForStart waits for the service be started
func (d *vimInstallerInMacOS) WaitForStart() (ok bool, err error) {
	ok = true
	return
}

// Start starts the vim service
func (d *vimInstallerInMacOS) Start() error {
	fmt.Println("not supported yet")
	return nil
}

// Stop stops the vim service
func (d *vimInstallerInMacOS) Stop() error {
	fmt.Println("not supported yet")
	return nil
}
