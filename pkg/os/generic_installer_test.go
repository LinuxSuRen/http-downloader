package os

import (
	"errors"
	"io"
	"os"
	"testing"

	fakeruntime "github.com/linuxsuren/go-fake-runtime"
	"github.com/linuxsuren/http-downloader/pkg/os/fake"
	"github.com/stretchr/testify/assert"
)

func TestURLReplace(t *testing.T) {
	genericPkg := &genericPackage{
		env: map[string]string{
			"key": "value",
		},
		execer: fakeruntime.FakeExecer{ExpectOS: fakeruntime.OSLinux},
	}
	genericPkg.SetURLReplace(map[string]string{
		"github": "ghproxy",
	})
	genericPkg.loadEnv()
	assert.Equal(t, "ghproxy-value", genericPkg.urlReplace("github-{{.key}}"))
	assert.Equal(t, "value", genericPkg.urlReplace("{{.key}}"))
	assert.Equal(t, []string{"value"}, genericPkg.sliceReplace([]string{"{{.key}}"}))

	emptyGenericPkg := &genericPackage{
		execer: fakeruntime.FakeExecer{ExpectOS: fakeruntime.OSLinux},
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
		execer: fakeruntime.FakeExecer{
			ExpectOS: fakeruntime.OSLinux,
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
		execer: fakeruntime.FakeExecer{
			ExpectOS:    fakeruntime.OSLinux,
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
		execer: fakeruntime.FakeExecer{
			ExpectOS: fakeruntime.OSLinux,
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

func TestShould(t *testing.T) {
	tests := []struct {
		name    string
		writeTo *WriteTo
		wantOK  bool
		wantErr bool
	}{{
		name:    "expr is empty",
		writeTo: &WriteTo{},
		wantOK:  true,
		wantErr: false,
	}, {
		name: "1==1",
		writeTo: &WriteTo{
			When: "1==1",
		},
		wantOK:  true,
		wantErr: false,
	}, {
		name: "not bool expr",
		writeTo: &WriteTo{
			When: "not-expect",
		},
		wantOK:  false,
		wantErr: true,
	}, {
		name: "false",
		writeTo: &WriteTo{
			When: "false",
		},
		wantOK:  false,
		wantErr: false,
	}, {
		name: "true",
		writeTo: &WriteTo{
			When: "true",
		},
		wantOK:  true,
		wantErr: false,
	}, {
		name: "expr is number",
		writeTo: &WriteTo{
			When: "123",
		},
		wantOK:  false,
		wantErr: true,
	}, {
		name: "with env, equal",
		writeTo: &WriteTo{
			env: map[string]string{
				"OS": "ubuntu",
			},
			When: "OS=='ubuntu'",
		},
		wantOK:  true,
		wantErr: false,
	}, {
		name: "with env, not equal",
		writeTo: &WriteTo{
			env: map[string]string{
				"OS": "ubuntu",
			},
			When: "OS!='ubuntu'",
		},
		wantOK:  false,
		wantErr: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := tt.writeTo.Should()
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantErr, err != nil, err)
		})
	}
}
