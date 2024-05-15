package pip

import (
	"fmt"

	fakeruntime "github.com/linuxsuren/go-fake-runtime"
)

// Tool is the tool name of this intergration
const Tool = "pip"

// CommonInstaller is the installer of Conntrack in CentOS
type CommonInstaller struct {
	Name   string
	Execer fakeruntime.Execer
}

// Available check if support current platform
func (d *CommonInstaller) Available() (ok bool) {
	_, err := d.Execer.LookPath(Tool)
	ok = err == nil
	return
}

// Install installs the Conntrack
func (d *CommonInstaller) Install() (err error) {
	err = d.Execer.RunCommand(Tool, "install", "-i", "https://pypi.tuna.tsinghua.edu.cn/simple", d.Name)
	return
}

// Uninstall uninstalls the Conntrack
func (d *CommonInstaller) Uninstall() (err error) {
	err = d.Execer.RunCommand(Tool, "uninstall", d.Name)
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
