package apt

import (
	// Enable go embed
	_ "embed"
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"io"
	"io/ioutil"
	"net/http"
	"runtime"
)

//go:embed resource/kubernetes.list
var kuberneteList string

// kubectlInstallerInUbuntu is the installer of kubectl in CentOS
type kubectlInstallerInUbuntu struct {
	count int
}

// Available check if support current platform
func (d *kubectlInstallerInUbuntu) Available() (ok bool) {
	if runtime.GOOS == "linux" {
		_, err := exec.LookPath("apt-get")
		ok = err == nil
	}
	return
}

func fetchKeyring() (err error) {
	var response *http.Response
	if response, err = http.Get("https://packages.cloud.google.com/apt/doc/apt-key.gpg"); err != nil {
		err = fmt.Errorf("failed to download kubernetes apt-key.gpg, error: %v", err)
		return
	}

	var data []byte
	if data, err = io.ReadAll(response.Body); err != nil {
		err = fmt.Errorf("failed to read response body from apt-key.gpg, error: %v", err)
		return
	}

	keyring := "/usr/share/keyrings/kubernetes-archive-keyring.gpg"
	if err = ioutil.WriteFile(keyring, data, 0644); err != nil {
		err = fmt.Errorf("failed to save %s, error: %v", keyring, err)
	}
	return
}

// Install installs the kubectl
func (d *kubectlInstallerInUbuntu) Install() (err error) {
	if err = fetchKeyring(); err != nil {
		err = fmt.Errorf("failed to save kubernetes-archive-kerying.gpg")
		return
	}

	repo := "/etc/apt/sources.list.d/kubernetes.list"
	if err = ioutil.WriteFile(repo, []byte(kuberneteList), 0644); err != nil {
		err = fmt.Errorf("failed to save %s, error: %v", repo, err)
		return
	}

	if err = exec.RunCommand("apt-get", "update", "-y"); err != nil {
		return
	}
	if err = exec.RunCommand("apt-get", "install", "-y", "apt-transport-https",
		"ca-certificates"); err != nil {
		return
	}
	if err = exec.RunCommand("apt-get", "update", "-y"); err != nil {
		return
	}
	if err = exec.RunCommand("apt-get", "install", "-y", "kubectl"); err != nil {
		return
	}
	return
}

// Uninstall uninstalls the kubectl
func (d *kubectlInstallerInUbuntu) Uninstall() (err error) {
	err = exec.RunCommand("apt-get", "remove", "-y", "kubectl")
	return
}

// WaitForStart waits for the service be started
func (d *kubectlInstallerInUbuntu) WaitForStart() (ok bool, err error) {
	ok = true
	return
}

// Start starts the kubectl service
func (d *kubectlInstallerInUbuntu) Start() error {
	fmt.Println("not supported yet")
	return nil
}

// Stop stops the kubectl service
func (d *kubectlInstallerInUbuntu) Stop() error {
	fmt.Println("not supported yet")
	return nil
}
