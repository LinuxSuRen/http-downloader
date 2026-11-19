package installer

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// debianPackagesIndex carries the rfc822 stanza form of the Debian
// Packages indexes: continuation lines extend the previous field
const debianPackagesIndex = `Package: curl
Version: 8.14.1-2+deb13u7
Architecture: amd64
Section: web
Description: command line tool for transferring data with URL syntax
 The curl tool transfers data from or to a server.
 .
 This package handles the HTTP protocol.
Filename: pool/main/c/curl/curl_8.14.1-2+deb13u7_amd64.deb
Size: 716704
SHA256: 6df1735b66f5b6fdd7e4486ba1c8a60e

Package: curlftpfs
Version: 0.9.2-10
Architecture: amd64
Section: utils
Description: filesystem to access FTP hosts based on FUSE and cURL
Filename: pool/main/c/curlftpfs/curlftpfs_0.9.2-10_amd64.deb
SHA256: abc1735b66f5b6fdd7e4486ba1c8a60d

Package: tree
Version: 2.1.1-2
Architecture: amd64
Section: utils
Description: displays directory tree, in (classic) format
Filename: pool/main/t/tree/tree_2.1.1-2_amd64.deb
SHA256: def1735b66f5b6fdd7e4486ba1c8a60e
`

// newAptIndexServer serves a Packages index for any architecture path
// and counts the downloads
func newAptIndexServer() (server *httptest.Server, downloads *int) {
	count := 0
	downloads = &count
	gz := &bytes.Buffer{}
	writer := gzip.NewWriter(gz)
	_, _ = writer.Write([]byte(debianPackagesIndex))
	_ = writer.Close()

	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet ||
			fmt.Sprintf("/dists/stable/main/binary-%s/Packages.gz", debianArchOf(runtime.GOARCH)) != r.URL.Path {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		count++
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write(gz.Bytes())
	}))
	return server, downloads
}

func writeRepoDescriptor(t *testing.T, configDir, mirror string) {
	t.Helper()
	assert.NoError(t, os.MkdirAll(filepath.Join(configDir, "repos"), 0750))
	assert.NoError(t, os.WriteFile(filepath.Join(configDir, "repos", "debian.yml"), []byte(`
name: debian-stable
kind: apt-index
mirror: `+mirror+`
suite: stable
components:
  - main
platforms: [linux, darwin, windows]
`), 0644))
}

func TestFindRepoSources(t *testing.T) {
	// no descriptors at all
	assert.Empty(t, FindRepoSources(t.TempDir()))

	server, _ := newAptIndexServer()
	defer server.Close()
	configDir := t.TempDir()
	writeRepoDescriptor(t, configDir, server.URL)

	sources := FindRepoSources(configDir)
	assert.Len(t, sources, 1)
	assert.Equal(t, "debian-stable", sources[0].Name)
	assert.Equal(t, "apt-index", sources[0].Kind)
	assert.Equal(t, []string{"main"}, sources[0].Components)

	// a broken or anonymous descriptor is skipped
	assert.NoError(t, os.WriteFile(filepath.Join(configDir, "repos", "broken.yml"),
		[]byte("not a mapping: ["), 0644))
	assert.NoError(t, os.WriteFile(filepath.Join(configDir, "repos", "anonymous.yml"),
		[]byte("kind: apt-index\n"), 0644))
	assert.Len(t, FindRepoSources(configDir), 1)
}

func TestRepoSourceSearch(t *testing.T) {
	server, downloads := newAptIndexServer()
	defer server.Close()
	configDir := t.TempDir()
	writeRepoDescriptor(t, configDir, server.URL)
	source := FindRepoSources(configDir)[0]

	packages, err := source.Search(context.Background(), configDir, "curl")
	assert.NoError(t, err)
	// the name hits lead, the description hit (tree does not match) keeps out
	assert.Len(t, packages, 2)

	first := packages[0]
	assert.Equal(t, "debian-stable", first.Source)
	assert.Equal(t, "curl", first.Name)
	assert.Equal(t, "8.14.1-2+deb13u7", first.Version)
	assert.Equal(t, "web", first.Section)
	assert.Equal(t, "command line tool for transferring data with URL syntax", first.Description)
	assert.Equal(t, server.URL+"/pool/main/c/curl/curl_8.14.1-2+deb13u7_amd64.deb", first.URL)
	assert.Equal(t, "6df1735b66f5b6fdd7e4486ba1c8a60e", first.SHA256)

	// a keyword of the description matches too
	packages, err = source.Search(context.Background(), configDir, "directory tree")
	assert.NoError(t, err)
	assert.Len(t, packages, 1)
	assert.Equal(t, "tree", packages[0].Name)

	// the second search reuses the cached index
	assert.Equal(t, 1, *downloads)

	// an aged cache entry downloads again
	cacheDir := filepath.Join(configDir, ".cache", "repos")
	files, err := os.ReadDir(cacheDir)
	assert.NoError(t, err)
	assert.Len(t, files, 1)
	old := time.Now().Add(-2 * repoIndexTTL)
	assert.NoError(t, os.Chtimes(filepath.Join(cacheDir, files[0].Name()), old, old))
	_, err = source.Search(context.Background(), configDir, "curl")
	assert.NoError(t, err)
	assert.Equal(t, 2, *downloads)
}

func TestRepoSourceSearchFailures(t *testing.T) {
	// a mirror without the index answers 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	source := RepoSource{Name: "debian-stable", Kind: "apt-index",
		Mirror: server.URL, Suite: "stable", Components: []string{"main"}}
	_, err := source.Search(context.Background(), t.TempDir(), "curl")
	assert.NotNil(t, err)

	// an unknown kind is rejected before any request leaves
	unknown := RepoSource{Name: "x", Kind: "unknown", Mirror: server.URL, Suite: "stable"}
	_, err = unknown.Search(context.Background(), t.TempDir(), "curl")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unsupported repo kind")
}

func TestFindRepoSourcesByPlatform(t *testing.T) {
	configDir := t.TempDir()
	repos := filepath.Join(configDir, "repos")
	assert.NoError(t, os.MkdirAll(repos, 0750))

	descriptors := map[string]string{
		// no explicit platforms: the kind decides, an apt source never
		// serves windows or darwin even without a list
		"debian.yml":     "name: debian-stable\nkind: apt-index\nmirror: https://deb.debian.org/debian\nsuite: stable\n",
		"homebrew.yml":   "name: homebrew\nkind: brew-api\napiURL: https://formulae.brew.sh/api\n",
		"chocolatey.yml": "name: chocolatey\nkind: choco-api\napiURL: https://community.chocolatey.org/api/v2\n",
		// an explicit list overrides the kind default, linuxbrew style
		"linuxbrew.yml": "name: linuxbrew\nkind: brew-api\napiURL: https://example.com/api\nplatforms: [linux]\n",
	}
	for file, body := range descriptors {
		assert.NoError(t, os.WriteFile(filepath.Join(repos, file), []byte(body), 0644))
	}

	// the kind defaults
	assert.Equal(t, []string{"linux"}, defaultPlatformsOf("apt-index"))
	assert.Equal(t, []string{"darwin"}, defaultPlatformsOf("brew-api"))
	assert.Equal(t, []string{"windows"}, defaultPlatformsOf("choco-api"))
	assert.Nil(t, defaultPlatformsOf("unknown"))

	// an explicit platforms list wins over the kind default
	assert.True(t, RepoSource{Name: "x", Kind: "brew-api", Platforms: []string{"linux"}}.servesPlatform("linux"))
	assert.False(t, RepoSource{Name: "x", Kind: "brew-api", Platforms: []string{"linux"}}.servesPlatform("darwin"))

	// FindRepoSources keeps the sources of the running platform only
	var names []string
	for _, source := range FindRepoSources(configDir) {
		names = append(names, source.Name)
	}
	switch runtime.GOOS {
	case "linux":
		assert.ElementsMatch(t, []string{"debian-stable", "linuxbrew"}, names)
	case "darwin":
		assert.ElementsMatch(t, []string{"homebrew"}, names)
	case "windows":
		assert.ElementsMatch(t, []string{"chocolatey"}, names)
	}
}

func TestRepoSourceSearchBrew(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/formula.json", r.URL.Path)
		_, _ = w.Write([]byte(`[
{"name":"curl","desc":"Get a file from an HTTP, HTTPS or FTP server","homepage":"https://curl.se","versions":{"stable":"8.14.1"}},
{"name":"curlie","desc":"The power of curl, the ease of use of httpie","homepage":"","versions":{"stable":"1.7.0"}},
{"name":"tree","desc":"Display directories as trees","homepage":"","versions":{"stable":"2.2.1"}}
]`))
	}))
	defer server.Close()

	source := RepoSource{Name: "homebrew", Kind: "brew-api", APIURL: server.URL}
	packages, err := source.Search(context.Background(), t.TempDir(), "curl")
	assert.NoError(t, err)
	assert.Len(t, packages, 2)
	assert.Equal(t, "curl", packages[0].Name)
	assert.Equal(t, "8.14.1", packages[0].Version)
	assert.Equal(t, "Get a file from an HTTP, HTTPS or FTP server", packages[0].Description)
	assert.Equal(t, "https://curl.se", packages[0].URL)
	assert.Equal(t, "curlie", packages[1].Name)

	// a description hit ranks behind the name hits
	packages, err = source.Search(context.Background(), t.TempDir(), "trees")
	assert.NoError(t, err)
	assert.Len(t, packages, 1)
	assert.Equal(t, "tree", packages[0].Name)
}

func TestRepoSourceSearchChoco(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/Search()", r.URL.Path)
		assert.Equal(t, "'curl'", r.URL.Query().Get("searchTerm"))
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:m="http://schemas.microsoft.com/ado/2007/08/dataservices/metadata" xmlns:d="http://schemas.microsoft.com/ado/2007/08/dataservices">
  <title type="text">Search</title>
  <entry>
    <title type="text">curl</title>
    <summary type="text"> command line tool for transferring data  </summary>
    <content type="application/zip" src="` + server.URL + `/package/curl/8.23.0"/>
    <m:properties>
      <d:Version>8.23.0</d:Version>
      <d:PackageID>curl</d:PackageID>
    </m:properties>
  </entry>
</feed>`))
	}))
	defer server.Close()

	source := RepoSource{Name: "chocolatey", Kind: "choco-api", APIURL: server.URL}
	packages, err := source.Search(context.Background(), t.TempDir(), "curl")
	assert.NoError(t, err)
	assert.Len(t, packages, 1)
	assert.Equal(t, "curl", packages[0].Name)
	assert.Equal(t, "8.23.0", packages[0].Version)
	assert.Equal(t, "command line tool for transferring data", packages[0].Description)
	assert.Equal(t, server.URL+"/package/curl/8.23.0", packages[0].URL)
}
