package apt

import (
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"runtime"
)

// ffmpegInstallerInUbuntu is the installer of ffmpeg in CentOS
type ffmpegInstallerInUbuntu struct {
	count int
}

// Available check if support current platform
func (d *ffmpegInstallerInUbuntu) Available() (ok bool) {
	if runtime.GOOS == "linux" {
		_, err := exec.LookPath("apt-get")
		ok = err == nil
	}
	return
}

// Install installs the ffmpeg
func (d *ffmpegInstallerInUbuntu) Install() (err error) {
	if err = exec.RunCommand("apt-get", "update", "-y"); err != nil {
		return
	}
	if err = exec.RunCommand("apt-get", "install", "-y",
		"ffmpeg"); err != nil {
		return
	}
	return
}

// Uninstall uninstalls the ffmpeg
func (d *ffmpegInstallerInUbuntu) Uninstall() (err error) {
	err = exec.RunCommand("apt-get", "remove", "-y",
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
