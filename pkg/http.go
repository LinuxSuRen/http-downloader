package pkg

/**
 * This file was deprecated, please use the following package instead
 * github.com/linuxsuren/http-downloader/pkg/net
 */

import (
	"github.com/linuxsuren/http-downloader/pkg/net"
	"net/http"
)

const (
	// ContentType is for the http header of content type
	ContentType = net.ContentType
	// ApplicationForm is for the form submit
	ApplicationForm = net.ApplicationForm
)

// HTTPDownloader is the downloader for http request
type HTTPDownloader net.HTTPDownloader

// DownloadFile download a file with the progress
// deprecated
func (h *HTTPDownloader) DownloadFile() error {
	return (*net.HTTPDownloader)(h).DownloadFile()
}

// SetProxy set the proxy for a http
func SetProxy(proxy, proxyAuth string, tr *http.Transport) (err error) {
	return net.SetProxy(proxy, proxyAuth, tr)
}

// DownloadFileWithMultipleThread downloads the files with multiple threads
func DownloadFileWithMultipleThread(targetURL, targetFilePath string, thread int, showProgress bool) (err error) {
	return net.DownloadFileWithMultipleThreadKeepParts(targetURL, targetFilePath, thread, false, showProgress)
}

// DownloadFileWithMultipleThreadKeepParts downloads the files with multiple threads
func DownloadFileWithMultipleThreadKeepParts(targetURL, targetFilePath string, thread int, keepParts, showProgress bool) (err error) {
	return net.DownloadFileWithMultipleThreadKeepParts(targetURL, targetFilePath, thread, keepParts, showProgress)
}

// DownloadWithContinue downloads the files continuously
func DownloadWithContinue(targetURL, output string, index, continueAt, end int64, showProgress bool) (err error) {
	downloader := &net.ContinueDownloader{}
	return downloader.DownloadWithContinue(targetURL, output, index, continueAt, end, showProgress)
}

// DetectSize returns the size of target resource
func DetectSize(targetURL, output string, showProgress bool) (total int64, rangeSupport bool, err error) {
	return net.DetectSize(targetURL, output, showProgress)
}
