package brew

import (
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"runtime"
)

// bashCompletionInstallerInMacOS is the installer of bashCompletion in CentOS
type bashCompletionInstallerInMacOS struct {
	count int
}

// Available check if support current platform
func (d *bashCompletionInstallerInMacOS) Available() (ok bool) {
	if runtime.GOOS == "darwin" {
		_, err := exec.LookPath("brew")
		ok = err == nil
	}
	return
}

// Install installs the bashCompletion
func (d *bashCompletionInstallerInMacOS) Install() (err error) {
	err = exec.RunCommand("brew", "install", "bash-completion@2")
	return
}

// Uninstall uninstalls the bashCompletion
func (d *bashCompletionInstallerInMacOS) Uninstall() (err error) {
	err = exec.RunCommand("brew", "remove", "bash-completion@2")
	return
}

// WaitForStart waits for the service be started
func (d *bashCompletionInstallerInMacOS) WaitForStart() (ok bool, err error) {
	ok = true
	return
}

// Start starts the bashCompletion service
func (d *bashCompletionInstallerInMacOS) Start() error {
	fmt.Println("not supported yet")
	return nil
}

// Stop stops the bashCompletion service
func (d *bashCompletionInstallerInMacOS) Stop() error {
	fmt.Println("not supported yet")
	return nil
}
