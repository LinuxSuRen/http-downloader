package installer

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	sysos "os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/AlecAivazis/survey/v2"
	"github.com/linuxsuren/http-downloader/pkg"
	"github.com/linuxsuren/http-downloader/pkg/common"
	"github.com/linuxsuren/http-downloader/pkg/compress"
	"github.com/linuxsuren/http-downloader/pkg/net"
	"github.com/linuxsuren/http-downloader/pkg/os"
	"github.com/mitchellh/go-homedir"
	"gopkg.in/yaml.v3"

	fakeruntime "github.com/linuxsuren/go-fake-runtime"
)

const (
	// ProviderGitHub represents https://github.com
	ProviderGitHub = "github"
)

// CheckDepAndInstall checks the desired tools, install the missing packages
func (o *Installer) CheckDepAndInstall(tools map[string]string) (err error) {
	if o.Execer == nil {
		o.Execer = &fakeruntime.DefaultExecer{}
	}
	if o.OS == "" {
		o.OS = o.Execer.OS()
	}
	if o.Arch == "" {
		o.Arch = o.Execer.Arch()
	}
	if o.TargetDirectory == "" {
		o.TargetDirectory = "/usr/local/bin"
	}

	for tool, formula := range tools {
		if _, lookErr := o.Execer.LookPath(tool); lookErr == nil {
			continue
		}

		fmt.Printf("start to install missing tool: %s\n", tool)

		// check if it's a native package
		if os.HasPackage(tool) {
			if err = os.InstallWithProxy(tool, nil); err != nil {
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

			if o.ProxyGitHub != "" {
				targetURL = strings.Replace(targetURL, "github.com", fmt.Sprintf("%s/github.com", o.ProxyGitHub), 1)
			}
			log.Println("start to download", targetURL)
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

		if potentialOrgs := findOrgsByRepo(repo); len(potentialOrgs) == 0 {
			err = fmt.Errorf("cannot found the package: %s", repo)
			return
		} else if len(potentialOrgs) == 1 {
			org, repo = getOrgAndRepo(potentialOrgs[0])
		} else {
			var result string
			if result, err = chooseOneFromArray(potentialOrgs); err != nil {
				err = fmt.Errorf("failed to choose the potential organizations of your desired package")
				return
			}
			org, repo = getOrgAndRepo(result)
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

		// try to get the real name of a tool
		fetcher := &DefaultFetcher{}
		var configDir string
		if configDir, err = fetcher.GetConfigDir(); err != nil {
			return
		}
		config := getHDConfig(configDir, fmt.Sprintf("%s/%s", org, repo))
		if config != nil && config.Name != "" {
			name = config.Name
		}
	}

	o.Org = org
	o.Repo = repo
	o.Name = name
	return
}

func getOrgAndRepo(orgAndRepo string) (org string, repo string) {
	items := strings.Split(orgAndRepo, "/")
	if len(items) >= 2 {
		org = items[0]
		repo = items[1]
	}
	return
}

// ProviderURLParse parse the URL
func (o *Installer) ProviderURLParse(path string, acceptPreRelease bool) (packageURL string, err error) {
	packageURL = path

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

	// set the default values
	o.Tar = true

	// try to parse from config
	userHome, _ := homedir.Dir()
	configDir := userHome + "/.config/hd-home"
	matchedFile := configDir + "/config/" + o.Org + "/" + o.Repo + ".yml"
	log.Printf("start to find '%s' from local cache\n", path)
	if ok, _ := common.PathExists(matchedFile); ok {
		var data []byte
		if data, err = sysos.ReadFile(matchedFile); err == nil {
			cfg := HDConfig{}
			if !IsSupport(cfg) {
				err = fmt.Errorf("not support this platform, os: %s, arch: %s", o.OS, o.Arch)
				return
			}

			if err = yaml.Unmarshal(data, &cfg); err == nil {
				hdPkg := &HDPackage{
					Name:             o.Name,
					Version:          version,
					OS:               common.GetReplacement(o.OS, cfg.Replacements),
					Arch:             common.GetReplacement(o.Arch, cfg.Replacements),
					AdditionBinaries: cfg.AdditionBinaries,
					VersionNum:       strings.TrimPrefix(version, "v"),
				}
				cfg.Org = o.Org
				cfg.Repo = o.Repo
				cfg.FormatOverrides.Format = o.Package.FormatOverrides.Format
				o.Package = &cfg
				o.AdditionBinaries = cfg.AdditionBinaries
				o.Tar = cfg.Tar != "false"
				packagingFormat = getPackagingFormat(o) // rewrite the packing format due to the package config might be read from git repository

				if cfg.LatestVersion != "" {
					version = getVersionOrDefault(cfg.LatestVersion, version)
				}

				if version == "latest" || version == "" {
					log.Println("try to find the latest version")
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
					log.Printf("using provided version: %s\n", version)
					hdPkg.VersionNum = common.ParseVersionNum(version)
					if packageURL == "" {
						packageURL = fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s-%s-%s.%s",
							o.Org, o.Repo, version, o.Name, o.OS, o.Arch, packagingFormat)
					}
				}

				var ver string
				if ver, err = getDynamicVersion(cfg.Version); ver != "" {
					hdPkg.Version = ver
				} else if err != nil {
					return
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
						o.Output = buf.String()
						if o.Tar && !hasPackageSuffix(packageURL) {
							packageURL = fmt.Sprintf("%s.%s", packageURL, packagingFormat)
							o.Output = fmt.Sprintf("%s.%s", o.Output, packagingFormat)
						}
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

				if cfg.Binary != "" {
					if cfg.Binary, err = renderTemplate(cfg.Binary, hdPkg); err != nil {
						return
					}
					o.Name = cfg.Binary
				}
				o.Package.Version = hdPkg.Version
			} else {
				err = fmt.Errorf("failed to parse YAML file: %s, error: %v", matchedFile, err)
			}
		}
	}
	return
}

// parse the version if it's an URL
func getDynamicVersion(version string) (realVersion string, err error) {
	if version != "" && (strings.HasPrefix(version, "http://") || strings.HasPrefix(version, "https://")) {
		var resp *http.Response
		log.Println("get dynamic version", version)
		if resp, err = http.Get(version); err != nil || resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("cannot get version from '%s', error is '%v', status code is '%d'", version, err, resp.StatusCode)
			return
		}
		var data []byte
		if data, err = io.ReadAll(resp.Body); err != nil {
			err = fmt.Errorf("failed to get version from '%s', error is '%v'", version, err)
			return
		}
		realVersion = string(data)
	}
	return
}

func getVersionOrDefault(version string, defaultVer string) (target string) {
	target = defaultVer
	// for the security reason, only support https
	if strings.HasPrefix(version, "https://") {
		if response, err := http.Get(version); err == nil {
			if data, err := io.ReadAll(response.Body); err == nil {
				target = string(data)
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

func findOrgsByRepo(repo string) (result []string) {
	userHome, _ := homedir.Dir()
	configDir := userHome + "/.config/hd-home"

	result = FindByKeyword(repo, configDir)
	return
}

// FindByKeyword find org/repo by a keyword
func FindByKeyword(keyword, configDir string) (result []string) {
	if files, err := filepath.Glob(path.Join(configDir, "config/**/*.yml")); err == nil {
		for _, metaFile := range files {
			ext := path.Ext(metaFile)
			fileName := filepath.Base(metaFile)
			org := filepath.Base(filepath.Dir(metaFile))
			repo := strings.TrimSuffix(fileName, ext)

			if !strings.Contains(repo, keyword) && !hasKeyword(metaFile, keyword) {
				continue
			}

			result = append(result, path.Join(org, repo))
		}
	}

	// find in the generic packages
	result = append(result, os.SearchPackages(keyword)...)
	return
}

func hasKeyword(metaFile, keyword string) (ok bool) {
	data, err := sysos.ReadFile(metaFile)
	if err != nil {
		return
	}

	config := &HDConfig{}
	if err = yaml.Unmarshal(data, config); err == nil {
		ok = strings.Contains(config.Name, keyword) || strings.Contains(config.Binary, keyword) ||
			strings.Contains(config.TargetBinary, keyword)
	}
	return
}

func getHDConfig(configDir, orgAndRepo string) (config *HDConfig) {
	configFile := path.Join(configDir, "config", orgAndRepo+".yml")
	data, err := sysos.ReadFile(configFile)
	if err != nil {
		return
	}

	config = &HDConfig{}
	_ = yaml.Unmarshal(data, config)
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
	if installer.Package != nil && installer.Package.FormatOverrides.Format != "" {
		return installer.Package.FormatOverrides.Format
	}
	platformType := strings.ToLower(installer.OS)
	if platformType == "windows" {
		if installer.Package != nil && installer.Package.FormatOverrides.Windows != "" {
			return installer.Package.FormatOverrides.Windows
		}
		return "zip"
	}
	if installer.Package != nil && installer.Package.FormatOverrides.Linux != "" {
		return installer.Package.FormatOverrides.Linux
	}
	return "tar.gz"
}

func hasPackageSuffix(packageURL string) bool {
	return compress.IsSupport(path.Ext(packageURL))
}

// FindCategories returns the whole supported categories
func FindCategories() (result []string) {
	categories := make(map[string]string, 0)
	configDir := sysos.ExpandEnv("$HOME/.config/hd-home/config/")
	_ = filepath.Walk(configDir, func(basepath string, info fs.FileInfo, err error) error {
		if !strings.HasSuffix(basepath, ".yml") {
			return nil
		}

		if data, err := sysos.ReadFile(basepath); err == nil {
			hdCfg := &HDConfig{}
			if err := yaml.Unmarshal(data, hdCfg); err == nil {
				for _, category := range hdCfg.Categories {
					categories[category] = ""
				}
			}
		}
		return nil
	})
	for key := range categories {
		result = append(result, key)
	}
	return
}

// FindPackagesByCategory returns the HDConfigs by category
func FindPackagesByCategory(category string) (result []HDConfig) {
	configDir := sysos.ExpandEnv("$HOME/.config/hd-home/config/")
	_ = filepath.Walk(configDir, func(basepath string, info fs.FileInfo, err error) error {
		if !strings.HasSuffix(basepath, ".yml") {
			return nil
		}

		if data, err := sysos.ReadFile(basepath); err == nil {
			hdCfg := &HDConfig{}
			if err := yaml.Unmarshal(data, hdCfg); err == nil {
				orgAndRepo := strings.TrimPrefix(basepath, configDir)

				hdCfg.Org = strings.Split(orgAndRepo, "/")[0]
				hdCfg.Repo = strings.TrimSuffix(strings.Split(orgAndRepo, "/")[1], ".yml")

				for _, item := range hdCfg.Categories {
					if item == category {
						result = append(result, *hdCfg)
						break
					}
				}
			}
		}
		return nil
	})
	return
}

type proxyServer struct {
	Servers []string `yaml:"servers"`
}

// GetProxyServers returns the proxy servers
func GetProxyServers() []string {
	configFile := sysos.ExpandEnv("$HOME/.config/hd-home/proxy.yaml")
	data, err := sysos.ReadFile(configFile)
	if err == nil {
		proxyServer := &proxyServer{}

		if err = yaml.Unmarshal(data, proxyServer); err == nil {
			return proxyServer.Servers
		} else {
			log.Println("failed to parse config file", err)
		}
	} else {
		log.Println("failed to read config file", err)
	}

	return nil
}
