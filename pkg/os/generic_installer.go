package os

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"
	"text/template"

	"github.com/antonmedv/expr"
	"github.com/linuxsuren/http-downloader/pkg"

	"github.com/linuxsuren/http-downloader/pkg/os/apk"
	"github.com/linuxsuren/http-downloader/pkg/os/dnf"
	"github.com/linuxsuren/http-downloader/pkg/os/npm"
	"github.com/linuxsuren/http-downloader/pkg/os/scoop"
	"github.com/linuxsuren/http-downloader/pkg/os/snap"
	"github.com/linuxsuren/http-downloader/pkg/os/winget"

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
	env      map[string]string
	execer   exec.Execer
}

// CmdWithArgs is a command with arguments
type CmdWithArgs struct {
	Cmd        string   `yaml:"cmd"`
	Args       []string `yaml:"args"`
	SystemCall bool     `yaml:"systemCall"`
	WriteTo    *WriteTo `yaml:"writeTo"`
}

// WriteTo represents the task to write content to file
type WriteTo struct {
	File    string
	Mod     string
	Content string
	When    string

	// inner fields
	env map[string]string
}

// Write writes content to file
func (w *WriteTo) Write() (err error) {
	w.Content = strings.TrimSpace(w.Content)
	var should bool
	if should, err = w.Should(); err != nil || !should || w.Content == "" {
		return
	}

	var mod int
	if mod, err = strconv.Atoi(w.Mod); err != nil {
		mod = 0750
	}

	if len(w.env) > 0 {
		var tpl *template.Template
		if tpl, err = template.New("write").Parse(w.Content); err != nil {
			return
		}

		buf := bytes.NewBuffer([]byte{})
		if err = tpl.Execute(buf, w.env); err != nil {
			return
		}
		w.Content = buf.String()
	}

	parent := path.Dir(w.File)
	if err = os.MkdirAll(parent, 0750); err == nil {
		err = os.WriteFile(w.File, []byte(w.Content), fs.FileMode(mod))
	}
	return
}

// Should eval the "when" expr, then return bool value.
// Return true if the "when" expr is empty.
// See also https://github.com/antonmedv/expr/blob/master/docs/Language-Definition.md
func (w *WriteTo) Should() (ok bool, err error) {
	var result interface{}
	if w.When == "" {
		ok = true
		return
	}
	if result, err = expr.Eval(w.When, w.env); err == nil {
		switch tt := result.(type) {
		case bool:
			ok = tt
		default:
			err = fmt.Errorf("unexpect type: %s", reflect.TypeOf(result))
		}
	}
	return
}

func parseGenericPackages(configFile string, genericPackages *genericPackages) (err error) {
	var data []byte
	if data, err = os.ReadFile(configFile); err != nil {
		err = fmt.Errorf("cannot read config file [%s], error: %v", configFile, err)
		return
	}

	err = yaml.Unmarshal(data, genericPackages)
	err = pkg.ErrorWrap(err, "failed to parse config file [%s], error: %v", configFile, err)
	return
}

// GenericInstallerRegistry registries a generic installer
func GenericInstallerRegistry(configFile string, registry core.InstallerRegistry) (err error) {
	defaultExecer := exec.DefaultExecer{}
	genericPackages := &genericPackages{}
	if err = parseGenericPackages(configFile, genericPackages); err != nil {
		return
	}

	// registry all the packages
	for i := range genericPackages.Packages {
		genericPackage := genericPackages.Packages[i]
		genericPackage.execer = defaultExecer

		switch genericPackage.PackageManager {
		case apt.Tool:
			genericPackage.CommonInstaller = &apt.CommonInstaller{
				Name:   genericPackage.Name,
				Execer: defaultExecer,
			}
		case yum.Tool:
			genericPackage.CommonInstaller = &yum.CommonInstaller{
				Name:   genericPackage.Name,
				Execer: defaultExecer,
			}
		case brew.Tool:
			genericPackage.CommonInstaller = &brew.CommonInstaller{
				Name:   genericPackage.Name,
				Execer: defaultExecer,
			}
		case apk.Tool:
			genericPackage.CommonInstaller = &apk.CommonInstaller{
				Name:   genericPackage.Name,
				Execer: defaultExecer,
			}
		case snap.SnapName:
			genericPackage.CommonInstaller = &snap.CommonInstaller{
				Name:   genericPackage.Name,
				Args:   genericPackage.InstallCmd.Args,
				Execer: defaultExecer,
			}
		case dnf.DNFName:
			genericPackage.CommonInstaller = &dnf.CommonInstaller{
				Name:   genericPackage.Name,
				Execer: defaultExecer,
			}
		case npm.NPMName:
			genericPackage.CommonInstaller = &npm.CommonInstaller{
				Name:   genericPackage.Name,
				Execer: defaultExecer,
			}
		case winget.Tool:
			genericPackage.CommonInstaller = &winget.CommonInstaller{
				Name:   genericPackage.Name,
				Execer: defaultExecer,
			}
		case scoop.Tool:
			genericPackage.CommonInstaller = &scoop.CommonInstaller{
				Name:   genericPackage.Name,
				Execer: defaultExecer,
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
				Execer: defaultExecer,
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
	i.loadEnv()
	for index := range i.PreInstall {
		preInstall := i.PreInstall[index]

		needInstall := false
		if preInstall.IssuePrefix != "" && i.execer.OS() == exec.OSLinux {
			var data []byte
			if data, err = os.ReadFile("/etc/issue"); err != nil {
				return
			}

			if strings.HasPrefix(string(data), preInstall.IssuePrefix) {
				needInstall = true
			}
		} else if preInstall.IssuePrefix == "" {
			needInstall = true
		}

		if needInstall {
			cmd := preInstall.Cmd
			if cmd.WriteTo != nil {
				cmd.WriteTo.env = i.env
				if err = cmd.WriteTo.Write(); err != nil {
					return
				}
			}

			if cmd.Cmd != "" {
				cmd.Args = i.sliceReplace(cmd.Args)
				fmt.Println(cmd.Args)

				if err = i.execer.RunCommand(cmd.Cmd, cmd.Args...); err != nil {
					return
				}
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
func (i *genericPackage) SetURLReplace(data map[string]string) {
	i.proxyMap = data
}
func (i *genericPackage) sliceReplace(args []string) []string {
	for index, arg := range args {
		if result := i.urlReplace(arg); result != arg {
			args[index] = strings.TrimSpace(result)
		}
	}
	return args
}
func (i *genericPackage) urlReplace(old string) string {
	if tpl, err := template.New("env").Parse(old); err == nil {
		buf := bytes.NewBuffer([]byte{})

		if err = tpl.Execute(buf, i.env); err == nil {
			old = buf.String()
		}
	}
	for k, v := range i.proxyMap {
		if !strings.Contains(old, k) {
			continue
		}
		old = strings.ReplaceAll(old, k, v)
	}
	return old
}
func (i *genericPackage) loadEnv() {
	if i.env == nil {
		i.env = map[string]string{}
	}
	if i.execer.OS() == exec.OSLinux {
		if data, readErr := os.ReadFile("/etc/os-release"); readErr == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if pair := strings.Split(line, "="); len(pair) == 2 {
					i.env[fmt.Sprintf("OS_%s", pair[0])] = strings.TrimPrefix(strings.TrimSuffix(pair[1], `"`), `"`)
				}
			}
		}
	}
}
