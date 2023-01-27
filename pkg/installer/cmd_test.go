package installer

import (
	"errors"
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRunCommandList(t *testing.T) {
	i := &Installer{
		Execer: &exec.FakeExecer{},
	}
	assert.Nil(t, i.runCommandList(nil))
	assert.Nil(t, i.runCommandList([]CmdWithArgs{{
		Cmd: "ls",
	}}))

	errInstaller := &Installer{
		Execer: exec.FakeExecer{
			ExpectError: errors.New("error"),
		},
	}
	assert.NotNil(t, errInstaller.runCommandList([]CmdWithArgs{{
		Cmd: "ls",
	}}))
}
