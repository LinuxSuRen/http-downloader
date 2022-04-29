package cmd

import (
	"context"
	cotesting "github.com/linuxsuren/cobra-extension/pkg/testing"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_newInstallCmd(t *testing.T) {
	cmd := newInstallCmd(context.Background())
	assert.Equal(t, "install", cmd.Name())

	test := cotesting.FlagsValidation{{
		Name:      "category",
		Shorthand: "c",
	}, {
		Name: "show-progress",
	}, {
		Name: "accept-preRelease",
	}, {
		Name: "pre",
	}, {
		Name: "from-source",
	}, {
		Name: "from-branch",
	}, {
		Name: "goget",
	}, {
		Name: "download",
	}, {
		Name:      "force",
		Shorthand: "f",
	}, {
		Name: "clean-package",
	}, {
		Name:      "thread",
		Shorthand: "t",
	}, {
		Name: "keep-part",
	}, {
		Name: "os",
	}, {
		Name: "arch",
	}, {
		Name: "proxy-github",
	}, {
		Name: "fetch",
	}, {
		Name: "provider",
	}}
	test.Valid(t, cmd.Flags())
}
