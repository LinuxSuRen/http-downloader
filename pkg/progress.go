package pkg

/**
 * This file was deprecated, please use the following package instead
 * github.com/linuxsuren/http-downloader/pkg/net
 */

import (
	"github.com/linuxsuren/http-downloader/pkg/net"
)

// ProgressIndicator hold the progress of io operation
type ProgressIndicator net.ProgressIndicator

// Init set the default value for progress indicator
func (i *ProgressIndicator) Init() {
	(*net.ProgressIndicator)(i).Init()
}

// Write writes the progress
func (i *ProgressIndicator) Write(p []byte) (n int, err error) {
	return (*net.ProgressIndicator)(i).Write(p)
}

// Read reads the progress
func (i *ProgressIndicator) Read(p []byte) (n int, err error) {
	return (*net.ProgressIndicator)(i).Read(p)
}
