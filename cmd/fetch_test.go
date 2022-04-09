package cmd

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_newFetchCmd(t *testing.T) {
	cmd := newFetchCmd(context.Background())
	assert.Equal(t, "fetch", cmd.Name())

	flags := []struct {
		name      string
		shorthand string
	}{{
		name:      "branch",
		shorthand: "b",
	}, {
		name: "reset",
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
