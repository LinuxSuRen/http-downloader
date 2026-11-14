package cmd

import (
	"testing"

	fakeruntime "github.com/linuxsuren/go-fake-runtime"
	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestRemoveViaPackageManager(t *testing.T) {
	tmpHome := t.TempDir()
	oldCache := homedir.DisableCache
	homedir.DisableCache = true
	t.Setenv("HOME", tmpHome)
	t.Cleanup(func() { homedir.DisableCache = oldCache })

	assert.NoError(t, installer.RecordInstalledPackage(installer.InstalledPackage{
		Name:           "fake-cli",
		Version:        "1.0.0",
		Native:         true,
		PackageManager: "apt",
		Binaries:       []string{tmpHome + "/bin/fake-cli"},
	}))

	opt := &removeOption{
		execer: &fakeruntime.FakeExecer{ExpectOS: "linux"},
	}
	assert.NoError(t, opt.runE(&cobra.Command{}, []string{"fake-cli"}))

	tools, err := installer.LoadInstalledPackages()
	assert.NoError(t, err)
	assert.Empty(t, tools)

	// the package manager is not available on this platform
	assert.NoError(t, installer.RecordInstalledPackage(installer.InstalledPackage{
		Name:           "fake-cli",
		PackageManager: "winget",
	}))
	optUnavailable := &removeOption{
		execer: &fakeruntime.FakeExecer{ExpectOS: "linux"},
	}
	assert.Error(t, optUnavailable.runE(&cobra.Command{}, []string{"fake-cli"}))
}
