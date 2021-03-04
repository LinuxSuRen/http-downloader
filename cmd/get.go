package cmd

import (
	"bytes"
	"fmt"
	"github.com/ghodss/yaml"
	"github.com/linuxsuren/http-downloader/pkg"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net/url"
	"path"
	"runtime"
	"strings"
	"text/template"
)

// NewGetCmd return the get command
func NewGetCmd() (cmd *cobra.Command) {
	opt := &downloadOption{}
	cmd = &cobra.Command{
		Use:     "get",
		Short:   "download the file",
		Example: "hd get jenkins-zh/jenkins-cli/jcli --thread 6",
		PreRunE: opt.preRunE,
		RunE:    opt.runE,
	}

	// set flags
	flags := cmd.Flags()
	flags.StringVarP(&opt.Output, "output", "o", "", "Write output to <file> instead of stdout.")
	flags.BoolVarP(&opt.Fetch, "fetch", "", true,
		"If fetch the latest config from https://github.com/LinuxSuRen/hd-home")
	flags.BoolVarP(&opt.ShowProgress, "show-progress", "", true, "If show the progress of download")
	flags.Int64VarP(&opt.ContinueAt, "continue-at", "", -1, "ContinueAt")
	flags.IntVarP(&opt.Thread, "thread", "t", 0,
		`Download file with multi-threads. It only works when its value is bigger than 1`)
	flags.BoolVarP(&opt.KeepPart, "keep-part", "", false,
		"If you want to keep the part files instead of deleting them")
	flags.StringVarP(&opt.Provider, "provider", "", ProviderGitHub, "The file provider")
	flags.StringVarP(&opt.OS, "os", "", runtime.GOOS, "The OS of target binary file")
	flags.StringVarP(&opt.Arch, "arch", "", runtime.GOARCH, "The arch of target binary file")
	return
}

type downloadOption struct {
	URL          string
	Output       string
	ShowProgress bool
	Fetch        bool

	ContinueAt int64

	Provider string
	Arch     string
	OS       string

	Thread   int
	KeepPart bool

	// inner fields
	name    string
	Tar     bool
	Package *hdConfig
}

const (
	// ProviderGitHub represents https://github.com
	ProviderGitHub = "github"
)

func (o *downloadOption) providerURLParse(path string) (url string, err error) {
	url = path
	if o.Provider != ProviderGitHub {
		return
	}

	var (
		org     string
		repo    string
		name    string
		version string
	)

	addr := strings.Split(url, "/")
	if len(addr) >= 2 {
		org = addr[0]
		repo = addr[1]
		name = repo
	} else {
		err = fmt.Errorf("only support format xx/xx or xx/xx/xx")
		return
	}

	if len(addr) == 3 {
		name = addr[2]
	} else if len(addr) > 3 {
		err = fmt.Errorf("only support format xx/xx or xx/xx/xx")
	}

	// extract version from name
	if strings.Contains(name, "@") {
		nameWithVer := strings.Split(name, "@")
		name = nameWithVer[0]
		version = nameWithVer[1]

		url = fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s-%s-%s.tar.gz",
			org, repo, version, name, o.OS, o.Arch)
	} else {
		version = "latest"
		url = fmt.Sprintf("https://github.com/%s/%s/releases/%s/download/%s-%s-%s.tar.gz",
			org, repo, version, name, o.OS, o.Arch)
	}
	o.name = name

	// try to parse from config
	userHome, _ := homedir.Dir()
	configDir := userHome + "/.config/hd-home"
	matchedFile := configDir + "/config/" + org + "/" + repo + ".yml"
	if ok, _ := pathExists(matchedFile); ok {
		var data []byte
		if data, err = ioutil.ReadFile(matchedFile); err == nil {
			cfg := hdConfig{}

			if err = yaml.Unmarshal(data, &cfg); err == nil {
				hdPkg := &hdPackage{
					Name:       o.name,
					Version:    version,
					OS:         getReplacement(runtime.GOOS, cfg.Replacements),
					Arch:       getReplacement(runtime.GOARCH, cfg.Replacements),
					VersionNum: strings.TrimPrefix(version, "v"),
				}
				o.Package = &cfg

				if version == "latest" {
					ghClient := pkg.ReleaseClient{
						Org:  org,
						Repo: repo,
					}
					ghClient.Init()
					if asset, err := ghClient.GetLatestJCLIAsset(); err == nil {
						hdPkg.Version = asset.TagName
						hdPkg.VersionNum = strings.TrimPrefix(asset.TagName, "v")
					} else {
						fmt.Println(err, "cannot get the asset")
					}
				}

				if cfg.URL != "" {
					// it does not come from GitHub release
					tmp, _ := template.New("hd").Parse(cfg.URL)

					var buf bytes.Buffer
					if err = tmp.Execute(&buf, hdPkg); err == nil {
						url = buf.String()
					} else {
						return
					}
				} else if cfg.Filename != "" {
					tmp, _ := template.New("hd").Parse(cfg.Filename)

					var buf bytes.Buffer
					if err = tmp.Execute(&buf, hdPkg); err == nil {
						url = fmt.Sprintf("https://github.com/%s/%s/releases/%s/download/%s",
							org, repo, version, buf.String())

						o.Output = buf.String()
					} else {
						return
					}
				}

				if err = renderCmdWithArgs(cfg.PreInstall, hdPkg); err != nil {
					return
				}
				if err = renderCmdWithArgs(cfg.Installation, hdPkg); err != nil {
					return
				}
				if err = renderCmdWithArgs(cfg.PostInstall, hdPkg); err != nil {
					return
				}
				if err = renderCmdWithArgs(cfg.TestInstall, hdPkg); err != nil {
					return
				}

				o.Tar = cfg.Tar != "false"
				if cfg.Binary != "" {
					if cfg.Binary, err = renderTemplate(cfg.Binary, hdPkg); err != nil {
						return
					}
					o.name = cfg.Binary
				}
			}
		}
	}
	return
}

func renderTemplate(text string, hdPkg *hdPackage) (result string, err error) {
	tmp, _ := template.New("hd").Parse(text)

	var buf bytes.Buffer
	if err = tmp.Execute(&buf, hdPkg); err == nil {
		result = buf.String()
	}
	return
}

func renderCmdWithArgs(cmd *cmdWithArgs, hdPkg *hdPackage) (err error) {
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

type hdConfig struct {
	Name         string
	Filename     string
	Binary       string
	TargetBinary string
	URL          string `yaml:"url"`
	Tar          string
	Replacements map[string]string
	Installation *cmdWithArgs
	PreInstall   *cmdWithArgs
	PostInstall  *cmdWithArgs
	TestInstall  *cmdWithArgs
}

type cmdWithArgs struct {
	Cmd  string
	Args []string
}

type hdPackage struct {
	Name       string
	Version    string // e.g. v1.0.1
	VersionNum string // e.g. 1.0.1
	OS         string // e.g. linux, darwin
	Arch       string // e.g. amd64
}

func (o *downloadOption) preRunE(cmd *cobra.Command, args []string) (err error) {
	o.Tar = true
	if len(args) <= 0 {
		return fmt.Errorf("no URL provided")
	}

	if o.Fetch {
		cmd.Println("start to fetch the config")
		if err = fetchHomeConfig(); err != nil {
			err = fmt.Errorf("failed with fetching home config: %v", err)
			return
		}
	}

	targetURL := args[0]
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		if targetURL, err = o.providerURLParse(targetURL); err != nil {
			err = fmt.Errorf("only http:// or https:// supported, error: %v", err)
			return
		}
		cmd.Printf("start to download from %s\n", targetURL)
	}
	o.URL = targetURL

	if o.Output == "" {
		var urlObj *url.URL
		if urlObj, err = url.Parse(o.URL); err == nil {
			o.Output = path.Base(urlObj.Path)

			if o.Output == "" {
				err = fmt.Errorf("output cannot be empty")
			}
		} else {
			err = fmt.Errorf("cannot parse the target URL, error: '%v'", err)
		}
	}
	return
}

func (o *downloadOption) runE(cmd *cobra.Command, args []string) (err error) {
	if o.Thread <= 1 {
		err = pkg.DownloadWithContinue(o.URL, o.Output, o.ContinueAt, 0, o.ShowProgress)
	} else {
		err = pkg.DownloadFileWithMultipleThreadKeepParts(o.URL, o.Output, o.Thread, o.KeepPart, o.ShowProgress)
	}
	return
}
