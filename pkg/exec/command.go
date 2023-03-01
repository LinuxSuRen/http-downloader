package exec

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
)

// Execer is an interface for OS-related operations
type Execer interface {
	LookPath(string) (string, error)
	Command(name string, arg ...string) ([]byte, error)
	RunCommand(name string, arg ...string) (err error)
	RunCommandInDir(name, dir string, args ...string) error
	RunCommandAndReturn(name, dir string, args ...string) (result string, err error)
	RunCommandWithSudo(name string, args ...string) (err error)
	RunCommandWithBuffer(name, dir string, stdout, stderr *bytes.Buffer, args ...string) error
	RunCommandWithIO(name, dir string, stdout, stderr io.Writer, args ...string) (err error)
	SystemCall(name string, argv []string, envv []string) (err error)
	OS() string
	Arch() string
}

const (
	// OSLinux is the alias of Linux
	OSLinux = "linux"
	// OSDarwin is the alias of Darwin
	OSDarwin = "darwin"
	// OSWindows is the alias of Windows
	OSWindows = "windows"
)

// DefaultExecer is a wrapper for the OS exec
type DefaultExecer struct {
}

// LookPath is the wrapper of os/exec.LookPath
func (e DefaultExecer) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

// Command is the wrapper of os/exec.Command
func (e DefaultExecer) Command(name string, arg ...string) ([]byte, error) {
	return exec.Command(name, arg...).CombinedOutput()
}

// RunCommand runs a command
func (e DefaultExecer) RunCommand(name string, arg ...string) error {
	return e.RunCommandWithIO(name, "", os.Stdout, os.Stderr, arg...)
}

// RunCommandWithIO runs a command with given IO
func (e DefaultExecer) RunCommandWithIO(name, dir string, stdout, stderr io.Writer, args ...string) (err error) {
	command := exec.Command(name, args...)
	if dir != "" {
		command.Dir = dir
	}

	//var stdout []byte
	//var errStdout error
	stdoutIn, _ := command.StdoutPipe()
	stderrIn, _ := command.StderrPipe()
	err = command.Start()
	if err == nil {
		// cmd.Wait() should be called only after we finish reading
		// from stdoutIn and stderrIn.
		// wg ensures that we finish
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			_, _ = copyAndCapture(stdout, stdoutIn)
			wg.Done()
		}()

		_, _ = copyAndCapture(stderr, stderrIn)

		wg.Wait()

		err = command.Wait()
	}
	return
}

// OS returns the os name
func (e DefaultExecer) OS() string {
	return runtime.GOOS
}

// Arch returns the os arch
func (e DefaultExecer) Arch() string {
	return runtime.GOARCH
}

// RunCommandAndReturn runs a command, then returns the output
func (e DefaultExecer) RunCommandAndReturn(name, dir string, args ...string) (result string, err error) {
	stdout := &bytes.Buffer{}
	if err = e.RunCommandWithBuffer(name, dir, stdout, nil, args...); err == nil {
		result = stdout.String()
	}
	return
}

// RunCommandWithBuffer runs a command with buffer
// stdout and stderr could be nil
func (e DefaultExecer) RunCommandWithBuffer(name, dir string, stdout, stderr *bytes.Buffer, args ...string) error {
	if stdout == nil {
		stdout = &bytes.Buffer{}
	}
	if stderr == nil {
		stderr = &bytes.Buffer{}
	}
	return e.RunCommandWithIO(name, dir, stdout, stderr, args...)
}

// RunCommandInDir runs a command
func (e DefaultExecer) RunCommandInDir(name, dir string, args ...string) error {
	return e.RunCommandWithIO(name, dir, os.Stdout, os.Stderr, args...)
}

// RunCommandWithSudo runs a command with sudo
func (e DefaultExecer) RunCommandWithSudo(name string, args ...string) (err error) {
	newArgs := make([]string, 0)
	newArgs = append(newArgs, name)
	newArgs = append(newArgs, args...)
	return e.RunCommand("sudo", newArgs...)
}

// SystemCall is the wrapper of syscall.Exec
func (e DefaultExecer) SystemCall(name string, argv []string, envv []string) (err error) {
	return syscall.Exec(name, argv, envv)
}

func copyAndCapture(w io.Writer, r io.Reader) ([]byte, error) {
	var out []byte
	buf := make([]byte, 1024)
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
