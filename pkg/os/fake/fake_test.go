package fake

import (
	"testing"

	"github.com/linuxsuren/http-downloader/pkg/os/core"
	"github.com/stretchr/testify/assert"
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

	proxyAbleInstaller := &Installer{}
	proxyAbleInstaller.SetURLReplace(map[string]string{
		"key": "value",
	})
	assert.Equal(t, map[string]string{
		"key": "value",
	}, proxyAbleInstaller.data)

	var _ core.ProxyAble = &Installer{}
}
