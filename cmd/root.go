package cmd

import (
	"fmt"
	extver "github.com/linuxsuren/cobra-extension/version"
	"github.com/linuxsuren/http-downloader/pkg"
	"github.com/spf13/cobra"
	"net/url"
	"path"
	"runtime"
	"strings"
)

// NewRoot returns the root command
func NewRoot() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "hd",
		Short: "HTTP download tool",
	}

	opt := &downloadOption{}
	getCmd := &cobra.Command{
		Use:     "get",
		Short:   "download the file",
		Example: "hd get jenkins-zh/jenkins-cli/jcli -o jcli.tar.gz --thread 3",
		PreRunE: opt.preRunE,
		RunE:    opt.runE,
	}

	// set flags
	flags := getCmd.Flags()
	flags.StringVarP(&opt.Output, "output", "o", "", "Write output to <file> instead of stdout.")
	flags.BoolVarP(&opt.ShowProgress, "show-progress", "", true, "If show the progress of download")
	flags.Int64VarP(&opt.ContinueAt, "continue-at", "", -1, "ContinueAt")
	flags.IntVarP(&opt.Thread, "thread", "", 0, "")
	flags.StringVarP(&opt.Provider, "provider", "", ProviderGitHub, "The file provider")
	flags.StringVarP(&opt.OS, "os", "", "", "The OS of target binary file")
	flags.StringVarP(&opt.Arch, "arch", "", "", "The arch of target binary file")

	cmd.AddCommand(
		getCmd,
		extver.NewVersionCmd("linuxsuren", "http-downloader", "hd", nil))
	return
}

type downloadOption struct {
	URL          string
	Output       string
	ShowProgress bool

	ContinueAt int64

	Provider string
	Arch     string
	OS       string

	Thread int
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
	return
}

func (o *downloadOption) preRunE(cmd *cobra.Command, args []string) (err error) {
	if len(args) <= 0 {
		return fmt.Errorf("no URL provided")
	}

	if o.OS == "" {
		o.OS = runtime.GOOS
	}

	if o.Arch == "" {
		o.Arch = runtime.GOARCH
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
		err = pkg.DownloadFileWithMultipleThread(o.URL, o.Output, o.Thread, o.ShowProgress)
	}
	return
}
