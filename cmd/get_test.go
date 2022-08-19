package cmd

import (
	"context"
	"errors"
	"testing"

	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
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

func TestFetch(t *testing.T) {
	opt := &downloadOption{}
	opt.Fetch = false
	opt.fetcher = &installer.FakeFetcher{FetchLatestRepoErr: errors.New("fake")}

	// do not fetch
	assert.Nil(t, opt.fetch())

	// fetch flag is true
	opt.Fetch = true
	assert.NotNil(t, opt.fetch())

	// failed when fetching
	opt.fetcher = &installer.FakeFetcher{}
	assert.Nil(t, opt.fetch())
}

func TestPreRunE(t *testing.T) {
	opt := &downloadOption{}
	opt.Fetch = true
	opt.fetcher = &installer.FakeFetcher{FetchLatestRepoErr: errors.New("fake")}
	opt.PrintSchema = true

	// only print schema
	assert.Nil(t, opt.preRunE(nil, nil))

	// failed to fetch
	opt.PrintSchema = false
	assert.NotNil(t, opt.preRunE(nil, nil))

	// pripnt categories
	opt.fetcher = &installer.FakeFetcher{}
	opt.PrintCategories = true
	assert.Nil(t, opt.preRunE(nil, nil))

	// not args provided
	opt.PrintCategories = false
	assert.NotNil(t, opt.preRunE(nil, nil))
}

func TestRunE(t *testing.T) {
	fakeCmd := &cobra.Command{}

	opt := &downloadOption{}
	opt.fetcher = &installer.FakeFetcher{}

	// print schema
	opt.PrintSchema = true
	assert.Nil(t, opt.runE(fakeCmd, nil))
}
