package installer

import (
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg/common"
	"github.com/linuxsuren/http-downloader/pkg/compress"
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// Install installs a package
func (o *Installer) Install() (err error) {
	targetBinary := o.Name
	if o.Package != nil && o.Package.TargetBinary != "" {
		// this is the desired binary file
		targetBinary = o.Package.TargetBinary
	}

	var source string
	var target string
	tarFile := o.Output
	if o.Tar {
		if err = o.extractFiles(tarFile, o.Name); err == nil {
			source = fmt.Sprintf("%s/%s", filepath.Dir(tarFile), o.Name)
			target = fmt.Sprintf("/usr/local/bin/%s", targetBinary)
		} else {
			err = fmt.Errorf("cannot extract %s from tar file, error: %v", tarFile, err)
		}
	} else {
		source = o.Source
		target = fmt.Sprintf("/usr/local/bin/%s", targetBinary)
	}

	if err == nil {
		if o.Package != nil && o.Package.PreInstalls != nil {
			if err = runCommandList(o.Package.PreInstalls); err != nil {
				return
			}
		}

		if o.Package != nil && o.Package.Installation != nil {
			err = exec.RunCommand(o.Package.Installation.Cmd, o.Package.Installation.Args...)
		} else {
			if err = o.OverWriteBinary(source, target); err != nil {
				return
			}

			for i := range o.AdditionBinaries {
				addition := o.AdditionBinaries[i]
				if err = o.OverWriteBinary(addition, fmt.Sprintf("/usr/local/bin/%s", filepath.Base(addition))); err != nil {
					return
				}
			}
		}

		if err == nil && o.Package != nil && o.Package.PostInstalls != nil {
			err = runCommandList(o.Package.PostInstalls)
		}

		if err == nil && o.Package != nil && o.Package.TestInstalls != nil {
			err = runCommandList(o.Package.TestInstalls)
		}

		if err == nil && o.CleanPackage {
			if cleanErr := os.RemoveAll(tarFile); cleanErr != nil {
				fmt.Println("cannot remove file", tarFile, ", error:", cleanErr)
			}
		}
	}
	return
}

// OverWriteBinary install a binrary file
func (o *Installer) OverWriteBinary(sourceFile, targetPath string) (err error) {
	fmt.Println("install", sourceFile, "to", targetPath)
	switch runtime.GOOS {
	case "linux", "darwin":
		if err = exec.RunCommand("chmod", "u+x", sourceFile); err != nil {
			return
		}

		if common.IsDirWriteable(path.Dir(targetPath)) != nil {
			if err = exec.RunCommandWithSudo("rm", "-rf", targetPath); err != nil {
				return
			}
		} else {
			if err = exec.RunCommand("rm", "-rf", targetPath); err != nil {
				return
			}
		}

		if common.IsDirWriteable(path.Dir(targetPath)) != nil {
			err = exec.RunCommandWithSudo("mv", sourceFile, targetPath)
		} else {
			err = exec.RunCommand("mv", sourceFile, targetPath)
		}
	default:
		sourceF, _ := os.Open(sourceFile)
		targetF, _ := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR, 0600)
		if _, err = io.Copy(targetF, sourceF); err != nil {
			err = fmt.Errorf("cannot copy %s from %s to %v, error: %v", o.Name, sourceFile, targetPath, err)
		}

		if err == nil {
			_ = os.RemoveAll(sourceFile)
		}
	}
	return
}

func (o *Installer) extractFiles(tarFile, targetName string) (err error) {
	// TODO choose a correct compress instance
	extension := strings.Join(strings.Split(tarFile, ".")[1:], "")
	if extension == "tar.xz" {
		compressor := compress.NewXz()
		err = compressor.ExtractFiles(tarFile, targetName)
	} else {
		compressor := compress.NewGZip(o.AdditionBinaries)
		err = compressor.ExtractFiles(tarFile, targetName)
	}
	return
}
