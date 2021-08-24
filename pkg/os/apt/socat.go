package apt

import (
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"runtime"
)

// socatInstallerInUbuntu is the installer of socat in CentOS
type socatInstallerInUbuntu struct {
	count int
}

// Available check if support current platform
func (d *socatInstallerInUbuntu) Available() (ok bool) {
	if runtime.GOOS == "linux" {
		_, err := exec.LookPath("apt-get")
		ok = err == nil
	}
	return
}

// Install installs the socat
func (d *socatInstallerInUbuntu) Install() (err error) {
	if err = exec.RunCommand("apt-get", "update", "-y"); err != nil {
		return
	}
	if err = exec.RunCommand("apt-get", "install", "-y",
		"socat"); err != nil {
		return
	}
	return
}

// Uninstall uninstalls the socat
func (d *socatInstallerInUbuntu) Uninstall() (err error) {
	err = exec.RunCommand("apt-get", "remove", "-y",
		"socat")
	return
}

// WaitForStart waits for the service be started
func (d *socatInstallerInUbuntu) WaitForStart() (ok bool, err error) {
	ok = true
	return
}

// Start starts the socat service
func (d *socatInstallerInUbuntu) Start() error {
	fmt.Println("not supported yet")
	return nil
}

// Stop stops the socat service
func (d *socatInstallerInUbuntu) Stop() error {
	fmt.Println("not supported yet")
	return nil
}
