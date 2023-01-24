package installer

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"os/user"
	"testing"
)

func TestGetConfigDir(t *testing.T) {
	u, err := user.Current()
	assert.Nil(t, err)

	var fetcher Fetcher
	fetcher = &DefaultFetcher{}
	dir, err := fetcher.GetConfigDir()
	assert.Nil(t, err)
	assert.Equal(t, fmt.Sprintf("%s/.config/hd-home", u.HomeDir), dir)
	fetcher.SetContext(context.TODO())

	// test the fake fetcher
	fetcher = &FakeFetcher{ConfigDir: "fake"}
	dir, err = fetcher.GetConfigDir()
	assert.Nil(t, err)
	assert.Equal(t, "fake", dir)
	err = fetcher.FetchLatestRepo("", "", nil)
	assert.Nil(t, err)
	fetcher.SetContext(context.TODO())
}
