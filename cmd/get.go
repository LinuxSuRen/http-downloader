package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	sysos "os"
	"path"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/linuxsuren/http-downloader/pkg/common"
	"github.com/linuxsuren/http-downloader/pkg/log"

	"github.com/AlecAivazis/survey/v2"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"

	fakeruntime "github.com/linuxsuren/go-fake-runtime"
	"github.com/linuxsuren/http-downloader/pkg"
	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/linuxsuren/http-downloader/pkg/net"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// newGetCmd return the get command
func newGetCmd(ctx context.Context) (cmd *cobra.Command) {
	opt := newDownloadOption(ctx)
	cmd = &cobra.Command{
		Use:     "get",
		Aliases: []string{"download"},
		Short:   "Download the file",
		Example: "hd get jenkins-zh/jenkins-cli/jcli --thread 6",
		PreRunE: opt.preRunE,
		RunE:    opt.runE,
		GroupID: coreGroup.ID,
	}

	// set flags
	flags := cmd.Flags()
	opt.addFlags(flags)
	opt.addPlatformFlags(flags)
	opt.addDownloadFlags(flags)
	flags.StringVarP(&opt.Output, "output", "o", "", "Write output to <file> instead of stdout.")
	flags.BoolVarP(&opt.AcceptPreRelease, "accept-preRelease", "", false,
		"If you accept preRelease as the binary asset from GitHub")
	flags.BoolVarP(&opt.AcceptPreRelease, "pre", "", false,
		"Same with option --accept-preRelease")
	flags.BoolVarP(&opt.Force, "force", "f", false, "Overwrite the exist file if this is true")

	flags.DurationVarP(&opt.Timeout, "timeout", "", 15*time.Minute,
		`The default timeout in seconds with the HTTP request`)
	flags.IntVarP(&opt.MaxAttempts, "max-attempts", "", 10,
		`Max times to attempt to download, zero means there's no retry action'`)
	flags.Int64VarP(&opt.ContinueAt, "continue-at", "", -1, "ContinueAt")
	flags.BoolVarP(&opt.KeepPart, "keep-part", "", false,
		"If you want to keep the part files instead of deleting them")
	flags.BoolVarP(&opt.PrintSchema, "print-schema", "", false,
		"Print the schema of HDConfig if the flag is true without other function")
	flags.BoolVarP(&opt.PrintVersion, "print-version", "", false,
		"Print the version list")
	flags.BoolVarP(&opt.PrintCategories, "print-categories", "", false,
		"Print the category list")
	flags.IntVarP(&opt.PrintVersionCount, "print-version-count", "", 20,
		"The number of the version list")
	flags.BoolVarP(&opt.Magnet, "magnet", "", false, "Fetch magnet list from a website")
	flags.StringVarP(&opt.Format, "format", "", "", "Specific the file format, for instance: tar, zip, msi")
	return
}

func newDownloadOption(ctx context.Context) *downloadOption {
	return &downloadOption{
		RoundTripper: getRoundTripper(ctx),
		fetcher:      &installer.DefaultFetcher{},
		wait:         &sync.WaitGroup{},
		execer:       fakeruntime.DefaultExecer{},
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
	Timeout          time.Duration
	NoProxy          bool
	MaxAttempts      int
	AcceptPreRelease bool
	RoundTripper     http.RoundTripper
	Magnet           bool
	Force            bool
	Mod              int
	SkipTLS          bool
	Format           string

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
	execer        fakeruntime.Execer
	ExpectVersion string // should be like >v1.1.0
}

const (
	// ProviderGitHub represents https://github.com
	ProviderGitHub = "github"
	// ProviderGitee represents https://gitee.com
	ProviderGitee = "gitee"
)

func (o *downloadOption) addDownloadFlags(flags *pflag.FlagSet) {
	flags.IntVarP(&o.Mod, "mod", "", -1, "The file permission, -1 means using the system default")
	flags.BoolVarP(&o.SkipTLS, "skip-tls", "k", false, "Skip the TLS")
	flags.BoolVarP(&o.ShowProgress, "show-progress", "", true, "If show the progress of download")
	flags.IntVarP(&o.Thread, "thread", "t", viper.GetInt("thread"),
		`Download file with multi-threads. It only works when its value is bigger than 1`)
	flags.BoolVarP(&o.NoProxy, "no-proxy", "", viper.GetBool("no-proxy"), "Indicate no HTTP proxy taken")
}

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
	o.Package = &installer.HDConfig{
		FormatOverrides: installer.PackagingFormat{
			Format: o.Format,
		},
	}
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
			Package:  o.Package,
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

func (o *downloadOption) addPlatformFlags(flags *pflag.FlagSet) {
	flags.StringVarP(&o.OS, "os", "", runtime.GOOS, "The OS of target binary file")
	flags.StringVarP(&o.Arch, "arch", "", runtime.GOARCH, "The arch of target binary file")
}

func findAnchor(n *html.Node) (items []string) {
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
	logger := log.GetLoggerFromContextOrDefault(cmd)
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

	// check if want to overwrite the exist file
	logger.Println("output file is", o.Output)
	if common.Exist(o.Output) && !o.Force {
		logger.Printf("The output file: '%s' was exist, please use flag --force if you want to overwrite it.\n", o.Output)
		return
	}

	if o.Magnet || strings.HasPrefix(o.URL, "magnet:?") {
		err = downloadMagnetFile(o.ProxyGitHub, o.URL, o.execer)
		return
	}

	targetURL := o.URL
	if o.ProxyGitHub != "" {
		targetURL = strings.Replace(targetURL, "https://github.com", fmt.Sprintf("https://%s/github.com", o.ProxyGitHub), 1)
		targetURL = strings.Replace(targetURL, "https://raw.githubusercontent.com", fmt.Sprintf("https://%s/https://raw.githubusercontent.com", o.ProxyGitHub), 1)
	}
	logger.Printf("start to download from %s\n", targetURL)
	var suggestedFilenameAware net.SuggestedFilenameAware
	if o.Thread <= 1 {
		downloader := &net.ContinueDownloader{}
		suggestedFilenameAware = downloader
		downloader.WithoutProxy(o.NoProxy).
			WithRoundTripper(o.RoundTripper).
			WithInsecureSkipVerify(o.SkipTLS).
			WithTimeout(o.Timeout)
		err = downloader.DownloadWithContinue(targetURL, o.Output, o.ContinueAt, -1, 0, o.ShowProgress)
	} else {
		downloader := &net.MultiThreadDownloader{}
		suggestedFilenameAware = downloader
		downloader.WithKeepParts(o.KeepPart).
			WithShowProgress(o.ShowProgress).
			WithoutProxy(o.NoProxy).
			WithRoundTripper(o.RoundTripper).
			WithInsecureSkipVerify(o.SkipTLS).
			WithTimeout(o.Timeout)
		err = downloader.Download(targetURL, o.Output, o.Thread)
	}

	// set file permission
	if o.Mod != -1 {
		logger.Printf("Setting file permission to %d", o.Mod)
		if err = sysos.Chmod(o.Output, fs.FileMode(o.Mod)); err != nil {
			return
		}
	}

	if err == nil {
		logger.Printf("downloaded: %s\n", o.Output)
	}

	if suggested := suggestedFilenameAware.GetSuggestedFilename(); suggested != "" {
		confirm := &survey.Confirm{
			Message: fmt.Sprintf("Do you want to rename filename from '%s' to '%s'?", o.Output, suggested),
		}
		var yes bool
		if confirmErr := survey.AskOne(confirm, &yes); confirmErr == nil && yes {
			err = sysos.Rename(o.Output, suggested)
		}
	}
	return
}

func downloadMagnetFile(proxyGitHub, target string, execer fakeruntime.Execer) (err error) {
	targetCmd := "gotorrent"
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

			if len(items) > 1 {
				selector := &survey.Select{
					Message: "Select item",
					Options: items,
				}
				err = survey.AskOne(selector, &target)
			} else if len(items) > 0 {
				target = items[0]
			}
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
