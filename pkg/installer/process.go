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
		sourceF, sourceE := os.Open(sourceFile)
		targetF, targetE := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR, 0600)
		if sourceE != nil || targetE != nil {
			err = fmt.Errorf("failed to open source file: %v, or target file: %v", sourceE, targetE)
			return
		}

		if _, err = io.Copy(targetF, sourceF); err != nil {
			err = fmt.Errorf("cannot copy %s from %s to %v, error: %v", o.Name, sourceFile, targetPath, err)
		} else {
			_ = os.RemoveAll(sourceFile)
		}
	}
	return
}

// StagePackage extracts the wanted binaries into a local temporary
// directory. Remote installs use it to prepare the files before uploading
// them, without running any package hooks on the machine running hd.
func StagePackage(archive, stageDir, binaryName string, additions []string) (binaries []string, err error) {
	if err = os.MkdirAll(stageDir, 0750); err != nil {
		return
	}

	stagedArchive := filepath.Join(stageDir, filepath.Base(archive))
	var data []byte
	if data, err = os.ReadFile(archive); err != nil {
		return
	}
	if err = os.WriteFile(stagedArchive, data, 0600); err != nil {
		return
	}

	compressor := compress.GetCompressor(path.Ext(stagedArchive), additions)
	if compressor == nil {
		err = fmt.Errorf("no compressor support for %s", stagedArchive)
		return
	}
	if err = compressor.ExtractFiles(stagedArchive, binaryName); err != nil {
		return
	}

	for _, name := range append([]string{binaryName}, additions...) {
		candidate := filepath.Join(stageDir, filepath.Base(name))
		if exists, _ := common.PathExists(candidate); exists {
			binaries = append(binaries, candidate)
		}
	}
	if len(binaries) == 0 {
		err = fmt.Errorf("cannot find the binary %s in %s", binaryName, archive)
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
