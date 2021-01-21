package pkg

import "fmt"

// DownloadError represents the error of HTTP download
type DownloadError struct {
	StatusCode int
	Message    string
}

// Error print the error message
func (e *DownloadError) Error() string {
	return fmt.Sprintf("%s: status code: %d", e.Message, e.StatusCode)
}
