package installer

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"runtime"
	"testing"

	"github.com/h2non/gock"
	fakeruntime "github.com/linuxsuren/go-fake-runtime"
	"github.com/stretchr/testify/assert"
)

func TestGetVersion(t *testing.T) {
	tests := []struct {
		name    string
		appInfo string
		verify  func(o *Installer, version string, t *testing.T)
		wantErr bool
	}{
		{
			name:    "empty version, Repo as default name",
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
		{
			name:    "invalid version",
			appInfo: "xx@xx@xx",
			wantErr: true,
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
			name:       "empty version, Repo as default name",
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
		{
			name:       "invalid version",
			packageURL: "xx@xx@xx",
			wantErr:    true,
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
			name: "empty version, Repo as default name",
			args: args{
				"https://github.com/orgtest/repotest/releases/latest/download/repotest-%s-%s.%s",
			},
			want: true,
		},
		{
			name: "empty version, Repo as default name",
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
				t.Errorf("hasPackageSuffix() = %v, wantOrg %v", got, tt.want)
			}
		})
	}
}

func TestA(t *testing.T) {
	a := FindPackagesByCategory("k8s")
	fmt.Println(a)
}

func Test_hasKeyword(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config")
	assert.Nil(t, err)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	type args struct {
		metaFile func() string
		keyword  string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		name: "has the same name",
		args: args{
			metaFile: func() (metaFile string) {
				metaFile = path.Join(tmpDir, "repo.yml")
				err = os.WriteFile(metaFile, []byte(`name: fake`), 0400)
				assert.Nil(t, err)
				return
			},
			keyword: "fake",
		},
		want: true,
	}, {
		name: "has the same binary",
		args: args{
			metaFile: func() (metaFile string) {
				metaFile = path.Join(tmpDir, "repo-1.yml")
				err = os.WriteFile(metaFile, []byte(`binary: fake`), 0400)
				assert.Nil(t, err)
				return
			},
			keyword: "fake",
		},
		want: true,
	}, {
		name: "has the same targetBinary",
		args: args{
			metaFile: func() (metaFile string) {
				metaFile = path.Join(tmpDir, "repo-2.yml")
				err = os.WriteFile(metaFile, []byte(`targetBinary: fake`), 0400)
				assert.Nil(t, err)
				return
			},
			keyword: "fake",
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, hasKeyword(tt.args.metaFile(), tt.args.keyword), "hasKeyword(%v, %v)", tt.args.metaFile, tt.args.keyword)
		})
	}
}

func Test_getOrgAndRepo(t *testing.T) {
	type args struct {
		orgAndRepo string
	}
	tests := []struct {
		name     string
		args     args
		wantOrg  string
		wantRepo string
	}{{
		name:     "normal case, org/repo",
		args:     args{orgAndRepo: "org/repo"},
		wantOrg:  "org",
		wantRepo: "repo",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := getOrgAndRepo(tt.args.orgAndRepo)
			assert.Equalf(t, tt.wantOrg, got, "getOrgAndRepo(%v)", tt.args.orgAndRepo)
			assert.Equalf(t, tt.wantRepo, got1, "getOrgAndRepo(%v)", tt.args.orgAndRepo)
		})
	}
}

func Test_getHDConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config")
	assert.Nil(t, err)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	type args struct {
		configDir  string
		orgAndRepo func() string
	}
	tests := []struct {
		name       string
		args       args
		wantConfig *HDConfig
	}{{
		name:       "not exist",
		wantConfig: nil,
	}, {
		name: "valid file",
		args: args{
			configDir: tmpDir,
			orgAndRepo: func() (orgAndRepo string) {
				orgAndRepo = "org/repo"
				configFile := path.Join(tmpDir, "config", orgAndRepo+".yml")
				err := os.MkdirAll(path.Dir(configFile), 0755)
				assert.Nil(t, err)
				err = os.WriteFile(configFile, []byte("name: fake"), 0400)
				assert.Nil(t, err)
				return
			},
		},
		wantConfig: &HDConfig{Name: "fake"},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.args.orgAndRepo == nil {
				tt.args.orgAndRepo = func() string {
					return ""
				}
			}
			assert.Equalf(t, tt.wantConfig, getHDConfig(tt.args.configDir, tt.args.orgAndRepo()), "getHDConfig(%v, %v)", tt.args.configDir, tt.args.orgAndRepo)
		})
	}
}

func Test_getDynamicVersion(t *testing.T) {
	const fakeVersionURL = "https://fake.com"
	const expectVersion = "v1.1.1"

	tests := []struct {
		name          string
		prepare       func()
		expectVersion string
		expectError   bool
	}{{
		name: "normal case",
		prepare: func() {
			gock.New(fakeVersionURL).Get("/").Reply(http.StatusOK).BodyString(expectVersion)
		},
		expectVersion: expectVersion,
		expectError:   false,
	}, {
		name: "got statusCode which is not 200",
		prepare: func() {
			gock.New(fakeVersionURL).Get("/").Reply(http.StatusNotFound)
		},
		expectVersion: "",
		expectError:   true,
	}}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			defer gock.Off()
			tt.prepare()
			version, err := getDynamicVersion(fakeVersionURL)
			assert.Equal(t, tt.expectVersion, version)
			if tt.expectError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestCheckDepAndInstall(t *testing.T) {
	installer := &Installer{
		Execer: &fakeruntime.FakeExecer{},
	}

	err := installer.CheckDepAndInstall(map[string]string{})
	assert.Nil(t, err)

	err = installer.CheckDepAndInstall(map[string]string{
		"fake": "fake",
	})
	assert.Nil(t, err)
}

func TestIsSupport(t *testing.T) {
	type args struct {
		cfg HDConfig
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		name: "support case",
		args: args{
			cfg: HDConfig{
				SupportOS:   []string{runtime.GOOS},
				SupportArch: []string{runtime.GOARCH},
			},
		},
		want: true,
	}, {
		name: "not os and arch setting",
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, IsSupport(tt.args.cfg), "IsSupport(%v)", tt.args.cfg)
		})
	}
}

func Test_getVersionOrDefault(t *testing.T) {
	type args struct {
		version    string
		defaultVer string
	}
	tests := []struct {
		name       string
		args       args
		wantTarget string
		prepare    func(string)
	}{{
		name: "version is not a HTTP",
		args: args{
			version:    "v1.2.3",
			defaultVer: "1.2.3",
		},
		wantTarget: "1.2.3",
	}, {
		name: "version is a HTTP address",
		args: args{
			version:    "https://foo.com/",
			defaultVer: "",
		},
		wantTarget: "v1.2.3",
		prepare: func(address string) {
			gock.New(address).Get("/").Reply(http.StatusOK).BodyString("v1.2.3")
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer gock.Off()
			if tt.prepare != nil {
				tt.prepare(tt.args.version)
			}
			assert.Equalf(t, tt.wantTarget, getVersionOrDefault(tt.args.version, tt.args.defaultVer), "getVersionOrDefault(%v, %v)", tt.args.version, tt.args.defaultVer)
		})
	}
}

func TestGetPackagingFormat(t *testing.T) {
	tests := []struct {
		name      string
		installer *Installer
		expect    string
	}{{
		name: "have specific format",
		installer: &Installer{
			OS: "Darwin",
			Package: &HDConfig{
				FormatOverrides: PackagingFormat{
					Format: "msi",
				},
			},
		},
		expect: "msi",
	}, {
		name: "windows with config",
		installer: &Installer{
			OS: "Windows",
			Package: &HDConfig{
				FormatOverrides: PackagingFormat{
					Windows: "msi",
				},
			},
		},
		expect: "msi",
	}, {
		name: "windows without config",
		installer: &Installer{
			OS: "Windows",
		},
		expect: "zip",
	}, {
		name:      "darwin without config",
		installer: &Installer{},
		expect:    "tar.gz",
	}, {
		name: "linux, and package is not nil, but the override is empty",
		installer: &Installer{
			Package: &HDConfig{},
		},
		expect: "tar.gz",
	}, {
		name: "linux with override",
		installer: &Installer{
			Package: &HDConfig{
				FormatOverrides: PackagingFormat{
					Linux: "tar.gz",
				},
			},
		},
		expect: "tar.gz",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getPackagingFormat(tt.installer)
			assert.Equal(t, tt.expect, result)
		})
	}
}
