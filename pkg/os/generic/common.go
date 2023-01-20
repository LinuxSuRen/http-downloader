package generic

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"

	"github.com/linuxsuren/http-downloader/pkg/exec"
)

// CommonInstaller is the installer of a common bash
type CommonInstaller struct {
	Name         string
	OS           string
	InstallCmd   CmdWithArgs
	UninstallCmd CmdWithArgs

	// inner fields
	proxyMap map[string]string
}

// CmdWithArgs is a command and with args
type CmdWithArgs struct {
	Cmd        string   `yaml:"cmd"`
	Args       []string `yaml:"args"`
	SystemCall bool     `yaml:"systemCall"`
}

// Run runs the current command
func (c CmdWithArgs) Run() (err error) {
	execer := exec.DefaultExecer{}

	if c.SystemCall {
		var targetBinary string
		if targetBinary, err = execer.LookPath(c.Cmd); err != nil {
			err = fmt.Errorf("cannot find %s", c.Cmd)
		} else {
			sysCallArgs := []string{c.Cmd}
			sysCallArgs = append(sysCallArgs, c.Args...)
			fmt.Println(c.Cmd, strings.Join(sysCallArgs, " "))
			err = syscall.Exec(targetBinary, sysCallArgs, os.Environ())
		}
	} else {
		fmt.Println(c.Cmd, strings.Join(c.Args, " "))
		err = exec.RunCommand(c.Cmd, c.Args...)
	}
	return
}

// SetURLReplace set the URL replace map
func (d *CommonInstaller) SetURLReplace(data map[string]string) {
	d.proxyMap = data
}

func (d *CommonInstaller) sliceReplace(args []string) []string {
	for i, arg := range args {
		if result := d.urlReplace(arg); result != arg {
			args[i] = result
		}
	}
	return args
}

func (d *CommonInstaller) urlReplace(old string) string {
	if d.proxyMap == nil {
		return old
	}

	for k, v := range d.proxyMap {
		if !strings.Contains(old, k) {
			continue
		}
		old = strings.ReplaceAll(old, k, v)
	}
	return old
}

// Available check if support current platform
func (d *CommonInstaller) Available() (ok bool) {
	ok = d.OS == "" || runtime.GOOS == d.OS
	return
}

// Install installs the target package
func (d *CommonInstaller) Install() (err error) {
	d.InstallCmd.Args = d.sliceReplace(d.InstallCmd.Args)
	err = d.InstallCmd.Run()
	return
}

// Uninstall uninstalls the target package
func (d *CommonInstaller) Uninstall() (err error) {
	d.InstallCmd.Args = d.sliceReplace(d.InstallCmd.Args)
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
