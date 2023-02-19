package net_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/linuxsuren/http-downloader/mock/mhttp"
	"github.com/linuxsuren/http-downloader/pkg/net"
	"github.com/stretchr/testify/assert"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("http test", func() {
	var (
		ctrl           *gomock.Controller
		roundTripper   *mhttp.MockRoundTripper
		downloader     net.HTTPDownloader
		targetFilePath string
		responseBody   string
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		roundTripper = mhttp.NewMockRoundTripper(ctrl)
		targetFilePath = "test.log"
		downloader = net.HTTPDownloader{
			TargetFilePath: targetFilePath,
			RoundTripper:   roundTripper,
		}
		responseBody = "fake body"
	})

	AfterEach(func() {
		os.Remove(targetFilePath)
		ctrl.Finish()
	})

	Context("SetProxy", func() {
		It("basic test", func() {
			proxy, proxyAuth := "http://localhost", "admin:admin"

			tr := &http.Transport{}
			err := net.SetProxy(proxy, proxyAuth, tr)
			Expect(err).To(BeNil())
			Expect(tr.ProxyConnectHeader.Get("Proxy-Authorization")).To(Equal("Basic YWRtaW46YWRtaW4="))
		})

		It("empty proxy", func() {
			err := net.SetProxy("", "", nil)
			Expect(err).To(BeNil())
		})
	})

	Context("DownloadFile", func() {
		It("no progress indication", func() {
			request, _ := http.NewRequest(http.MethodGet, "", nil)
			response := &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{},
				Request:    request,
				Body:       io.NopCloser(bytes.NewBufferString(responseBody)),
			}
			roundTripper.EXPECT().
				RoundTrip(request).Return(response, nil)
			err := downloader.DownloadFile()
			Expect(err).To(BeNil())

			_, err = os.Stat(targetFilePath)
			Expect(err).To(BeNil())

			content, readErr := os.ReadFile(targetFilePath)
			Expect(readErr).To(BeNil())
			Expect(string(content)).To(Equal(responseBody))
		})

		It("with BasicAuth", func() {
			downloader = net.HTTPDownloader{
				TargetFilePath: targetFilePath,
				RoundTripper:   roundTripper,
				UserName:       "UserName",
				Password:       "Password",
			}

			request, _ := http.NewRequest(http.MethodGet, "", nil)
			request.SetBasicAuth(downloader.UserName, downloader.Password)
			response := &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{},
				Request:    request,
				Body:       io.NopCloser(bytes.NewBufferString(responseBody)),
			}
			roundTripper.EXPECT().
				RoundTrip(request).Return(response, nil)
			err := downloader.DownloadFile()
			Expect(err).To(BeNil())

			_, err = os.Stat(targetFilePath)
			Expect(err).To(BeNil())

			content, readErr := os.ReadFile(targetFilePath)
			Expect(readErr).To(BeNil())
			Expect(string(content)).To(Equal(responseBody))
		})

		It("with error request", func() {
			downloader = net.HTTPDownloader{
				URL: "fake url",
			}
			err := downloader.DownloadFile()
			Expect(err).To(HaveOccurred())
		})

		It("with error response", func() {
			downloader = net.HTTPDownloader{
				RoundTripper: roundTripper,
			}

			request, _ := http.NewRequest(http.MethodGet, "", nil)
			response := &http.Response{}
			roundTripper.EXPECT().
				RoundTrip(request).Return(response, fmt.Errorf("fake error"))
			err := downloader.DownloadFile()
			Expect(err).To(HaveOccurred())
		})

		It("status code isn't 200", func() {
			const debugFile = "debug-download.html"
			downloader = net.HTTPDownloader{
				RoundTripper:   roundTripper,
				Debug:          true,
				TargetFilePath: debugFile,
			}

			request, _ := http.NewRequest(http.MethodGet, "", nil)
			response := &http.Response{
				StatusCode: 400,
				Proto:      "HTTP/1.1",
				Request:    request,
				Body:       io.NopCloser(bytes.NewBufferString(responseBody)),
			}
			roundTripper.EXPECT().
				RoundTrip(request).Return(response, nil)
			err := downloader.DownloadFile()
			Expect(err).To(HaveOccurred())

			_, err = os.Stat(debugFile)
			Expect(err).NotTo(BeNil())
		})

		It("showProgress", func() {
			downloader = net.HTTPDownloader{
				RoundTripper:   roundTripper,
				ShowProgress:   true,
				TargetFilePath: targetFilePath,
			}

			request, _ := http.NewRequest(http.MethodGet, "", nil)
			response := &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Request:    request,
				Body:       io.NopCloser(bytes.NewBufferString(responseBody)),
			}
			roundTripper.EXPECT().
				RoundTrip(request).Return(response, nil)
			err := downloader.DownloadFile()
			Expect(err).To(BeNil())
		})
	})
})

func TestSetProxy(t *testing.T) {
	type args struct {
		proxy     string
		proxyAuth string
		tr        *http.Transport
	}
	tests := []struct {
		name    string
		args    args
		verify  func(transport *http.Transport, t *testing.T) error
		wantErr bool
	}{{
		name:    "empty proxy",
		args:    args{},
		wantErr: false,
	}, {
		name: "abc.com as proxy",
		args: args{
			proxy:     "http://abc.com",
			proxyAuth: "user:password",
			tr:        &http.Transport{},
		},
		verify: func(tr *http.Transport, t *testing.T) error {
			proxy, err := tr.Proxy(&http.Request{})
			if proxy.Host != "abc.com" {
				err = fmt.Errorf("expect proxy host is: %s, got %s", "abc.com", proxy.Host)
			}
			auth := tr.ProxyConnectHeader.Get("Proxy-Authorization")
			assert.Equal(t, "Basic dXNlcjpwYXNzd29yZA==", auth)
			return err
		},
		wantErr: false,
	}, {
		name: "invalid proxy",
		args: args{
			proxy: "http://foo\u007F.com/",
		},
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := net.SetProxy(tt.args.proxy, tt.args.proxyAuth, tt.args.tr); (err != nil) != tt.wantErr {
				t.Errorf("SetProxy() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.verify != nil {
				if err := tt.verify(tt.args.tr, t); err != nil {
					t.Errorf("SetProxy() error %v", err)
				}
			}
		})
	}
}

func TestDetectSize(t *testing.T) {
	const targetURL = "https://foo.com/"
	ctrl := gomock.NewController(t)
	roundTripper := mhttp.NewMockRoundTripper(ctrl)

	mockRequest, _ := http.NewRequest(http.MethodGet, targetURL, nil)
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
	roundTripper.EXPECT().
		RoundTrip(mockRequest).Return(mockResponse, nil)

	total, rangeSupport, err := net.DetectSizeWithRoundTripper(targetURL, os.TempDir(), false, false, false, roundTripper)
	assert.Nil(t, err)
	assert.Equal(t, int64(102), total)
	assert.True(t, rangeSupport)
}

func TestMultiThreadDownloader(t *testing.T) {
	const url = "https://foo.com"
	tests := []struct {
		name           string
		thread         int
		expectFilename string
		prepare        func(*testing.T, *net.MultiThreadDownloader)
		wantErr        bool
	}{{
		name:   "download with 2 threads",
		thread: 2,
		prepare: func(t *testing.T, downloader *net.MultiThreadDownloader) {
			ctrl := gomock.NewController(t)
			roundTripper := mhttp.NewMockRoundTripper(ctrl)

			// for size detecting
			mockRequest, _ := http.NewRequest(http.MethodGet, url, nil)
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

			downloader.WithoutProxy(true).
				WithShowProgress(false).
				WithRoundTripper(roundTripper).
				WithKeepParts(false).
				WithInsecureSkipVerify(true)
		},
		wantErr: false,
	}, {
		name:   "download with 1 thread",
		thread: 1,
		prepare: func(t *testing.T, downloader *net.MultiThreadDownloader) {
			// for regular download
			ctrl := gomock.NewController(t)
			roundTripper := mhttp.NewMockRoundTripper(ctrl)
			mockRequest4, _ := http.NewRequest(http.MethodGet, url, nil)
			mockRequest4.Header.Set("Range", "bytes=2-")
			mockResponse4 := &http.Response{
				StatusCode: http.StatusOK,
				Proto:      "HTTP/1.1",
				Request:    mockRequest4,
				Header: map[string][]string{
					"Content-Length": {"100"},
				},
				Body: io.NopCloser(bytes.NewBufferString("responseBody")),
			}
			roundTripper.EXPECT().
				RoundTrip(mockRequest4).Return(mockResponse4, nil)

			mockRequest3, _ := http.NewRequest(http.MethodGet, url, nil)
			mockRequest3.Header.Set("Range", "bytes=0-")
			mockResponse3 := &http.Response{
				StatusCode: http.StatusOK,
				Proto:      "HTTP/1.1",
				Request:    mockRequest3,
				Header: map[string][]string{
					"Content-Length": {"100"},
				},
				Body: io.NopCloser(bytes.NewBufferString("responseBody")),
			}
			roundTripper.EXPECT().
				RoundTrip(mockRequest3).Return(mockResponse3, nil)
			downloader.WithoutProxy(true).
				WithShowProgress(false).
				WithRoundTripper(roundTripper).
				WithKeepParts(false)
		},
		wantErr: false,
	}, {
		name:           "invalid content length",
		thread:         1,
		expectFilename: "suggestedFilename",
		prepare: func(t *testing.T, downloader *net.MultiThreadDownloader) {
			// for regular download
			ctrl := gomock.NewController(t)
			roundTripper := mhttp.NewMockRoundTripper(ctrl)
			mockRequest4, _ := http.NewRequest(http.MethodGet, url, nil)
			mockRequest4.Header.Set("Range", "bytes=2-")
			mockResponse4 := &http.Response{
				StatusCode: http.StatusPartialContent,
				Proto:      "HTTP/1.1",
				Request:    mockRequest4,
				Header: map[string][]string{
					"Content-Length": {"not-a-number"},
				},
				Body: io.NopCloser(bytes.NewBufferString("responseBody")),
			}
			roundTripper.EXPECT().
				RoundTrip(mockRequest4).Return(mockResponse4, nil)

			mockRequest5, _ := http.NewRequest(http.MethodGet, url, nil)
			mockRequest5.Header.Set("Range", "bytes=0-")
			mockResponse5 := &http.Response{
				StatusCode: http.StatusOK,
				Proto:      "HTTP/1.1",
				Header: map[string][]string{
					"Content-Disposition": {`filename="suggestedFilename"`},
				},
				Request: mockRequest5,
				Body:    io.NopCloser(bytes.NewBufferString("responseBody")),
			}
			roundTripper.EXPECT().
				RoundTrip(mockRequest5).Return(mockResponse5, nil)

			downloader.WithoutProxy(true).
				WithShowProgress(false).
				WithRoundTripper(roundTripper).
				WithKeepParts(false)
		},
		wantErr: false,
	}}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			downloader := &net.MultiThreadDownloader{}
			tt.prepare(t, downloader)

			var f *os.File
			var err error
			f, err = os.CreateTemp(os.TempDir(), "fake")
			assert.Nil(t, err)
			assert.NotNil(t, f)
			if err == nil {
				defer func() {
					_ = os.RemoveAll(f.Name())
				}()
			}

			err = downloader.Download(url, f.Name(), tt.thread)
			if tt.wantErr {
				assert.NotNil(t, err, "should have error in case [%d]-[%s]", i, tt.name)
			} else {
				assert.Nil(t, err, "should not have error in case [%d]-[%s]", i, tt.name)
			}
			assert.Equal(t, tt.expectFilename, downloader.GetSuggestedFilename())
		})
	}
}
