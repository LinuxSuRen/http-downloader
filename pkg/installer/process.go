package installer

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	fakeruntime "github.com/linuxsuren/go-fake-runtime"

	"github.com/linuxsuren/http-downloader/pkg/common"
	"github.com/linuxsuren/http-downloader/pkg/compress"
)

// Install installs a package
func (o *Installer) Install() (err error) {
	if o.Execer.OS() == "windows" {
		o.Name = fmt.Sprintf("%s.exe", o.Name)
	}
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
			target = path.Join(o.TargetDirectory, targetBinary)
		} else {
			err = fmt.Errorf("cannot extract %s from tar file, error: %v", tarFile, err)
		}
	} else {
		source = o.Source
		target = path.Join(o.TargetDirectory, targetBinary)
	}

	if err == nil {
		if o.Package != nil && o.Package.PreInstalls != nil {
			if err = o.runCommandList(o.Package.PreInstalls); err != nil {
				return
			}
		}

		if o.Package != nil && o.Package.Installation != nil {
			err = o.Execer.RunCommand(o.Package.Installation.Cmd, o.Package.Installation.Args...)
		} else {
			if err = o.OverWriteBinary(source, target); err != nil {
				return
			}

			for i := range o.AdditionBinaries {
				addition := o.AdditionBinaries[i]
				if err = o.OverWriteBinary(addition, path.Join(o.TargetDirectory, filepath.Base(addition))); err != nil {
					return
				}
			}
		}

		if o.Package != nil {
			for i := range o.Package.DefaultConfigFile {
				configFile := o.Package.DefaultConfigFile[i]
				configFilePath := configFile.Path
				configDir := filepath.Dir(configFilePath)

				if configFile.OS == o.Execer.OS() {
					if err = os.MkdirAll(configDir, 0750); err != nil {
						if strings.Contains(err.Error(), "permission denied") {
							err = o.Execer.RunCommandWithSudo("mkdir", "-p", configDir)
						}

						if err != nil {
							err = fmt.Errorf("cannot create config dir: %s, error: %v", configDir, err)
							return
						}
					}

					if err = os.WriteFile(configFilePath, []byte(configFile.Content), 0622); err != nil {
						if strings.Contains(err.Error(), "permission denied") {
							if err = o.Execer.RunCommandWithSudo("touch", configFilePath); err == nil {
								err = o.Execer.RunCommandWithSudo("chmod", "+w", configFilePath)
							}
						}

						if err != nil {
							err = fmt.Errorf("cannot write config file: %s, error: %v", configFilePath, err)
							return
						}
					}

					fmt.Printf("config file [%s] is ready.\n", configFilePath)
				}
			}
		}

		if err == nil && o.Package != nil && o.Package.PostInstalls != nil {
			err = o.runCommandList(o.Package.PostInstalls)
		}

		if err == nil && o.Package != nil && o.Package.TestInstalls != nil {
			err = o.runCommandList(o.Package.TestInstalls)
		}

		if err == nil && o.CleanPackage {
			if cleanErr := os.RemoveAll(tarFile); cleanErr != nil {
				fmt.Println("cannot remove file", tarFile, ", error:", cleanErr)
			}
		}
	}
	return
}

// OverWriteBinary install a binary file
func (o *Installer) OverWriteBinary(sourceFile, targetPath string) (err error) {
	fmt.Println("install", sourceFile, "to", targetPath)
	switch o.Execer.OS() {
	case fakeruntime.OSLinux, fakeruntime.OSDarwin:
		if err = o.Execer.RunCommand("chmod", "u+x", sourceFile); err != nil {
			return
		}

		if common.IsDirWriteable(path.Dir(targetPath)) != nil {
			if err = o.Execer.RunCommandWithSudo("rm", "-rf", targetPath); err != nil {
				return
			}
		} else {
			if err = o.Execer.RunCommand("rm", "-rf", targetPath); err != nil {
				return
			}
		}

		if common.IsDirWriteable(path.Dir(targetPath)) != nil {
			err = o.Execer.RunCommandWithSudo("mv", sourceFile, targetPath)
		} else {
			err = o.Execer.RunCommand("mv", sourceFile, targetPath)
		}
	default:
		// A running binary cannot be overwritten on Windows, but it can
		// be renamed: move the current file aside, stage the new one and
		// drop the old file when nothing holds it anymore. A self
		// upgrade keeps the .old file behind until the next replace,
		// because the old process still runs it at this point.
		backup := targetPath + ".old"
		if _, statErr := os.Stat(targetPath); statErr == nil {
			// the backup of an earlier upgrade is not in use anymore
			_ = os.Remove(backup)
			if err = os.Rename(targetPath, backup); err != nil {
				err = fmt.Errorf("cannot replace %s, it may be in use, error: %v", targetPath, err)
				return
			}
		}
		defer func() {
			if err != nil {
				// put the previous binary back when the new one did not
				// make it into place
				if _, statErr := os.Stat(targetPath); statErr != nil {
					_ = os.Rename(backup, targetPath)
				}
			}
		}()

		staged := targetPath + ".new"
		var sourceF *os.File
		var targetF *os.File
		if sourceF, err = os.Open(sourceFile); err != nil {
			err = fmt.Errorf("cannot open the downloaded package %s, error: %v", sourceFile, err)
			return
		}
		defer func() { _ = sourceF.Close() }()
		// O_TRUNC keeps a smaller binary from inheriting the tail of the
		// bigger one it replaces, 0600 is enough because Windows derives
		// the executable state from the file extension
		if targetF, err = os.OpenFile(staged, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600); err != nil {
			err = fmt.Errorf("cannot create the staged file %s, error: %v", staged, err)
			return
		}
		if _, err = io.Copy(targetF, sourceF); err != nil {
			_ = targetF.Close()
			err = fmt.Errorf("cannot copy %s from %s to %s, error: %v", o.Name, sourceFile, staged, err)
			return
		}
		if err = targetF.Close(); err != nil {
			return
		}
		if err = os.Rename(staged, targetPath); err != nil {
			err = fmt.Errorf("cannot put %s into place, error: %v", targetPath, err)
			return
		}
		_ = os.RemoveAll(sourceFile)
		// the old binary is unlocked once no process runs it, failing to
		// remove it right now is fine
		_ = os.Remove(backup)
	}
	return
}

func (o *Installer) extractFiles(tarFile, targetName string) (err error) {
	compressor := compress.GetCompressor(path.Ext(tarFile), o.AdditionBinaries)
	if compressor == nil {
		err = fmt.Errorf("no compressor support for %s", tarFile)
	} else {
		err = compressor.ExtractFiles(tarFile, targetName)
	}
	return
}
