package cmd

import (
	"context"
	"errors"
	"sync"
	"testing"

	cotesting "github.com/linuxsuren/cobra-extension/pkg/testing"
	fakeruntime "github.com/linuxsuren/go-fake-runtime"
	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/mitchellh/go-homedir"
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
		Name: "format",
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
			execer:         &fakeruntime.FakeExecer{},
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
		execer: &fakeruntime.FakeExecer{
			ExpectOS: "linux",
		},
		expect: "/usr/local/bin",
	}, {
		name: "darwin",
		execer: &fakeruntime.FakeExecer{
			ExpectOS: "darwin",
		},
		expect: "/usr/local/bin",
	}, {
		name: "windows",
		execer: &fakeruntime.FakeExecer{
			ExpectOS: "windows",
		},
		expect: `C:\Program Files (x86)\Common Files`,
	}, {
		name:   "unknown",
		execer: &fakeruntime.FakeExecer{},
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

func TestInstallPreRunEManagerRef(t *testing.T) {
	for i, tt := range []struct {
		name             string
		ref              string
		expectManager    string
		expectTool       string
		expectNativePack bool
	}{{
		name:             "apt package",
		ref:              "apt:jenkins",
		expectManager:    "apt",
		expectTool:       "jenkins",
		expectNativePack: true,
	}, {
		name:             "winget package",
		ref:              "winget:Git.Git",
		expectManager:    "winget",
		expectTool:       "Git.Git",
		expectNativePack: true,
	}, {
		name:             "the version pinning is stripped",
		ref:              "apt:jenkins@1.0",
		expectManager:    "apt",
		expectTool:       "jenkins",
		expectNativePack: true,
	}, {
		name:             "an unknown manager falls back to the normal flow",
		ref:              "unknown:jenkins",
		expectManager:    "",
		expectNativePack: false,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			opt := &installOption{
				downloadOption: &downloadOption{
					searchOption: searchOption{Fetch: false},
					wait:         &sync.WaitGroup{},
				},
				execer: &fakeruntime.FakeExecer{ExpectOS: "linux"},
			}
			// the fallback flow needs a valid package path, give the
			// unknown manager a broken one on purpose
			err := opt.preRunE(&cobra.Command{}, []string{tt.ref})
			if tt.expectNativePack {
				assert.NoError(t, err, "failed with case [%d] - [%s]", i, tt.name)
				assert.Equal(t, tt.expectTool, opt.tool,
					"failed with case [%d] - [%s]", i, tt.name)
			} else {
				// the fallback flow rejects the broken name and rewrites
				// the tool with the empty package config
				assert.Error(t, err, "failed with case [%d] - [%s]", i, tt.name)
			}
			assert.Equal(t, tt.expectNativePack, opt.nativePackage,
				"failed with case [%d] - [%s]", i, tt.name)
			assert.Equal(t, tt.expectManager, opt.packageManager,
				"failed with case [%d] - [%s]", i, tt.name)
		})
	}
}

func TestInstallViaPackageManager(t *testing.T) {
	tmpHome := t.TempDir()
	oldCache := homedir.DisableCache
	homedir.DisableCache = true
	t.Setenv("HOME", tmpHome)
	t.Cleanup(func() { homedir.DisableCache = oldCache })

	// the tool itself is missing while apt-get is available
	execer := &selectiveLookPathExecer{
		FakeExecer: fakeruntime.FakeExecer{ExpectOS: "linux"},
		missing:    map[string]bool{"fake-cli": true},
	}
	opt := &installOption{
		downloadOption: &downloadOption{},
		nativePackage:  true,
		packageManager: "apt",
		tool:           "fake-cli",
		execer:         execer,
	}
	assert.NoError(t, opt.install(&cobra.Command{}, []string{"apt:fake-cli"}))

	tools, err := installer.LoadInstalledPackages()
	assert.NoError(t, err)
	assert.Len(t, tools, 1)
	assert.Equal(t, "fake-cli", tools[0].Name)
	assert.True(t, tools[0].Native)
	assert.Equal(t, "apt", tools[0].PackageManager)

	// the package manager is not available on this platform
	optUnavailable := &installOption{
		downloadOption: &downloadOption{},
		nativePackage:  true,
		packageManager: "winget",
		tool:           "fake-cli",
		execer: &fakeruntime.FakeExecer{
			ExpectOS:            "linux",
			ExpectLookPathError: errors.New("not found"),
		},
	}
	assert.Error(t, optUnavailable.install(&cobra.Command{}, []string{"winget:fake-cli"}))
}

// selectiveLookPathExecer fails the look path of the given commands only
type selectiveLookPathExecer struct {
	fakeruntime.FakeExecer
	missing map[string]bool
}

func (l *selectiveLookPathExecer) LookPath(name string) (string, error) {
	if l.missing[name] {
		return "", errors.New("not found")
	}
	return name, nil
}
