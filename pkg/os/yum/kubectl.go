package yum

import (
	// Enable go embed
	_ "embed"
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"io/ioutil"
	"runtime"
)

//go:embed resource/kubernetes.repo
var kubernetesRepo string

// kubectlInstallerInCentOS is the installer of kubectl in CentOS
type kubectlInstallerInCentOS struct {
	count int
}

// Available check if support current platform
func (d *kubectlInstallerInCentOS) Available() (ok bool) {
	if runtime.GOOS == "linux" {
		_, err := exec.LookPath("yum")
		ok = err == nil
	}
	return
}

// Install installs the kubectl
func (d *kubectlInstallerInCentOS) Install() (err error) {
	repo := "/etc/yum.repos.d/kubernetes.repo"
	if err = ioutil.WriteFile(repo, []byte(kubernetesRepo), 0644); err != nil {
		err = fmt.Errorf("failed to save %s, error: %v", repo, err)
		return
	}

	if err = exec.RunCommand("yum", "install", "-y", "kubectl"); err != nil {
		return
	}
	return
}

// Uninstall uninstalls the kubectl
func (d *kubectlInstallerInCentOS) Uninstall() (err error) {
	err = exec.RunCommand("yum", "remove", "-y", "kubectl")
	return
}

// WaitForStart waits for the service be started
func (d *kubectlInstallerInCentOS) WaitForStart() (ok bool, err error) {
	ok = true
	return
}

// Start starts the kubectl service
func (d *kubectlInstallerInCentOS) Start() error {
	fmt.Println("not supported yet")
	return nil
}

// Stop stops the kubectl service
func (d *kubectlInstallerInCentOS) Stop() error {
	fmt.Println("not supported yet")
	return nil
}
