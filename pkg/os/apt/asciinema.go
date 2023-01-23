package apt

import (
	"fmt"
	"runtime"

	"github.com/linuxsuren/http-downloader/pkg/exec"
)

// asciinemaInstallerInUbuntu is the installer of asciinema in CentOS
type asciinemaInstallerInUbuntu struct {
	Execer exec.Execer
}

// Available check if support current platform
func (d *asciinemaInstallerInUbuntu) Available() (ok bool) {
	if runtime.GOOS == "linux" {
		_, err := d.Execer.LookPath("apt-get")
		ok = err == nil
	}
	return
}

// Install installs the asciinema
func (d *asciinemaInstallerInUbuntu) Install() (err error) {
	if err = exec.RunCommand("apt-get", "update", "-y"); err != nil {
		return
	}
	if err = exec.RunCommand("apt-get", "install", "-y",
		"asciinema"); err != nil {
		return
	}
	return
}

// Uninstall uninstalls the asciinema
func (d *asciinemaInstallerInUbuntu) Uninstall() (err error) {
	err = exec.RunCommand("apt-get", "remove", "-y",
		"asciinema")
	return
}

// WaitForStart waits for the service be started
func (d *asciinemaInstallerInUbuntu) WaitForStart() (ok bool, err error) {
	ok = true
	return
}

// Start starts the asciinema service
func (d *asciinemaInstallerInUbuntu) Start() error {
	fmt.Println("not supported yet")
	return nil
}

// Stop stops the asciinema service
func (d *asciinemaInstallerInUbuntu) Stop() error {
	fmt.Println("not supported yet")
	return nil
}
