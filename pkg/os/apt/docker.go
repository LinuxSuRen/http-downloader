package apt

import (
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"io/ioutil"
	"runtime"
	"strings"
	"time"
)

// DockerInstallerInUbuntu is the installer of Docker in Ubuntu
type DockerInstallerInUbuntu struct {
	count int
}

// Available check if support current platform
func (d *DockerInstallerInUbuntu) Available() (ok bool) {
	if runtime.GOOS == "linux" {
		_, err := exec.LookPath("apt-get")
		ok = err == nil
	}
	return
}

// Install installs the Docker
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

// Uninstall uninstalls the Docker
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

// WaitForStart waits for the service be started
func (d *DockerInstallerInUbuntu) WaitForStart() (ok bool, err error) {
	var result string
	if result, err = exec.RunCommandAndReturn("systemctl", "", "status", "docker"); err != nil {
		return
	} else if strings.Contains(result, "Unit docker.service could not be found") {
		err = fmt.Errorf("unit docker.service could not be found")
	} else if strings.Contains(result, "Active: active") {
		ok = true
	} else {
		if d.count > 0 {
			fmt.Println("waiting for Docker service start")
		} else if d.count > 9 {
			return
		}

		d.count++
		time.Sleep(time.Second * 1)
		return d.WaitForStart()
	}
	return
}

// Start starts the Docker service
func (d *DockerInstallerInUbuntu) Start() error {
	fmt.Println("not implemented yet")
	return nil
}

// Stop stops the Docker service
func (d *DockerInstallerInUbuntu) Stop() error {
	return exec.RunCommand("systemctl", "start", "docker")
}
