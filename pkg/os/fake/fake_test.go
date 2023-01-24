package fake

import (
	"github.com/linuxsuren/http-downloader/pkg/os/core"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewFakeInstaller(t *testing.T) {
	var installer core.Installer = NewFakeInstaller(true, true)

	assert.True(t, installer.Available())
	assert.NotNil(t, installer.Install())
	assert.NotNil(t, installer.Uninstall())
	assert.NotNil(t, installer.Start())
	assert.NotNil(t, installer.Stop())

	ok, err := installer.WaitForStart()
	assert.True(t, ok)
	assert.NotNil(t, err)
}
