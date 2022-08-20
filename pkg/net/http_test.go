package net_test

import (
	"bytes"
	"fmt"
	"github.com/linuxsuren/http-downloader/mock/mhttp"
	"github.com/linuxsuren/http-downloader/pkg/net"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

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
				Body:       ioutil.NopCloser(bytes.NewBufferString(responseBody)),
			}
			roundTripper.EXPECT().
				RoundTrip(request).Return(response, nil)
			err := downloader.DownloadFile()
			Expect(err).To(BeNil())

			_, err = os.Stat(targetFilePath)
			Expect(err).To(BeNil())

			content, readErr := ioutil.ReadFile(targetFilePath)
			Expect(readErr).To(BeNil())
			Expect(string(content)).To(Equal(responseBody))
			return
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
				Body:       ioutil.NopCloser(bytes.NewBufferString(responseBody)),
			}
			roundTripper.EXPECT().
				RoundTrip(request).Return(response, nil)
			err := downloader.DownloadFile()
			Expect(err).To(BeNil())

			_, err = os.Stat(targetFilePath)
			Expect(err).To(BeNil())

			content, readErr := ioutil.ReadFile(targetFilePath)
			Expect(readErr).To(BeNil())
			Expect(string(content)).To(Equal(responseBody))
			return
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
				Body:       ioutil.NopCloser(bytes.NewBufferString(responseBody)),
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
				Body:       ioutil.NopCloser(bytes.NewBufferString(responseBody)),
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
		Body: ioutil.NopCloser(bytes.NewBufferString("responseBody")),
	}
	roundTripper.EXPECT().
		RoundTrip(mockRequest).Return(mockResponse, nil)

	total, rangeSupport, err := net.DetectSizeWithRoundTripper(targetURL, os.TempDir(), false, roundTripper)
	assert.Nil(t, err)
	assert.Equal(t, int64(102), total)
	assert.True(t, rangeSupport)
}
