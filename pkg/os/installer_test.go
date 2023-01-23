package os

import (
	"runtime"
	"testing"

	"github.com/linuxsuren/http-downloader/pkg/os/fake"
	"github.com/stretchr/testify/assert"
)

func TestHasPackage(t *testing.T) {
	// currently, this function only support Linux
	if runtime.GOOS != "linux" {
		return
	}

	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		name: "fake-package",
		args: args{
			name: "fake-package",
		},
		want: false,
	}, {
		name: "docker",
		args: args{
			name: "docker",
		},
		want: true,
	}, {
		name: "golang",
		args: args{
			name: "golang",
		},
		want: true,
	}, {
		name: "git",
		args: args{
			name: "git",
		},
		want: true,
	}, {
		name: "bash-completion",
		args: args{
			name: "bash-completion",
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasPackage(tt.args.name); got != tt.want {
				t.Errorf("test: %s, HasPackage() = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestPackageInstallInAllPlatforms(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		name: "vim",
		args: args{
			name: "vim",
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasPackage(tt.args.name); got != tt.want {
				t.Errorf("HasPackage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWithFakeInstaller(t *testing.T) {
	// test uninstall a fake package
	err := Uninstall("fake")
	assert.Nil(t, err)
	assert.False(t, HasPackage("fake"))

	defaultInstallerRegistry.Registry("fake", fake.NewFakeInstaller(true, false))
	err = Uninstall("fake")
	assert.Nil(t, err)
	err = InstallWithProxy("fake", nil)
	assert.Nil(t, err)

	defaultInstallerRegistry.Registry("fake-with-err", fake.NewFakeInstaller(true, true))
	err = Uninstall("fake-with-err")
	assert.NotNil(t, err)
}
