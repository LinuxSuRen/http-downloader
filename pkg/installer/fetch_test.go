package installer

import (
	"context"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetConfigDir(t *testing.T) {
	u, err := user.Current()
	assert.Nil(t, err)

	var fetcher Fetcher
	fetcher = &DefaultFetcher{}
	dir, err := fetcher.GetConfigDir()
	assert.Nil(t, err)
	assert.Equal(t, filepath.Join(u.HomeDir, ".config", "hd-home"), dir)
	fetcher.SetContext(context.TODO())

	// test the fake fetcher
	fetcher = &FakeFetcher{ConfigDir: "fake"}
	dir, err = fetcher.GetConfigDir()
	assert.Nil(t, err)
	assert.Equal(t, "fake", dir)
	err = fetcher.FetchLatestRepo("", "", nil)
	assert.Nil(t, err)
	fetcher.SetContext(context.TODO())

	dir, err = fetcher.GetHomeDir()
	assert.Equal(t, "", dir)
	assert.Nil(t, err)
}
