package cmd

import (
	"context"
	"errors"
	"testing"

	"github.com/linuxsuren/http-downloader/pkg/exec"
	"github.com/stretchr/testify/assert"
)

func TestNewRoot(t *testing.T) {
	cmd := NewRoot(context.Background())
	assert.Equal(t, "hd", cmd.Name())
}

func TestShouldInstall(t *testing.T) {
	opt := &installOption{
		execer: &exec.FakeExecer{},
		tool:   "fake",
	}
	should, exist := opt.shouldInstall()
	assert.False(t, should)
	assert.True(t, exist)

	// force to install
	opt.force = true
	should, exist = opt.shouldInstall()
	assert.True(t, should)
	assert.True(t, exist)

	// not exist
	opt.execer = &exec.FakeExecer{ExpectError: errors.New("fake")}
	should, exist = opt.shouldInstall()
	assert.True(t, should)
	assert.False(t, exist)
}
