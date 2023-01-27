package dnf

import (
	"errors"
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCommon(t *testing.T) {
	tests := []struct {
		name            string
		installer       CommonInstaller
		expectAvailable bool
		hasErr          bool
	}{{
		name: "normal",
		installer: CommonInstaller{
			Execer: exec.FakeExecer{
				ExpectError:  nil,
				ExpectOutput: "",
				ExpectOS:     "linux",
				ExpectArch:   "amd64",
			},
		},
		expectAvailable: true,
		hasErr:          false,
	}, {
		name: "not is linux",
		installer: CommonInstaller{
			Execer: exec.FakeExecer{ExpectOS: "darwin"},
		},
		expectAvailable: false,
		hasErr:          false,
	}, {
		name: "command not found",
		installer: CommonInstaller{
			Execer: exec.FakeExecer{ExpectError: errors.New("error")},
		},
		expectAvailable: false,
		hasErr:          true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectAvailable, tt.installer.Available())
			assert.Nil(t, tt.installer.Start())
			assert.Nil(t, tt.installer.Stop())

			ok, err := tt.installer.WaitForStart()
			assert.True(t, ok)
			assert.Nil(t, err)
			assert.Equal(t, tt.hasErr, tt.installer.Install() != nil)
			assert.Equal(t, tt.hasErr, tt.installer.Uninstall() != nil)
		})
	}
}
