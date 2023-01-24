package apt

import (
	"errors"
	"testing"

	"github.com/linuxsuren/http-downloader/pkg/exec"
	"github.com/linuxsuren/http-downloader/pkg/os/core"
	"github.com/stretchr/testify/assert"
)

func TestCommonCase(t *testing.T) {
	registry := &core.FakeRegistry{}
	SetInstallerRegistry(registry, exec.FakeExecer{
		ExpectOS: "linux",
	})

	registry.Walk(func(s string, i core.Installer) {
		t.Run(s, func(t *testing.T) {
			assert.True(t, i.Available())
			assert.Nil(t, i.Uninstall())
			if s != "docker" {
				assert.Nil(t, i.Install())
				assert.Nil(t, i.Start())
				assert.Nil(t, i.Stop())
				ok, err := i.WaitForStart()
				assert.True(t, ok)
				assert.Nil(t, err)
			}
		})
	})

	errRegistry := &core.FakeRegistry{}
	SetInstallerRegistry(errRegistry, exec.FakeExecer{
		ExpectError: errors.New("error"),
		ExpectOS:    "linux",
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
