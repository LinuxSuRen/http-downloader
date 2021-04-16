package pkg

//
//import (
//	"archive/tar"
//	"compress/gzip"
//	"fmt"
//	"github.com/spf13/cobra"
//	"io"
//	"os"
//	"os/exec"
//	"path/filepath"
//	"strings"
//	"syscall"
//)
//
//type Installer struct {
//}
//
//func (o *Installer) Download(cmd *cobra.Command, args []string) (err error) {
//	if o.Download {
//		if err = o.downloadOption.runE(cmd, args); err != nil {
//			return
//		}
//	}
//
//	targetBinary := o.name
//	if o.Package != nil && o.Package.TargetBinary != "" {
//		// this is the desired binary file
//		targetBinary = o.Package.TargetBinary
//	}
//
//	var source string
//	var target string
//	tarFile := o.Output
//	if o.Tar {
//		if err = o.extractFiles(tarFile, o.name); err == nil {
//			source = fmt.Sprintf("%s/%s", filepath.Dir(tarFile), o.name)
//			target = fmt.Sprintf("/usr/local/bin/%s", targetBinary)
//		} else {
//			err = fmt.Errorf("cannot extract %s from tar file, error: %v", tarFile, err)
//		}
//	} else {
//		source = o.downloadOption.Output
//		target = fmt.Sprintf("/usr/local/bin/%s", targetBinary)
//	}
//
//	if err == nil {
//		if o.Package != nil && o.Package.PreInstall != nil {
//			if err = execCommand(o.Package.PreInstall.Cmd, o.Package.PreInstall.Args...); err != nil {
//				return
//			}
//		}
//
//		if o.Package != nil && o.Package.Installation != nil {
//			err = execCommand(o.Package.Installation.Cmd, o.Package.Installation.Args...)
//		} else {
//			err = o.overWriteBinary(source, target)
//		}
//
//		if err == nil && o.Package != nil && o.Package.PostInstall != nil {
//			err = execCommand(o.Package.PostInstall.Cmd, o.Package.PostInstall.Args...)
//		}
//
//		if err == nil && o.Package != nil && o.Package.TestInstall != nil {
//			err = execCommand(o.Package.TestInstall.Cmd, o.Package.TestInstall.Args...)
//		}
//
//		if err == nil && o.CleanPackage {
//			if cleanErr := os.RemoveAll(tarFile); cleanErr != nil {
//				cmd.Println("cannot remove file", tarFile, ", error:", cleanErr)
//			}
//		}
//	}
//	return
//}
//
//func (o *Installer) overWriteBinary(sourceFile, targetPath string) (err error) {
//	fmt.Println("install", sourceFile, "to", targetPath)
//	switch runtime.GOOS {
//	case "linux", "darwin":
//		if err = execCommand("chmod", "u+x", sourceFile); err != nil {
//			return
//		}
//
//		if err = execCommand("rm", "-rf", targetPath); err != nil {
//			return
//		}
//
//		var cp string
//		if cp, err = exec.LookPath("mv"); err == nil {
//			err = syscall.Exec(cp, []string{"mv", sourceFile, targetPath}, os.Environ())
//		}
//	default:
//		sourceF, _ := os.Open(sourceFile)
//		targetF, _ := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR, 0600)
//		if _, err = io.Copy(targetF, sourceF); err != nil {
//			err = fmt.Errorf("cannot copy %s from %s to %v, error: %v", o.name, sourceFile, targetPath, err)
//		}
//
//		if err == nil {
//			_ = os.RemoveAll(sourceFile)
//		}
//	}
//	return
//}
//
//func (o *Installer) extractFiles(tarFile, targetName string) (err error) {
//	var f *os.File
//	var gzf *gzip.Reader
//	if f, err = os.Open(tarFile); err != nil {
//		return
//	}
//	defer func() {
//		_ = f.Close()
//	}()
//
//	if gzf, err = gzip.NewReader(f); err != nil {
//		return
//	}
//
//	tarReader := tar.NewReader(gzf)
//	var header *tar.Header
//	var found bool
//	for {
//		if header, err = tarReader.Next(); err == io.EOF {
//			err = nil
//			break
//		} else if err != nil {
//			break
//		}
//		name := header.Name
//
//		switch header.Typeflag {
//		case tar.TypeReg:
//			if name != targetName && !strings.HasSuffix(name, "/"+targetName) {
//				continue
//			}
//			var targetFile *os.File
//			if targetFile, err = os.OpenFile(fmt.Sprintf("%s/%s", filepath.Dir(tarFile), targetName),
//				os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode)); err != nil {
//				break
//			}
//			if _, err = io.Copy(targetFile, tarReader); err != nil {
//				break
//			}
//			found = true
//			_ = targetFile.Close()
//		}
//	}
//
//	if err == nil && !found {
//		err = fmt.Errorf("cannot found item '%s' from '%s'", targetName, tarFile)
//	}
//	return
//}
