package installer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mitchellh/go-homedir"
	"github.com/stretchr/testify/assert"
)

func TestRecordInstalledPackage(t *testing.T) {
	tmpHome := t.TempDir()
	oldHome := homedir.DisableCache
	homedir.DisableCache = true
	t.Setenv("HOME", tmpHome)
	defer func() {
		homedir.DisableCache = oldHome
	}()

	// no record file yet
	tools, err := LoadInstalledPackages()
	assert.NoError(t, err)
	assert.Empty(t, tools)

	// add a new record
	err = RecordInstalledPackage(InstalledPackage{
		Name:     "jcli",
		Org:      "jenkins-zh",
		Repo:     "jenkins-cli",
		Version:  "v1.0.0",
		Binaries: []string{"/usr/local/bin/jcli"},
	})
	assert.NoError(t, err)

	tools, err = LoadInstalledPackages()
	assert.NoError(t, err)
	assert.Len(t, tools, 1)
	assert.Equal(t, "jcli", tools[0].Name)

	// update the existing record
	err = RecordInstalledPackage(InstalledPackage{
		Name:    "jcli",
		Version: "v1.1.0",
	})
	assert.NoError(t, err)

	tool, index := FindInstalledPackage("jcli")
	assert.NotNil(t, tool)
	assert.GreaterOrEqual(t, index, 0)
	assert.Equal(t, "v1.1.0", tool.Version)

	// the record file is in the expected place
	_, err = os.Stat(filepath.Join(tmpHome, ".config", "hd-installed.yaml"))
	assert.NoError(t, err)

	// remove the record
	removed, err := RemoveInstalledPackage("jcli")
	assert.NoError(t, err)
	assert.NotNil(t, removed)
	assert.Equal(t, "jcli", removed.Name)

	tools, err = LoadInstalledPackages()
	assert.NoError(t, err)
	assert.Empty(t, tools)

	// remove a non-exist record
	_, err = RemoveInstalledPackage("non-exist")
	assert.Error(t, err)
}
