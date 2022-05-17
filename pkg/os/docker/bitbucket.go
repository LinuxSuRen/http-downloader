package docker

import (
	"github.com/linuxsuren/http-downloader/pkg/exec"
)

// bitbucket is the installer of bitbucket in CentOS
type bitbucket struct {
	count int
}

// Available check if support current platform
func (d *bitbucket) Available() (ok bool) {
	_, err := exec.LookPath("docker")
	ok = err == nil
	return
}

// Install installs the bitbucket
func (d *bitbucket) Install() (err error) {
	if err = exec.RunCommand("docker", "run", `--name=bitbucket`, "-d",
		"-p", "7990:7990", "-p", "7999:7999", "atlassian/bitbucket"); err != nil {
		return
	}
	return
}

// Uninstall uninstalls the bitbucket
func (d *bitbucket) Uninstall() (err error) {
	if err = d.Stop(); err == nil {
		err = exec.RunCommand("docker", "rm", "bitbucket")
	}
	return
}

// WaitForStart waits for the service be started
func (d *bitbucket) WaitForStart() (ok bool, err error) {
	ok = true
	return
}

// Start starts the bitbucket service
func (d *bitbucket) Start() (err error) {
	err = exec.RunCommand("docker", "start", "bitbucket")
	return
}

// Stop stops the bitbucket service
func (d *bitbucket) Stop() (err error) {
	err = exec.RunCommand("docker", "stop", "bitbucket")
	return
}
