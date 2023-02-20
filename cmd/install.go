package cmd

import (
	"context"
	"fmt"
	"log"
	sysos "os"
	"path"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/linuxsuren/http-downloader/pkg/common"
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/linuxsuren/http-downloader/pkg/os"
	"github.com/linuxsuren/http-downloader/pkg/version"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// newInstallCmd returns the install command
func newInstallCmd(ctx context.Context) (cmd *cobra.Command) {
	opt := &installOption{
		downloadOption: newDownloadOption(ctx),
		execer:         &exec.DefaultExecer{},
	}
	cmd = &cobra.Command{
		Use:     "install",
		Aliases: []string{"i", "add"},
		Short:   "Install a package from https://github.com/LinuxSuRen/hd-home",
		Long: `Install a package from https://github.com/LinuxSuRen/hd-home
Cannot find your desired package? Please run command: hd fetch --reset, then try it again`,
		Example: "hd install goget",
		PreRunE: opt.preRunE,
		RunE:    opt.runE,
	}

	flags := cmd.Flags()
	opt.addFlags(flags)
	opt.addPlatformFlags(flags)
	flags.StringVarP(&opt.Category, "category", "c", "",
		"The category of the potentials packages")
	flags.BoolVarP(&opt.ShowProgress, "show-progress", "", true, "If show the progress of download")
	flags.BoolVarP(&opt.AcceptPreRelease, "accept-preRelease", "", false,
		"If you accept preRelease as the binary asset from GitHub")
	flags.BoolVarP(&opt.AcceptPreRelease, "pre", "", false,
		"Same with option --accept-preRelease")
	flags.BoolVarP(&opt.fromSource, "from-source", "", false,
		"Indicate if install it via go install github.com/xxx/xxx")
	flags.StringVarP(&opt.fromBranch, "from-branch", "", "master",
		"Only works if the flag --from-source is true")
	flags.StringVarP(&opt.target, "target", "", "/usr/local/bin", "The target installation directory")
	flags.BoolVarP(&opt.goget, "goget", "", viper.GetBool("fetch"),
		"Use command goget to download the binary, only works if the flag --from-source is true")

	flags.BoolVarP(&opt.Download, "download", "", true,
		"If download the package")
	flags.BoolVarP(&opt.force, "force", "f", false,
		"Indicate if force to download the package even it is exist")
	flags.BoolVarP(&opt.CleanPackage, "clean-package", "", true,
		"Clean the package if the installation is success")
	flags.IntVarP(&opt.Thread, "thread", "t", viper.GetInt("thread"),
		`Download file with multi-threads. It only works when its value is bigger than 1`)
	flags.BoolVarP(&opt.NoProxy, "no-proxy", "", viper.GetBool("no-proxy"), "Indicate no HTTP proxy taken")
	flags.BoolVarP(&opt.KeepPart, "keep-part", "", false,
		"If you want to keep the part files instead of deleting them")

	_ = cmd.RegisterFlagCompletionFunc("provider", ArrayCompletion(ProviderGitHub, ProviderGitee))
	return
}

type installOption struct {
	*downloadOption
	Download     bool
	CleanPackage bool
	fromSource   bool
	fromBranch   string
	target       string
	goget        bool
	force        bool

	// inner fields
	nativePackage bool
	tool          string
	execer        exec.Execer
}

func (o *installOption) shouldInstall() (should, exist bool) {
	var greater bool
	if name, lookErr := o.execer.LookPath(o.tool); lookErr == nil {
		exist = true

		var versionCmd string
		if o.downloadOption != nil && o.downloadOption.Package != nil && o.downloadOption.Package.VersionCmd != "" {
			versionCmd = o.downloadOption.Package.VersionCmd
		}

		if versionCmd != "" {
			log.Println("check target version via", name, versionCmd)
			if data, err := o.execer.Command(name, versionCmd); err == nil &&
				(o.downloadOption.Package != nil && o.downloadOption.Package.Version != "") {
				greater = version.GreatThan(o.downloadOption.Package.Version, string(data))
			}
		}
	}
	should = o.force || !exist || greater
	return
}

func (o *installOption) preRunE(cmd *cobra.Command, args []string) (err error) {
	if len(args) > 0 {
		o.tool = args[0]
	}

	if o.tool == "" && o.Category == "" {
		err = fmt.Errorf("tool or category name is requried")
		return
	}

	// try to find if it's a native package
	o.nativePackage = os.HasPackage(o.tool)
	if !o.nativePackage {
		if o.Category == "" {
			err = o.downloadOption.preRunE(cmd, args)

			if o.downloadOption.Package != nil {
				// try to find the real tool name
				if o.downloadOption.Package.TargetBinary != "" {
					o.tool = o.downloadOption.Package.TargetBinary
				} else if o.downloadOption.Package.Binary != "" {
					o.tool = o.downloadOption.Package.Binary
				} else {
					o.tool = o.downloadOption.Package.Repo
				}
			}
		} else {
			err = o.downloadOption.fetch()
		}
	}
	return
}

func (o *installOption) install(cmd *cobra.Command, args []string) (err error) {
	if o.nativePackage {
		// install a package
		if should, exist := o.shouldInstall(); !should {
			if exist {
				cmd.Printf("%s is already exist, please use the flag --force if you install it again\n", o.tool)
			}
			return
		}

		var proxy map[string]string
		if o.ProxyGitHub != "" {
			proxy = map[string]string{
				"https://raw.githubusercontent.com": fmt.Sprintf("https://%s/https://raw.githubusercontent.com", o.ProxyGitHub),
				"https://github.com":                fmt.Sprintf("https://%s/https://github.com", o.ProxyGitHub),
			}
		}
		err = os.InstallWithProxy(args[0], proxy)
		return
	}

	// aka go get github.com/xxx/xxx
	if o.fromSource {
		err = o.installFromSource()
		return
	}

	// install a package from hd-home
	if o.Download {
		log.Println("check if it should be installed")
		if should, exist := o.shouldInstall(); !should {
			if exist {
				cmd.Printf("%s is already exist, please use the flag --force if you install it again\n", o.tool)
				return
			}
		}

		if err = o.downloadOption.runE(cmd, args); err != nil {
			return
		}
	}

	if o.Package == nil {
		o.Package = &installer.HDConfig{}
	}
	if o.target != "" && o.Package.TargetDirectory == "" {
		o.Package.TargetDirectory = o.target
	}
	if o.Package.TargetDirectory == "" {
		o.Package.TargetDirectory = "/usr/local/bin"
	}
	if err = sysos.MkdirAll(o.Package.TargetDirectory, 0750); err != nil {
		return
	}

	log.Println("target directory", o.Package.TargetDirectory)
	process := &installer.Installer{
		Source:           o.downloadOption.Output,
		Name:             o.name,
		Package:          o.Package,
		Tar:              o.Tar,
		Output:           o.Output,
		CleanPackage:     o.CleanPackage,
		AdditionBinaries: o.Package.AdditionBinaries,
		TargetDirectory:  o.Package.TargetDirectory,
		Execer:           o.execer,
	}
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
	err = process.Install()
	return
}

func (o *installOption) runE(cmd *cobra.Command, args []string) (err error) {
	if o.Category != "" {
		packages := installer.FindPackagesByCategory(o.Category)
		orgAndRepos := make([]string, len(packages))
		for i := range packages {
			orgAndRepos[i] = fmt.Sprintf("%s/%s", packages[i].Org, packages[i].Repo)
		}
		if len(orgAndRepos) == 0 {
			err = fmt.Errorf("cannot find any tools by category: %s", o.Category)
			return
		}

		selector := &survey.MultiSelect{
			Message: "Select packages",
			Options: orgAndRepos,
		}

		var choose []string
		if err = survey.AskOne(selector, &choose); err != nil {
			return
		}

		for _, item := range choose {
			if err = o.downloadOption.preRunE(cmd, []string{item}); err != nil {
				return
			}

			// try to find the real tool name
			if o.downloadOption.Package.TargetBinary != "" {
				o.tool = o.downloadOption.Package.TargetBinary
			} else if o.downloadOption.Package.Binary != "" {
				o.tool = o.downloadOption.Package.Binary
			} else {
				o.tool = o.downloadOption.Package.Repo
			}

			if err = o.install(cmd, []string{item}); err != nil {
				return
			}
			o.Output = "" // TODO this field must be set to be empty for the next round, need a better solution here
		}
	} else {
		err = o.install(cmd, args)
	}
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

	if o.org == "" || o.repo == "" {
		err = fmt.Errorf("org: '%s' or repo: '%s' is empty", o.org, o.repo)
		return
	}

	var binaryPath string
	if o.goget {
		binaryPath, err = o.runGogetCommand(fmt.Sprintf("github.com/%s/%s", o.org, o.repo), o.repo)
	} else {
		binaryPath, err = o.buildGoSource()
	}

	if err == nil && binaryPath != "" {
		is := &installer.Installer{}
		targetName := o.name
		if o.Package != nil && o.Package.TargetBinary != "" {
			targetName = o.Package.TargetBinary
		}
		err = is.OverWriteBinary(binaryPath, path.Join(o.Package.TargetDirectory, targetName))
	}
	return
}

func (o *installOption) buildGoSource() (binaryPath string, err error) {
	gopath := sysos.Getenv("GOPATH")
	if gopath == "" {
		err = fmt.Errorf("GOPATH is required")
		return
	}

	if err = o.execer.RunCommandInDir("go", sysos.TempDir(), strings.Split(o.buildGoInstallCmd(), " ")[1:]...); err != nil {
		err = fmt.Errorf("faield to run go install command, error: %v", err)
		return
	}

	binaryPath = path.Join(gopath, fmt.Sprintf("bin/%s", o.name))
	if !common.Exist(binaryPath) {
		err = fmt.Errorf("no found %s from GOPATH", o.name)
	}
	return
}

func (o *installOption) buildGoInstallCmd() string {
	return fmt.Sprintf("go install github.com/%s/%s@%s", o.org, o.repo, o.fromBranch)
}

func (o *installOption) runGogetCommand(repo, name string) (binaryPath string, err error) {
	// make sure goget command exists
	is := installer.Installer{
		Provider: "github",
	}
	if err = is.CheckDepAndInstall(map[string]string{
		"goget": "linuxsuren/goget",
	}); err != nil {
		err = fmt.Errorf("cannot download goget, error: %v", err)
		return
	}

	// run goget command
	tmpPath := sysos.TempDir()
	binaryPath = path.Join(tmpPath, name)
	if err = o.execer.RunCommandInDir("goget", tmpPath, repo); err != nil {
		err = fmt.Errorf("faield to run go install command, error: %v", err)
	}
	return
}
