package cmd

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

// NewInstallCmd returns the install command
func NewInstallCmd() (cmd *cobra.Command) {
	opt := &installOption{}
	cmd = &cobra.Command{
		Use:     "install",
		Short:   "Install a package from https://github.com/LinuxSuRen/hd-home",
		Example: "hd install jenkins-zh/jenkins-cli/jcli -t 6",
		PreRunE: opt.preRunE,
		RunE:    opt.runE,
	}

	flags := cmd.Flags()
	//flags.StringVarP(&opt.Mode, "mode", "m", "package",
	//	"If you want to install it via platform package manager")
	flags.BoolVarP(&opt.ShowProgress, "show-progress", "", true, "If show the progress of download")
	flags.BoolVarP(&opt.Fetch, "fetch", "", true,
		"If fetch the latest config from https://github.com/LinuxSuRen/hd-home")
	flags.BoolVarP(&opt.Download, "download", "", true,
		"If download the package")
	flags.IntVarP(&opt.Thread, "thread", "t", 4,
		`Download file with multi-threads. It only works when its value is bigger than 1`)
	flags.BoolVarP(&opt.KeepPart, "keep-part", "", false,
		"If you want to keep the part files instead of deleting them")
	flags.StringVarP(&opt.Provider, "provider", "", ProviderGitHub, "The file provider")
	flags.StringVarP(&opt.OS, "os", "", runtime.GOOS, "The OS of target binary file")
	flags.StringVarP(&opt.Arch, "arch", "", runtime.GOARCH, "The arch of target binary file")
	return
}

type installOption struct {
	downloadOption
	Download bool
	Mode     string
}

func (o *installOption) preRunE(cmd *cobra.Command, args []string) (err error) {
	err = o.downloadOption.preRunE(cmd, args)
	return
}

func (o *installOption) runE(cmd *cobra.Command, args []string) (err error) {
	if o.Download {
		if err = o.downloadOption.runE(cmd, args); err != nil {
			return
		}
	}

	targetBinary := o.name
	if o.Package != nil && o.Package.TargetBinary != "" {
		// this is the desired binary file
		targetBinary = o.Package.TargetBinary
	}

	var source string
	var target string
	if o.Tar {
		if err = o.extractFiles(o.Output, o.name); err == nil {
			source = fmt.Sprintf("%s/%s", filepath.Dir(o.Output), o.name)
			target = fmt.Sprintf("/usr/local/bin/%s", targetBinary)
		} else {
			err = fmt.Errorf("cannot extract %s from tar file, error: %v", o.Output, err)
		}
	} else {
		source = o.downloadOption.Output
		target = fmt.Sprintf("/usr/local/bin/%s", targetBinary)
	}

	if err == nil {
		if o.Package != nil && o.Package.PreInstall != nil {
			if err = execCommand(o.Package.PreInstall.Cmd, o.Package.PreInstall.Args...); err != nil {
				return
			}
		}

		if o.Package != nil && o.Package.Installation != nil {
			err = execCommand(o.Package.Installation.Cmd, o.Package.Installation.Args...)
		} else {
			err = o.overWriteBinary(source, target)
		}

		if err == nil && o.Package != nil && o.Package.PostInstall != nil {
			err = execCommand(o.Package.PostInstall.Cmd, o.Package.PostInstall.Args...)
		}

		if err == nil && o.Package != nil && o.Package.TestInstall != nil {
			err = execCommand(o.Package.TestInstall.Cmd, o.Package.TestInstall.Args...)
		}
	}
	return
}

func (o *installOption) overWriteBinary(sourceFile, targetPath string) (err error) {
	fmt.Println("install", sourceFile, "to", targetPath)
	switch runtime.GOOS {
	case "linux", "darwin":
		if err = execCommand("chmod", "u+x", sourceFile); err != nil {
			return
		}

		if err = execCommand("rm", "-rf", targetPath); err != nil {
			return
		}

		var cp string
		if cp, err = exec.LookPath("cp"); err == nil {
			err = syscall.Exec(cp, []string{"cp", sourceFile, targetPath}, os.Environ())
		}
	default:
		sourceF, _ := os.Open(sourceFile)
		targetF, _ := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR, 0600)
		if _, err = io.Copy(targetF, sourceF); err != nil {
			err = fmt.Errorf("cannot copy %s from %s to %v, error: %v", o.name, sourceFile, targetPath, err)
		}
	}
	return
}

func (o *installOption) extractFiles(tarFile, targetName string) (err error) {
	var f *os.File
	var gzf *gzip.Reader
	if f, err = os.Open(tarFile); err != nil {
		return
	}
	defer func() {
		_ = f.Close()
	}()

	if gzf, err = gzip.NewReader(f); err != nil {
		return
	}

	tarReader := tar.NewReader(gzf)
	var header *tar.Header
	var found bool
	for {
		if header, err = tarReader.Next(); err == io.EOF {
			err = nil
			break
		} else if err != nil {
			break
		}
		name := header.Name

		switch header.Typeflag {
		case tar.TypeReg:
			if name != targetName && !strings.HasSuffix(name, "/"+targetName) {
				continue
			}
			var targetFile *os.File
			if targetFile, err = os.OpenFile(fmt.Sprintf("%s/%s", filepath.Dir(tarFile), targetName),
				os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode)); err != nil {
				break
			}
			if _, err = io.Copy(targetFile, tarReader); err != nil {
				break
			}
			found = true
			_ = targetFile.Close()
		}
	}

	if err == nil && !found {
		err = fmt.Errorf("cannot found item '%s' from '%s'", targetName, tarFile)
	}
	return
}
