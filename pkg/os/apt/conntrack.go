package apt

import (
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"runtime"
)

// conntrackInstallerInUbuntu is the installer of Conntrack in CentOS
type conntrackInstallerInUbuntu struct {
	count int
}

// Available check if support current platform
func (d *conntrackInstallerInUbuntu) Available() (ok bool) {
	if runtime.GOOS == "linux" {
		_, err := exec.LookPath("apt-get")
		ok = err == nil
	}
	return
}

// Install installs the Conntrack
func (d *conntrackInstallerInUbuntu) Install() (err error) {
	if err = exec.RunCommand("apt-get", "update", "-y"); err != nil {
		return
	}
	if err = exec.RunCommand("apt-get", "install", "-y",
		"conntrack"); err != nil {
		return
	}
	return
}

// Uninstall uninstalls the Conntrack
func (d *conntrackInstallerInUbuntu) Uninstall() (err error) {
	err = exec.RunCommand("apt-get", "remove", "-y",
		"conntrack")
	return
}

// WaitForStart waits for the service be started
func (d *conntrackInstallerInUbuntu) WaitForStart() (ok bool, err error) {
	ok = true
	return
}

// Start starts the Conntrack service
func (d *conntrackInstallerInUbuntu) Start() error {
	fmt.Println("not supported yet")
	return nil
}

// Stop stops the Conntrack service
func (d *conntrackInstallerInUbuntu) Stop() error {
	fmt.Println("not supported yet")
	return nil
}
