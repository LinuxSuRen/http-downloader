package apk

import (
	"fmt"
	"runtime"

	"github.com/linuxsuren/http-downloader/pkg/exec"
)

// CommonInstaller is the installer of a common apk
type CommonInstaller struct {
	Name   string
	Execer exec.Execer
}

// Available check if support current platform
func (d *CommonInstaller) Available() (ok bool) {
	if runtime.GOOS == "linux" {
		_, err := d.Execer.LookPath("apk")
		ok = err == nil
	}
	return
}

// Install installs the target package
func (d *CommonInstaller) Install() (err error) {
	if err = exec.RunCommand("apk", "add", d.Name); err != nil {
		return
	}
	return
}

// Uninstall uninstalls the target package
func (d *CommonInstaller) Uninstall() (err error) {
	err = exec.RunCommand("apk", "del", d.Name)
	return
}

// WaitForStart waits for the service be started
func (d *CommonInstaller) WaitForStart() (ok bool, err error) {
	ok = true
	return
}

// Start starts the target service
func (d *CommonInstaller) Start() error {
	fmt.Println("not supported yet")
	return nil
}

// Stop stops the target service
func (d *CommonInstaller) Stop() error {
	fmt.Println("not supported yet")
	return nil
}
