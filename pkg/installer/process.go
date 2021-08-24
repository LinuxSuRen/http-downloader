package installer

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg/common"
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
		if o.Package != nil && o.Package.PreInstall != nil {
			if err = exec.RunCommand(o.Package.PreInstall.Cmd, o.Package.PreInstall.Args...); err != nil {
				return
			}
		}

		if o.Package != nil && o.Package.Installation != nil {
			err = exec.RunCommand(o.Package.Installation.Cmd, o.Package.Installation.Args...)
		} else {
			if err = o.overWriteBinary(source, target); err != nil {
				return
			}

			for i := range o.AdditionBinaries {
				addition := o.AdditionBinaries[i]
				if err = o.overWriteBinary(addition, fmt.Sprintf("/usr/local/bin/%s", filepath.Base(addition))); err != nil {
					return
				}
			}
		}

		if err == nil && o.Package != nil && o.Package.PostInstall != nil {
			err = exec.RunCommand(o.Package.PostInstall.Cmd, o.Package.PostInstall.Args...)
		}

		if err == nil && o.Package != nil && o.Package.TestInstall != nil {
			err = exec.RunCommand(o.Package.TestInstall.Cmd, o.Package.TestInstall.Args...)
		}

		if err == nil && o.CleanPackage {
			if cleanErr := os.RemoveAll(tarFile); cleanErr != nil {
				fmt.Println("cannot remove file", tarFile, ", error:", cleanErr)
			}
		}
	}
	return
}

func (o *Installer) overWriteBinary(sourceFile, targetPath string) (err error) {
	fmt.Println("install", sourceFile, "to", targetPath)
	switch runtime.GOOS {
	case "linux", "darwin":
		if err = exec.RunCommand("chmod", "u+x", sourceFile); err != nil {
			return
		}

		if err = exec.RunCommand("rm", "-rf", targetPath); err != nil {
			return
		}

		if common.IsDirWriteable(path.Dir(targetPath)) != nil {
			err = exec.RunCommand("sudo", "mv", sourceFile, targetPath)
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
	if targetName == "" {
		err = errors.New("target filename is empty")
		return
	}

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
			if err = extraFile(name, targetName, tarFile, header, tarReader); err == nil {
				found = true
			} else {
				break
			}

			for i := range o.AdditionBinaries {
				addition := o.AdditionBinaries[i]
				if err = extraFile(addition, addition, tarFile, header, tarReader); err != nil {
					return
				}
			}
		}
	}

	if err == nil && !found {
		err = fmt.Errorf("cannot found item '%s' from '%s'", targetName, tarFile)
	}
	return
}

func extraFile(name, targetName, tarFile string, header *tar.Header, tarReader *tar.Reader) (err error) {
	if name != targetName && !strings.HasSuffix(name, "/"+targetName) {
		return
	}
	var targetFile *os.File
	if targetFile, err = os.OpenFile(fmt.Sprintf("%s/%s", filepath.Dir(tarFile), targetName),
		os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode)); err != nil {
		return
	}
	if _, err = io.Copy(targetFile, tarReader); err != nil {
		return
	}
	_ = targetFile.Close()
	return
}
