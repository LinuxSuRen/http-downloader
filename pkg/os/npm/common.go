package npm

import (
	"fmt"

	"github.com/linuxsuren/http-downloader/pkg/exec"
)

// NPMName is the name of npm
const NPMName = "npm"

// CommonInstaller is the installer of a common npm
type CommonInstaller struct {
	Name   string
	Execer exec.Execer
}

// Available check if support current platform
func (d *CommonInstaller) Available() (ok bool) {
	_, err := d.Execer.LookPath("npm")
	ok = err == nil
	return
}

// Install installs the target package
func (d *CommonInstaller) Install() (err error) {
	err = exec.RunCommand("npm", "i", "-g", d.Name)
	return
}

// Uninstall uninstalls the target package
func (d *CommonInstaller) Uninstall() (err error) {
	err = exec.RunCommand("npm", "uninstall", "-g", d.Name)
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
