package installer

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/linuxsuren/http-downloader/pkg/common"
)

const (
	// OCIDefaultTag is the default tag of the config OCI artifact
	OCIDefaultTag = "latest"

	mediaTypeOCIManifest = "application/vnd.oci.image.manifest.v1+json"
	mediaTypeOCIIndex    = "application/vnd.oci.image.index.v1+json"
	mediaTypeDockerMan   = "application/vnd.docker.distribution.manifest.v2+json"
	mediaTypeDockerList  = "application/vnd.docker.distribution.manifest.list.v2+json"
	mediaTypeLayerTarGz  = "application/vnd.oci.image.layer.v1.tar+gzip"
)

// OCIArtifactRepo is the default OCI artifact repository of the hd config.
// It's a variable to allow overriding in tests.
var OCIArtifactRepo = "ghcr.io/linuxsuren/hd-home-config"

// ociFetcher is a minimal OCI distribution client which pulls the hd config
// artifact from an OCI compatible registry. It only depends on the standard
// library, neither docker nor oras is required.
type ociFetcher struct {
	ctx    context.Context
	base   string // for example https://ghcr.io
	repo   string // for example linuxsuren/hd-home-config
	token  string
	client *http.Client
}

// FetchOCIConfig fetches the latest hd config from an OCI registry.
// The artifact parameter could be empty (the default repository), a repository
// reference like <registry>/<repository> or with a tag suffix :<tag>.
// The tag parameter is the fallback tag when the artifact does not carry one.
func (f *DefaultFetcher) FetchOCIConfig(artifact, tag string, progress io.Writer) (err error) {
	var configDir string
	if configDir, err = f.GetConfigDir(); err != nil {
		return
	}

	ref := OCIArtifactRepo
	if artifact != "" && artifact != "github" && artifact != "gitee" {
		if idx := strings.LastIndexByte(artifact, ':'); idx > 0 && !strings.Contains(artifact[idx:], "/") {
			ref, tag = artifact[:idx], artifact[idx+1:]
		} else {
			ref = artifact
		}
	}
	if tag == "" {
		tag = OCIDefaultTag
	}

	base := "https://"
	if strings.Contains(ref, "://") {
		// an explicit schema is mainly for testing purpose
		idx := strings.Index(ref, "://")
		base, ref = ref[:idx+3], ref[idx+3:]
	} else if host := ref[:strings.IndexByte(ref, '/')]; host == "localhost" ||
		strings.HasPrefix(host, "127.0.0.1") || strings.HasPrefix(host, "[::1]") {
		base = "http://"
	}

	var registry, repo string
	if idx := strings.IndexByte(ref, '/'); idx > 0 && strings.ContainsAny(ref[:idx], ".:") {
		registry, repo = ref[:idx], ref[idx+1:]
	} else {
		err = fmt.Errorf("invalid OCI artifact reference '%s', expect <registry>/<repository>[:<tag>]", artifact)
		return
	}
	base += registry

	fetcher := &ociFetcher{
		ctx:    f.ctx,
		base:   base,
		repo:   repo,
		client: http.DefaultClient,
	}
	if fetcher.ctx == nil {
		fetcher.ctx = context.Background()
	}

	_, _ = fmt.Fprintf(progress, "fetch config from OCI registry: %s/%s:%s\n", registry, repo, tag)
	if err = fetcher.auth(); err != nil {
		return
	}

	var layer *ociLayer
	if layer, err = fetcher.findConfigLayer(tag); err != nil {
		return
	}

	if err = os.MkdirAll(configDir, 0o750); err != nil {
		err = fmt.Errorf("failed to create config directory %s: %v", configDir, err)
		return
	}
	if layer, err = fetcher.fetchLayer(layer, configDir, progress); err != nil {
		return
	}

	// take over a legacy git-cloned config directory, otherwise the stale
	// .git metadata may drive other commands to sync it back and wipe the
	// files we just unpacked
	if ok, _ := common.PathExists(filepath.Join(configDir, ".git")); ok {
		_ = os.RemoveAll(filepath.Join(configDir, ".git"))
		_, _ = fmt.Fprintln(progress, "removed the legacy git metadata of the config directory")
	}
	return
}

type ociLayer struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

type ociManifest struct {
	MediaType    string     `json:"mediaType"`
	ArtifactType string     `json:"artifactType"`
	Config       ociLayer   `json:"config"`
	Layers       []ociLayer `json:"layers"`
	Manifests    []ociLayer `json:"manifests"`
}

func (f *ociFetcher) auth() (err error) {
	api := fmt.Sprintf("%s/v2/", f.base)
	var resp *http.Response
	if resp, err = f.do(http.MethodGet, api, ""); err != nil {
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		return // registry allows anonymous access without a token
	}
	if resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf("unexpected status %d from %s", resp.StatusCode, api)
	}

	challenge := resp.Header.Get("Www-Authenticate")
	if !strings.HasPrefix(challenge, "Bearer ") {
		return fmt.Errorf("unsupported auth challenge: %s", challenge)
	}
	params := map[string]string{}
	for _, part := range strings.Split(strings.TrimPrefix(challenge, "Bearer "), ",") {
		if idx := strings.IndexByte(part, '='); idx > 0 {
			params[strings.TrimSpace(part[:idx])] = strings.Trim(strings.TrimSpace(part[idx+1:]), `"`)
		}
	}
	realm := params["realm"]
	if realm == "" {
		return fmt.Errorf("no realm found in auth challenge: %s", challenge)
	}

	query := url.Values{}
	query.Set("service", params["service"])
	query.Set("scope", fmt.Sprintf("repository:%s:pull", f.repo))
	tokenURL := realm + "?" + query.Encode()

	if resp, err = f.do(http.MethodGet, tokenURL, ""); err != nil {
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get the registry token, status %d from %s", resp.StatusCode, tokenURL)
	}

	var payload struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return
	}
	if payload.Token == "" {
		payload.Token = payload.AccessToken
	}
	if payload.Token == "" {
		return fmt.Errorf("no token found in the response of %s", tokenURL)
	}
	f.token = payload.Token
	return
}

func (f *ociFetcher) do(method, api, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(f.ctx, method, api, nil)
	if err != nil {
		return nil, err
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if f.token != "" {
		req.Header.Set("Authorization", "Bearer "+f.token)
	}
	return f.client.Do(req)
}

func (f *ociFetcher) findConfigLayer(ref string) (layer *ociLayer, err error) {
	manifest, err := f.getManifest(ref)
	if err != nil {
		return
	}

	// an index references multiple manifests, pick the hd config artifact
	if manifest.MediaType == mediaTypeOCIIndex || manifest.MediaType == mediaTypeDockerList {
		for _, item := range manifest.Manifests {
			if item.Digest == "" {
				continue
			}
			var child *ociManifest
			if child, err = f.getManifest(item.Digest); err != nil {
				return
			}
			if len(child.Layers) > 0 && (child.ArtifactType != "" ||
				child.MediaType == mediaTypeOCIManifest || child.MediaType == mediaTypeDockerMan) {
				manifest = child
				break
			}
		}
	}

	for i := range manifest.Layers {
		mediaType := manifest.Layers[i].MediaType
		if mediaType == mediaTypeLayerTarGz || strings.HasSuffix(mediaType, "tar+gzip") ||
			mediaType == "application/vnd.docker.image.rootfs.diff.tar.gzip" {
			return &manifest.Layers[i], nil
		}
	}
	err = fmt.Errorf("no config layer found in the artifact %s/%s:%s", f.base, f.repo, ref)
	return
}

func (f *ociFetcher) getManifest(ref string) (manifest *ociManifest, err error) {
	api := fmt.Sprintf("%s/v2/%s/manifests/%s", f.base, f.repo, ref)
	accept := strings.Join([]string{mediaTypeOCIIndex, mediaTypeOCIManifest,
		mediaTypeDockerList, mediaTypeDockerMan}, ", ")

	var resp *http.Response
	if resp, err = f.do(http.MethodGet, api, accept); err != nil {
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get the manifest %s:%s, status %d", f.repo, ref, resp.StatusCode)
	}

	manifest = &ociManifest{}
	if err = json.NewDecoder(resp.Body).Decode(manifest); err != nil {
		return
	}
	return
}

func (f *ociFetcher) fetchLayer(layer *ociLayer, configDir string, progress io.Writer) (result *ociLayer, err error) {
	api := fmt.Sprintf("%s/v2/%s/blobs/%s", f.base, f.repo, layer.Digest)

	var resp *http.Response
	if resp, err = f.do(http.MethodGet, api, layer.MediaType); err != nil {
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download the layer %s, status %d", layer.Digest, resp.StatusCode)
	}

	hasher := sha256.New()
	reader := io.TeeReader(resp.Body, hasher)

	if err = unpackConfigArchive(reader, configDir, progress); err != nil {
		return
	}

	expect := strings.TrimPrefix(layer.Digest, "sha256:")
	if actual := hex.EncodeToString(hasher.Sum(nil)); actual != expect {
		err = fmt.Errorf("digest mismatch of the config layer, expect %s, got %s", expect, actual)
	}
	return
}

// unpackConfigArchive unpacks the config archive into the given directory.
// Only the files under the top-level config/ directory are accepted.
func unpackConfigArchive(reader io.Reader, configDir string, progress io.Writer) (err error) {
	var gzReader *gzip.Reader
	if gzReader, err = gzip.NewReader(reader); err != nil {
		err = fmt.Errorf("the config layer is not a gzip archive: %v", err)
		return
	}
	defer func() { _ = gzReader.Close() }()

	// the archive holds a top-level config/ directory, unpack it in a
	// replace mode to drop the config files which were removed upstream
	configRoot := filepath.Join(configDir, "config")
	if ok, _ := common.PathExists(configRoot); ok {
		if err = os.RemoveAll(configRoot); err != nil {
			return
		}
		_, _ = fmt.Fprintf(progress, "replaced the config directory: %s\n", configRoot)
	}

	count := 0
	tarReader := tar.NewReader(gzReader)
	var header *tar.Header
	for header, err = tarReader.Next(); err == nil; header, err = tarReader.Next() {
		name := filepath.Clean(header.Name)
		if strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			return fmt.Errorf("illegal file path in the config archive: %s", header.Name)
		}
		if !strings.HasPrefix(name, "config/") && name != "config" {
			continue
		}

		target := filepath.Join(configDir, name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err = os.MkdirAll(target, 0o750); err != nil {
				return
			}
		case tar.TypeReg:
			if err = os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
				return
			}
			var file *os.File
			if file, err = os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY,
				os.FileMode(header.Mode)&0o755); err != nil {
				return
			}
			if _, err = io.Copy(file, tarReader); err != nil {
				_ = file.Close()
				return
			}
			_ = file.Close()
			count++
		}
	}
	if err == io.EOF {
		err = nil
	}
	if err == nil {
		_, _ = fmt.Fprintf(progress, "unpacked %d config files into: %s\n", count, configRoot)
	}
	return
}
