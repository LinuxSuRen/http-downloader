package installer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxsuren/http-downloader/pkg/source"
	"github.com/stretchr/testify/assert"
)

func seedSearchConfig(t *testing.T, configDir, org, repo, content string) {
	t.Helper()
	dir := filepath.Join(configDir, "config", org)
	assert.NoError(t, os.MkdirAll(dir, 0750))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, repo+".yml"), []byte(content), 0644))
}

func TestSearchSourcesHDHome(t *testing.T) {
	configDir := t.TempDir()
	seedSearchConfig(t, configDir, "fake-org", "fake-cli", "name: fake-cli\nhomepage: https://example.com/fake-cli\n")
	seedSearchConfig(t, configDir, "other-org", "unrelated", "name: unrelated\n")

	results := SearchSources("fake", configDir, source.HDHome)
	assert.Len(t, results, 1)
	assert.Equal(t, source.HDHome, results[0].Source)
	assert.Equal(t, "fake-org", results[0].Org)
	assert.Equal(t, "fake-cli", results[0].Repo)
	assert.Equal(t, "fake-cli", results[0].Name)
	assert.Equal(t, "https://example.com/fake-cli", results[0].Homepage)
	assert.Equal(t, "fake-org/fake-cli", results[0].ID())
}

func TestSearchSourcesFilters(t *testing.T) {
	configDir := t.TempDir()
	seedSearchConfig(t, configDir, "fake-org", "fake-cli", "name: fake-cli\n")

	// npm only must not include the hd-home config
	results := SearchSources("fake", configDir, source.NPM)
	for _, item := range results {
		assert.NotEqual(t, source.HDHome, item.Source)
	}

	// an unknown source yields no results
	assert.Empty(t, SearchSources("fake", configDir, "unknown"))
}

func TestSourceOfInstalled(t *testing.T) {
	// old records without a source are hd-home installs
	assert.Equal(t, source.HDHome, SourceOfInstalled(""))
	assert.Equal(t, source.NPM, SourceOfInstalled(source.NPM))
	assert.Equal(t, "custom", SourceOfInstalled("custom"))
}
