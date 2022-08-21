package installer

import (
	"io/ioutil"
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
	installer := &Installer{}

	sourceFile := path.Join(os.TempDir(), "fake-1")
	targetFile := path.Join(os.TempDir(), "fake-2")

	ioutil.WriteFile(sourceFile, []byte("fake"), 0600)

	defer func() {
		_ = os.RemoveAll(sourceFile)
	}()
	defer func() {
		_ = os.RemoveAll(targetFile)
	}()

	assert.Nil(t, installer.OverWriteBinary(sourceFile, targetFile))
}
