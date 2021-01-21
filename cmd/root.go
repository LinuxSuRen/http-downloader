package cmd

import (
	"fmt"
	extver "github.com/linuxsuren/cobra-extension/version"
	"github.com/linuxsuren/http-downloader/pkg"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
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
	} else {
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

	url := args[0]
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		if url, err = o.providerURLParse(url); err != nil {
			err = fmt.Errorf("only http:// or https:// supported, error: %v", err)
			return
		} else {
			cmd.Printf("start to download from %s\n", url)
		}
	}
	o.URL = url

	if o.Output == "" {
		err = fmt.Errorf("output cannot be empty")
	}
	return
}

func (o *downloadOption) runE(cmd *cobra.Command, args []string) (err error) {
	if o.Thread <= 1 {
		err = o.download(o.Output, o.ContinueAt, 0)
	} else {
		// get the total size of the target file
		var total int64
		var rangeSupport bool
		if total, rangeSupport, err = o.detectSize(o.Output); err != nil {
			return
		}

		if rangeSupport {
			unit := total / int64(o.Thread)
			offset := total - unit*int64(o.Thread)
			var wg sync.WaitGroup

			cmd.Printf("start to download with %d threads, size: %d, unit: %d\n", o.Thread, total, unit)
			for i := 0; i < o.Thread; i++ {
				wg.Add(1)
				go func(index int, wg *sync.WaitGroup) {
					defer wg.Done()

					end := unit*int64(index+1) - 1
					if index == o.Thread-1 {
						// this is the last part
						end += offset
					}
					start := unit * int64(index)

					if downloadErr := o.download(fmt.Sprintf("%s-%d", o.Output, index), start, end); downloadErr != nil {
						cmd.PrintErrln(downloadErr)
					}
				}(i, &wg)
			}

			wg.Wait()

			// concat all these partial files
			var f *os.File
			if f, err = os.OpenFile(o.Output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
				defer func() {
					_ = f.Close()
				}()

				for i := 0; i < o.Thread; i++ {
					partFile := fmt.Sprintf("%s-%d", o.Output, i)
					if data, ferr := ioutil.ReadFile(partFile); ferr == nil {
						if _, err = f.Write(data); err != nil {
							err = fmt.Errorf("failed to write file: '%s'", partFile)
							break
						} else {
							_ = os.RemoveAll(partFile)
						}
					} else {
						err = fmt.Errorf("failed to read file: '%s'", partFile)
						break
					}
				}
			}
		} else {
			cmd.Println("cannot download it using multiple threads, failed to one")
			err = o.download(o.Output, o.ContinueAt, 0)
		}
	}
	return
}

func (o *downloadOption) detectSize(output string) (total int64, rangeSupport bool, err error) {
	downloader := pkg.HTTPDownloader{
		TargetFilePath: output,
		URL:            o.URL,
		ShowProgress:   o.ShowProgress,
	}

	var detectOffset int64
	var lenErr error

	detectOffset = 2
	downloader.Header = make(map[string]string, 1)
	downloader.Header["Range"] = fmt.Sprintf("bytes=%d-", detectOffset)

	downloader.PreStart = func(resp *http.Response) bool {
		rangeSupport = resp.StatusCode == http.StatusPartialContent
		contentLen := resp.Header.Get("Content-Length")
		if total, lenErr = strconv.ParseInt(contentLen, 10, 0); lenErr == nil {
			total += detectOffset
		}
		//  always return false because we just want to get the header from response
		return false
	}

	if err = downloader.DownloadFile(); err != nil || lenErr != nil {
		err = fmt.Errorf("cannot download from %s, response error: %v, content length error: %v", o.URL, err, lenErr)
	}
	return
}

func (o *downloadOption) download(output string, continueAt, end int64) (err error) {
	downloader := pkg.HTTPDownloader{
		TargetFilePath: output,
		URL:            o.URL,
		ShowProgress:   o.ShowProgress,
	}

	if continueAt >= 0 {
		downloader.Header = make(map[string]string, 1)

		//fmt.Println("range", continueAt, end)
		if end > continueAt {
			downloader.Header["Range"] = fmt.Sprintf("bytes=%d-%d", continueAt, end)
		} else {
			downloader.Header["Range"] = fmt.Sprintf("bytes=%d-", continueAt)
		}
	}

	if err = downloader.DownloadFile(); err != nil {
		err = fmt.Errorf("cannot download from %s, error: %v", o.URL, err)
	}
	return
}
