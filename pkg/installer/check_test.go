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
		verify  func(o *Installer, version string, t *testing.T)
		wantErr bool
	}{
		{
			name:    "empty version, repo as default name",
			appInfo: "kubernetes-sigs/kustomize",
			verify: func(o *Installer, version string, t *testing.T) {
				assert.Equal(t, version, "latest")
				assert.Equal(t, o.Org, "kubernetes-sigs")
				assert.Equal(t, o.Repo, "kustomize")
				assert.Equal(t, o.Name, "kustomize")
			},
			wantErr: false,
		},
		{
			name:    "empty version, specific app name",
			appInfo: "kubernetes-sigs/kustomize/kustomize",
			verify: func(o *Installer, version string, t *testing.T) {
				assert.Equal(t, version, "latest")
				assert.Equal(t, o.Org, "kubernetes-sigs")
				assert.Equal(t, o.Repo, "kustomize")
				assert.Equal(t, o.Name, "kustomize")
			},
			wantErr: false,
		},
		{
			name:    "semver version, specific app name",
			appInfo: "kubernetes-sigs/kustomize/kustomize@v1.0",
			verify: func(o *Installer, version string, t *testing.T) {
				assert.Equal(t, version, "v1.0")
				assert.Equal(t, o.Org, "kubernetes-sigs")
				assert.Equal(t, o.Repo, "kustomize")
				assert.Equal(t, o.Name, "kustomize")
			},
			wantErr: false,
		},
		{
			name:    "specific version with a slash, specific app name",
			appInfo: "kubernetes-sigs/kustomize/kustomize@kustomize/v1.0",
			verify: func(o *Installer, version string, t *testing.T) {
				assert.Equal(t, version, "kustomize/v1.0")
				assert.Equal(t, o.Org, "kubernetes-sigs")
				assert.Equal(t, o.Repo, "kustomize")
				assert.Equal(t, o.Name, "kustomize")
			},
			wantErr: false,
		},
		{
			name:    "specific version with a underlined, specific app name",
			appInfo: "kubernetes-sigs/kustomize/kustomize@kustomize_v1.0",
			verify: func(o *Installer, version string, t *testing.T) {
				assert.Equal(t, version, "kustomize_v1.0")
				assert.Equal(t, o.Org, "kubernetes-sigs")
				assert.Equal(t, o.Repo, "kustomize")
				assert.Equal(t, o.Name, "kustomize")
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
				tt.verify(is, version, t)
			}
		})
	}
}

func TestProviderURLParseNoConfig(t *testing.T) {
	tests := []struct {
		name       string
		packageURL string
		verify     func(o *Installer, packageURL string, t *testing.T)
		wantErr    bool
	}{
		{
			name:       "empty version, repo as default name",
			packageURL: "orgtest/repotest",
			verify: func(o *Installer, packageURL string, t *testing.T) {
				expectURL := fmt.Sprintf(
					"https://github.com/orgtest/repotest/releases/latest/download/repotest-%s-%s.%s",
					o.OS, o.Arch, getPackagingFormat(o))
				assert.Equal(t, packageURL, expectURL)
			},
			wantErr: false,
		},
		{
			name:       "empty version, specific app name",
			packageURL: "orgtest/repotest/hello",
			verify: func(o *Installer, packageURL string, t *testing.T) {
				expectURL := fmt.Sprintf(
					"https://github.com/orgtest/repotest/releases/latest/download/hello-%s-%s.%s",
					o.OS, o.Arch, getPackagingFormat(o))
				assert.Equal(t, packageURL, expectURL)
			},
			wantErr: false,
		},
		{
			name:       "semver version, specific app name",
			packageURL: "orgtest/repotest/hello@v1.0",
			verify: func(o *Installer, packageURL string, t *testing.T) {
				expectURL := fmt.Sprintf(
					"https://github.com/orgtest/repotest/releases/download/v1.0/hello-%s-%s.%s",
					o.OS, o.Arch, getPackagingFormat(o))
				assert.Equal(t, packageURL, expectURL)
			},
			wantErr: false,
		},
		{
			name:       "specific version with a slash, specific app name",
			packageURL: "orgtest/repotest/hello@hello/v1.0",
			verify: func(o *Installer, packageURL string, t *testing.T) {
				expectURL := fmt.Sprintf(
					"https://github.com/orgtest/repotest/releases/download/hello/v1.0/hello-%s-%s.%s",
					o.OS, o.Arch, getPackagingFormat(o))
				assert.Equal(t, packageURL, expectURL)
			},
			wantErr: false,
		},
		{
			name:       "specific version with a underlined, specific app name",
			packageURL: "orgtest/repotest/hello@hello_v1.0",
			verify: func(o *Installer, packageURL string, t *testing.T) {
				expectURL := fmt.Sprintf(
					"https://github.com/orgtest/repotest/releases/download/hello_v1.0/hello-%s-%s.%s",
					o.OS, o.Arch, getPackagingFormat(o))
				assert.Equal(t, packageURL, expectURL)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := &Installer{
				Package: &HDConfig{FormatOverrides: PackagingFormat{
					Windows: "zip",
					Linux:   "tar.gz",
				}},
				OS:   runtime.GOOS,
				Arch: runtime.GOARCH,
			}
			packageURL, err := is.ProviderURLParse(tt.packageURL, false)
			if (err != nil) != tt.wantErr {
				t.Errorf("ProviderURLParse error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.verify != nil {
				tt.verify(is, packageURL, t)
			}
		})
	}
}

func TestValidPackageSuffix(t *testing.T) {
	type args struct {
		packageURL string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "empty version, repo as default name",
			args: args{
				"https://github.com/orgtest/repotest/releases/latest/download/repotest-%s-%s.%s",
			},
			want: true,
		},
		{
			name: "empty version, repo as default name",
			args: args{
				"https://github.com/orgtest/repotest/releases/latest/download/hello-%s-%s.%s",
			},
			want: true,
		},
		{
			name: "semver version, specific app name",
			args: args{
				"https://github.com/orgtest/repotest/releases/download/v1.0/hello-%s-%s.%s",
			},
			want: true,
		},
		{
			name: "specific version with a slash, specific app name",
			args: args{
				"https://github.com/orgtest/repotest/releases/download/hello/v1.0/hello-%s-%s.%s",
			},
			want: true,
		},
		{
			name: "url of download without an compress extension",
			args: args{
				"https://github.com/orgtest/repotest/releases/download/hello/v1.0/hello-%s-%s.%s.abcdef",
			},
			want: false,
		},
	}

	is := &Installer{
		Package: &HDConfig{FormatOverrides: PackagingFormat{
			Windows: "zip",
			Linux:   "tar.gz",
		}},
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packageURL := fmt.Sprintf(
				tt.args.packageURL,
				is.OS, is.Arch, getPackagingFormat(is))
			if got := hasPackageSuffix(packageURL); got != tt.want {
				t.Errorf("hasPackageSuffix() = %v, want %v", got, tt.want)
			}
		})
	}
}
