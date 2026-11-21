package installer

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// repoHTTPClient is shared by the repo index downloads, a bounded timeout
// keeps a slow mirror from blocking the search
var repoHTTPClient = &http.Client{Timeout: 5 * time.Minute}

// repoIndexTTL is how long a downloaded package index stays fresh, the
// indexes of a stable suite change rarely
const repoIndexTTL = 24 * time.Hour

// RepoSource describes an external package repository which hd can
// search. The descriptors live in hd-home next to the config directory
// (repos/*.yml): they tell hd which platforms a source serves and where
// its machine readable index lives. hd consumes the same indexes the
// native package managers use, it never scrapes human facing pages, and
// it picks the sources matching the running platform.
type RepoSource struct {
	Name       string   `yaml:"name"`
	Kind       string   `yaml:"kind"`       // apt-index, brew-api, choco-api
	Mirror     string   `yaml:"mirror"`     // of apt-index: the package mirror
	APIURL     string   `yaml:"apiURL"`     // of brew-api and choco-api: the API root
	Suite      string   `yaml:"suite"`      // of apt-index
	Components []string `yaml:"components"` // of apt-index
	// Platforms limits a source to linux, darwin or windows, an empty
	// list serves every platform
	Platforms []string `yaml:"platforms"`
}

// RepoPackage is a search hit of a repo source
type RepoPackage struct {
	Source      string `json:"source"`
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	Section     string `json:"section,omitempty"`
	Description string `json:"description,omitempty"`
	// URL points to the downloadable package file on the mirror
	URL string `json:"url,omitempty"`
	// SHA256 is the checksum of the package file, kept for the install
	// step to verify the download
	SHA256 string `json:"sha256,omitempty"`
}

// FindRepoSources loads the repo descriptors of the local hd-home cache
// and keeps the ones serving the running platform, a broken descriptor
// is skipped
func FindRepoSources(configDir string) (sources []RepoSource) {
	return findRepoSources(configDir, runtime.GOOS)
}

// AllRepoSources loads every repo descriptor regardless of the running
// platform, an explicit address like debian-stable/curl resolves on any
// platform and reports the mismatch instead
func AllRepoSources(configDir string) (sources []RepoSource) {
	return findRepoSources(configDir, "")
}

func findRepoSources(configDir, osName string) (sources []RepoSource) {
	files, err := filepath.Glob(path.Join(configDir, "repos", "*.yml"))
	if err != nil {
		return
	}
	for i := range files {
		data, readErr := os.ReadFile(files[i]) // #nosec G304 -- a fixed pattern under the config cache
		if readErr != nil {
			continue
		}
		source := RepoSource{}
		if err = yaml.Unmarshal(data, &source); err != nil {
			continue
		}
		if source.Name == "" || (source.Mirror == "" && source.APIURL == "") {
			continue
		}
		if len(source.Components) == 0 && source.Kind == "apt-index" {
			source.Components = []string{"main"}
		}
		if osName == "" || source.servesPlatform(osName) {
			sources = append(sources, source)
		}
	}
	return
}

// servesPlatform tells whether the source serves the given os. The kind
// of a source carries its platform: an apt index is linux, the Homebrew
// API is darwin, the Chocolatey feed is windows, so a descriptor works
// without an explicit list. An explicit platforms list overrides the
// default, a linuxbrew style source declares darwin and linux itself.
func (s RepoSource) servesPlatform(osName string) bool {
	platforms := s.Platforms
	if len(platforms) == 0 {
		platforms = defaultPlatformsOf(s.Kind)
	}
	for i := range platforms {
		if platforms[i] == osName {
			return true
		}
	}
	return false
}

// defaultPlatformsOf returns the platforms a repo kind belongs to, an
// unknown kind serves every platform and fails later on search
func defaultPlatformsOf(kind string) []string {
	switch kind {
	case "apt-index":
		return []string{"linux"}
	case "brew-api":
		return []string{"darwin"}
	case "choco-api":
		return []string{"windows"}
	default:
		return nil
	}
}

// Search queries the repo source for a keyword, configDir is the root
// of the local hd-home cache, the downloaded indexes live under it
func (s RepoSource) Search(ctx context.Context, configDir, keyword string) (packages []RepoPackage, err error) {
	switch s.Kind {
	case "apt-index":
		return s.searchAptIndex(ctx, configDir, keyword)
	case "brew-api":
		return s.searchBrewAPI(ctx, configDir, keyword)
	case "choco-api":
		return s.searchChocoAPI(ctx, configDir, keyword)
	default:
		return nil, fmt.Errorf("unsupported repo kind: %s", s.Kind)
	}
}

// debianArchOf maps the Go architecture to the Debian architecture name
// of the binary indexes
func debianArchOf(goarch string) string {
	switch goarch {
	case "386":
		return "i386"
	case "arm":
		return "armhf"
	default:
		return goarch
	}
}

// searchAptIndex downloads (or reuses the cached) Packages indexes of
// the source and searches them locally, the same way apt does
func (s RepoSource) searchAptIndex(ctx context.Context, configDir, keyword string) (ranked []RepoPackage, err error) {
	arch := debianArchOf(runtime.GOARCH)
	var hits []RepoPackage
	for _, component := range s.Components {
		indexURL := fmt.Sprintf("%s/dists/%s/%s/binary-%s/Packages.gz",
			strings.TrimSuffix(s.Mirror, "/"), s.Suite, component, arch)
		var data io.ReadCloser
		if data, err = s.cachedIndex(ctx, configDir, indexURL); err != nil {
			return nil, err
		}
		hits = append(hits, searchPackagesIndex(s, data, keyword)...)
		_ = data.Close()
	}

	// the cap applies after the ranking so a late exact hit in the
	// index is never dropped
	return rankPackages(hits, keyword), nil
}

// rankPackages orders the hits: exact name matches lead, then the other
// name hits in index order, description matches follow. The result is
// capped at maxHits.
func rankPackages(hits []RepoPackage, keyword string) (ranked []RepoPackage) {
	const maxHits = 50
	exact, nameHits, otherHits := make([]RepoPackage, 0, 2), make([]RepoPackage, 0, 10), make([]RepoPackage, 0, 10)
	for i := range hits {
		switch {
		case hits[i].Name == keyword:
			exact = append(exact, hits[i])
		case strings.Contains(hits[i].Name, keyword):
			nameHits = append(nameHits, hits[i])
		default:
			otherHits = append(otherHits, hits[i])
		}
	}
	ranked = append(append(exact, nameHits...), otherHits...)
	if len(ranked) > maxHits {
		ranked = ranked[:maxHits]
	}
	return
}

// brewFormula carries the fields of a Homebrew formula hd cares about
type brewFormula struct {
	Name     string `json:"name"`
	Desc     string `json:"desc"`
	Homepage string `json:"homepage"`
	Versions struct {
		Stable string `json:"stable"`
	} `json:"versions"`
}

// searchBrewAPI consumes the official Homebrew formula API: one big
// JSON array, cached like the apt indexes and decoded entry by entry so
// the memory stays flat
func (s RepoSource) searchBrewAPI(ctx context.Context, configDir, keyword string) (packages []RepoPackage, err error) {
	apiURL := strings.TrimSuffix(s.APIURL, "/") + "/formula.json"
	var data io.ReadCloser
	if data, err = s.cachedIndex(ctx, configDir, apiURL); err != nil {
		return nil, err
	}
	defer func() { _ = data.Close() }()

	decoder := json.NewDecoder(data)
	if _, err = decoder.Token(); err != nil { // the opening bracket
		return nil, err
	}
	var hits []RepoPackage
	for decoder.More() {
		formula := brewFormula{}
		if err = decoder.Decode(&formula); err != nil {
			return nil, err
		}
		if keyword == "" || strings.Contains(formula.Name, keyword) ||
			strings.Contains(formula.Desc, keyword) {
			hits = append(hits, RepoPackage{
				Source:      s.Name,
				Name:        formula.Name,
				Version:     formula.Versions.Stable,
				Description: formula.Desc,
				URL:         formula.Homepage,
			})
		}
	}
	return rankPackages(hits, keyword), nil
}

// chocoEntry is an entry of the Chocolatey NuGet search feed
type chocoEntry struct {
	Title     string   `xml:"title"`
	Summary   string   `xml:"summary"`
	Content   chocoURL `xml:"content"`
	Version   string   `xml:"properties>Version"`
	PackageID string   `xml:"properties>PackageID"`
}

type chocoURL struct {
	Src string `xml:"src,attr"`
}

// searchChocoAPI queries the official Chocolatey community feed, the
// search answers the latest versions only which needs no local cache
func (s RepoSource) searchChocoAPI(ctx context.Context, configDir, keyword string) (packages []RepoPackage, err error) {
	query := url.Values{}
	query.Set("searchTerm", "'"+keyword+"'")
	query.Set("targetFramework", "''")
	query.Set("includePrerelease", "false")
	query.Set("$filter", "IsLatestVersion eq true")
	query.Set("$top", "50")
	// spaces travel as %20, the OData endpoint does not read the form
	// encoding plus
	target := strings.TrimSuffix(s.APIURL, "/") + "/Search()?" +
		strings.ReplaceAll(query.Encode(), "+", "%20")

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if reqErr != nil {
		return nil, reqErr
	}
	resp, err := repoHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, s.Name)
	}

	var feed struct {
		Entries []chocoEntry `xml:"entry"`
	}
	if err = xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, err
	}

	hits := make([]RepoPackage, 0, len(feed.Entries))
	for i := range feed.Entries {
		entry := feed.Entries[i]
		name := entry.PackageID
		if name == "" {
			name = entry.Title
		}
		hits = append(hits, RepoPackage{
			Source:      s.Name,
			Name:        name,
			Version:     entry.Version,
			Description: strings.TrimSpace(entry.Summary),
			// the content link points to the installable package file
			URL: entry.Content.Src,
		})
	}
	return rankPackages(hits, keyword), nil
}

// indexDownloads serializes the work on one index cache path. Two
// searches racing on the same staged .part file would interleave their
// writes and corrupt the cache, the web console fires a search per
// keystroke pause which makes the race likely
var indexDownloads sync.Map // cache path -> *sync.Mutex

// lockIndexDownload returns the mutex of an index cache path
func lockIndexDownload(cachePath string) *sync.Mutex {
	lock, _ := indexDownloads.LoadOrStore(cachePath, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

// cachePathOf returns the local cache file of an index URL
func cachePathOf(configDir, source, indexURL string) string {
	safe := strings.NewReplacer(":", "", "/", "-", ".", "_").Replace(indexURL)
	return path.Join(configDir, ".cache", "repos", source+"-"+safe+".gz")
}

// cachedIndex returns a reader of the index behind the URL: the cached
// copy while it is fresh, otherwise a new download which revalidates
// the cached copy with If-Modified-Since
func (s RepoSource) cachedIndex(ctx context.Context, configDir, indexURL string) (data io.ReadCloser, err error) {
	cachePath := cachePathOf(configDir, s.Name, indexURL)

	lock := lockIndexDownload(cachePath)
	lock.Lock()
	defer lock.Unlock()

	var modTime time.Time
	if stat, statErr := os.Stat(cachePath); statErr == nil {
		modTime = stat.ModTime()
		if time.Since(modTime) < repoIndexTTL {
			return os.Open(cachePath) // #nosec G304 -- the path is built from the config cache
		}
	}

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, indexURL, nil)
	if reqErr != nil {
		return nil, reqErr
	}
	if !modTime.IsZero() {
		req.Header.Set("If-Modified-Since", modTime.UTC().Format(http.TimeFormat))
	}
	resp, err := repoHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusNotModified:
		// the cache is still valid on the mirror side, renew its age
		now := time.Now()
		_ = os.Chtimes(cachePath, now, now)
		return os.Open(cachePath) // #nosec G304 -- the path is built from the config cache
	case http.StatusOK:
	default:
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, s.Name)
	}

	if err = os.MkdirAll(path.Dir(cachePath), 0750); err != nil {
		return nil, err
	}
	staged := cachePath + ".part"
	if err = writeAll(staged, resp.Body); err != nil {
		return nil, err
	}
	if err = os.Rename(staged, cachePath); err != nil {
		return nil, err
	}
	return os.Open(cachePath) // #nosec G304 -- the path is built from the config cache
}

// writeAll drains the reader into a new file
func writeAll(target string, reader io.Reader) (err error) {
	file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	if _, err = io.Copy(file, reader); err != nil {
		_ = file.Close()
		return
	}
	return file.Close()
}

// searchPackagesIndex scans a gzip Packages index of the Debian form
// (rfc822 stanzas) and keeps the entries whose name or short
// description carry the keyword
func searchPackagesIndex(source RepoSource, data io.ReadCloser, keyword string) (packages []RepoPackage) {
	gz, err := gzip.NewReader(data)
	if err != nil {
		return
	}
	defer func() { _ = gz.Close() }()

	scanner := bufio.NewScanner(gz)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	const maxHits = 50
	stanza := map[string]string{}
	flush := func() {
		if name := stanza["Package"]; name != "" {
			if keyword == "" || strings.Contains(name, keyword) ||
				strings.Contains(stanza["Description"], keyword) {
				packages = append(packages, RepoPackage{
					Source:      source.Name,
					Name:        name,
					Version:     stanza["Version"],
					Section:     stanza["Section"],
					Description: stanza["Description"],
					URL:         strings.TrimSuffix(source.Mirror, "/") + "/" + stanza["Filename"],
					SHA256:      stanza["SHA256"],
				})
			}
		}
		stanza = map[string]string{}
	}

	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			flush()
		case strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t"):
			// a continuation line of the previous field, the short
			// description of the tag line is all hd keeps
		default:
			if idx := strings.Index(line, ": "); idx > 0 {
				stanza[line[:idx]] = strings.TrimSpace(line[idx+2:])
			}
		}
	}
	flush()
	return
}
