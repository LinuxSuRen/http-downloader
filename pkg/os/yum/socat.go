package yum

import (
	"fmt"
	"runtime"

	"github.com/linuxsuren/http-downloader/pkg/exec"
)

// socatInstallerInCentOS is the installer of socat in CentOS
type socatInstallerInCentOS struct {
	Execer exec.Execer
}

// Available check if support current platform
func (d *socatInstallerInCentOS) Available() (ok bool) {
	if runtime.GOOS == "linux" {
		_, err := d.Execer.LookPath("yum")
		ok = err == nil
	}
	return
}

// Install installs the socat
func (d *socatInstallerInCentOS) Install() (err error) {
	if err = exec.RunCommand("yum", "install", "-y",
		"socat"); err != nil {
		return
	}
	return
}

// Uninstall uninstalls the socat
func (d *socatInstallerInCentOS) Uninstall() (err error) {
	err = exec.RunCommand("yum", "remove", "-y",
		"socat")
	return
}

// WaitForStart waits for the service be started
func (d *socatInstallerInCentOS) WaitForStart() (ok bool, err error) {
	ok = true
	return
}

// Start starts the socat service
func (d *socatInstallerInCentOS) Start() error {
	fmt.Println("not supported yet")
	return nil
}

// Stop stops the socat service
func (d *socatInstallerInCentOS) Stop() error {
	fmt.Println("not supported yet")
	return nil
}
