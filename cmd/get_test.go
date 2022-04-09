package cmd

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_newGetCmd(t *testing.T) {
	cmd := newGetCmd(context.Background())
	assert.Equal(t, "get", cmd.Name())

	flags := []struct {
		name      string
		shorthand string
	}{{
		name:      "output",
		shorthand: "o",
	}, {
		name: "pre",
	}, {
		name: "time",
	}, {
		name: "max-attempts",
	}, {
		name: "show-progress",
	}, {
		name: "continue-at",
	}, {
		name: "keep-part",
	}, {
		name: "os",
	}, {
		name: "arch",
	}, {
		name: "print-schema",
	}, {
		name: "print-version",
	}, {
		name: "print-categories",
	}, {
		name: "print-version-count",
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
