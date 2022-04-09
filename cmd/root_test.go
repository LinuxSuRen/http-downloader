package cmd

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewRoot(t *testing.T) {
	cmd := NewRoot(context.Background())
	assert.Equal(t, "hd", cmd.Name())
}
