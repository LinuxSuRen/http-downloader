package os

import (
	"fmt"
	"io/ioutil"
	"runtime"
	"strings"

	"github.com/linuxsuren/http-downloader/pkg/os/apk"
	"github.com/linuxsuren/http-downloader/pkg/os/dnf"
	"github.com/linuxsuren/http-downloader/pkg/os/npm"
	"github.com/linuxsuren/http-downloader/pkg/os/snap"

	"github.com/linuxsuren/http-downloader/pkg/exec"
	"github.com/linuxsuren/http-downloader/pkg/os/apt"
	"github.com/linuxsuren/http-downloader/pkg/os/brew"
	"github.com/linuxsuren/http-downloader/pkg/os/core"
	"github.com/linuxsuren/http-downloader/pkg/os/generic"
	"github.com/linuxsuren/http-downloader/pkg/os/yum"
	"gopkg.in/yaml.v3"
)

type genericPackages struct {
	Version  string           `yaml:"version"`
	Packages []genericPackage `yaml:"packages"`
}

type preInstall struct {
	IssuePrefix string      `yaml:"issuePrefix"`
	Cmd         CmdWithArgs `yaml:"cmd"`
}

type genericPackage struct {
	Alias          string       `yaml:"alias"`
	Name           string       `yaml:"name"`
	OS             string       `yaml:"os"`
	PackageManager string       `yaml:"packageManager"`
	PreInstall     []preInstall `yaml:"preInstall"`
	Dependents     []string     `yaml:"dependents"`
	InstallCmd     CmdWithArgs  `yaml:"install"`
	UninstallCmd   CmdWithArgs  `yaml:"uninstall"`
	Service        bool         `yaml:"isService"`
	StartCmd       CmdWithArgs  `yaml:"start"`
	StopCmd        CmdWithArgs  `yaml:"stop"`

	CommonInstaller core.Installer

	// inner fields
	proxyMap map[string]string
}

// CmdWithArgs is a command with arguments
type CmdWithArgs struct {
	Cmd        string   `yaml:"cmd"`
	Args       []string `yaml:"args"`
	SystemCall bool     `yaml:"systemCall"`
}

func parseGenericPackages(configFile string, genericPackages *genericPackages) (err error) {
	var data []byte
	if data, err = ioutil.ReadFile(configFile); err != nil {
		err = fmt.Errorf("cannot read config file [%s], error: %v", configFile, err)
		return
	}

	if err = yaml.Unmarshal(data, genericPackages); err != nil {
		err = fmt.Errorf("failed to parse config file [%s], error: %v", configFile, err)
		return
	}
	return
}

// GenericInstallerRegistry registries a generic installer
func GenericInstallerRegistry(configFile string, registry core.InstallerRegistry) (err error) {
	genericPackages := &genericPackages{}
	if err = parseGenericPackages(configFile, genericPackages); err != nil {
		return
	}

	// registry all the packages
	for i := range genericPackages.Packages {
		genericPackage := genericPackages.Packages[i]

		switch genericPackage.PackageManager {
		case "apt-get":
			genericPackage.CommonInstaller = &apt.CommonInstaller{Name: genericPackage.Name}
		case "yum":
			genericPackage.CommonInstaller = &yum.CommonInstaller{Name: genericPackage.Name}
		case "brew":
			genericPackage.CommonInstaller = &brew.CommonInstaller{Name: genericPackage.Name}
		case "apk":
			genericPackage.CommonInstaller = &apk.CommonInstaller{Name: genericPackage.Name}
		case snap.SnapName:
			genericPackage.CommonInstaller = &snap.CommonInstaller{
				Name: genericPackage.Name,
				Args: genericPackage.InstallCmd.Args,
			}
		case dnf.DNFName:
			genericPackage.CommonInstaller = &dnf.CommonInstaller{
				Name: genericPackage.Name,
			}
		case npm.NPMName:
			genericPackage.CommonInstaller = &npm.CommonInstaller{
				Name: genericPackage.Name,
			}
		default:
			genericPackage.CommonInstaller = &generic.CommonInstaller{
				Name: genericPackage.Name,
				OS:   genericPackage.OS,
				InstallCmd: generic.CmdWithArgs{
					Cmd:        genericPackage.InstallCmd.Cmd,
					Args:       genericPackage.InstallCmd.Args,
					SystemCall: genericPackage.InstallCmd.SystemCall,
				},
				UninstallCmd: generic.CmdWithArgs{
					Cmd:        genericPackage.UninstallCmd.Cmd,
					Args:       genericPackage.UninstallCmd.Args,
					SystemCall: genericPackage.UninstallCmd.SystemCall,
				},
			}
		}

		registry.Registry(genericPackage.Name, &genericPackage)
	}
	return
}

func (i *genericPackage) Available() (ok bool) {
	if i.CommonInstaller != nil {
		ok = i.CommonInstaller.Available()
	}
	return
}
func (i *genericPackage) Install() (err error) {
	for index := range i.PreInstall {
		preInstall := i.PreInstall[index]

		needInstall := false
		if preInstall.IssuePrefix != "" && runtime.GOOS == "linux" {
			var data []byte
			if data, err = ioutil.ReadFile("/etc/issue"); err != nil {
				return
			}

			if strings.HasPrefix(string(data), preInstall.IssuePrefix) {
				needInstall = true
			}
		} else if preInstall.IssuePrefix == "" {
			needInstall = true
		}

		if needInstall {
			preInstall.Cmd.Args = i.sliceReplace(preInstall.Cmd.Args)

			if err = exec.RunCommand(preInstall.Cmd.Cmd, preInstall.Cmd.Args...); err != nil {
				return
			}
		}
	}

	if i.CommonInstaller != nil {
		if proxyAble, ok := i.CommonInstaller.(core.ProxyAble); ok {
			proxyAble.SetURLReplace(i.proxyMap)
		}
		err = i.CommonInstaller.Install()
	} else {
		err = fmt.Errorf("not support yet")
	}
	return
}
func (i *genericPackage) Uninstall() (err error) {
	if i.CommonInstaller != nil {
		err = i.CommonInstaller.Uninstall()
	} else {
		err = fmt.Errorf("not support yet")
	}
	return
}
func (i *genericPackage) IsService() bool {
	return i.Service
}
func (i *genericPackage) WaitForStart() (bool, error) {
	return true, nil
}
func (i *genericPackage) Start() error {
	return nil
}
func (i *genericPackage) Stop() error {
	return nil
}

// SetURLReplace set the URL replace map
func (d *genericPackage) SetURLReplace(data map[string]string) {
	d.proxyMap = data
}
func (d *genericPackage) sliceReplace(args []string) []string {
	for i, arg := range args {
		if result := d.urlReplace(arg); result != arg {
			args[i] = result
		}
	}
	return args
}
func (d *genericPackage) urlReplace(old string) string {
	if d.proxyMap == nil {
		return old
	}

	for k, v := range d.proxyMap {
		if !strings.Contains(old, k) {
			continue
		}
		old = strings.ReplaceAll(old, k, v)
	}
	return old
}
