package apt

import (
	"fmt"

	fakeruntime "github.com/linuxsuren/go-fake-runtime"
)

// ffmpegInstallerInUbuntu is the installer of ffmpeg in CentOS
type ffmpegInstallerInUbuntu struct {
	Execer fakeruntime.Execer
}

// Available check if support current platform
func (d *ffmpegInstallerInUbuntu) Available() (ok bool) {
	if d.Execer.OS() == "linux" {
		_, err := d.Execer.LookPath("apt-get")
		ok = err == nil
	}
	return
}

// Install installs the ffmpeg
func (d *ffmpegInstallerInUbuntu) Install() (err error) {
	if err = d.Execer.RunCommand("apt-get", "update", "-y"); err != nil {
		return
	}
	if err = d.Execer.RunCommand("apt-get", "install", "-y",
		"ffmpeg"); err != nil {
		return
	}
	return
}

// Uninstall uninstalls the ffmpeg
func (d *ffmpegInstallerInUbuntu) Uninstall() (err error) {
	err = d.Execer.RunCommand("apt-get", "remove", "-y",
		"ffmpeg")
	return
}

// WaitForStart waits for the service be started
func (d *ffmpegInstallerInUbuntu) WaitForStart() (ok bool, err error) {
	ok = true
	return
}

// Start starts the ffmpeg service
func (d *ffmpegInstallerInUbuntu) Start() error {
	fmt.Println("not supported yet")
	return nil
}

// Stop stops the ffmpeg service
func (d *ffmpegInstallerInUbuntu) Stop() error {
	fmt.Println("not supported yet")
	return nil
}
