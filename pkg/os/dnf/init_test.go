package dnf

import (
	"errors"
	"testing"

	fakeruntime "github.com/linuxsuren/go-fake-runtime"
	"github.com/linuxsuren/http-downloader/pkg/os/core"
	"github.com/stretchr/testify/assert"
)

func TestCommonCase(t *testing.T) {
	registry := &core.FakeRegistry{}
	SetInstallerRegistry(registry, fakeruntime.FakeExecer{
		ExpectOS: "linux",
	})

	registry.Walk(func(s string, i core.Installer) {
		t.Run(s, func(t *testing.T) {
			assert.True(t, i.Available())
			assert.Nil(t, i.Uninstall())
			assert.Nil(t, i.Install())
			assert.Nil(t, i.Start())
			assert.Nil(t, i.Stop())
		})
	})

	errRegistry := &core.FakeRegistry{}
	SetInstallerRegistry(errRegistry, fakeruntime.FakeExecer{
		ExpectLookPathError: errors.New("error"),
		ExpectError:         errors.New("error"),
		ExpectOS:            "linux",
	})
	errRegistry.Walk(func(s string, i core.Installer) {
		t.Run(s, func(t *testing.T) {
			assert.False(t, i.Available())
			assert.NotNil(t, i.Uninstall())
			if s != "docker" {
				assert.NotNil(t, i.Install())
				assert.Nil(t, i.Start())
				assert.Nil(t, i.Stop())
				ok, err := i.WaitForStart()
				assert.True(t, ok)
				assert.Nil(t, err)
			}
		})
	})
}
