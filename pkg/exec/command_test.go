package exec

import (
	"github.com/stretchr/testify/assert"
	"os"
	osexec "os/exec"
	"runtime"
	"testing"
)

func TestRuntime(t *testing.T) {
	execer := DefaultExecer{}
	assert.Equal(t, runtime.GOOS, execer.OS())
	assert.Equal(t, runtime.GOARCH, execer.Arch())
}

func TestDefaultLookPath(t *testing.T) {
	tests := []struct {
		name string
		arg  string
	}{{
		name: "ls",
		arg:  "ls",
	}, {
		name: "unknown",
		arg:  "unknown",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execer := DefaultExecer{}

			expectPath, expectErr := osexec.LookPath(tt.arg)
			resultPath, resultErr := execer.LookPath(tt.arg)

			assert.Equal(t, expectPath, resultPath)
			assert.Equal(t, expectErr, resultErr)
		})
	}
}

func TestDefaultExecer(t *testing.T) {
	tests := []struct {
		name      string
		cmd       string
		args      []string
		expectErr bool
		verify    func(t *testing.T, out string)
	}{{
		name:      "go version",
		cmd:       "go",
		args:      []string{"version"},
		expectErr: false,
		verify: func(t *testing.T, out string) {
			assert.Contains(t, out, "go version")
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ex := &DefaultExecer{}
			out, err := ex.Command(tt.cmd, tt.args...)
			assert.Equal(t, tt.expectErr, err != nil, err)
			if tt.verify != nil {
				tt.verify(t, string(out))
			}
			err = ex.RunCommand(tt.cmd, tt.args...)
			assert.Equal(t, tt.expectErr, err != nil, err)

			arch := ex.Arch()
			assert.Equal(t, runtime.GOARCH, arch)

			var outStr string
			outStr, err = ex.RunCommandAndReturn(tt.cmd, "", tt.args...)
			assert.Equal(t, tt.expectErr, err != nil, err)
			if tt.verify != nil {
				tt.verify(t, outStr)
			}

			err = ex.RunCommandWithIO(tt.cmd, os.TempDir(), os.Stdout, os.Stderr, tt.args...)
			assert.Equal(t, tt.expectErr, err != nil, err)

			err = ex.RunCommandInDir(tt.cmd, "", tt.args...)
			assert.Equal(t, tt.expectErr, err != nil, err)

			err = ex.RunCommandWithBuffer(tt.cmd, "", nil, nil, tt.args...)
			assert.Equal(t, tt.expectErr, err != nil, err)
		})
	}
}
