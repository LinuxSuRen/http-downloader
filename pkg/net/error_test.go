package net_test

import (
	"github.com/linuxsuren/http-downloader/pkg/net"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestError(t *testing.T) {
	err := net.DownloadError{
		Message: "message",
		StatusCode: 200,
	}
	assert.Contains(t, err.Error(), "message")
	assert.Contains(t, err.Error(), "200")
}
