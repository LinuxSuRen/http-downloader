package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	sysos "os"
	"path"
	"runtime"
	"strings"
	"sync"

	"github.com/AlecAivazis/survey/v2"
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"

	"github.com/linuxsuren/http-downloader/pkg"
	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/linuxsuren/http-downloader/pkg/net"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// newGetCmd return the get command
func newGetCmd(ctx context.Context) (cmd *cobra.Command) {
	opt := newDownloadOption(ctx)
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
	flags.BoolVarP(&opt.NoProxy, "no-proxy", "", false, "Indicate no HTTP proxy taken")
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
	flags.BoolVarP(&opt.Magnet, "magnet", "", false, "Fetch magnet list from a website")

	_ = cmd.RegisterFlagCompletionFunc("proxy-github", ArrayCompletion("gh.api.99988866.xyz",
		"ghproxy.com", "mirror.ghproxy.com"))
	_ = cmd.RegisterFlagCompletionFunc("provider", ArrayCompletion(ProviderGitHub, ProviderGitee))
	return
}

func newDownloadOption(ctx context.Context) *downloadOption {
	return &downloadOption{
		RoundTripper: getRoundTripper(ctx),
		fetcher:      &installer.DefaultFetcher{},
		wait:         &sync.WaitGroup{},
	}
}

type downloadOption struct {
	searchOption
	cancel context.CancelFunc
	wait   *sync.WaitGroup

	URL              string
	Category         string
	Output           string
	ShowProgress     bool
	Timeout          int
	NoProxy          bool
	MaxAttempts      int
	AcceptPreRelease bool
	RoundTripper     http.RoundTripper
	Magnet           bool

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
	name          string
	Tar           bool
	Package       *installer.HDConfig
	org           string
	repo          string
	fetcher       installer.Fetcher
	ExpectVersion string // should be like >v1.1.0
}

const (
	// ProviderGitHub represents https://github.com
	ProviderGitHub = "github"
	// ProviderGitee represents https://gitee.com
	ProviderGitee = "gitee"
)

func (o *downloadOption) fetch() (err error) {
	if !o.Fetch {
		o.wait.Add(1)
		go func() {
			// no need to handle the error due to this is a background task
			if o.fetcher != nil {
				err = o.fetcher.FetchLatestRepo(o.Provider, installer.ConfigBranch, bytes.NewBuffer([]byte{}))
			}
			o.wait.Done()
		}()
		return
	}

	// fetch the latest config
	fmt.Println("start to fetch the config")
	if err = o.fetcher.FetchLatestRepo(o.Provider, installer.ConfigBranch, sysos.Stdout); err != nil {
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

	if cmd.Context() != nil {
		ctx, cancel := context.WithCancel(cmd.Context())
		o.cancel = cancel
		o.fetcher.SetContext(ctx)
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
	o.Package = &installer.HDConfig{}
	if o.Magnet || strings.HasPrefix(targetURL, "magnet:?") {
		// download via external tool
		o.URL = targetURL
		return
	} else if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
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

func findAnchor(n *html.Node) (items []string) {
	if n == nil {
		return nil
	}
	if n.Type == html.ElementNode && n.Data == "a" {
		for _, a := range n.Attr {
			if a.Key == "href" && strings.Contains(a.Val, "magnet") {
				items = append(items, strings.TrimSpace(n.FirstChild.Data))
				break
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		items = append(items, findAnchor(c)...)
	}
	return
}

func (o *downloadOption) runE(cmd *cobra.Command, args []string) (err error) {
	defer func() {
		if o.cancel != nil {
			o.cancel()
			o.wait.Wait()
		}
	}()

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

	if o.Magnet || strings.HasPrefix(o.URL, "magnet:?") {
		err = downloadMagnetFile(o.ProxyGitHub, o.URL)
		return
	}

	targetURL := o.URL
	if o.ProxyGitHub != "" {
		targetURL = strings.Replace(targetURL, "github.com", fmt.Sprintf("%s/github.com", o.ProxyGitHub), 1)
	}
	cmd.Printf("start to download from %s\n", targetURL)
	if o.Thread <= 1 {
		downloader := &net.ContinueDownloader{}
		downloader.WithoutProxy(o.NoProxy).
			WithRoundTripper(o.RoundTripper)
		err = downloader.DownloadWithContinue(targetURL, o.Output, o.ContinueAt, -1, 0, o.ShowProgress)
	} else {
		downloader := &net.MultiThreadDownloader{}
		downloader.WithKeepParts(o.KeepPart).
			WithShowProgress(o.ShowProgress).
			WithoutProxy(o.NoProxy).
			WithRoundTripper(o.RoundTripper)
		err = downloader.Download(targetURL, o.Output, o.Thread)
	}
	return
}

func downloadMagnetFile(proxyGitHub, target string) (err error) {
	targetCmd := "gotorrent"
	execer := exec.DefaultExecer{}
	is := installer.Installer{
		Provider:    "github",
		Execer:      execer,
		ProxyGitHub: proxyGitHub,
	}
	if err = is.CheckDepAndInstall(map[string]string{
		targetCmd: "linuxsuren/gotorrent",
	}); err != nil {
		return
	}

	if strings.HasPrefix(target, "http") {
		var resp *http.Response
		if resp, err = http.Get(target); err == nil && resp.StatusCode == http.StatusOK {
			var data []byte
			data, err = io.ReadAll(resp.Body)
			if err != nil {
				return
			}

			var reader io.Reader
			if reader, err = charset.NewReader(strings.NewReader(string(data)), "UTF-8"); err != nil {
				return
			}

			var docutf8 *html.Node
			if docutf8, err = html.Parse(reader); err != nil {
				return
			}
			items := findAnchor(docutf8)

			selector := &survey.Select{
				Message: "Select item",
				Options: items,
			}
			err = survey.AskOne(selector, &target)
		}
	}

	fmt.Println(target)
	if target == "" || err != nil {
		return
	}

	var targetBinary string
	if targetBinary, err = execer.LookPath(targetCmd); err == nil {
		sysCallArgs := []string{targetCmd}
		sysCallArgs = append(sysCallArgs, []string{"download", target}...)
		err = execer.SystemCall(targetBinary, sysCallArgs, sysos.Environ())
	}
	return
}
