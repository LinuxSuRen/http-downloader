package docker

import (
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
			assert.Nil(t, i.Start())
			assert.Nil(t, i.Stop())
			assert.Nil(t, i.Install())
			assert.Nil(t, i.Uninstall())
			ok, err := i.WaitForStart()
			assert.Nil(t, err)
			assert.True(t, ok)
		})
	})
}
