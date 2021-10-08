package installer

import (
	"bytes"
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/linuxsuren/http-downloader/pkg"
	"github.com/linuxsuren/http-downloader/pkg/common"
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"github.com/linuxsuren/http-downloader/pkg/net"
	"github.com/linuxsuren/http-downloader/pkg/os"
	"github.com/mitchellh/go-homedir"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/url"
	sysos "os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
)

const (
	// ProviderGitHub represents https://github.com
	ProviderGitHub = "github"
)

var supportedPackages = []string{
	"tar.gz",
	"zip",
	"tar.xz",
}

// CheckDepAndInstall checks the desired tools, install the missing packages
func (o *Installer) CheckDepAndInstall(tools map[string]string) (err error) {
	for tool, formula := range tools {
		if _, lookErr := exec.LookPath(tool); lookErr == nil {
			continue
		}

		fmt.Printf("start to install missing tool: %s\n", tool)

		// check if it's a native package
		if os.HasPackage(tool) {
			if err = os.Install(tool); err != nil {
				return
			}
			continue
		}

		var targetURL string
		if targetURL, err = o.ProviderURLParse(formula, false); err != nil {
			return
		}

		var urlObj *url.URL
		var output string
		if urlObj, err = url.Parse(targetURL); err == nil {
			if output = path.Base(urlObj.Path); output == "" {
				err = fmt.Errorf("output cannot be empty")
				return
			}

			if err = net.DownloadFileWithMultipleThreadKeepParts(targetURL, output, 4, true, true); err == nil {
				o.CleanPackage = true
				o.Source = output
				if err = o.Install(); err != nil {
					return
				}
			} else {
				return
			}
		} else {
			err = fmt.Errorf("cannot parse the target URL, error: '%v'", err)
		}
	}
	return
}

// GetVersion parse install app info
func (o *Installer) GetVersion(path string) (version string, err error) {

	var (
		org  string
		repo string
		name string
	)

	// 1. split app info and app version
	var appName string
	appInfo := strings.Split(path, "@")
	switch len(appInfo) {
	case 1:
		appName = appInfo[0]
		version = "latest"
	case 2:
		appName = appInfo[0]
		version = appInfo[1]
	default:
		err = fmt.Errorf("only support format xxx or xxx@version")
		return
	}

	// 2. split app info
	addr := strings.Split(appName, "/")
	if len(addr) >= 2 {
		org = addr[0]
		repo = addr[1]
	} else if len(addr) > 0 {
		repo = addr[0]

		if potentialOrgs := findByRepo(repo); len(potentialOrgs) == 0 {
			err = fmt.Errorf("cannot found the package: %s", repo)
			return
		} else if len(potentialOrgs) == 1 {
			org = potentialOrgs[0]
		} else {
			if org, err = chooseOneFromArray(potentialOrgs); err != nil {
				err = fmt.Errorf("failed to choose the potential organizations of your desired package")
				return
			}
		}

		fmt.Printf("target package is %s/%s\n", org, repo)
	} else {
		err = fmt.Errorf("name only support format xx, xx/xx or xx/xx/xx")
		return
	}

	if len(addr) == 3 {
		name = addr[2]
	} else {
		name = repo
	}

	o.Org = org
	o.Repo = repo
	o.Name = name

	return
}

// ProviderURLParse parse the URL
func (o *Installer) ProviderURLParse(path string, acceptPreRelease bool) (packageURL string, err error) {
	packageURL = path
	if o.Fetch {
		// fetch the latest config
		fmt.Println("start to fetch the config")
		if err = FetchLatestRepo(o.Provider, ConfigBranch, sysos.Stdout); err != nil {
			err = fmt.Errorf("unable to fetch the latest config, error: %v", err)
			return
		}
	}

	version, err := o.GetVersion(packageURL)
	if err != nil {
		return
	}
	packagingFormat := getPackagingFormat(o)
	if version == "latest" {
		packageURL = fmt.Sprintf("https://github.com/%s/%s/releases/%s/download/%s-%s-%s.%s",
			o.Org, o.Repo, version, o.Name, o.OS, o.Arch, packagingFormat)
	} else {
		packageURL = fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s-%s-%s.%s",
			o.Org, o.Repo, version, o.Name, o.OS, o.Arch, packagingFormat)
	}

	// try to parse from config
	userHome, _ := homedir.Dir()
	configDir := userHome + "/.config/hd-home"
	matchedFile := configDir + "/config/" + o.Org + "/" + o.Repo + ".yml"
	if ok, _ := common.PathExists(matchedFile); ok {
		var data []byte
		if data, err = ioutil.ReadFile(matchedFile); err == nil {
			cfg := HDConfig{}
			if !IsSupport(cfg) {
				err = fmt.Errorf("not support this platform, os: %s, arch: %s", runtime.GOOS, runtime.GOARCH)
				return
			}

			if err = yaml.Unmarshal(data, &cfg); err == nil {
				hdPkg := &HDPackage{
					Name:             o.Name,
					Version:          version,
					OS:               common.GetReplacement(runtime.GOOS, cfg.Replacements),
					Arch:             common.GetReplacement(runtime.GOARCH, cfg.Replacements),
					AdditionBinaries: cfg.AdditionBinaries,
					VersionNum:       strings.TrimPrefix(version, "v"),
				}
				o.Package = &cfg
				o.AdditionBinaries = cfg.AdditionBinaries

				if version == "latest" || version == "" {
					ghClient := pkg.ReleaseClient{
						Org:  o.Org,
						Repo: o.Repo,
					}
					ghClient.Init()
					if asset, err := ghClient.GetLatestAsset(acceptPreRelease); err == nil {
						hdPkg.Version = url.QueryEscape(asset.TagName) // the version name might have some special string
						hdPkg.VersionNum = common.ParseVersionNum(asset.TagName)

						version = hdPkg.Version
					} else {
						fmt.Println(err, "cannot get the asset")
					}

					if packageURL == "" {
						packageURL = fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s-%s-%s.%s",
							o.Org, o.Repo, version, o.Name, o.OS, o.Arch, packagingFormat)
					}
				} else {
					hdPkg.VersionNum = common.ParseVersionNum(version)
					if packageURL == "" {
						packageURL = fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s-%s-%s.%s",
							o.Org, o.Repo, version, o.Name, o.OS, o.Arch, packagingFormat)
					}
				}

				if cfg.URL != "" {
					// it does not come from GitHub release
					tmp, _ := template.New("hd").Parse(cfg.URL)

					var buf bytes.Buffer
					if err = tmp.Execute(&buf, hdPkg); err == nil {
						packageURL = buf.String()
					} else {
						return
					}
				} else if cfg.Filename != "" {
					tmp, _ := template.New("hd").Parse(cfg.Filename)

					var buf bytes.Buffer
					if err = tmp.Execute(&buf, hdPkg); err == nil {
						packageURL = fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s",
							o.Org, o.Repo, version, buf.String())
						if !hasPackageSuffix(packageURL) {
							packageURL = fmt.Sprintf("%s.%s", packageURL, packagingFormat)
						}
						o.Output = buf.String()
					} else {
						return
					}
				}

				if err = renderCmdsWithArgs(cfg.PreInstalls, hdPkg); err != nil {
					return
				}
				if err = renderCmdWithArgs(cfg.Installation, hdPkg); err != nil {
					return
				}
				if err = renderCmdsWithArgs(cfg.PostInstalls, hdPkg); err != nil {
					return
				}
				if err = renderCmdsWithArgs(cfg.TestInstalls, hdPkg); err != nil {
					return
				}

				o.Tar = cfg.Tar != "false"
				if cfg.Binary != "" {
					if cfg.Binary, err = renderTemplate(cfg.Binary, hdPkg); err != nil {
						return
					}
					o.Name = cfg.Binary
				}
			}
		}
	}
	return
}

// IsSupport checks if support
func IsSupport(cfg HDConfig) bool {
	var osSupport, archSupport bool

	if len(cfg.SupportOS) > 0 {
		for _, item := range cfg.SupportOS {
			if runtime.GOOS == item {
				osSupport = true
				break
			}
		}
	} else {
		osSupport = true
	}

	if len(cfg.SupportArch) > 0 {
		for _, item := range cfg.SupportArch {
			if runtime.GOARCH == item {
				archSupport = true
				break
			}
		}
	} else {
		archSupport = true
	}
	return osSupport && archSupport
}

func renderTemplate(text string, hdPkg *HDPackage) (result string, err error) {
	tmp, _ := template.New("hd").Parse(text)

	var buf bytes.Buffer
	if err = tmp.Execute(&buf, hdPkg); err == nil {
		result = buf.String()
	}
	return
}

func renderCmdsWithArgs(cmds []CmdWithArgs, hdPkg *HDPackage) (err error) {
	for i := range cmds {
		cmd := cmds[i]
		if err = renderCmdWithArgs(&cmd, hdPkg); err != nil {
			return
		}
	}
	return
}

func renderCmdWithArgs(cmd *CmdWithArgs, hdPkg *HDPackage) (err error) {
	if cmd == nil {
		return
	}

	if cmd.Cmd, err = renderTemplate(cmd.Cmd, hdPkg); err != nil {
		return
	}

	for i := range cmd.Args {
		arg := cmd.Args[i]
		if cmd.Args[i], err = renderTemplate(arg, hdPkg); err != nil {
			return
		}
	}
	return
}

func findByRepo(repo string) (result []string) {
	userHome, _ := homedir.Dir()
	configDir := userHome + "/.config/hd-home"
	matchedFile := configDir + "/config/*/" + repo + ".yml"

	if files, err := filepath.Glob(matchedFile); err == nil {
		for _, metaFile := range files {
			result = append(result, filepath.Base(filepath.Dir(metaFile)))
		}
	}
	return
}

func chooseOneFromArray(options []string) (result string, err error) {
	prompt := &survey.Select{
		Message: "Please select:",
		Options: options,
	}
	err = survey.AskOne(prompt, &result)
	return
}

func getPackagingFormat(installer *Installer) string {
	platformType := strings.ToLower(installer.OS)
	if platformType == "windows" {
		if installer.Package != nil {
			return installer.Package.FormatOverrides.Windows
		}
		return "zip"
	}
	if installer.Package != nil {
		return installer.Package.FormatOverrides.Linux
	}
	return "tar.gz"
}

func hasPackageSuffix(packageURL string) bool {
	for i := 0; i < len(supportedPackages); i++ {
		if strings.HasSuffix(packageURL, supportedPackages[i]) {
			return true
		}
	}
	return false
}
