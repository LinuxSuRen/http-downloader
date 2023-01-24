package apt

import (
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg/exec"
)

// bashCompletionInstallerInUbuntu is the installer of bashCompletion in CentOS
type bashCompletionInstallerInUbuntu struct {
	Execer exec.Execer
}

// Available check if support current platform
func (d *bashCompletionInstallerInUbuntu) Available() (ok bool) {
	if d.Execer.OS() == "linux" {
		_, err := d.Execer.LookPath("apt-get")
		ok = err == nil
	}
	return
}

// Install installs the bashCompletion
func (d *bashCompletionInstallerInUbuntu) Install() (err error) {
	if err = d.Execer.RunCommand("apt-get", "update", "-y"); err != nil {
		return
	}
	if err = d.Execer.RunCommand("apt-get", "install", "-y",
		"bash-completion"); err != nil {
		return
	}
	return
}

// Uninstall uninstalls the bashCompletion
func (d *bashCompletionInstallerInUbuntu) Uninstall() (err error) {
	err = d.Execer.RunCommand("apt-get", "remove", "-y",
		"bash-completion")
	return
}

// WaitForStart waits for the service be started
func (d *bashCompletionInstallerInUbuntu) WaitForStart() (ok bool, err error) {
	ok = true
	return
}

// Start starts the bashCompletion service
func (d *bashCompletionInstallerInUbuntu) Start() error {
	fmt.Println("not supported yet")
	return nil
}

// Stop stops the bashCompletion service
func (d *bashCompletionInstallerInUbuntu) Stop() error {
	fmt.Println("not supported yet")
	return nil
}
