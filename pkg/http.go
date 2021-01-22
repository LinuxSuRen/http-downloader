package pkg

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"sync"
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

	Thread int

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

// DownloadFileWithMultipleThread downloads the files with multiple threads
func DownloadFileWithMultipleThread(targetURL, targetFilePath string, thread int, showProgress bool) (err error) {
	// get the total size of the target file
	var total int64
	var rangeSupport bool
	if total, rangeSupport, err = DetectSize(targetURL, targetFilePath, true); err != nil {
		return
	}

	if rangeSupport {
		unit := total / int64(thread)
		offset := total - unit*int64(thread)
		var wg sync.WaitGroup

		fmt.Printf("start to download with %d threads, size: %d, unit: %d\n", thread, total, unit)
		for i := 0; i < thread; i++ {
			wg.Add(1)
			go func(index int, wg *sync.WaitGroup) {
				defer wg.Done()

				end := unit*int64(index+1) - 1
				if index == thread-1 {
					// this is the last part
					end += offset
				}
				start := unit * int64(index)

				if downloadErr := DownloadWithContinue(targetURL, fmt.Sprintf("%s-%d", targetFilePath, index), start, end, showProgress); downloadErr != nil {
					fmt.Println(downloadErr)
				}
			}(i, &wg)
		}

		wg.Wait()

		// concat all these partial files
		var f *os.File
		if f, err = os.OpenFile(targetFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			defer func() {
				_ = f.Close()
			}()

			for i := 0; i < thread; i++ {
				partFile := fmt.Sprintf("%s-%d", targetFilePath, i)
				if data, ferr := ioutil.ReadFile(partFile); ferr == nil {
					if _, err = f.Write(data); err != nil {
						err = fmt.Errorf("failed to write file: '%s'", partFile)
						break
					} else {
						_ = os.RemoveAll(partFile)
					}
				} else {
					err = fmt.Errorf("failed to read file: '%s'", partFile)
					break
				}
			}
		}
	} else {
		fmt.Println("cannot download it using multiple threads, failed to one")
		err = DownloadWithContinue(targetURL, targetFilePath, 0, 0, true)
	}
	return
}

// DownloadWithContinue downloads the files continuously
func DownloadWithContinue(targetURL, output string, continueAt, end int64, showProgress bool) (err error) {
	downloader := HTTPDownloader{
		TargetFilePath: output,
		URL:            targetURL,
		ShowProgress:   showProgress,
	}

	if continueAt >= 0 {
		downloader.Header = make(map[string]string, 1)

		if end > continueAt {
			downloader.Header["Range"] = fmt.Sprintf("bytes=%d-%d", continueAt, end)
		} else {
			downloader.Header["Range"] = fmt.Sprintf("bytes=%d-", continueAt)
		}
	}

	if err = downloader.DownloadFile(); err != nil {
		err = fmt.Errorf("cannot download from %s, error: %v", targetURL, err)
	}
	return
}

// DetectSize returns the size of target resource
func DetectSize(targetURL, output string, showProgress bool) (total int64, rangeSupport bool, err error) {
	downloader := HTTPDownloader{
		TargetFilePath: output,
		URL:            targetURL,
		ShowProgress:   showProgress,
	}

	var detectOffset int64
	var lenErr error

	detectOffset = 2
	downloader.Header = make(map[string]string, 1)
	downloader.Header["Range"] = fmt.Sprintf("bytes=%d-", detectOffset)

	downloader.PreStart = func(resp *http.Response) bool {
		rangeSupport = resp.StatusCode == http.StatusPartialContent
		contentLen := resp.Header.Get("Content-Length")
		if total, lenErr = strconv.ParseInt(contentLen, 10, 0); lenErr == nil {
			total += detectOffset
		}
		//  always return false because we just want to get the header from response
		return false
	}

	if err = downloader.DownloadFile(); err != nil || lenErr != nil {
		err = fmt.Errorf("cannot download from %s, response error: %v, content length error: %v", targetURL, err, lenErr)
	}
	return
}
