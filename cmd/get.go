package cmd

import (
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg"
	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"net/url"
	"path"
	"runtime"
	"strings"
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
	flags.IntVarP(&opt.Thread, "thread", "t", 0,
		`Download file with multi-threads. It only works when its value is bigger than 1`)
	flags.BoolVarP(&opt.KeepPart, "keep-part", "", false,
		"If you want to keep the part files instead of deleting them")
	flags.StringVarP(&opt.Provider, "provider", "", ProviderGitHub, "The file provider")
	flags.StringVarP(&opt.OS, "os", "", runtime.GOOS, "The OS of target binary file")
	flags.StringVarP(&opt.Arch, "arch", "", runtime.GOARCH, "The arch of target binary file")
	flags.BoolVarP(&opt.PrintSchema, "print-schema", "", false,
		"Print the schema of HDConfig if the flag is true without other function")
	return
}

type downloadOption struct {
	URL              string
	Output           string
	ShowProgress     bool
	Fetch            bool
	Timeout          int
	MaxAttempts      int
	AcceptPreRelease bool

	ContinueAt int64

	Provider string
	Arch     string
	OS       string

	Thread      int
	KeepPart    bool
	PrintSchema bool

	// inner fields
	name    string
	Tar     bool
	Package *installer.HDConfig
}

const (
	// ProviderGitHub represents https://github.com
	ProviderGitHub = "github"
)

func (o *downloadOption) preRunE(cmd *cobra.Command, args []string) (err error) {
	// this might not be the best way to print schema
	if o.PrintSchema {
		return
	}

	o.Tar = true
	if len(args) <= 0 {
		return fmt.Errorf("no URL provided")
	}

	targetURL := args[0]
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		ins := installer.Installer{
			Provider: o.Provider,
			OS:       o.OS,
			Arch:     o.Arch,
			Fetch:    o.Fetch,
		}
		if targetURL, err = ins.ProviderURLParse(targetURL, o.AcceptPreRelease); err != nil {
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
	// only print the schema for documentation
	if o.PrintSchema {
		var data []byte
		if data, err = yaml.Marshal(installer.HDConfig{
			Installation: &installer.CmdWithArgs{},
			PreInstall:   &installer.CmdWithArgs{},
			PostInstall:  &installer.CmdWithArgs{},
			TestInstall:  &installer.CmdWithArgs{},
		}); err == nil {
			cmd.Print(string(data))
		}
		return
	}

	if o.Thread <= 1 {
		err = pkg.DownloadWithContinue(o.URL, o.Output, o.ContinueAt, -1, 0, o.ShowProgress)
	} else {
		err = pkg.DownloadFileWithMultipleThreadKeepParts(o.URL, o.Output, o.Thread, o.KeepPart, o.ShowProgress)
	}
	return
}
