package apk

import (
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"runtime"
)

// CommonInstaller is the installer of a common apk
type CommonInstaller struct {
	Name string
}

// Available check if support current platform
func (d *CommonInstaller) Available() (ok bool) {
	if runtime.GOOS == "linux" {
		_, err := exec.LookPath("apk")
		ok = err == nil
	}
	return
}

// Install installs the target package
func (d *CommonInstaller) Install() (err error) {
	if err = exec.RunCommand("apk", "add", d.Name); err != nil {
		return
	}
	return
}

// Uninstall uninstalls the Conntrack
func (d *CommonInstaller) Uninstall() (err error) {
	err = exec.RunCommand("apk", "del", d.Name)
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
