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
	"sync"
	"syscall"
)

// NewInstallCmd returns the install command
func NewInstallCmd() (cmd *cobra.Command) {
	opt := &installOption{}
	cmd = &cobra.Command{
		Use:     "install",
		PreRunE: opt.preRunE,
		RunE:    opt.runE,
	}

	flags := cmd.Flags()
	//flags.StringVarP(&opt.Mode, "mode", "m", "package",
	//	"If you want to install it via platform package manager")
	flags.BoolVarP(&opt.ShowProgress, "show-progress", "", true, "If show the progress of download")
	flags.IntVarP(&opt.Thread, "thread", "t", 0,
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
	Mode string
}

func (o *installOption) preRunE(cmd *cobra.Command, args []string) (err error) {
	o.fetchHomeConfig()
	err = o.downloadOption.preRunE(cmd, args)
	return
}

func (o *installOption) runE(cmd *cobra.Command, args []string) (err error) {
	if err = o.downloadOption.runE(cmd, args); err != nil {
		return
	}

	if err = o.extractFiles(o.Output, o.name); err == nil {
		err = o.overWriteBinary(fmt.Sprintf("%s/%s", filepath.Dir(o.Output), o.name), fmt.Sprintf("/usr/local/bin/%s", o.name))
	} else {
		err = fmt.Errorf("cannot extract %s from tar file, error: %v", o.Output, err)
	}
	return
}

func (o *installOption) overWriteBinary(sourceFile, targetPath string) (err error) {
	switch runtime.GOOS {
	case "linux":
		var cp string
		if cp, err = exec.LookPath("cp"); err == nil {
			err = syscall.Exec(cp, []string{"cp", sourceFile, targetPath}, os.Environ())
		}
	default:
		sourceF, _ := os.Open(sourceFile)
		targetF, _ := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR, 0664)
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
			if name != targetName {
				continue
			}
			var targetFile *os.File
			if targetFile, err = os.OpenFile(fmt.Sprintf("%s/%s", filepath.Dir(tarFile), name),
				os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode)); err != nil {
				break
			}
			if _, err = io.Copy(targetFile, tarReader); err != nil {
				break
			}
			_ = targetFile.Close()
		}
	}
	return
}

func execCommand(name string, arg ...string) (err error) {
	command := exec.Command(name, arg...)

	var stdout []byte
	var errStdout error
	stdoutIn, _ := command.StdoutPipe()
	stderrIn, _ := command.StderrPipe()
	err = command.Start()
	if err != nil {
		return err
	}

	// cmd.Wait() should be called only after we finish reading
	// from stdoutIn and stderrIn.
	// wg ensures that we finish
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		stdout, errStdout = copyAndCapture(os.Stdout, stdoutIn)
		wg.Done()
	}()

	copyAndCapture(os.Stderr, stderrIn)

	wg.Wait()

	err = command.Wait()
	return
}

func copyAndCapture(w io.Writer, r io.Reader) ([]byte, error) {
	var out []byte
	buf := make([]byte, 1024, 1024)
	for {
		n, err := r.Read(buf[:])
		if n > 0 {
			d := buf[:n]
			out = append(out, d...)
			_, err := w.Write(d)
			if err != nil {
				return out, err
			}
		}
		if err != nil {
			// Read returns io.EOF at the end of file, which is not an error for us
			if err == io.EOF {
				err = nil
			}
			return out, err
		}
	}
}
