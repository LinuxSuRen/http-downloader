package installer

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxsuren/http-downloader/pkg/common"
	"github.com/mitchellh/go-homedir"
	"github.com/stretchr/testify/assert"
)

// resetHome makes the fetcher resolve the home directory again, so that
// t.Setenv("HOME", ...) takes effect (go-homedir caches the result).
func resetHome(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	homedir.Reset()
	t.Cleanup(homedir.Reset)
}

const testOCIRepo = "linuxsuren/hd-home-config"

// startFakeRegistry starts an OCI distribution compatible registry which
// serves the given config files as an artifact
func startFakeRegistry(t *testing.T, files map[string]string) (server *httptest.Server) {
	layer := createConfigArchive(t, files)
	layerDigest := fmt.Sprintf("sha256:%s", sha256hex(layer))

	manifest := map[string]any{
		"schemaVersion": 2,
		"mediaType":     mediaTypeOCIManifest,
		"artifactType":  "application/vnd.linuxsuren.hd.config.v1",
		"config": map[string]any{
			"mediaType": "application/vnd.oci.empty.v1+json",
			"digest":    "sha256:" + sha256hex([]byte("{}")),
			"size":      2,
		},
		"layers": []map[string]any{{
			"mediaType": mediaTypeLayerTarGz,
			"digest":    layerDigest,
			"size":      int64(len(layer)),
		}},
	}
	manifestData, _ := json.Marshal(manifest)

	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v2/":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == fmt.Sprintf("/v2/%s/manifests/latest", testOCIRepo):
			w.Header().Set("Content-Type", mediaTypeOCIManifest)
			_, _ = w.Write(manifestData)
		case r.URL.Path == fmt.Sprintf("/v2/%s/blobs/%s", testOCIRepo, layerDigest):
			w.Header().Set("Content-Type", mediaTypeLayerTarGz)
			_, _ = w.Write(layer)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return
}

func createConfigArchive(t *testing.T, files map[string]string) []byte {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	_ = tw.WriteHeader(&tar.Header{Name: "config", Typeflag: tar.TypeDir})
	for name, content := range files {
		_ = tw.WriteHeader(&tar.Header{
			Name:     filepath.Join("config", name),
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Size:     int64(len(content)),
		})
		_, _ = tw.Write([]byte(content))
	}
	_ = tw.Close()
	_ = gw.Close()
	return buf.Bytes()
}

func sha256hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestFetchOCIConfig(t *testing.T) {
	server := startFakeRegistry(t, map[string]string{
		"linuxsuren/tool.yml": "filename: tool",
	})

	resetHome(t)
	home, _ := homedir.Dir()

	// point the default artifact repository to the test server
	original := OCIArtifactRepo
	OCIArtifactRepo = fmt.Sprintf("%s/%s", server.URL, testOCIRepo)
	t.Cleanup(func() { OCIArtifactRepo = original })

	fetcher := &DefaultFetcher{ctx: context.Background()}
	progress := &bytes.Buffer{}
	err := fetcher.FetchOCIConfig("", "latest", progress)
	assert.NoError(t, err)
	assert.Contains(t, progress.String(), "unpacked 1 config files")

	data, err := os.ReadFile(filepath.Join(home, ".config", "hd-home",
		"config", "linuxsuren", "tool.yml"))
	assert.NoError(t, err)
	assert.Equal(t, "filename: tool", string(data))
}

func TestFetchOCIConfigTakesOverGitClone(t *testing.T) {
	server := startFakeRegistry(t, map[string]string{"a.yml": "a"})

	resetHome(t)
	home, _ := homedir.Dir()
	configDir := filepath.Join(home, ".config", "hd-home")
	assert.NoError(t, os.MkdirAll(filepath.Join(configDir, ".git"), 0o755))
	_ = os.WriteFile(filepath.Join(configDir, ".git", "HEAD"), []byte("ref: refs/heads/master"), 0o644)

	original := OCIArtifactRepo
	OCIArtifactRepo = fmt.Sprintf("%s/%s", server.URL, testOCIRepo)
	t.Cleanup(func() { OCIArtifactRepo = original })

	progress := &bytes.Buffer{}
	err := (&DefaultFetcher{ctx: context.Background()}).FetchOCIConfig("", "latest", progress)
	assert.NoError(t, err)
	assert.Contains(t, progress.String(), "removed the legacy git metadata")

	ok, _ := common.PathExists(filepath.Join(configDir, ".git"))
	assert.False(t, ok, "the legacy .git metadata should be removed")
}

func TestFetchOCIConfigDigestMismatch(t *testing.T) {
	// serve a manifest whose layer digest does not match the blob content
	layer := createConfigArchive(t, map[string]string{"x.yml": "x"})
	wrongDigest := fmt.Sprintf("sha256:%s", sha256hex([]byte("tampered")))
	manifest, _ := json.Marshal(map[string]any{
		"schemaVersion": 2,
		"mediaType":     mediaTypeOCIManifest,
		"layers": []map[string]any{{
			"mediaType": mediaTypeLayerTarGz,
			"digest":    wrongDigest,
			"size":      int64(len(layer)),
		}},
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v2/":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == fmt.Sprintf("/v2/%s/manifests/latest", testOCIRepo):
			w.Header().Set("Content-Type", mediaTypeOCIManifest)
			_, _ = w.Write(manifest)
		case r.URL.Path == fmt.Sprintf("/v2/%s/blobs/%s", testOCIRepo, wrongDigest):
			_, _ = w.Write(layer)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	resetHome(t)

	original := OCIArtifactRepo
	OCIArtifactRepo = fmt.Sprintf("%s/%s", server.URL, testOCIRepo)
	t.Cleanup(func() { OCIArtifactRepo = original })

	err := (&DefaultFetcher{ctx: context.Background()}).FetchOCIConfig("", "latest", &bytes.Buffer{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "digest mismatch")
}

func TestUnpackConfigArchiveRejectsPathTraversal(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	_ = tw.WriteHeader(&tar.Header{Name: "../../etc/passwd", Typeflag: tar.TypeReg, Size: 5})
	_, _ = tw.Write([]byte("hello"))
	_ = tw.Close()
	_ = gw.Close()

	err := unpackConfigArchive(&buf, t.TempDir(), &bytes.Buffer{})
	assert.Error(t, err)
}

func TestUnpackConfigArchiveReplaceMode(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "config", "legacy")
	assert.NoError(t, os.MkdirAll(legacy, 0o755))
	_ = os.WriteFile(filepath.Join(legacy, "old.yml"), []byte("old"), 0o644)

	archive := bytes.NewReader(createConfigArchive(t, map[string]string{"new.yml": "new"}))
	err := unpackConfigArchive(archive, dir, &bytes.Buffer{})
	assert.NoError(t, err)

	ok, _ := common.PathExists(filepath.Join(legacy, "old.yml"))
	assert.False(t, ok, "legacy config should be removed in replace mode")

	data, err := os.ReadFile(filepath.Join(dir, "config", "new.yml"))
	assert.NoError(t, err)
	assert.Equal(t, "new", string(data))
}
