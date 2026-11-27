package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	sysos "os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	fakeruntime "github.com/linuxsuren/go-fake-runtime"
	"github.com/linuxsuren/http-downloader/pkg/common"
	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/linuxsuren/http-downloader/pkg/os"
	npminstaller "github.com/linuxsuren/http-downloader/pkg/os/npm"
	"github.com/linuxsuren/http-downloader/pkg/remote"
	"github.com/linuxsuren/http-downloader/pkg/source"
	"github.com/linuxsuren/http-downloader/pkg/version"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// newInstallCmd returns the install command
func newInstallCmd(ctx context.Context) (cmd *cobra.Command) {
	opt := &installOption{
		downloadOption: newDownloadOption(ctx),
		execer:         fakeruntime.NewDefaultExecer(),
		sourceName:     source.HDHome,
		targetConfig:   loadRemoteConfig(),
	}
	cmd = &cobra.Command{
		Use:     "install",
		Aliases: []string{"i", "add"},
		Short:   "Install a package from hd-home, npm or a native package manager",
		Long: `Install a package from hd-home, npm or a native package manager.
Use --mode ssh to install to a remote server, and --source npm to install an npm package.
Cannot find your desired package? Please run command: hd fetch --reset, then try it again`,
		Example: "hd install goget\nhd install --source npm typescript\nhd install --mode ssh --ssh-host 10.0.0.1 jcli",
		PreRunE: opt.preRunE,
		RunE:    opt.runE,
		GroupID: coreGroup.ID,
	}

	flags := cmd.Flags()
	opt.addFlags(flags)
	opt.addPlatformFlags(flags)
	opt.addDownloadFlags(flags)
	flags.StringVarP(&opt.Category, "category", "c", "",
		"The category of the potentials packages")
	flags.BoolVarP(&opt.AcceptPreRelease, "accept-preRelease", "", false,
		"If you accept preRelease as the binary asset from GitHub")
	flags.BoolVarP(&opt.AcceptPreRelease, "pre", "", false,
		"Same with option --accept-preRelease")
	flags.BoolVarP(&opt.fromSource, "from-source", "", false,
		"Indicate if install it via go install github.com/xxx/xxx")
	flags.StringVarP(&opt.fromBranch, "from-branch", "", "master",
		"Only works if the flag --from-source is true")
	target := configuredInstallDir()
	if target == "" {
		target = opt.getDefaultInstallDir()
	}
	flags.StringVarP(&opt.target, "target", "", target, "The target installation directory")
	flags.BoolVarP(&opt.goget, "goget", "", viper.GetBool("fetch"),
		"Use command goget to download the binary, only works if the flag --from-source is true")

	flags.BoolVarP(&opt.Download, "download", "", true,
		"If download the package")
	flags.BoolVarP(&opt.force, "force", "f", false,
		"Indicate if force to download the package even it is exist")
	flags.BoolVarP(&opt.CleanPackage, "clean-package", "", true,
		"Clean the package if the installation is success")
	flags.BoolVarP(&opt.KeepPart, "keep-part", "", false,
		"If you want to keep the part files instead of deleting them")

	flags.StringVar(&opt.sourceName, "source", source.HDHome,
		"The package source: hd-home, npm, native or all")
	flags.StringVar(&opt.targetConfig.Mode, "mode", opt.targetConfig.Mode,
		"The install mode: local or ssh")
	flags.StringVar(&opt.targetConfig.Host, "ssh-host", opt.targetConfig.Host,
		"The host of the target server, only works with --mode ssh")
	flags.IntVar(&opt.targetConfig.Port, "ssh-port", opt.targetConfig.Port,
		"The SSH port of the target server")
	flags.StringVar(&opt.targetConfig.User, "ssh-user", opt.targetConfig.User,
		"The SSH user of the target server")
	flags.StringVar(&opt.targetConfig.Password, "ssh-password", opt.targetConfig.Password,
		"The SSH password, a private key is recommended instead")
	flags.StringVar(&opt.targetConfig.KeyFile, "ssh-key", opt.targetConfig.KeyFile,
		"The SSH private key file")
	flags.BoolVar(&opt.targetConfig.Insecure, "ssh-insecure", opt.targetConfig.Insecure,
		"Skip the SSH host key verification")
	flags.StringVar(&opt.targetConfig.OS, "ssh-os", opt.targetConfig.OS,
		"The OS of the SSH target, it selects the release asset")
	flags.StringVar(&opt.targetConfig.Arch, "ssh-arch", opt.targetConfig.Arch,
		"The architecture of the SSH target, it selects the release asset")
	return
}

func (o *installOption) getDefaultInstallDir() string {
	return platformInstallDir(o.execer.OS(), o.execer.Getenv("SHELL"))
}

func (o *installOption) targetOS() string {
	if o.targetConfig.Mode == remote.ModeSSH {
		return o.targetConfig.Normalize().OS
	}
	if o.execer != nil {
		return o.execer.OS()
	}
	return runtime.GOOS
}

// resolveTargetDir keeps the CLI flag, the package config and the configured
// default in sync without ever falling back to a Unix path on Windows
func (o *installOption) resolveTargetDir() string {
	pkgDir := ""
	if o.Package != nil {
		pkgDir = o.Package.TargetDirectory
	}
	return installer.ResolveInstallDir(o.target, pkgDir, o.targetOS())
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

	sourceName   string
	targetConfig remote.Config
	npmPackage   string
	npmVersion   string

	// inner fields
	nativePackage bool
	tool          string
	execer        fakeruntime.Execer
	remoteExec    remote.Executor
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

	o.targetConfig = o.targetConfig.Normalize()
	if o.targetConfig.Mode != remote.ModeLocal && o.targetConfig.Mode != remote.ModeSSH {
		err = fmt.Errorf("invalid install mode: %s, expected local or ssh", o.targetConfig.Mode)
		return
	}
	if o.targetConfig.Mode == remote.ModeSSH {
		if err = o.targetConfig.Validate(); err != nil {
			return
		}
		// the flag default follows the local platform, a remote target uses
		// its own platform unless --target was given explicitly
		if cmd == nil || cmd.Flags() == nil || !cmd.Flags().Changed("target") {
			o.target = configuredInstallDir()
		}
		if cmd == nil || cmd.Flags() == nil || !cmd.Flags().Changed("os") {
			o.OS = o.targetConfig.OS
		}
		if cmd == nil || cmd.Flags() == nil || !cmd.Flags().Changed("arch") {
			o.Arch = o.targetConfig.Arch
		}
	}

	switch o.sourceName {
	case "":
		o.sourceName = source.HDHome
	case source.NPM:
		o.npmPackage, o.npmVersion = splitPackageVersion(o.tool)
		o.tool = o.npmPackage
		return
	case source.HDHome, source.Native, source.All:
	default:
		err = fmt.Errorf("unknown package source: %s", o.sourceName)
		return
	}

	// native packages are only detected on the local machine, a remote
	// target gets the same binary installation flow as a local one. An
	// explicit --source native does not need a config cache lookup.
	o.nativePackage = o.targetConfig.Mode == remote.ModeLocal &&
		(o.sourceName == source.Native || os.HasPackage(o.tool))
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

// splitPackageVersion supports both plain and scoped npm package names
func splitPackageVersion(name string) (pkg, version string) {
	if index := strings.LastIndex(name, "@"); index > 0 {
		return name[:index], name[index+1:]
	}
	return name, ""
}

func (o *installOption) install(cmd *cobra.Command, args []string) (err error) {
	if o.sourceName == source.NPM {
		if err = o.installNPM(); err != nil {
			return
		}
		return o.recordNPMInstall()
	}

	if o.targetConfig.Mode == remote.ModeSSH {
		if err = o.installRemote(cmd, args); err != nil {
			return
		}
		return o.recordInstall(false)
	}

	if o.Package == nil {
		o.Package = &installer.HDConfig{}
	}
	o.Package.TargetDirectory = o.resolveTargetDir()

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
		if err = os.InstallWithProxy(args[0], proxy); err == nil {
			err = o.recordInstall(true)
		}
		return
	}

	// aka go get github.com/xxx/xxx
	if o.fromSource {
		if err = sysos.MkdirAll(o.Package.TargetDirectory, 0750); err != nil {
			return
		}
		if err = o.installFromSource(); err == nil {
			err = o.recordInstall(false)
		}
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
	if err = process.Install(); err == nil {
		err = o.recordInstall(false)
	}
	return
}

// installNPM uses the npm installer either locally or on the SSH target
func (o *installOption) installNPM() (err error) {
	name := o.npmPackage
	if o.npmVersion != "" {
		name = fmt.Sprintf("%s@%s", name, o.npmVersion)
	}
	if o.targetConfig.Mode == remote.ModeSSH {
		var exec remote.Executor
		if exec, err = o.getRemoteExecutor(); err != nil {
			return
		}
		log.Println("install npm package on", o.targetConfig.Host, name)
		err = exec.Run("npm install -g "+remote.Quote(name), nil, sysos.Stdout, sysos.Stderr)
		return
	}
	installer := &npminstaller.CommonInstaller{Name: name, Execer: o.execer}
	err = installer.Install()
	return
}

// installRemote downloads and extracts the package locally, then uploads
// only the resulting binaries to the target server over SSH
func (o *installOption) installRemote(cmd *cobra.Command, args []string) (err error) {
	exec, err := o.getRemoteExecutor()
	if err != nil {
		return
	}

	targetDir := installer.ResolveInstallDir(o.target, "", o.targetConfig.Normalize().OS)
	if err = exec.Run("mkdir -p "+remote.Quote(targetDir), nil, sysos.Stdout, sysos.Stderr); err != nil {
		return
	}

	if o.Download {
		log.Println("check if it should be installed on", o.targetConfig.Host)
		if should, exist := o.shouldInstallRemote(exec, targetDir); !should {
			if exist {
				cmd.Printf("%s already exists on %s, please use --force to install it again\n", o.tool, o.targetConfig.Host)
			}
			return
		}
		if err = o.downloadOption.runE(cmd, args); err != nil {
			return
		}
	}

	if o.Package == nil {
		o.Package = &installer.HDConfig{}
	}
	targetBinary := o.name
	if o.Package.TargetBinary != "" {
		targetBinary = o.Package.TargetBinary
	}

	stageDir, err := sysos.MkdirTemp("", "hd-remote-")
	if err != nil {
		return
	}
	defer func() { _ = sysos.RemoveAll(stageDir) }()

	var binaries []string
	if o.Tar {
		binaries, err = installer.StagePackage(o.Output, stageDir, targetBinary, o.Package.AdditionBinaries)
	} else {
		var data []byte
		if data, err = sysos.ReadFile(o.downloadOption.Output); err == nil {
			staged := filepath.Join(stageDir, targetBinary)
			if err = sysos.WriteFile(staged, data, 0750); err == nil {
				binaries = append(binaries, staged)
			}
		}
	}
	if err != nil {
		return
	}

	for _, binary := range binaries {
		remotePath := path.Join(targetDir, filepath.Base(binary))
		if err = o.uploadBinary(exec, binary, remotePath); err != nil {
			return
		}
	}

	if err = o.runRemoteHooks(exec); err != nil {
		return
	}
	o.Package.TargetDirectory = targetDir
	return
}

func (o *installOption) getRemoteExecutor() (remote.Executor, error) {
	if o.remoteExec != nil {
		return o.remoteExec, nil
	}
	o.targetConfig = o.targetConfig.Normalize()
	if err := o.targetConfig.Validate(); err != nil {
		return nil, err
	}
	exec, err := remote.NewSSHExecutor(context.Background(), o.targetConfig)
	if err != nil {
		return nil, err
	}
	o.remoteExec = exec
	return exec, nil
}

func (o *installOption) shouldInstallRemote(exec remote.Executor, targetDir string) (should, exist bool) {
	binary := o.tool
	if o.Package != nil && o.Package.TargetBinary != "" {
		binary = o.Package.TargetBinary
	}
	if err := exec.Run("test -x "+remote.Quote(path.Join(targetDir, binary)), nil, io.Discard, io.Discard); err == nil {
		exist = true
	}
	should = o.force || !exist
	return
}

func (o *installOption) uploadBinary(exec remote.Executor, localPath, remotePath string) (err error) {
	fmt.Println("install", localPath, "to", o.targetConfig.Host+":"+remotePath)
	if err = exec.Run("mkdir -p "+remote.Quote(path.Dir(remotePath)), nil, sysos.Stdout, sysos.Stderr); err != nil {
		return
	}
	if err = exec.Run("rm -f "+remote.Quote(remotePath), nil, sysos.Stdout, sysos.Stderr); err != nil {
		return
	}
	if err = exec.Copy(localPath, remotePath); err != nil {
		return
	}
	err = exec.Run("chmod u+x "+remote.Quote(remotePath), nil, sysos.Stdout, sysos.Stderr)
	return
}

// remoteCmd joins a configured command with shell quoted arguments
func remoteCmd(cmd installer.CmdWithArgs) string {
	parts := []string{remote.Quote(cmd.Cmd)}
	for _, arg := range cmd.Args {
		parts = append(parts, remote.Quote(arg))
	}
	return strings.Join(parts, " ")
}

func (o *installOption) runRemoteHooks(exec remote.Executor) (err error) {
	cfg := o.Package
	if cfg == nil {
		return
	}
	for _, cmds := range [][]installer.CmdWithArgs{cfg.PreInstalls, cfg.PostInstalls, cfg.TestInstalls} {
		for i := range cmds {
			if cmds[i].Cmd == "" {
				continue
			}
			if err = exec.Run(remoteCmd(cmds[i]), nil, sysos.Stdout, sysos.Stderr); err != nil {
				return
			}
		}
	}
	if cfg.Installation != nil && cfg.Installation.Cmd != "" {
		if err = exec.Run(remoteCmd(*cfg.Installation), nil, sysos.Stdout, sysos.Stderr); err != nil {
			return
		}
	}
	for _, file := range cfg.DefaultConfigFile {
		if file.OS != "" && file.OS != o.targetOS() {
			continue
		}
		parent := path.Dir(file.Path)
		if err = exec.Run("mkdir -p "+remote.Quote(parent)+" && cat > "+remote.Quote(file.Path),
			strings.NewReader(file.Content), sysos.Stdout, sysos.Stderr); err != nil {
			return
		}
	}
	return
}

// recordInstall saves the installed tool into the record file
func (o *installOption) recordInstall(native bool) (err error) {
	targetDir := o.resolveTargetDir()
	pkg := o.Package
	if pkg == nil {
		pkg = &installer.HDConfig{}
	}

	targetBinary := o.tool
	if pkg.TargetBinary != "" {
		targetBinary = pkg.TargetBinary
	}

	binaries := []string{path.Join(targetDir, targetBinary)}
	for _, addition := range pkg.AdditionBinaries {
		binaries = append(binaries, path.Join(targetDir, filepath.Base(addition)))
	}

	installedVersion := pkg.LatestVersion
	if installedVersion == "" {
		installedVersion = pkg.Version
	}

	sourceName := o.sourceName
	if sourceName == source.All {
		sourceName = source.HDHome
	}
	record := installer.InstalledPackage{
		Name:            o.tool,
		Org:             o.org,
		Repo:            o.repo,
		Version:         installedVersion,
		Binaries:        binaries,
		TargetDirectory: targetDir,
		InstalledAt:     time.Now().Format(time.RFC3339),
		FromSource:      o.fromSource,
		Native:          native,
		Source:          installer.SourceOfInstalled(sourceName),
	}
	if o.targetConfig.Mode == remote.ModeSSH {
		record.Mode = remote.ModeSSH
		record.Host = o.targetConfig.Host
	}
	return installer.RecordInstalledPackage(record)
}

func (o *installOption) recordNPMInstall() (err error) {
	record := installer.InstalledPackage{
		Name:        o.npmPackage,
		Version:     o.npmVersion,
		Source:      source.NPM,
		InstalledAt: time.Now().Format(time.RFC3339),
	}
	if o.targetConfig.Mode == remote.ModeSSH {
		record.Mode = remote.ModeSSH
		record.Host = o.targetConfig.Host
	}
	return installer.RecordInstalledPackage(record)
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
