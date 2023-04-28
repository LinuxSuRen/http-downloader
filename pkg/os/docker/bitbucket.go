package docker

import fakeruntime "github.com/linuxsuren/go-fake-runtime"

// bitbucket is the installer of bitbucket in CentOS
type bitbucket struct {
	Execer fakeruntime.Execer
}

// Available check if support current platform
func (d *bitbucket) Available() (ok bool) {
	_, err := d.Execer.LookPath("docker")
	ok = err == nil
	return
}

// Install installs the bitbucket
func (d *bitbucket) Install() (err error) {
	err = d.Execer.RunCommand("docker", "run", `--name=bitbucket`, "-d",
		"-p", "7990:7990", "-p", "7999:7999", "atlassian/bitbucket")
	return
}

// Uninstall uninstalls the bitbucket
func (d *bitbucket) Uninstall() (err error) {
	if err = d.Stop(); err == nil {
		err = d.Execer.RunCommand("docker", "rm", "bitbucket")
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
	err = d.Execer.RunCommand("docker", "start", "bitbucket")
	return
}

// Stop stops the bitbucket service
func (d *bitbucket) Stop() (err error) {
	err = d.Execer.RunCommand("docker", "stop", "bitbucket")
	return
}
