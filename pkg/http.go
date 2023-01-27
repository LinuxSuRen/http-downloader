package pkg

/**
 * This file was deprecated, please use the following package instead
 * github.com/linuxsuren/http-downloader/pkg/net
 */

import (
	"github.com/linuxsuren/http-downloader/pkg/net"
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

// DownloadFileWithMultipleThread downloads the files with multiple threads
func DownloadFileWithMultipleThread(targetURL, targetFilePath string, thread int, showProgress bool) (err error) {
	return net.DownloadFileWithMultipleThreadKeepParts(targetURL, targetFilePath, thread, false, showProgress)
}

// DownloadFileWithMultipleThreadKeepParts downloads the files with multiple threads
// deprecated
func DownloadFileWithMultipleThreadKeepParts(targetURL, targetFilePath string, thread int, keepParts, showProgress bool) (err error) {
	return net.DownloadFileWithMultipleThreadKeepParts(targetURL, targetFilePath, thread, keepParts, showProgress)
}

// DownloadWithContinue downloads the files continuously
// deprecated
func DownloadWithContinue(targetURL, output string, index, continueAt, end int64, showProgress bool) (err error) {
	downloader := &net.ContinueDownloader{}
	return downloader.DownloadWithContinue(targetURL, output, index, continueAt, end, showProgress)
}
