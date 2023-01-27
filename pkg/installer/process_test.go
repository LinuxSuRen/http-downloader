package installer

import (
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInstallerExtractFiles(t *testing.T) {
	installer := &Installer{}

	assert.NotNil(t, installer.extractFiles("fake.fake", ""))
	assert.NotNil(t, installer.extractFiles("a.tar.gz", ""))
}

func TestOverwriteBinary(t *testing.T) {
	installer := &Installer{
		Execer: &exec.FakeExecer{},
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

func TestInstall(t *testing.T) {
	tests := []struct {
		name      string
		installer *Installer
		hasErr    bool
	}{{
		name: "empty",
		installer: &Installer{
			Execer: exec.FakeExecer{},
		},
		hasErr: true,
	}, {
		name: "fake linux",
		installer: &Installer{
			Execer: exec.FakeExecer{
				ExpectOS: exec.OSLinux,
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
