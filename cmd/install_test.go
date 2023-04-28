package cmd

import (
	"context"
	"errors"
	"sync"
	"testing"

	cotesting "github.com/linuxsuren/cobra-extension/pkg/testing"
	fakeruntime "github.com/linuxsuren/go-fake-runtime"
	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func Test_newInstallCmd(t *testing.T) {
	cmd := newInstallCmd(context.Background())
	assert.Equal(t, "install", cmd.Name())

	test := cotesting.FlagsValidation{{
		Name:      "category",
		Shorthand: "c",
	}, {
		Name: "show-progress",
	}, {
		Name: "accept-preRelease",
	}, {
		Name: "pre",
	}, {
		Name: "from-source",
	}, {
		Name: "from-branch",
	}, {
		Name: "goget",
	}, {
		Name: "download",
	}, {
		Name:      "force",
		Shorthand: "f",
	}, {
		Name: "clean-package",
	}, {
		Name:      "thread",
		Shorthand: "t",
	}, {
		Name: "keep-part",
	}, {
		Name: "os",
	}, {
		Name: "arch",
	}, {
		Name: "proxy-github",
	}, {
		Name: "fetch",
	}, {
		Name: "provider",
	}, {
		Name: "no-proxy",
	}}
	test.Valid(t, cmd.Flags())
}

func TestInstallPreRunE(t *testing.T) {
	type args struct {
		cmd  *cobra.Command
		args []string
	}
	for i, tt := range []struct {
		name      string
		opt       *installOption
		args      args
		expectErr bool
	}{{
		name: "tool and category are empty",
		opt: &installOption{
			downloadOption: &downloadOption{},
		},
		expectErr: true,
	}, {
		name: "a fake tool that have an invalid path, no category",
		opt: &installOption{
			downloadOption: &downloadOption{
				searchOption: searchOption{Fetch: false},
				wait:         &sync.WaitGroup{},
			},
		},
		args: args{
			args: []string{"xx@xx@xx"},
			cmd:  &cobra.Command{},
		},
		expectErr: true,
	}, {
		name: "have category",
		opt: &installOption{
			downloadOption: &downloadOption{
				searchOption: searchOption{Fetch: false},
				Category:     "tool",
				wait:         &sync.WaitGroup{},
			},
		},
		args: args{
			args: []string{"xx@xx@xx"},
			cmd:  &cobra.Command{},
		},
		expectErr: false,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opt.preRunE(tt.args.cmd, tt.args.args)
			if tt.expectErr {
				assert.NotNil(t, err, "failed with [%d] - case [%s]", i, tt.name)
			} else {
				assert.Nil(t, err, "failed with [%d] - case [%s]", i, tt.name)
			}
		})
	}
}

func TestShouldInstall(t *testing.T) {
	opt := &installOption{
		downloadOption: &downloadOption{},
		execer: &fakeruntime.FakeExecer{
			ExpectOutput: "v1.2.3",
		},
		tool: "fake",
	}
	should, exist := opt.shouldInstall()
	assert.False(t, should)
	assert.True(t, exist)

	{
		optGreater := &installOption{
			execer: &fakeruntime.FakeExecer{
				ExpectOutput: "v1.2.3",
			},
			downloadOption: &downloadOption{
				Package: &installer.HDConfig{
					Version:    "v1.2.4",
					VersionCmd: "-v",
				},
			},
			tool: "fake",
		}
		should, exist := optGreater.shouldInstall()
		assert.True(t, should)
		assert.True(t, exist)
	}

	// force to install
	opt.force = true
	should, exist = opt.shouldInstall()
	assert.True(t, should)
	assert.True(t, exist)

	// not exist
	opt.execer = &fakeruntime.FakeExecer{
		ExpectError:         errors.New("fake"),
		ExpectLookPathError: errors.New("error"),
	}
	should, exist = opt.shouldInstall()
	assert.True(t, should)
	assert.False(t, exist)
}

func TestInstall(t *testing.T) {
	type args struct {
		cmd  *cobra.Command
		args []string
	}
	for i, tt := range []struct {
		name      string
		opt       *installOption
		args      args
		expectErr bool
	}{{
		name: "is a nativePackage, but it's exist",
		opt: &installOption{
			downloadOption: &downloadOption{},
			nativePackage:  true,
			execer:         fakeruntime.FakeExecer{},
		},
		args:      args{cmd: &cobra.Command{}},
		expectErr: false,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opt.install(tt.args.cmd, tt.args.args)
			if tt.expectErr {
				assert.NotNil(t, err, "failed with [%d] - case [%s]", i, tt.name)
			} else {
				assert.Nil(t, err, "failed with [%d] - case [%s]", i, tt.name)
			}
		})
	}
}

func TestGetDefaultInstallDir(t *testing.T) {
	tests := []struct {
		name   string
		execer fakeruntime.Execer
		expect string
	}{{
		name: "linux",
		execer: fakeruntime.FakeExecer{
			ExpectOS: "linux",
		},
		expect: "/usr/local/bin",
	}, {
		name: "darwin",
		execer: fakeruntime.FakeExecer{
			ExpectOS: "darwin",
		},
		expect: "/usr/local/bin",
	}, {
		name: "windows",
		execer: fakeruntime.FakeExecer{
			ExpectOS: "windows",
		},
		expect: `C:\Program Files (x86)\Common Files`,
	}, {
		name:   "unknown",
		execer: fakeruntime.FakeExecer{},
		expect: "",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opt := &installOption{execer: tt.execer}
			result := opt.getDefaultInstallDir()
			assert.Equal(t, tt.expect, result)
		})
	}
}
