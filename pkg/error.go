package pkg

/**
 * This file was deprecated, please use the following package instead
 * github.com/linuxsuren/http-downloader/pkg/net
 */

import (
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg/net"
)

// DownloadError represents the error of HTTP download
type DownloadError net.DownloadError

// Error print the error message
func (e *DownloadError) Error() string {
	return (*net.DownloadError)(e).Error()
}

// ErrorWrap warps the error if it is not nil
func ErrorWrap(err error, format string, a ...any) error {
	if err != nil {
		return fmt.Errorf(format, a...)
	}
	return nil
}
