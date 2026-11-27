// Package npmregistry is a small client for the npm search API. It is used
// to find packages from npm without requiring the npm CLI locally.
package npmregistry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultBaseURL is the public npm registry
const DefaultBaseURL = "https://registry.npmjs.org"

// Package is one npm package returned by the search API
type Package struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Homepage    string `json:"homepage"`
}

type searchResponse struct {
	Objects []struct {
		Package Package `json:"package"`
	} `json:"objects"`
}

type searchClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Search queries the npm registry for packages matching the keyword. An
// empty baseURL uses the public npm registry, a custom one makes the client
// testable without network access.
func Search(ctx context.Context, keyword string, size int, client searchClient, baseURL string) ([]Package, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, fmt.Errorf("the npm search keyword is empty")
	}
	if size <= 0 {
		size = 20
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if ctx == nil {
		ctx = context.Background()
	}

	endpoint := strings.TrimSuffix(baseURL, "/") + "/-/v1/search?text=" +
		url.QueryEscape(keyword) + "&size=" + fmt.Sprintf("%d", size)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("npm registry returned %s", resp.Status)
	}

	result := &searchResponse{}
	if err = json.NewDecoder(resp.Body).Decode(result); err != nil {
		return nil, fmt.Errorf("cannot parse the npm search result: %v", err)
	}

	packages := make([]Package, 0, len(result.Objects))
	for i := range result.Objects {
		if result.Objects[i].Package.Name != "" {
			packages = append(packages, result.Objects[i].Package)
		}
	}
	return packages, nil
}
