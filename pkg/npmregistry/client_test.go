package npmregistry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestSearch(t *testing.T) {
	var requestedURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedURL = r.URL.String()
		assert.Equal(t, "/-/v1/search", r.URL.Path)
		assert.Equal(t, "chalk", r.URL.Query().Get("text"))
		assert.Equal(t, "5", r.URL.Query().Get("size"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"objects":[
			{"package":{"name":"chalk","version":"5.3.0","description":"Terminal string styling","homepage":"https://github.com/chalk/chalk"}},
			{"package":{"name":"@types/chalk","version":"1.0.0"}},
			{"package":{}}
		]}`))
	}))
	t.Cleanup(server.Close)

	packages, err := Search(context.Background(), "chalk", 5, server.Client(), server.URL)
	assert.NoError(t, err)
	assert.Len(t, packages, 2)
	assert.Equal(t, "chalk", packages[0].Name)
	assert.Equal(t, "5.3.0", packages[0].Version)
	assert.Equal(t, "Terminal string styling", packages[0].Description)
	assert.Equal(t, "@types/chalk", packages[1].Name)
	assert.Contains(t, requestedURL, "text=chalk")
}

func TestSearchErrors(t *testing.T) {
	t.Run("empty keyword", func(t *testing.T) {
		_, err := Search(context.Background(), "  ", 0, nil, "")
		assert.Error(t, err)
	})

	t.Run("bad status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		}))
		t.Cleanup(server.Close)
		_, err := Search(context.Background(), "foo", 0, server.Client(), server.URL)
		assert.ErrorContains(t, err, "500")
	})

	t.Run("bad json", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("not-json"))
		}))
		t.Cleanup(server.Close)
		_, err := Search(context.Background(), "foo", 0, server.Client(), server.URL)
		assert.ErrorContains(t, err, "cannot parse")
	})

	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		client := roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, req.Context().Err()
		})
		_, err := Search(ctx, "foo", 0, client, "")
		assert.Error(t, err)
	})

	t.Run("default size", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "20", r.URL.Query().Get("size"))
			_, _ = w.Write([]byte(`{"objects":[]}`))
		}))
		t.Cleanup(server.Close)
		packages, err := Search(context.Background(), "foo", 0, server.Client(), server.URL)
		assert.NoError(t, err)
		assert.Empty(t, packages)
	})
}
