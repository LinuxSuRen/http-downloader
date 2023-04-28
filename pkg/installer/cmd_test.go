package installer

import (
	"errors"
	"testing"

	fakeruntime "github.com/linuxsuren/go-fake-runtime"
	"github.com/stretchr/testify/assert"
)

func TestRunCommandList(t *testing.T) {
	i := &Installer{
		Execer: &fakeruntime.FakeExecer{},
	}
	assert.Nil(t, i.runCommandList(nil))
	assert.Nil(t, i.runCommandList([]CmdWithArgs{{
		Cmd: "ls",
	}}))

	errInstaller := &Installer{
		Execer: fakeruntime.FakeExecer{
			ExpectError: errors.New("error"),
		},
	}
	assert.NotNil(t, errInstaller.runCommandList([]CmdWithArgs{{
		Cmd: "ls",
	}}))
}
