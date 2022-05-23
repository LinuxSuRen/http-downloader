package generic

import (
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"runtime"
)

// CommonInstaller is the installer of a common bash
type CommonInstaller struct {
	Name         string
	OS           string
	InstallCmd   CmdWithArgs
	UninstallCmd CmdWithArgs
}

// CmdWithArgs is a command and with args
type CmdWithArgs struct {
	Cmd  string   `yaml:"cmd"`
	Args []string `yaml:"args"`
}

// Available check if support current platform
func (d *CommonInstaller) Available() (ok bool) {
	ok = d.OS == "" || runtime.GOOS == d.OS
	return
}

// Install installs the target package
func (d *CommonInstaller) Install() (err error) {
	err = exec.RunCommand(d.InstallCmd.Cmd, d.InstallCmd.Args...)
	return
}

// Uninstall uninstalls the Conntrack
func (d *CommonInstaller) Uninstall() (err error) {
	err = exec.RunCommand(d.UninstallCmd.Cmd, d.UninstallCmd.Args...)
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
