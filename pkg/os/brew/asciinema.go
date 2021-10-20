package brew

import (
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"runtime"
)

// asciinemaInstallerInMacOS is the installer of asciinema in CentOS
type asciinemaInstallerInMacOS struct {
	count int
}

// Available check if support current platform
func (d *asciinemaInstallerInMacOS) Available() (ok bool) {
	if runtime.GOOS == "darwin" {
		_, err := exec.LookPath("brew")
		ok = err == nil
	}
	return
}

// Install installs the asciinema
func (d *asciinemaInstallerInMacOS) Install() (err error) {
	err = exec.RunCommand("brew", "install", "asciinema")
	return
}

// Uninstall uninstalls the asciinema
func (d *asciinemaInstallerInMacOS) Uninstall() (err error) {
	err = exec.RunCommand("brew", "remove", "asciinema")
	return
}

// WaitForStart waits for the service be started
func (d *asciinemaInstallerInMacOS) WaitForStart() (ok bool, err error) {
	ok = true
	return
}

// Start starts the asciinema service
func (d *asciinemaInstallerInMacOS) Start() error {
	fmt.Println("not supported yet")
	return nil
}

// Stop stops the asciinema service
func (d *asciinemaInstallerInMacOS) Stop() error {
	fmt.Println("not supported yet")
	return nil
}
