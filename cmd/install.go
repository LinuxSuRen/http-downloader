package cmd

import (
	"context"
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg/common"
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/linuxsuren/http-downloader/pkg/os"
	"github.com/spf13/cobra"
	sysos "os"
	"path"
	"runtime"
	"strings"
)

// newInstallCmd returns the install command
func newInstallCmd(ctx context.Context) (cmd *cobra.Command) {
	opt := &installOption{
		downloadOption: downloadOption{
			RoundTripper: getRoundTripper(ctx),
		},
	}
	cmd = &cobra.Command{
		Use:     "install",
		Short:   "Install a package from https://github.com/LinuxSuRen/hd-home",
		Example: "hd install jenkins-zh/jenkins-cli/jcli -t 6",
		Args:    cobra.MinimumNArgs(1),
		PreRunE: opt.preRunE,
		RunE:    opt.runE,
	}

	flags := cmd.Flags()
	flags.BoolVarP(&opt.ShowProgress, "show-progress", "", true, "If show the progress of download")
	flags.BoolVarP(&opt.Fetch, "fetch", "", true,
		"If fetch the latest config from https://github.com/LinuxSuRen/hd-home")
	flags.BoolVarP(&opt.AcceptPreRelease, "accept-preRelease", "", false,
		"If you accept preRelease as the binary asset from GitHub")
	flags.BoolVarP(&opt.AcceptPreRelease, "pre", "", false,
		"Same with option --accept-preRelease")
	flags.BoolVarP(&opt.fromSource, "from-source", "", false,
		"Indicate if install it via go install github.com/xxx/xxx")
	flags.StringVarP(&opt.fromBranch, "from-branch", "", "master",
		"Only works if the flag --from-source is true")
	flags.StringVarP(&opt.ProxyGitHub, "proxy-github", "", "",
		`The proxy address of github.com, the proxy address will be the prefix of the final address.
Available proxy: gh.api.99988866.xyz
Thanks to https://github.com/hunshcn/gh-proxy`)

	flags.BoolVarP(&opt.Download, "download", "", true,
		"If download the package")
	flags.BoolVarP(&opt.force, "force", "f", false,
		"Indicate if force to download the package even it is exist")
	flags.BoolVarP(&opt.CleanPackage, "clean-package", "", true,
		"Clean the package if the installation is success")
	flags.IntVarP(&opt.Thread, "thread", "t", 4,
		`Download file with multi-threads. It only works when its value is bigger than 1`)
	flags.BoolVarP(&opt.KeepPart, "keep-part", "", false,
		"If you want to keep the part files instead of deleting them")
	flags.StringVarP(&opt.Provider, "provider", "", ProviderGitHub, "The file provider")
	flags.StringVarP(&opt.OS, "os", "", runtime.GOOS, "The OS of target binary file")
	flags.StringVarP(&opt.Arch, "arch", "", runtime.GOARCH, "The arch of target binary file")

	_ = cmd.RegisterFlagCompletionFunc("provider", ArrayCompletion(ProviderGitHub, "gitee"))
	return
}

type installOption struct {
	downloadOption
	Download     bool
	CleanPackage bool
	fromSource   bool
	fromBranch   string
	force        bool

	// inner fields
	nativePackage bool
	tool          string
}

func (o *installOption) shouldInstall() (should, exist bool) {
	if _, lookErr := exec.LookPath(o.tool); lookErr == nil {
		exist = true
	}
	should = o.force || !exist
	return
}

func (o *installOption) preRunE(cmd *cobra.Command, args []string) (err error) {
	o.tool = args[0]

	// try to find if it's a native package
	o.nativePackage = os.HasPackage(o.tool)
	if !o.nativePackage {
		err = o.downloadOption.preRunE(cmd, args)
	}
	return
}

func (o *installOption) runE(cmd *cobra.Command, args []string) (err error) {
	if should, exist := o.shouldInstall(); !should {
		if exist {
			cmd.Printf("%s is already exist\n", o.tool)
		}
		return
	}

	if o.nativePackage {
		// install a package
		err = os.Install(args[0])
		return
	}

	// aka go get github.com/xxx/xxx
	if o.fromSource {
		err = o.installFromSource()
		return
	}

	// install a package from hd-home
	if o.Download {
		if err = o.downloadOption.runE(cmd, args); err != nil {
			return
		}
	}

	process := &installer.Installer{
		Source:       o.downloadOption.Output,
		Name:         o.name,
		Package:      o.Package,
		Tar:          o.Tar,
		Output:       o.Output,
		CleanPackage: o.CleanPackage,
	}

	// TODO consider how to handle if the package is missing
	// maybe it's possible to provide a default strategy for it.
	if o.Package != nil {
		process.AdditionBinaries = o.Package.AdditionBinaries

		// install requirements tools in the post phase
		if len(o.Package.Requirements) > 0 {
			if len(o.Package.PostInstalls) == 0 {
				o.Package.PostInstalls = make([]installer.CmdWithArgs, 0)
			}
			for i := range o.Package.Requirements {
				tool := o.Package.Requirements[i]
				o.Package.PostInstalls = append(o.Package.PostInstalls, installer.CmdWithArgs{
					Cmd:  "hd",
					Args: []string{"install", tool},
				})
			}
		}
	}
	err = process.Install()
	return
}

func (o *installOption) installFromSource() (err error) {
	if !o.Package.FromSource {
		err = fmt.Errorf("not support install it from source")
		return
	}

	if o.Provider != "github" {
		err = fmt.Errorf("only support github.com")
		return
	}

	gopath := sysos.Getenv("GOPATH")
	if gopath == "" {
		err = fmt.Errorf("GOPATH is required")
		return
	}

	if o.org == "" || o.repo == "" {
		err = fmt.Errorf("org: '%s' or repo: '%s' is empty", o.org, o.repo)
		return
	}

	if err = exec.RunCommandInDir("go", sysos.TempDir(), strings.Split(o.buildGoInstallCmd(), " ")[1:]...); err != nil {
		err = fmt.Errorf("faield to run go install command, error: %v", err)
		return
	}

	sourcePath := path.Join(gopath, fmt.Sprintf("bin/%s", o.name))
	if common.Exist(sourcePath) {
		is := &installer.Installer{}
		targetName := o.name
		if o.Package != nil && o.Package.TargetBinary != "" {
			targetName = o.Package.TargetBinary
		}
		err = is.OverWriteBinary(sourcePath, fmt.Sprintf("/usr/local/bin/%s", targetName))
	} else {
		err = fmt.Errorf("no found %s from GOPATH", o.name)
	}
	return
}

func (o *installOption) buildGoInstallCmd() string {
	return fmt.Sprintf("go install github.com/%s/%s@%s", o.org, o.repo, o.fromBranch)
}
