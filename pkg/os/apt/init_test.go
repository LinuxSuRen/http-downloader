package apt

import (
	"runtime"
	"testing"

	"github.com/linuxsuren/http-downloader/pkg/exec"
	"github.com/linuxsuren/http-downloader/pkg/os/core"
	"github.com/stretchr/testify/assert"
)

func TestCommonCase(t *testing.T) {
	registry := &core.FakeRegistry{}
	SetInstallerRegistry(registry, exec.FakeExecer{})

	registry.Walk(func(s string, i core.Installer) {
		t.Run(s, func(t *testing.T) {
			if runtime.GOOS == "linux" {
				assert.True(t, i.Available())
			} else {
				assert.False(t, i.Available())
			}

			if s != "docker" {
				assert.Nil(t, i.Start())
				assert.Nil(t, i.Stop())
			}
		})
	})
}
