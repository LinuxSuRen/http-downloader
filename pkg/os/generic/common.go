package generic

import (
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"os"
	"runtime"
	"syscall"
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
	Cmd        string   `yaml:"cmd"`
	Args       []string `yaml:"args"`
	SystemCall bool     `yaml:"systemCall"`
}

// Run runs the current command
func (c CmdWithArgs) Run() (err error) {
	fmt.Println(c.SystemCall)
	if c.SystemCall {
		var targetBinary string
		if targetBinary, err = exec.LookPath(c.Cmd); err != nil {
			err = fmt.Errorf("cannot find %s", c.Cmd)
		} else {
			sysCallArgs := []string{c.Cmd}
			sysCallArgs = append(sysCallArgs, c.Args...)
			err = syscall.Exec(targetBinary, sysCallArgs, os.Environ())
		}
	} else {
		err = exec.RunCommand(c.Cmd, c.Args...)
	}
	return
}

// Available check if support current platform
func (d *CommonInstaller) Available() (ok bool) {
	ok = d.OS == "" || runtime.GOOS == d.OS
	return
}

// Install installs the target package
func (d *CommonInstaller) Install() (err error) {
	err = d.InstallCmd.Run()
	return
}

// Uninstall uninstalls the target package
func (d *CommonInstaller) Uninstall() (err error) {
	err = d.UninstallCmd.Run()
	return
}

// WaitForStart waits for the service be started
func (d *CommonInstaller) WaitForStart() (ok bool, err error) {
	ok = true
	return
}

// Start starts the desired service
func (d *CommonInstaller) Start() error {
	fmt.Println("not supported yet")
	return nil
}

// Stop stops the desired service
func (d *CommonInstaller) Stop() error {
	fmt.Println("not supported yet")
	return nil
}
