package pkg

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"

	"github.com/gosuri/uiprogress"
)

const (
	// ContentType is for the http header of content type
	ContentType = "Content-Type"
	// ApplicationForm is for the form submit
	ApplicationForm = "application/x-www-form-urlencoded"
)

// HTTPDownloader is the downloader for http request
type HTTPDownloader struct {
	TargetFilePath     string
	URL                string
	ShowProgress       bool
	InsecureSkipVerify bool

	UserName string
	Password string

	Proxy     string
	ProxyAuth string

	Header map[string]string

	// PreStart returns false will don't continue
	PreStart func(*http.Response) bool

	Debug        bool
	RoundTripper http.RoundTripper
}

// SetProxy set the proxy for a http
func SetProxy(proxy, proxyAuth string, tr *http.Transport) (err error) {
	if proxy == "" {
		return
	}

	var proxyURL *url.URL
	if proxyURL, err = url.Parse(proxy); err != nil {
		return
	}

	tr.Proxy = http.ProxyURL(proxyURL)

	if proxyAuth != "" {
		basicAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(proxyAuth))
		tr.ProxyConnectHeader = http.Header{}
		tr.ProxyConnectHeader.Add("Proxy-Authorization", basicAuth)
	}
	return
}

func (h *HTTPDownloader) fetchProxyFromEnv(scheme string) {
	allProxy := os.Getenv("all_proxy")
	httpProxy := os.Getenv("http_proxy")
	httpsProxy := os.Getenv("https_proxy")

	if allProxy != "" {
		h.Proxy = allProxy
	} else {
		switch scheme {
		case "http":
			if httpProxy != "" {
				h.Proxy = httpProxy
			}
		case "https":
			if httpsProxy != "" {
				h.Proxy = httpsProxy
			}
		}
	}
}

//Range: bytes=10-
//HTTP/1.1 206 Partial Content

// DownloadFile download a file with the progress
func (h *HTTPDownloader) DownloadFile() error {
	filepath, downloadURL, showProgress := h.TargetFilePath, h.URL, h.ShowProgress
	// Get the data
	req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}

	for k, v := range h.Header {
		req.Header.Set(k, v)
	}

	if h.UserName != "" && h.Password != "" {
		req.SetBasicAuth(h.UserName, h.Password)
	}
	var tr http.RoundTripper
	if h.RoundTripper != nil {
		tr = h.RoundTripper
	} else {
		trp := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: h.InsecureSkipVerify},
		}
		h.fetchProxyFromEnv(req.URL.Scheme)
		if err = SetProxy(h.Proxy, h.ProxyAuth, trp); err != nil {
			return err
		}

		if h.Proxy != "" {
			basicAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(h.ProxyAuth))
			req.Header.Add("Proxy-Authorization", basicAuth)
		}
		tr = trp
	}
	client := &http.Client{Transport: tr}
	var resp *http.Response

	if resp, err = client.Do(req); err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return &DownloadError{
			Message:    fmt.Sprintf("failed to download from '%s'", downloadURL),
			StatusCode: resp.StatusCode,
		}
	}

	// pre-hook before get started to download file
	if h.PreStart != nil && !h.PreStart(resp) {
		return nil
	}

	writer := &ProgressIndicator{
		Title: "Downloading",
	}
	if showProgress {
		if total, ok := resp.Header["Content-Length"]; ok && len(total) > 0 {
			fileLength, err := strconv.ParseInt(total[0], 10, 64)
			if err == nil {
				writer.Total = float64(fileLength)
			}
		}
	}

	if err := os.MkdirAll(path.Dir(filepath), os.FileMode(0755)); err != nil {
		return err
	}

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	writer.Writer = out

	if showProgress {
		writer.Init()
	}

	// Write the body to file
	_, err = io.Copy(writer, resp.Body)
	return err
}

// ProgressIndicator hold the progress of io operation
type ProgressIndicator struct {
	Writer io.Writer
	Reader io.Reader
	Title  string

	// bytes.Buffer
	Total float64
	count float64
	bar   *uiprogress.Bar
}

// Init set the default value for progress indicator
func (i *ProgressIndicator) Init() {
	uiprogress.Start()             // start rendering
	i.bar = uiprogress.AddBar(100) // Add a new bar

	// optionally, append and prepend completion and elapsed time
	i.bar.AppendCompleted()
	// i.bar.PrependElapsed()

	if i.Title != "" {
		i.bar.PrependFunc(func(_ *uiprogress.Bar) string {
			return fmt.Sprintf("%s: ", i.Title)
		})
	}
}

// Write writes the progress
func (i *ProgressIndicator) Write(p []byte) (n int, err error) {
	n, err = i.Writer.Write(p)
	i.setBar(n)
	return
}

// Read reads the progress
func (i *ProgressIndicator) Read(p []byte) (n int, err error) {
	n, err = i.Reader.Read(p)
	i.setBar(n)
	return
}

func (i *ProgressIndicator) setBar(n int) {
	i.count += float64(n)

	if i.bar != nil {
		i.bar.Set((int)(i.count * 100 / i.Total))
	}
}
