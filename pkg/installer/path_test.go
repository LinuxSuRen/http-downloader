package installer

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultInstallDir(t *testing.T) {
	assert.Equal(t, UnixBinDir, DefaultInstallDir("linux"))
	assert.Equal(t, UnixBinDir, DefaultInstallDir("darwin"))
	assert.Equal(t, UnixBinDir, DefaultInstallDir("Linux"))
	assert.Equal(t, WindowsBinDir, DefaultInstallDir("windows"))
	assert.Equal(t, WindowsBinDir, DefaultInstallDir("WINDOWS"))
	// no Unix path may leak into the Windows default
	assert.NotContains(t, DefaultInstallDir("windows"), "/usr")
	assert.Empty(t, DefaultInstallDir("plan9"))
}

func TestResolveInstallDir(t *testing.T) {
	tests := []struct {
		name       string
		configured string
		packageDir string
		targetOS   string
		want       string
	}{{
		name:       "configured value wins",
		configured: "D:\\tools\\bin",
		packageDir: "/opt/bin",
		targetOS:   "linux",
		want:       "D:\\tools\\bin",
	}, {
		name:       "package value wins over platform",
		packageDir: "/opt/bin",
		targetOS:   "windows",
		want:       "/opt/bin",
	}, {
		name:     "remote platform default",
		targetOS: "linux",
		want:     UnixBinDir,
	}, {
		name:     "windows platform default",
		targetOS: "windows",
		want:     WindowsBinDir,
	}, {
		name:       "whitespace is ignored",
		configured: "   ",
		targetOS:   "darwin",
		want:       UnixBinDir,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ResolveInstallDir(tt.configured, tt.packageDir, tt.targetOS))
		})
	}

	// an unknown remote platform falls back to the machine hd runs on
	got := ResolveInstallDir("", "", "plan9")
	assert.Equal(t, DefaultInstallDir(runtime.GOOS), got)
}
