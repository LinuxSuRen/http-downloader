package cmd

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_newInstallCmd(t *testing.T) {
	cmd := newInstallCmd(context.Background())
	assert.Equal(t, "install", cmd.Name())

	flags := []struct {
		name      string
		shorthand string
	}{{
		name:      "category",
		shorthand: "c",
	}, {
		name: "show-progress",
	}, {
		name: "accept-preRelease",
	}, {
		name: "pre",
	}, {
		name: "from-source",
	}, {
		name: "from-branch",
	}, {
		name: "goget",
	}, {
		name: "download",
	}, {
		name:      "force",
		shorthand: "f",
	}, {
		name: "clean-package",
	}, {
		name:      "thread",
		shorthand: "t",
	}, {
		name: "keep-part",
	}, {
		name: "os",
	}, {
		name: "arch",
	}}
	for i := range flags {
		tt := flags[i]
		t.Run(tt.name, func(t *testing.T) {
			flag := cmd.Flag(tt.name)
			assert.NotNil(t, flag)
			assert.NotEmpty(t, flag.Usage)
			assert.Equal(t, tt.shorthand, flag.Shorthand)
		})
	}
}
