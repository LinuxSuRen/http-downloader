package yum

import (
	"fmt"
	"runtime"

	"github.com/linuxsuren/http-downloader/pkg/exec"
)

// CommonInstaller is the installer of Conntrack in CentOS
type CommonInstaller struct {
	Name   string
	Execer exec.Execer
}

// Available check if support current platform
func (d *CommonInstaller) Available() (ok bool) {
	if runtime.GOOS == "linux" {
		_, err := d.Execer.LookPath("yum")
		ok = err == nil
	}
	return
}

// Install installs the Conntrack
func (d *CommonInstaller) Install() (err error) {
	if err = exec.RunCommand("yum", "update", "-y"); err != nil {
		return
	}
	if err = exec.RunCommand("yum", "install", "-y", d.Name); err != nil {
		return
	}
	return
}

// Uninstall uninstalls the Conntrack
func (d *CommonInstaller) Uninstall() (err error) {
	err = exec.RunCommand("yum", "remove", "-y", d.Name)
	return
}

// WaitForStart waits for the service be started
func (d *CommonInstaller) WaitForStart() (ok bool, err error) {
	ok = true
	return
}

// Start starts the Conntrack service
func (d *CommonInstaller) Start() error {
	fmt.Println("not supported yet")
	return nil
}

// Stop stops the Conntrack service
func (d *CommonInstaller) Stop() error {
	fmt.Println("not supported yet")
	return nil
}
