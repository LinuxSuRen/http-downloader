package apt

import (
	"fmt"

	fakeruntime "github.com/linuxsuren/go-fake-runtime"
)

const (
	// Tool is the tool name of apt-get
	Tool = "apt-get"
)

// CommonInstaller is the installer of Conntrack in CentOS
type CommonInstaller struct {
	Name   string
	Execer fakeruntime.Execer
}

// Available check if support current platform
func (d *CommonInstaller) Available() (ok bool) {
	if d.Execer.OS() == fakeruntime.OSLinux {
		_, err := d.Execer.LookPath(Tool)
		ok = err == nil
	}
	return
}

// Install installs the Conntrack
func (d *CommonInstaller) Install() (err error) {
	if err = d.Execer.RunCommand(Tool, "update", "-y"); err == nil {
		err = d.Execer.RunCommand(Tool, "install", "-y", d.Name)
	}
	return
}

// Uninstall uninstalls the Conntrack
func (d *CommonInstaller) Uninstall() (err error) {
	err = d.Execer.RunCommand(Tool, "remove", "-y", d.Name)
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
