package installer

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"runtime"
	"testing"
)

func TestGetVersion(t *testing.T) {

	tests := []struct {
		name    string
		appInfo string
		verify  func(o *Installer, version string, t *testing.T) error
		wantErr bool
	}{
		{
			name:    "empty version, repo as default name",
			appInfo: "kubernetes-sigs/kustomize",
			verify: func(o *Installer, version string, t *testing.T) error {
				assert.Equal(t, version, "latest")
				assert.Equal(t, o.Org, "kubernetes-sigs")
				assert.Equal(t, o.Repo, "kustomize")
				assert.Equal(t, o.Name, "kustomize")
				return nil
			},
			wantErr: false,
		},
		{
			name:    "empty version, specific app name",
			appInfo: "kubernetes-sigs/kustomize/kustomize",
			verify: func(o *Installer, version string, t *testing.T) error {
				assert.Equal(t, version, "latest")
				assert.Equal(t, o.Org, "kubernetes-sigs")
				assert.Equal(t, o.Repo, "kustomize")
				assert.Equal(t, o.Name, "kustomize")
				return nil
			},
			wantErr: false,
		},
		{
			name:    "semver version, specific app name",
			appInfo: "kubernetes-sigs/kustomize/kustomize@v1.0",
			verify: func(o *Installer, version string, t *testing.T) error {
				assert.Equal(t, version, "v1.0")
				assert.Equal(t, o.Org, "kubernetes-sigs")
				assert.Equal(t, o.Repo, "kustomize")
				assert.Equal(t, o.Name, "kustomize")
				return nil
			},
			wantErr: false,
		},
		{
			name:    "specific version with a slash, specific app name",
			appInfo: "kubernetes-sigs/kustomize/kustomize@kustomize/v1.0",
			verify: func(o *Installer, version string, t *testing.T) error {
				assert.Equal(t, version, "kustomize/v1.0")
				assert.Equal(t, o.Org, "kubernetes-sigs")
				assert.Equal(t, o.Repo, "kustomize")
				assert.Equal(t, o.Name, "kustomize")
				return nil
			},
			wantErr: false,
		},
		{
			name:    "specific version with a underlined, specific app name",
			appInfo: "kubernetes-sigs/kustomize/kustomize@kustomize_v1.0",
			verify: func(o *Installer, version string, t *testing.T) error {
				assert.Equal(t, version, "kustomize_v1.0")
				assert.Equal(t, o.Org, "kubernetes-sigs")
				assert.Equal(t, o.Repo, "kustomize")
				assert.Equal(t, o.Name, "kustomize")
				return nil
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := &Installer{}
			version, err := is.GetVersion(tt.appInfo)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetVersion error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.verify != nil {
				if err := tt.verify(is, version, t); err != nil {
					t.Errorf("GetVersion() error %v", err)
				}
			}
		})
	}
}

func TestProviderURLParseNoConfig(t *testing.T) {

	tests := []struct {
		name       string
		packageURL string
		verify     func(o *Installer, packageURL string, t *testing.T) error
		wantErr    bool
	}{
		{
			name:       "empty version, repo as default name",
			packageURL: "orgtest/repotest",
			verify: func(o *Installer, packageURL string, t *testing.T) error {
				expectURL := fmt.Sprintf("https://github.com/%s/%s/releases/%s/download/%s-%s-%s.tar.gz",
					"orgtest", "repotest", "latest", "repotest", o.OS, o.Arch)
				assert.Equal(t, packageURL, expectURL)
				return nil
			},
			wantErr: false,
		},
		{
			name:       "empty version, specific app name",
			packageURL: "orgtest/repotest/hello",
			verify: func(o *Installer, packageURL string, t *testing.T) error {
				expectURL := fmt.Sprintf("https://github.com/%s/%s/releases/%s/download/%s-%s-%s.tar.gz",
					"orgtest", "repotest", "latest", "hello", o.OS, o.Arch)
				assert.Equal(t, packageURL, expectURL)
				return nil
			},
			wantErr: false,
		},
		{
			name:       "semver version, specific app name",
			packageURL: "orgtest/repotest/hello@v1.0",
			verify: func(o *Installer, packageURL string, t *testing.T) error {
				expectURL := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s-%s-%s.tar.gz",
					"orgtest", "repotest", "v1.0", "hello", o.OS, o.Arch)
				assert.Equal(t, packageURL, expectURL)
				return nil
			},
			wantErr: false,
		},
		{
			name:       "specific version with a slash, specific app name",
			packageURL: "orgtest/repotest/hello@hello/v1.0",
			verify: func(o *Installer, packageURL string, t *testing.T) error {
				expectURL := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s-%s-%s.tar.gz",
					"orgtest", "repotest", "hello/v1.0", "hello", o.OS, o.Arch)
				assert.Equal(t, packageURL, expectURL)
				return nil
			},
			wantErr: false,
		},
		{
			name:       "specific version with a underlined, specific app name",
			packageURL: "orgtest/repotest/hello@hello_v1.0",
			verify: func(o *Installer, packageURL string, t *testing.T) error {
				expectURL := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s-%s-%s.tar.gz",
					"orgtest", "repotest", "hello_v1.0", "hello", o.OS, o.Arch)
				assert.Equal(t, packageURL, expectURL)
				return nil
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := &Installer{
				OS:   runtime.GOOS,
				Arch: runtime.GOARCH,
			}
			packageURL, err := is.ProviderURLParse(tt.packageURL, false)
			if (err != nil) != tt.wantErr {
				t.Errorf("ProviderURLParse error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.verify != nil {
				if err := tt.verify(is, packageURL, t); err != nil {
					t.Errorf("ProviderURLParse() error %v", err)
				}
			}
		})
	}
}
