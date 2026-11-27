package cmd

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	cotesting "github.com/linuxsuren/cobra-extension/pkg/testing"
	fakeruntime "github.com/linuxsuren/go-fake-runtime"
	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/linuxsuren/http-downloader/pkg/remote"
	"github.com/linuxsuren/http-downloader/pkg/source"
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

func TestGetDefaultInstallDirWithBash(t *testing.T) {
	opt := &installOption{execer: &fakeruntime.FakeExecer{
		ExpectOS: "windows",
		Env:      map[string]string{"SHELL": "/usr/bin/bash"},
	}}
	assert.Equal(t, "/usr/bin", opt.getDefaultInstallDir())
}

func TestResolveTargetDir(t *testing.T) {
	t.Run("local uses the machine platform", func(t *testing.T) {
		opt := &installOption{
			downloadOption: &downloadOption{},
			execer:         &fakeruntime.FakeExecer{ExpectOS: "windows"},
			target:         "",
		}
		assert.Equal(t, installer.WindowsBinDir, opt.resolveTargetDir())
	})

	t.Run("remote uses the target platform", func(t *testing.T) {
		opt := &installOption{
			downloadOption: &downloadOption{},
			execer:         &fakeruntime.FakeExecer{ExpectOS: "windows"},
			targetConfig:   remote.Config{Mode: remote.ModeSSH, OS: "linux"},
			target:         "",
		}
		assert.Equal(t, installer.UnixBinDir, opt.resolveTargetDir())
	})

	t.Run("flag wins", func(t *testing.T) {
		opt := &installOption{
			downloadOption: &downloadOption{},
			execer:         &fakeruntime.FakeExecer{ExpectOS: "windows"},
			target:         "D:\\tools",
		}
		assert.Equal(t, "D:\\tools", opt.resolveTargetDir())
	})

	t.Run("package config wins over platform but not the flag", func(t *testing.T) {
		opt := &installOption{
			execer: &fakeruntime.FakeExecer{ExpectOS: "windows"},
			target: "",
			downloadOption: &downloadOption{
				Package: &installer.HDConfig{TargetDirectory: "/opt/tools"},
			},
		}
		assert.Equal(t, "/opt/tools", opt.resolveTargetDir())
	})
}

func TestSplitPackageVersion(t *testing.T) {
	pkg, ver := splitPackageVersion("typescript")
	assert.Equal(t, "typescript", pkg)
	assert.Empty(t, ver)

	pkg, ver = splitPackageVersion("typescript@5.3.0")
	assert.Equal(t, "typescript", pkg)
	assert.Equal(t, "5.3.0", ver)

	// scoped packages keep their scope
	pkg, ver = splitPackageVersion("@types/node@18.0.0")
	assert.Equal(t, "@types/node", pkg)
	assert.Equal(t, "18.0.0", ver)
}

func TestInstallNPM(t *testing.T) {
	home := t.TempDir()
	oldCache := homedir.DisableCache
	homedir.DisableCache = true
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Cleanup(func() { homedir.DisableCache = oldCache })

	opt := &installOption{
		downloadOption: &downloadOption{},
		execer:         &fakeruntime.FakeExecer{},
		npmPackage:     "chalk",
		npmVersion:     "5.3.0",
		sourceName:     source.NPM,
	}
	assert.NoError(t, opt.install(&cobra.Command{}, []string{"chalk"}))

	tool, _ := installer.FindInstalledPackage("chalk")
	assert.NotNil(t, tool)
	assert.Equal(t, source.NPM, tool.Source)
	assert.Equal(t, "5.3.0", tool.Version)
}

func TestInstallPreRunESources(t *testing.T) {
	t.Run("unknown source", func(t *testing.T) {
		opt := &installOption{
			downloadOption: &downloadOption{},
			sourceName:     "unknown",
		}
		err := opt.preRunE(&cobra.Command{}, []string{"fake"})
		assert.ErrorContains(t, err, "unknown package source")
	})

	t.Run("npm does not need a config cache", func(t *testing.T) {
		opt := &installOption{
			downloadOption: &downloadOption{},
			sourceName:     source.NPM,
		}
		err := opt.preRunE(&cobra.Command{}, []string{"chalk@5.3.0"})
		assert.NoError(t, err)
		assert.Equal(t, "chalk", opt.npmPackage)
		assert.Equal(t, "5.3.0", opt.npmVersion)
	})

	t.Run("ssh requires a host", func(t *testing.T) {
		opt := &installOption{
			downloadOption: &downloadOption{},
			sourceName:     source.HDHome,
			targetConfig:   remote.Config{Mode: remote.ModeSSH},
		}
		err := opt.preRunE(&cobra.Command{}, []string{"fake"})
		assert.ErrorContains(t, err, "host is required")
	})

	t.Run("ssh selects the target platform", func(t *testing.T) {
		opt := &installOption{
			downloadOption: &downloadOption{},
			sourceName:     source.NPM,
			execer:         &fakeruntime.FakeExecer{ExpectOS: "windows"},
			targetConfig: remote.Config{
				Mode:     remote.ModeSSH,
				Host:     "example.com",
				Password: "secret",
				OS:       "linux",
				Arch:     "arm64",
			},
		}
		assert.NoError(t, opt.preRunE(&cobra.Command{}, []string{"chalk"}))
		assert.Equal(t, "linux", opt.OS)
		assert.Equal(t, "arm64", opt.Arch)
	})
}

func TestRecordInstallRemote(t *testing.T) {
	home := t.TempDir()
	oldCache := homedir.DisableCache
	homedir.DisableCache = true
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Cleanup(func() { homedir.DisableCache = oldCache })

	target := filepath.Join(home, "bin")
	opt := &installOption{
		downloadOption: &downloadOption{},
		tool:           "fake-cli",
		target:         target,
		sourceName:     source.HDHome,
		targetConfig: remote.Config{
			Mode: remote.ModeSSH,
			Host: "example.com",
		},
	}
	assert.NoError(t, opt.recordInstall(false))

	tool, _ := installer.FindInstalledPackage("fake-cli")
	assert.NotNil(t, tool)
	assert.Equal(t, remote.ModeSSH, tool.Mode)
	assert.Equal(t, "example.com", tool.Host)
	assert.Equal(t, source.HDHome, tool.Source)
}
