package installer

import (
	"os"
	"path"
	"testing"

	fakeruntime "github.com/linuxsuren/go-fake-runtime"
	"github.com/stretchr/testify/assert"
)

func TestInstallerExtractFiles(t *testing.T) {
	installer := &Installer{}

	assert.NotNil(t, installer.extractFiles("fake.fake", ""))
	assert.NotNil(t, installer.extractFiles("a.tar.gz", ""))
}

func TestOverwriteBinary(t *testing.T) {
	installer := &Installer{
		Execer: &fakeruntime.FakeExecer{},
	}

	sourceFile := path.Join(os.TempDir(), "fake-1")
	targetFile := path.Join(os.TempDir(), "fake-2")

	_ = os.WriteFile(sourceFile, []byte("fake"), 0600)

	defer func() {
		_ = os.RemoveAll(sourceFile)
	}()
	defer func() {
		_ = os.RemoveAll(targetFile)
	}()

	assert.Nil(t, installer.OverWriteBinary(sourceFile, targetFile))
}

// TestOverwriteBinaryWindows covers the replace semantics of Windows: the
// running binary moves aside instead of being overwritten, the staged
// file truncates, and a failed install keeps the current binary.
func TestOverwriteBinaryWindows(t *testing.T) {
	dir := t.TempDir()
	sourceFile := path.Join(dir, "hd-src")
	targetFile := path.Join(dir, "hd.exe")

	installer := &Installer{
		Name:   "hd",
		Execer: &fakeruntime.FakeExecer{ExpectOS: fakeruntime.OSWindows},
	}

	// fresh install puts the new binary in place and drops the source
	assert.NoError(t, os.WriteFile(sourceFile, []byte("new-binary"), 0755))
	assert.NoError(t, installer.OverWriteBinary(sourceFile, targetFile))
	content, err := os.ReadFile(targetFile)
	assert.NoError(t, err)
	assert.Equal(t, "new-binary", string(content))
	_, err = os.Stat(sourceFile)
	assert.True(t, os.IsNotExist(err))

	// a smaller binary leaves no tail of the bigger one it replaces, the
	// stale backup of an earlier upgrade is cleaned and nothing is left
	// behind
	assert.NoError(t, os.WriteFile(sourceFile, []byte("v2"), 0755))
	assert.NoError(t, os.WriteFile(targetFile+".old", []byte("stale"), 0755))
	assert.NoError(t, installer.OverWriteBinary(sourceFile, targetFile))
	content, err = os.ReadFile(targetFile)
	assert.NoError(t, err)
	assert.Equal(t, "v2", string(content))
	for _, leftover := range []string{targetFile + ".old", targetFile + ".new"} {
		_, err = os.Stat(leftover)
		assert.True(t, os.IsNotExist(err), leftover+" should be gone")
	}

	// a failed install keeps the current binary in place
	assert.NoError(t, os.WriteFile(targetFile, []byte("current"), 0755))
	assert.Error(t, installer.OverWriteBinary(path.Join(dir, "missing"), targetFile))
	content, err = os.ReadFile(targetFile)
	assert.NoError(t, err)
	assert.Equal(t, "current", string(content))
	_, err = os.Stat(targetFile + ".old")
	assert.True(t, os.IsNotExist(err), "the backup goes back on failure")
}

func TestInstall(t *testing.T) {
	tests := []struct {
		name      string
		installer *Installer
		hasErr    bool
	}{{
		name: "empty",
		installer: &Installer{
			Execer: &fakeruntime.FakeExecer{},
		},
		hasErr: true,
	}, {
		name: "fake linux",
		installer: &Installer{
			Execer: &fakeruntime.FakeExecer{
				ExpectOS: fakeruntime.OSLinux,
			},
		},
		hasErr: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.installer.Install()
			if tt.hasErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
