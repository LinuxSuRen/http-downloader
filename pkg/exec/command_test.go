package exec

import (
	"github.com/stretchr/testify/assert"
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
