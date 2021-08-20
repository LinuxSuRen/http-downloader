package yum

import (
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"runtime"
)

type DockerInstallerInCentOS struct {
}

func (d *DockerInstallerInCentOS) Available() (ok bool) {
	if runtime.GOOS == "linux" {
		_, err := exec.LookPath("yum")
		ok = err == nil
	}
	return
}

func (d *DockerInstallerInCentOS) Install() (err error) {
	if err = exec.RunCommand("yum", "install", "-y",
		"yum-utils"); err != nil {
		return
	}
	if err = exec.RunCommand("yum-config-manager", "--add-repo",
		"https://download.docker.com/linux/centos/docker-ce.repo"); err != nil {
		return
	}
	if err = exec.RunCommand("yum", "install", "-y",
		"docker-ce",
		"docker-ce-cli",
		"containerd.io"); err != nil {
		return
	}
	return
}

func (d *DockerInstallerInCentOS) Uninstall() (err error) {
	err = exec.RunCommand("yum", "remove", "-y",
		"docker",
		"docker-client",
		"docker-client-latest",
		"docker-common",
		"docker-latest",
		"docker-latest-logrotate",
		"docker-logrotate",
		"docker-engine",
		"docker-ce",
		"docker-ce-cli",
		"containerd.io")
	return
}
