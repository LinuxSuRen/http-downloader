package apt

import (
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg/exec"
)

// golangInstallerInUbuntu is the installer of golang in CentOS
type golangInstallerInUbuntu struct {
	Execer exec.Execer
}

// Available check if support current platform
func (d *golangInstallerInUbuntu) Available() (ok bool) {
	if d.Execer.OS() == "linux" {
		_, err := d.Execer.LookPath("apt-get")
		ok = err == nil
	}
	return
}

// Install installs the golang
func (d *golangInstallerInUbuntu) Install() (err error) {
	if err = d.Execer.RunCommand("apt-get", "update", "-y"); err != nil {
		return
	}
	if err = d.Execer.RunCommand("apt-get", "install", "-y", "golang-go"); err != nil {
		return
	}
	return
}

// Uninstall uninstalls the golang
func (d *golangInstallerInUbuntu) Uninstall() (err error) {
	err = exec.RunCommand("apt-get", "remove", "-y", "golang-go")
	return
}

// WaitForStart waits for the service be started
func (d *golangInstallerInUbuntu) WaitForStart() (ok bool, err error) {
	ok = true
	return
}

// Start starts the golang service
func (d *golangInstallerInUbuntu) Start() error {
	fmt.Println("not supported yet")
	return nil
}

// Stop stops the golang service
func (d *golangInstallerInUbuntu) Stop() error {
	fmt.Println("not supported yet")
	return nil
}
