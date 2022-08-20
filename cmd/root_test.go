package cmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRoot(t *testing.T) {
	cmd := NewRoot(context.Background())
	assert.Equal(t, "hd", cmd.Name())
}
