package pkg

/**
 * This file was deprecated, please use the following package instead
 * github.com/linuxsuren/http-downloader/pkg/net
 */

import (
	"github.com/linuxsuren/http-downloader/pkg/net"
	"net/http"
)

// RetryClient is the wrap of http.Client
type RetryClient net.RetryClient

// Do is the wrap of http.Client.Do
func (c *RetryClient) Do(req *http.Request) (rsp *http.Response, err error) {
	return (*net.RetryClient)(c).Do(req)
}
