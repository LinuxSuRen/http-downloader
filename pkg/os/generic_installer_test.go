package os

import (
	"errors"
	"io"
	"os"
	"testing"

	"github.com/linuxsuren/http-downloader/pkg/exec"
	"github.com/linuxsuren/http-downloader/pkg/os/fake"
	"github.com/stretchr/testify/assert"
)

func TestURLReplace(t *testing.T) {
	genericPkg := &genericPackage{
		env: map[string]string{
			"key": "value",
		},
		execer: exec.FakeExecer{ExpectOS: exec.OSLinux},
	}
	genericPkg.SetURLReplace(map[string]string{
		"github": "ghproxy",
	})
	genericPkg.loadEnv()
	assert.Equal(t, "ghproxy-value", genericPkg.urlReplace("github-{{.key}}"))
	assert.Equal(t, "value", genericPkg.urlReplace("{{.key}}"))
	assert.Equal(t, []string{"value"}, genericPkg.sliceReplace([]string{"{{.key}}"}))

	emptyGenericPkg := &genericPackage{
		execer: exec.FakeExecer{ExpectOS: exec.OSLinux},
	}
	emptyGenericPkg.loadEnv()
	assert.NotNil(t, emptyGenericPkg.env)
	assert.Nil(t, emptyGenericPkg.Start())
	assert.Nil(t, emptyGenericPkg.Stop())
	assert.False(t, emptyGenericPkg.IsService())
	assert.False(t, emptyGenericPkg.Available())
	assert.NotNil(t, emptyGenericPkg.Install())
	assert.NotNil(t, emptyGenericPkg.Uninstall())
	ok, err := emptyGenericPkg.WaitForStart()
	assert.True(t, ok)
	assert.Nil(t, err)

	withPreInstall := &genericPackage{
		execer: exec.FakeExecer{
			ExpectOS: exec.OSLinux,
		},
		PreInstall: []preInstall{{
			Cmd: CmdWithArgs{
				Cmd: "ls",
			},
		}, {
			IssuePrefix: "good",
			Cmd: CmdWithArgs{
				Cmd: "ls",
			},
		}, {
			IssuePrefix: "Ubuntu",
			Cmd: CmdWithArgs{
				Cmd: "ls",
			},
		}},
		CommonInstaller: fake.NewFakeInstaller(true, false),
	}
	assert.Nil(t, withPreInstall.Install())

	withErrorPreInstall := &genericPackage{
		execer: exec.FakeExecer{
			ExpectOS:    exec.OSLinux,
			ExpectError: errors.New("error"),
		},
		PreInstall: []preInstall{{
			Cmd: CmdWithArgs{
				Cmd: "ls",
			},
		}},
		CommonInstaller: fake.NewFakeInstaller(true, true),
	}
	assert.NotNil(t, withErrorPreInstall.Install())
	assert.NotNil(t, withErrorPreInstall.Uninstall())
	assert.True(t, withErrorPreInstall.Available())

	tmpFile, err := os.CreateTemp(os.TempDir(), "installer")
	assert.Nil(t, err)
	defer func() {
		os.Remove(tmpFile.Name())
	}()
	writeToFileInstall := &genericPackage{
		execer: exec.FakeExecer{
			ExpectOS: exec.OSLinux,
		},
		PreInstall: []preInstall{{
			Cmd: CmdWithArgs{
				WriteTo: &WriteTo{
					File:    tmpFile.Name(),
					Content: "sample",
				},
			},
		}},
		CommonInstaller: fake.NewFakeInstaller(true, false),
	}
	err = writeToFileInstall.Install()
	assert.Nil(t, err)
	data, err := io.ReadAll(tmpFile)
	assert.Nil(t, err)
	assert.Equal(t, "sample", string(data))
}
