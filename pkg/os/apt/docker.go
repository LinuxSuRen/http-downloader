package apt

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/linuxsuren/http-downloader/pkg/exec"
)

// dockerInstallerInUbuntu is the installer of Docker in Ubuntu
type dockerInstallerInUbuntu struct {
	Execer exec.Execer
	count  int
}

// Available check if support current platform
func (d *dockerInstallerInUbuntu) Available() (ok bool) {
	if d.Execer.OS() == "linux" {
		_, err := d.Execer.LookPath("apt-get")
		ok = err == nil
	}
	return
}

// Install installs the Docker
func (d *dockerInstallerInUbuntu) Install() (err error) {
	if d.isDebian() {
		return d.installOnDebian()
	}

	if err = d.Execer.RunCommand("apt-get", "update", "-y"); err != nil {
		return
	}
	if err = d.Execer.RunCommand("apt-get", "install", "-y",
		"apt-transport-https",
		"ca-certificates",
		"curl",
		"gnupg",
		"lsb-release"); err != nil {
		return
	}
	if err = d.Execer.RunCommand("rm", "-rf", "/usr/share/keyrings/docker-archive-keyring.gpg"); err != nil {
		return
	}
	const dockerGPG = "docker.gpg"
	defer func() {
		_ = d.Execer.RunCommand("rm", "-rf", dockerGPG)
	}()
	if err = d.Execer.RunCommand("curl", "-fsSL",
		"https://download.docker.com/linux/ubuntu/gpg", "-o", dockerGPG); err == nil {
		if err = d.Execer.RunCommand("gpg",
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
	if release, err = d.Execer.RunCommandAndReturn("lsb_release", "", "-cs"); err == nil {
		item := fmt.Sprintf("deb [arch=amd64 signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu %s stable",
			strings.TrimSpace(release))
		if err = os.WriteFile("/etc/apt/sources.list.d/docker.list", []byte(item), 0622); err != nil {
			err = fmt.Errorf("failed to write docker.list, error: %v", err)
			return
		}
	} else {
		err = fmt.Errorf("failed to run command lsb_release -cs, error: %v", err)
		return
	}
	if err = d.Execer.RunCommand("apt-get", "update", "-y"); err != nil {
		return
	}
	if err = d.Execer.RunCommand("apt-get", "install", "-y",
		"docker-ce", "docker-ce-cli", "containerd.io"); err != nil {
		return
	}
	return
}

func (d *dockerInstallerInUbuntu) isDebian() bool {
	output, err := d.Execer.RunCommandAndReturn("lsb_release", "", "-d")
	if err == nil {
		return strings.Contains(output, "Debian")
	}
	return false
}

// see also https://docs.docker.com/engine/install/debian/
func (d *dockerInstallerInUbuntu) installOnDebian() (err error) {
	if err = d.Execer.RunCommand("apt-get", "update", "-y"); err != nil {
		return
	}

	if err = d.Execer.RunCommand("apt-get", "install", "-y",
		"ca-certificates", "curl", "gnupg", "lsb-release"); err != nil {
		return
	}

	if err = os.MkdirAll("/etc/apt/keyrings", 0644); err != nil {
		return
	}

	const dockerGPG = "docker.gpg"
	defer func() {
		_ = d.Execer.RunCommand("rm", "-rf", dockerGPG)
	}()
	if err = d.Execer.RunCommand("curl", "-fsSL",
		"https://download.docker.com/linux/debian/gpg", "-o", dockerGPG); err == nil {
		if err = d.Execer.RunCommand("gpg",
			"--dearmor",
			"-o",
			"/etc/apt/keyrings/docker.gpg", dockerGPG); err != nil {
			err = fmt.Errorf("failed to install docker-archive-keyring.gpg, error: %v", err)
			return
		}
	} else {
		err = fmt.Errorf("failed to download docker gpg file, error: %v", err)
		return
	}

	var release string
	if release, err = d.Execer.RunCommandAndReturn("lsb_release", "", "-cs"); err == nil {
		item := fmt.Sprintf("deb [arch=amd64 signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/debian %s stable",
			strings.TrimSpace(release))
		if err = os.WriteFile("/etc/apt/sources.list.d/docker.list", []byte(item), 0622); err != nil {
			err = fmt.Errorf("failed to write docker.list, error: %v", err)
			return
		}
	} else {
		err = fmt.Errorf("failed to run command lsb_release -cs, error: %v", err)
		return
	}

	if err = d.Execer.RunCommand("apt-get", "update", "-y"); err != nil {
		return
	}
	err = d.Execer.RunCommand("apt-get", "install", "docker-ce", "docker-ce-cli", "containerd.io", "-y")
	return
}

// Uninstall uninstalls the Docker
func (d *dockerInstallerInUbuntu) Uninstall() (err error) {
	if err = d.Execer.RunCommand("apt-get", "remove", "-y",
		"docker",
		"docker-engine",
		"docker.io",
		"containerd",
		"runc"); err == nil {
		err = d.Execer.RunCommand("apt-get", "purge", "-y",
			"docker-ce",
			"docker-ce-cli",
			"containerd.io")
	}
	return
}

// WaitForStart waits for the service be started
func (d *dockerInstallerInUbuntu) WaitForStart() (ok bool, err error) {
	var result string
	if result, err = d.Execer.RunCommandAndReturn("systemctl", "", "status", "docker"); err != nil {
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
func (d *dockerInstallerInUbuntu) Start() error {
	fmt.Println("not implemented yet")
	return nil
}

// Stop stops the Docker service
func (d *dockerInstallerInUbuntu) Stop() error {
	return d.Execer.RunCommand("systemctl", "start", "docker")
}
