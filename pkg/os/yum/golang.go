package yum

import (
	// Enable go embed
	_ "embed"
	"fmt"
	"os"
	"runtime"

	"github.com/linuxsuren/http-downloader/pkg/exec"
)

//go:embed resource/go-repo.repo
var goRepo string

// golangInstallerInCentOS is the installer of golang in CentOS
type golangInstallerInCentOS struct {
	Execer exec.Execer
}

// Available check if support current platform
func (d *golangInstallerInCentOS) Available() (ok bool) {
	if runtime.GOOS == "linux" {
		_, err := d.Execer.LookPath("yum")
		ok = err == nil
	}
	return
}

// Install installs the golang
func (d *golangInstallerInCentOS) Install() (err error) {
	if err = exec.RunCommand("rpm", "--import", "https://mirror.go-repo.io/centos/RPM-GPG-KEY-GO-REPO"); err != nil {
		return
	}

	if err = os.WriteFile("/etc/yum.repos.d/go-repo.repo", []byte(goRepo), 0644); err != nil {
		err = fmt.Errorf("failed to save go-repo.repo, error: %v", err)
		return
	}
	if err = exec.RunCommand("yum", "install", "-y", "golang"); err != nil {
		return
	}
	return
}

// Uninstall uninstalls the golang
func (d *golangInstallerInCentOS) Uninstall() (err error) {
	err = exec.RunCommand("yum", "remove", "-y", "golang")
	return
}

// WaitForStart waits for the service be started
func (d *golangInstallerInCentOS) WaitForStart() (ok bool, err error) {
	ok = true
	return
}

// Start starts the golang service
func (d *golangInstallerInCentOS) Start() error {
	fmt.Println("not supported yet")
	return nil
}

// Stop stops the golang service
func (d *golangInstallerInCentOS) Stop() error {
	fmt.Println("not supported yet")
	return nil
}
