package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/h2non/gock"
	"github.com/linuxsuren/http-downloader/mock/mhttp"
	"github.com/linuxsuren/http-downloader/pkg/exec"
	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func Test_newGetCmd(t *testing.T) {
	cmd := newGetCmd(context.Background())
	assert.Equal(t, "get", cmd.Name())

	flags := []struct {
		name      string
		shorthand string
	}{{
		name:      "output",
		shorthand: "o",
	}, {
		name: "pre",
	}, {
		name: "time",
	}, {
		name: "max-attempts",
	}, {
		name: "no-proxy",
	}, {
		name: "show-progress",
	}, {
		name: "continue-at",
	}, {
		name: "keep-part",
	}, {
		name: "os",
	}, {
		name: "arch",
	}, {
		name: "print-schema",
	}, {
		name: "print-version",
	}, {
		name: "print-categories",
	}, {
		name: "print-version-count",
	}}
	for i := range flags {
		tt := flags[i]
		t.Run(tt.name, func(t *testing.T) {
			flag := cmd.Flag(tt.name)
			assert.NotNil(t, flag)
			assert.NotEmpty(t, flag.Usage)
			assert.Equal(t, tt.shorthand, flag.Shorthand)
		})
	}
}

func TestFetch(t *testing.T) {
	opt := &downloadOption{
		wait: &sync.WaitGroup{},
	}
	opt.Fetch = false
	opt.fetcher = &installer.FakeFetcher{FetchLatestRepoErr: errors.New("fake")}

	// do not fetch
	assert.Nil(t, opt.fetch())

	// fetch flag is true
	opt.Fetch = true
	assert.NotNil(t, opt.fetch())

	// failed when fetching
	opt.fetcher = &installer.FakeFetcher{}
	assert.Nil(t, opt.fetch())
}

func TestPreRunE(t *testing.T) {
	opt := &downloadOption{
		wait: &sync.WaitGroup{},
	}
	opt.Fetch = true
	opt.fetcher = &installer.FakeFetcher{FetchLatestRepoErr: errors.New("fake")}
	opt.PrintSchema = true

	// only print schema
	assert.Nil(t, opt.preRunE(nil, nil))

	// failed to fetch
	fakeC := &cobra.Command{}
	opt.PrintSchema = false
	assert.NotNil(t, opt.preRunE(fakeC, nil))

	// pripnt categories
	opt.fetcher = &installer.FakeFetcher{}
	opt.PrintCategories = true
	assert.Nil(t, opt.preRunE(fakeC, nil))

	// not args provided
	opt.PrintCategories = false
	assert.NotNil(t, opt.preRunE(fakeC, nil))
}

func TestRunE(t *testing.T) {
	tests := []struct {
		name    string
		opt     *downloadOption
		args    []string
		prepare func(t *testing.T, do *downloadOption)
		wantErr bool
	}{{
		name: "print shcema only",
		opt: &downloadOption{
			fetcher:     &installer.FakeFetcher{},
			PrintSchema: true,
		},
		wantErr: false,
	}, {
		name: "download from an URL with one thread",
		opt: &downloadOption{
			fetcher: &installer.FakeFetcher{},
			NoProxy: true,
		},
		prepare: func(t *testing.T, do *downloadOption) {
			do.Output = path.Join(os.TempDir(), fmt.Sprintf("fake-%d", time.Now().Nanosecond()))
			do.URL = "https://foo.com"

			ctrl := gomock.NewController(t)
			roundTripper := mhttp.NewMockRoundTripper(ctrl)

			mockRequest, _ := http.NewRequest(http.MethodGet, do.URL, nil)
			mockResponse := &http.Response{
				StatusCode: http.StatusPartialContent,
				Proto:      "HTTP/1.1",
				Request:    mockRequest,
				Header: map[string][]string{
					"Content-Length": {"100"},
				},
				Body: io.NopCloser(bytes.NewBufferString("responseBody")),
			}
			roundTripper.EXPECT().
				RoundTrip(mockRequest).Return(mockResponse, nil)
			do.RoundTripper = roundTripper
		},
		wantErr: false,
	}, {
		name: "download from an URL with multi-threads",
		opt: &downloadOption{
			fetcher: &installer.FakeFetcher{},
			NoProxy: true,
			Thread:  2,
		},
		prepare: func(t *testing.T, do *downloadOption) {
			do.Output = path.Join(os.TempDir(), fmt.Sprintf("fake-%d", time.Now().Nanosecond()))
			do.URL = "https://foo.com"

			ctrl := gomock.NewController(t)
			roundTripper := mhttp.NewMockRoundTripper(ctrl)
			ctx, cancel := context.WithCancel(context.Background())
			defer func() {
				cancel()
			}()

			// for size detecting
			mockRequest, _ := http.NewRequestWithContext(ctx, http.MethodGet, do.URL, nil)
			mockRequest.Header.Set("Range", "bytes=2-")
			mockResponse := &http.Response{
				StatusCode: http.StatusPartialContent,
				Proto:      "HTTP/1.1",
				Request:    mockRequest,
				Header: map[string][]string{
					"Content-Length": {"100"},
				},
				Body: io.NopCloser(bytes.NewBufferString("responseBody")),
			}

			roundTripper.EXPECT().RoundTrip(gomock.Any()).Return(mockResponse, nil).AnyTimes()
			do.RoundTripper = roundTripper
		},
		wantErr: false,
	}}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				gock.Off()
			}()

			fakeCmd := &cobra.Command{}
			fakeCmd.SetOut(new(bytes.Buffer))
			if tt.prepare != nil {
				tt.prepare(t, tt.opt)
			}
			if tt.opt.Output != "" {
				defer func() {
					_ = os.RemoveAll(tt.opt.Output)
				}()
			}
			err := tt.opt.runE(fakeCmd, tt.args)
			if tt.wantErr {
				assert.NotNil(t, err, "should error in [%d][%s]", i, tt.name)
			} else {
				assert.Nil(t, err, "should not error in [%d][%s]", i, tt.name)
			}
		})
	}
}

func TestDownloadMagnetFile(t *testing.T) {
	tests := []struct {
		name        string
		proxyGitHub string
		target      string
		prepare     func()
		execer      exec.Execer
		expectErr   bool
	}{{
		name:        "proxyGitHub and target is empty",
		proxyGitHub: "",
		target:      "",
		execer:      exec.FakeExecer{},
	}, {
		name:        "failed to download",
		proxyGitHub: "",
		target:      "fake",
		execer:      exec.FakeExecer{ExpectError: errors.New("error")},
		expectErr:   true,
	}, {
		name:        "one target item",
		proxyGitHub: "",
		target:      "http://fake.com",
		prepare: func() {
			gock.New("http://fake.com").
				Get("/").
				Reply(http.StatusOK).
				File("testdata/magnet.html")
		},
		execer:    exec.FakeExecer{},
		expectErr: false,
	}, {
		name:        "HTTP server error response",
		proxyGitHub: "",
		target:      "http://fake.com",
		prepare: func() {
			gock.New("http://fake.com").
				Get("/").
				ReplyError(errors.New("error"))
		},
		execer:    exec.FakeExecer{},
		expectErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer gock.Off()
			if tt.prepare == nil {
				tt.prepare = func() {}
			}
			tt.prepare()
			err := downloadMagnetFile(tt.proxyGitHub, tt.target, tt.execer)
			assert.Equal(t, tt.expectErr, err != nil, err)
		})
	}
}
