package cmd

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sync"
)

func getOrDefault(key, def string, data map[string]string) (result string) {
	var ok bool
	if result, ok = data[key]; !ok {
		result = def
	}
	return
}

func getReplacement(key string, data map[string]string) (result string) {
	return getOrDefault(key, key, data)
}

func getRoundTripper(ctx context.Context) (tripper *http.RoundTripper) {
	roundTripper := ctx.Value("roundTripper")

	var ok bool
	if tripper, ok = roundTripper.(*http.RoundTripper); ok {
		tripper = nil
	}
	return
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func execCommandInDir(name, dir string, arg ...string) (err error) {
	command := exec.Command(name, arg...)
	if dir != "" {
		command.Dir = dir
	}

	//var stdout []byte
	//var errStdout error
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
		_, _ = copyAndCapture(os.Stdout, stdoutIn)
		wg.Done()
	}()

	_, _ = copyAndCapture(os.Stderr, stderrIn)

	wg.Wait()

	err = command.Wait()
	return
}

func execCommand(name string, arg ...string) (err error) {
	return execCommandInDir(name, "", arg...)
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
