package yum

import (
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"runtime"
)

// bashCompletionInstallerInCentOS is the installer of bashCompletion in CentOS
type bashCompletionInstallerInCentOS struct {
	count int
}

// Available check if support current platform
func (d *bashCompletionInstallerInCentOS) Available() (ok bool) {
	if runtime.GOOS == "linux" {
		_, err := exec.LookPath("yum")
		ok = err == nil
	}
	return
}

// Install installs the bashCompletion
func (d *bashCompletionInstallerInCentOS) Install() (err error) {
	if err = exec.RunCommand("yum", "install", "-y", "bash-completion"); err != nil {
		return
	}
	return
}

// Uninstall uninstalls the bashCompletion
func (d *bashCompletionInstallerInCentOS) Uninstall() (err error) {
	err = exec.RunCommand("yum", "remove", "-y", "bash-completion")
	return
}

// WaitForStart waits for the service be started
func (d *bashCompletionInstallerInCentOS) WaitForStart() (ok bool, err error) {
	ok = true
	return
}

// Start starts the bashCompletion service
func (d *bashCompletionInstallerInCentOS) Start() error {
	fmt.Println("not supported yet")
	return nil
}

// Stop stops the bashCompletion service
func (d *bashCompletionInstallerInCentOS) Stop() error {
	fmt.Println("not supported yet")
	return nil
}
