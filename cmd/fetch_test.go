package cmd

import (
	"context"
	"errors"
	"os"
	"path"
	"testing"
	"time"

	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
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

func TestSetTimeoutWithContext(t *testing.T) {
	tests := []struct {
		name     string
		context  context.Context
		timeout  time.Duration
		expected bool
	}{
		{
			name:     "context with timeout",
			context:  context.Background(),
			timeout:  time.Second * 10,
			expected: true,
		},
		{
			name:     "context without timeout",
			context:  nil,
			timeout:  time.Second * 10,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.SetContext(tt.context)
			opt := &fetchOption{
				timeout: tt.timeout,
				fetcher: &installer.FakeFetcher{},
			}
			opt.setTimeout(cmd)
			if tt.expected {
				assert.NotNil(t, opt.cancel)
			} else {
				assert.Nil(t, opt.cancel)
			}
		})
	}
}

func TestFetchPreRunE(t *testing.T) {
	tests := []struct {
		name   string
		opt    *fetchOption
		hasErr bool
	}{{
		name:   "not reset",
		opt:    &fetchOption{},
		hasErr: false,
	}, {
		name: "reset, cannot get config dir",
		opt: &fetchOption{
			reset: true,
			fetcher: &installer.FakeFetcher{
				GetConfigDirErr: errors.New("no config dir"),
			},
		},
		hasErr: true,
	}, {
		name: "reset, remove dir",
		opt: &fetchOption{
			reset: true,
			fetcher: &installer.FakeFetcher{
				ConfigDir: path.Join(os.TempDir(), "hd-config"),
			},
		},
		hasErr: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &cobra.Command{}
			err := tt.opt.preRunE(c, nil)
			if tt.hasErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestFetchRunE(t *testing.T) {
	tests := []struct {
		name   string
		opt    *fetchOption
		hasErr bool
	}{
		{
			name: "normal",
			opt: &fetchOption{
				fetcher: &installer.FakeFetcher{},
			},
			hasErr: false,
		},
		{
			name: "fetch with retry",
			opt: &fetchOption{
				fetcher: &installer.FakeFetcher{
					FetchLatestRepoErr: errors.New("context deadline exceeded"),
				},
				retry: 3,
			},
			hasErr: true,
		},
		{
			name: "fetch with non-retryable error",
			opt: &fetchOption{
				fetcher: &installer.FakeFetcher{
					FetchLatestRepoErr: errors.New("some other error"),
				},
				retry: 3,
			},
			hasErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &cobra.Command{}
			err := tt.opt.runE(c, nil)
			if tt.hasErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
