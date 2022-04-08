package cmd

import (
	"context"
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg"
	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
	"net/http"
	"net/url"
	sysos "os"
	"path"
	"runtime"
	"strings"
)

// newGetCmd return the get command
func newGetCmd(ctx context.Context) (cmd *cobra.Command) {
	opt := &downloadOption{
		RoundTripper: getRoundTripper(ctx),
	}
	cmd = &cobra.Command{
		Use:     "get",
		Short:   "download the file",
		Example: "hd get jenkins-zh/jenkins-cli/jcli --thread 6",
		PreRunE: opt.preRunE,
		RunE:    opt.runE,
	}

	// set flags
	flags := cmd.Flags()
	opt.addFlags(flags)
	flags.StringVarP(&opt.Output, "output", "o", "", "Write output to <file> instead of stdout.")
	flags.BoolVarP(&opt.AcceptPreRelease, "accept-preRelease", "", false,
		"If you accept preRelease as the binary asset from GitHub")
	flags.BoolVarP(&opt.AcceptPreRelease, "pre", "", false,
		"Same with option --accept-preRelease")

	flags.IntVarP(&opt.Timeout, "time", "", 10,
		`The default timeout in seconds with the HTTP request`)
	flags.IntVarP(&opt.MaxAttempts, "max-attempts", "", 10,
		`Max times to attempt to download, zero means there's no retry action'`)
	flags.BoolVarP(&opt.ShowProgress, "show-progress", "", true, "If show the progress of download")
	flags.Int64VarP(&opt.ContinueAt, "continue-at", "", -1, "ContinueAt")
	flags.IntVarP(&opt.Thread, "thread", "t", viper.GetInt("thread"),
		`Download file with multi-threads. It only works when its value is bigger than 1`)
	flags.BoolVarP(&opt.KeepPart, "keep-part", "", false,
		"If you want to keep the part files instead of deleting them")
	flags.StringVarP(&opt.OS, "os", "", runtime.GOOS, "The OS of target binary file")
	flags.StringVarP(&opt.Arch, "arch", "", runtime.GOARCH, "The arch of target binary file")
	flags.BoolVarP(&opt.PrintSchema, "print-schema", "", false,
		"Print the schema of HDConfig if the flag is true without other function")
	flags.BoolVarP(&opt.PrintVersion, "print-version", "", false,
		"Print the version list")
	flags.BoolVarP(&opt.PrintCategories, "print-categories", "", false,
		"Print the category list")
	flags.IntVarP(&opt.PrintVersionCount, "print-version-count", "", 20,
		"The number of the version list")

	_ = cmd.RegisterFlagCompletionFunc("proxy-github", ArrayCompletion("gh.api.99988866.xyz",
		"ghproxy.com", "mirror.ghproxy.com"))
	_ = cmd.RegisterFlagCompletionFunc("provider", ArrayCompletion(ProviderGitHub, ProviderGitee))
	return
}

type downloadOption struct {
	searchOption

	URL              string
	Category         string
	Output           string
	ShowProgress     bool
	Timeout          int
	MaxAttempts      int
	AcceptPreRelease bool
	RoundTripper     http.RoundTripper

	ContinueAt int64

	Arch string
	OS   string

	Thread            int
	KeepPart          bool
	PrintSchema       bool
	PrintVersion      bool
	PrintVersionCount int
	PrintCategories   bool

	// inner fields
	name    string
	Tar     bool
	Package *installer.HDConfig
	org     string
	repo    string
}

const (
	// ProviderGitHub represents https://github.com
	ProviderGitHub = "github"
	// ProviderGitee represents https://gitee.com
	ProviderGitee = "gitee"
)

func (o *downloadOption) fetch() (err error) {
	if !o.Fetch {
		return
	}

	// fetch the latest config
	fmt.Println("start to fetch the config")
	fetcher := &installer.DefaultFetcher{}
	if err = fetcher.FetchLatestRepo(o.Provider, installer.ConfigBranch, sysos.Stdout); err != nil {
		err = fmt.Errorf("unable to fetch the latest config, error: %v", err)
		return
	}
	o.Fetch = false
	return
}

func (o *downloadOption) preRunE(cmd *cobra.Command, args []string) (err error) {
	// this might not be the best way to print schema
	if o.PrintSchema {
		return
	}

	if err = o.fetch(); err != nil {
		return
	}

	if o.PrintCategories {
		return
	}

	o.Tar = true
	if len(args) <= 0 {
		return fmt.Errorf("no URL provided")
	}

	targetURL := args[0]
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		ins := &installer.Installer{
			Provider: o.Provider,
			OS:       o.OS,
			Arch:     o.Arch,
			Fetch:    o.Fetch,
		}
		if targetURL, err = ins.ProviderURLParse(targetURL, o.AcceptPreRelease); err != nil {
			err = fmt.Errorf("only http:// or https:// supported, error: %v", err)
			return
		}
		o.Package = ins.Package
		o.Tar = ins.Tar
		o.name = ins.Name
		o.org = ins.Org
		o.repo = ins.Repo
	}
	o.URL = targetURL

	if o.ProxyGitHub != "" {
		o.URL = strings.Replace(o.URL, "github.com", fmt.Sprintf("%s/github.com", o.ProxyGitHub), 1)
	}

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
	// only print the schema for documentation
	if o.PrintSchema {
		var data []byte
		if data, err = yaml.Marshal(installer.HDConfig{
			Installation: &installer.CmdWithArgs{},
			PreInstalls:  []installer.CmdWithArgs{},
			PostInstalls: []installer.CmdWithArgs{},
			TestInstalls: []installer.CmdWithArgs{},
		}); err == nil {
			cmd.Print(string(data))
		}
		return
	}

	if o.PrintVersion {
		client := &pkg.ReleaseClient{}
		client.Init()
		var list []pkg.ReleaseAsset
		if list, err = client.ListReleases(o.org, o.repo, o.PrintVersionCount); err == nil {
			for _, item := range list {
				cmd.Println(item.TagName)
			}
		}
		return
	}

	if o.PrintCategories {
		cmd.Println(installer.FindCategories())
		return
	}

	cmd.Printf("start to download from %s\n", o.URL)
	if o.Thread <= 1 {
		err = pkg.DownloadWithContinue(o.URL, o.Output, o.ContinueAt, -1, 0, o.ShowProgress)
	} else {
		err = pkg.DownloadFileWithMultipleThreadKeepParts(o.URL, o.Output, o.Thread, o.KeepPart, o.ShowProgress)
	}
	return
}
