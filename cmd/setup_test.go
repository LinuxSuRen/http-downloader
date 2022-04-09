package cmd

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_newSetupCommand(t *testing.T) {
	cmd := newSetupCommand()
	assert.Equal(t, "setup", cmd.Name())
}
