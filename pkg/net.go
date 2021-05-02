package pkg

/**
 * This file was deprecated, please use the following package instead
 * github.com/linuxsuren/http-downloader/pkg/net
 */

import (
	"github.com/linuxsuren/http-downloader/pkg/net"
)

// GetExternalIP returns an external ip of current machine
func GetExternalIP() (string, error) {
	return net.GetExternalIP()
}
