package brew

import (
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"runtime"
)

// ffmpegInstallerInMacOS is the installer of ffmpeg in CentOS
type ffmpegInstallerInMacOS struct {
	count int
}

// Available check if support current platform
func (d *ffmpegInstallerInMacOS) Available() (ok bool) {
	if runtime.GOOS == "darwin" {
		_, err := exec.LookPath("brew")
		ok = err == nil
	}
	return
}

// Install installs the ffmpeg
func (d *ffmpegInstallerInMacOS) Install() (err error) {
	err = exec.RunCommand("brew", "install", "ffmpeg")
	return
}

// Uninstall uninstalls the ffmpeg
func (d *ffmpegInstallerInMacOS) Uninstall() (err error) {
	err = exec.RunCommand("brew", "remove", "ffmpeg")
	return
}

// WaitForStart waits for the service be started
func (d *ffmpegInstallerInMacOS) WaitForStart() (ok bool, err error) {
	ok = true
	return
}

// Start starts the ffmpeg service
func (d *ffmpegInstallerInMacOS) Start() error {
	fmt.Println("not supported yet")
	return nil
}

// Stop stops the ffmpeg service
func (d *ffmpegInstallerInMacOS) Stop() error {
	fmt.Println("not supported yet")
	return nil
}
