package apt

import (
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"io/ioutil"
	"runtime"
	"strings"
)

type DockerInstallerInUbuntu struct {
}

func (d *DockerInstallerInUbuntu) Available() (ok bool) {
	if runtime.GOOS == "linux" {
		_, err := exec.LookPath("apt-get")
		ok = err == nil
	}
	return
}

func (d *DockerInstallerInUbuntu) Install() (err error) {
	if err = exec.RunCommand("apt-get", "update", "-y"); err != nil {
		return
	}
	if err = exec.RunCommand("apt-get", "install", "-y",
		"apt-transport-https",
		"ca-certificates",
		"curl",
		"gnupg",
		"lsb-release"); err != nil {
		return
	}
	if err = exec.RunCommand("rm", "-rf", "/usr/share/keyrings/docker-archive-keyring.gpg"); err != nil {
		return
	}
	const dockerGPG = "docker.gpg"
	defer func() {
		_ = exec.RunCommand("rm", "-rf", dockerGPG)
	}()
	if err = exec.RunCommand("curl", "-fsSL",
		"https://download.docker.com/linux/ubuntu/gpg", "-o", dockerGPG); err == nil {
		if err = exec.RunCommand("gpg",
			"--dearmor",
			"-o",
			"/usr/share/keyrings/docker-archive-keyring.gpg", dockerGPG); err != nil {
			err = fmt.Errorf("failed to install docker-archive-keyring.gpg, error: %v", err)
			return
		}
	} else {
		err = fmt.Errorf("failed to download docker gpg file, error: %v", err)
		return
	}
	var release string
	if release, err = exec.RunCommandAndReturn("lsb_release", "", "-cs"); err == nil {
		item := fmt.Sprintf("deb [arch=amd64 signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu %s stable",
			strings.TrimSpace(release))
		if err = ioutil.WriteFile("/etc/apt/sources.list.d/docker.list", []byte(item), 622); err != nil {
			err = fmt.Errorf("failed to write docker.list, error: %v", err)
			return
		}
	} else {
		err = fmt.Errorf("failed to run command lsb_release -cs, error: %v", err)
		return
	}
	if err = exec.RunCommand("apt-get", "update", "-y"); err != nil {
		return
	}
	if err = exec.RunCommand("apt-get", "install", "-y",
		"docker-ce", "docker-ce-cli", "containerd.io"); err != nil {
		return
	}
	return
}

func (d *DockerInstallerInUbuntu) Uninstall() (err error) {
	if err = exec.RunCommand("apt-get", "remove", "-y",
		"docker",
		"docker-engine",
		"docker.io",
		"containerd",
		"runc"); err != nil {
		return
	}
	err = exec.RunCommand("apt-get", "purge", "-y",
		"docker-ce",
		"docker-ce-cli",
		"containerd.io")
	return
}
