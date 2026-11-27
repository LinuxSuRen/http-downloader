package source

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKnown(t *testing.T) {
	for _, name := range []string{HDHome, NPM, Native, All} {
		assert.True(t, Known(name), name)
	}
	for _, name := range []string{"", "github", "HOMEBREW"} {
		assert.False(t, Known(name), name)
	}
}

func TestResultID(t *testing.T) {
	tests := []struct {
		name   string
		result Result
		want   string
	}{{
		name:   "hd-home keeps org/repo",
		result: Result{Source: HDHome, Org: "LinuxSuRen", Repo: "jcli", Name: "jcli"},
		want:   "LinuxSuRen/jcli",
	}, {
		name:   "hd-home without org",
		result: Result{Source: HDHome, Name: "jcli"},
		want:   "hd-home:jcli",
	}, {
		name:   "npm gets a prefix",
		result: Result{Source: NPM, Name: "chalk"},
		want:   "npm:chalk",
	}, {
		name:   "scoped npm package",
		result: Result{Source: NPM, Name: "@types/node"},
		want:   "npm:@types/node",
	}, {
		name:   "native package",
		result: Result{Source: Native, Name: "git"},
		want:   "git",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.result.ID())
			assert.Equal(t, tt.want, tt.result.String())
		})
	}
}
