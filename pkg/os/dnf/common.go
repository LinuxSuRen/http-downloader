package dnf

import (
	"fmt"
	"runtime"

	"github.com/linuxsuren/http-downloader/pkg/exec"
)

// DNFName is the tool name
const DNFName = "dnf"

// CommonInstaller is the installer of a common dnf
type CommonInstaller struct {
	Name   string
	Execer exec.Execer
}

// Available check if support current platform
func (d *CommonInstaller) Available() (ok bool) {
	if runtime.GOOS == "linux" {
		_, err := d.Execer.LookPath("dnf")
		ok = err == nil
	}
	return
}

// Install installs the target package
func (d *CommonInstaller) Install() (err error) {
	if err = exec.RunCommand("dnf", "install", d.Name, "-y"); err != nil {
		return
	}
	return
}

// Uninstall uninstalls the target
func (d *CommonInstaller) Uninstall() (err error) {
	err = exec.RunCommand("dnf", "remove", d.Name, "-y")
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
