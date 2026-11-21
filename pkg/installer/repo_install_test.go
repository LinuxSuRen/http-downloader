package installer

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	fakeruntime "github.com/linuxsuren/go-fake-runtime"
	"github.com/stretchr/testify/assert"
)

// recordExecer captures the commands run through the executer
type recordExecer struct {
	*fakeruntime.FakeExecer
	commands []string
	lookPath map[string]string
}

func (r *recordExecer) LookPath(name string) (string, error) {
	if path, ok := r.lookPath[name]; ok {
		return path, nil
	}
	return "", fmt.Errorf("%s not found", name)
}

func (r *recordExecer) RunCommand(name string, args ...string) error {
	r.commands = append(r.commands, strings.Join(append([]string{name}, args...), " "))
	return r.FakeExecer.ExpectError
}

func (r *recordExecer) RunCommandWithSudo(name string, args ...string) error {
	r.commands = append(r.commands, "sudo "+strings.Join(append([]string{name}, args...), " "))
	return nil
}

func newRecordExecer() *recordExecer {
	return &recordExecer{
		FakeExecer: &fakeruntime.FakeExecer{ExpectOS: "linux"},
		lookPath:   map[string]string{"dpkg": "/usr/bin/dpkg"},
	}
}

// newDebServer serves the package index and the package files of a fake
// apt mirror
func newDebServer(t *testing.T) (server *httptest.Server, configDir string) {
	t.Helper()
	deb := []byte("fake deb payload of curl")
	server, _ = newDebMirror(t, deb, sha256Of(deb))
	configDir = t.TempDir()
	writeRepoDescriptor(t, configDir, server.URL)
	return server, configDir
}

// newDebMirror serves the index with one curl package pointing to the
// given payload and checksum
func newDebMirror(t *testing.T, payload []byte, sha string) (*httptest.Server, string) {
	t.Helper()
	mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/Packages.gz"):
			gz := gzipBytes(t, fmt.Sprintf(`Package: curl
Version: 8.14.1
Section: web
Description: command line tool for transferring data with URL syntax
Filename: pool/main/c/curl/curl_8.14.1_amd64.deb
SHA256: %s
`, sha))
			_, _ = w.Write(gz)
		case strings.HasSuffix(r.URL.Path, ".deb"):
			_, _ = w.Write(payload)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(mirror.Close)
	return mirror, mirror.URL
}

func TestFindRepoPackage(t *testing.T) {
	server, configDir := newDebServer(t)

	source, pkg, err := FindRepoPackage(context.Background(), configDir, "debian-stable", "curl")
	assert.NoError(t, err)
	assert.Equal(t, "debian-stable", source.Name)
	assert.Equal(t, "apt-index", source.Kind)
	assert.Equal(t, "curl", pkg.Name)
	assert.Equal(t, "8.14.1", pkg.Version)
	assert.Equal(t, server.URL+"/pool/main/c/curl/curl_8.14.1_amd64.deb", pkg.URL)

	// an unknown source falls back to hd-home addressing
	_, _, err = FindRepoPackage(context.Background(), configDir, "no-such-source", "curl")
	assert.ErrorIs(t, err, ErrUnknownSource)

	// a registered source without the package is an error
	_, _, err = FindRepoPackage(context.Background(), configDir, "debian-stable", "no-such-package")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "not found in source")
}

func TestInstallAptPackage(t *testing.T) {
	deb := []byte("fake deb payload of curl")
	_, mirrorURL := newDebMirror(t, deb, sha256Of(deb))
	configDir := t.TempDir()
	writeRepoDescriptor(t, configDir, mirrorURL)

	source := RepoSource{Name: "debian-stable", Kind: "apt-index", Mirror: mirrorURL, Suite: "stable"}
	pkg := RepoPackage{Name: "curl", URL: mirrorURL + "/pool/main/c/curl/curl_8.14.1_amd64.deb",
		SHA256: sha256Of(deb)}
	execer := newRecordExecer()

	// the index checksum guards the download, dpkg owns the install
	err := InstallRepoPackage(context.Background(), source, pkg, execer)
	assert.NoError(t, err)
	assert.Len(t, execer.commands, 1)
	assert.True(t, strings.HasPrefix(execer.commands[0], "sudo dpkg -i "), execer.commands[0])
	assert.True(t, strings.HasSuffix(execer.commands[0], ".deb"))

	// a broken checksum stops before dpkg runs
	broken := RepoPackage{Name: "curl", URL: pkg.URL, SHA256: "deadbeef"}
	execer = newRecordExecer()
	err = InstallRepoPackage(context.Background(), source, broken, execer)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "checksum mismatch")
	assert.Empty(t, execer.commands)

	// without dpkg there is nothing hd can do on this box
	execer = &recordExecer{FakeExecer: &fakeruntime.FakeExecer{ExpectOS: "linux"}, lookPath: map[string]string{}}
	err = InstallRepoPackage(context.Background(), source, pkg, execer)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "dpkg is not available")

	// sources without an install channel answer clearly
	unsupported := RepoSource{Name: "homebrew", Kind: "brew-api"}
	err = InstallRepoPackage(context.Background(), unsupported, pkg, execer)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "is not supported yet")
}

func TestUninstallRepoPackage(t *testing.T) {
	deb := []byte("fake deb payload")
	_, mirrorURL := newDebMirror(t, deb, sha256Of(deb))
	configDir := t.TempDir()
	writeRepoDescriptor(t, configDir, mirrorURL)

	execer := newRecordExecer()
	handled, err := UninstallRepoPackage(configDir, execer, InstalledPackage{
		Name: "curl", Source: "debian-stable", Native: true})
	assert.NoError(t, err)
	assert.True(t, handled)
	assert.Equal(t, []string{"sudo dpkg -r curl"}, execer.commands)

	// a record without a source keeps the manual hint
	handled, err = UninstallRepoPackage(configDir, execer, InstalledPackage{Name: "x", Native: true})
	assert.NoError(t, err)
	assert.False(t, handled)
}

// writeChocoDescriptor registers a fake Chocolatey source in the config
// cache
func writeChocoDescriptor(t *testing.T, configDir string) {
	t.Helper()
	assert.NoError(t, os.MkdirAll(filepath.Join(configDir, "repos"), 0750))
	assert.NoError(t, os.WriteFile(filepath.Join(configDir, "repos", "chocolatey.yml"),
		[]byte("name: chocolatey\nkind: choco-api\napiURL: https://example.com/api/v2\n"), 0644))
}

// newChocoExecer knows the Chocolatey CLI and records what runs through it
func newChocoExecer() *recordExecer {
	return &recordExecer{
		FakeExecer: &fakeruntime.FakeExecer{ExpectOS: "windows"},
		lookPath:   map[string]string{"choco": `C:\ProgramData\chocolatey\bin\choco.exe`},
	}
}

func TestInstallChocoPackage(t *testing.T) {
	chocoSource := RepoSource{Name: "chocolatey", Kind: "choco-api", APIURL: "https://example.com/api/v2"}

	// the Chocolatey CLI is a hard requirement, nothing runs without it
	execer := &recordExecer{FakeExecer: &fakeruntime.FakeExecer{ExpectOS: "windows"}, lookPath: map[string]string{}}
	err := InstallRepoPackage(context.Background(), chocoSource, RepoPackage{Name: "7zip"}, execer)
	assert.EqualError(t, err, "choco is not available, please install it first")
	assert.Empty(t, execer.commands)

	// the version the search reported is pinned, so the installed record
	// matches the package on disk
	execer = newChocoExecer()
	assert.NoError(t, InstallRepoPackage(context.Background(), chocoSource,
		RepoPackage{Name: "7zip", Version: "24.8.0"}, execer))
	assert.Equal(t, []string{"choco install 7zip --yes --version 24.8.0"}, execer.commands)

	// without a version choco picks its latest one
	assert.NoError(t, InstallRepoPackage(context.Background(), chocoSource,
		RepoPackage{Name: "7zip"}, execer))
	assert.Equal(t, []string{
		"choco install 7zip --yes --version 24.8.0",
		"choco install 7zip --yes",
	}, execer.commands)

	// a failing choco command surfaces its error
	execer = newChocoExecer()
	execer.FakeExecer.ExpectError = errors.New("access denied")
	assert.Error(t, InstallRepoPackage(context.Background(), chocoSource,
		RepoPackage{Name: "7zip"}, execer))
}

func TestUninstallRepoPackageChoco(t *testing.T) {
	configDir := t.TempDir()
	writeChocoDescriptor(t, configDir)
	tool := InstalledPackage{Name: "7zip", Source: "chocolatey", Native: true}

	// choco available: the source owns the channel and removes the
	// package itself
	execer := newChocoExecer()
	handled, err := UninstallRepoPackage(configDir, execer, tool)
	assert.True(t, handled)
	assert.NoError(t, err)
	assert.Equal(t, []string{"choco uninstall 7zip --yes"}, execer.commands)

	// choco missing: the channel exists but cannot run, the guidance
	// stays actionable
	execer = &recordExecer{FakeExecer: &fakeruntime.FakeExecer{ExpectOS: "windows"}, lookPath: map[string]string{}}
	handled, err = UninstallRepoPackage(configDir, execer, tool)
	assert.True(t, handled)
	assert.EqualError(t, err, "choco is not available, please remove 7zip manually")
}

// sha256Of returns the hex checksum of the fixture payload
func sha256Of(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// gzipBytes packs an index fixture
func gzipBytes(t *testing.T, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	_, _ = writer.Write([]byte(content))
	assert.NoError(t, writer.Close())
	return buf.Bytes()
}
