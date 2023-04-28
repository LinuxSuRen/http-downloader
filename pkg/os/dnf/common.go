package dnf

import (
	"fmt"

	fakeruntime "github.com/linuxsuren/go-fake-runtime"
)

// DNFName is the tool name
const DNFName = "dnf"

// CommonInstaller is the installer of a common dnf
type CommonInstaller struct {
	Name   string
	Execer fakeruntime.Execer
}

// Available check if support current platform
func (d *CommonInstaller) Available() (ok bool) {
	if d.Execer.OS() == fakeruntime.OSLinux {
		_, err := d.Execer.LookPath(DNFName)
		ok = err == nil
	}
	return
}

// Install installs the target package
func (d *CommonInstaller) Install() (err error) {
	if err = d.Execer.RunCommand(DNFName, "install", d.Name, "-y"); err != nil {
		return
	}
	return
}

// Uninstall uninstalls the target
func (d *CommonInstaller) Uninstall() (err error) {
	err = d.Execer.RunCommand(DNFName, "remove", d.Name, "-y")
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
