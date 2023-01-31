package net_test

import (
	"errors"
	"testing"

	"github.com/linuxsuren/http-downloader/pkg/net"
	"github.com/stretchr/testify/assert"
)

func TestFakeReader(t *testing.T) {
	reader := &net.FakeReader{
		ExpectErr: errors.New("error"),
	}
	_, err := reader.Read(nil)
	assert.NotNil(t, err)
}
