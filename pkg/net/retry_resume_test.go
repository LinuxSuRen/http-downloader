package net

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// roundTripperFunc adapts a function into a http.RoundTripper
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// errorReader serves the data first and then the given error instead of EOF,
// which simulates a transfer that breaks in the middle
type errorReader struct {
	data string
	err  error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	if len(e.data) == 0 {
		return 0, e.err
	}
	n = copy(p, e.data)
	e.data = e.data[n:]
	return n, nil
}

func textResponse(req *http.Request, status int, length string, body io.Reader) *http.Response {
	header := http.Header{}
	if length != "" {
		header.Set("Content-Length", length)
	}
	return &http.Response{
		StatusCode: status,
		Header:     header,
		Body:       io.NopCloser(body),
		Request:    req,
	}
}

func TestDownloadWithContinueResumeBrokenTransfer(t *testing.T) {
	attempts := 0
	var ranges []string
	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		ranges = append(ranges, req.Header.Get("Range"))
		if attempts == 1 {
			// breaks in the middle of the transfer
			return textResponse(req, http.StatusOK, "10",
				&errorReader{data: "01234", err: io.ErrUnexpectedEOF}), nil
		}
		return textResponse(req, http.StatusPartialContent, "5",
			strings.NewReader("56789")), nil
	})

	target := filepath.Join(t.TempDir(), "out.bin")
	c := &ContinueDownloader{roundTripper: rt, maxAttempts: 3}
	err := c.DownloadWithContinue("http://fake/file", target, -1, -1, 0, false)
	assert.NoError(t, err)
	assert.Equal(t, 2, attempts)
	assert.Equal(t, []string{"", "bytes=5-"}, ranges)

	data, readErr := os.ReadFile(target)
	assert.NoError(t, readErr)
	assert.Equal(t, "0123456789", string(data))
}

func TestDownloadWithContinueResumeRangePart(t *testing.T) {
	// a multi-thread part which breaks: [5,14] of the whole file
	attempts := 0
	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return textResponse(req, http.StatusPartialContent, "6",
				&errorReader{data: "abcde", err: io.ErrUnexpectedEOF}), nil
		}
		assert.Equal(t, "bytes=10-14", req.Header.Get("Range"))
		return textResponse(req, http.StatusPartialContent, "1",
			strings.NewReader("f")), nil
	})

	target := filepath.Join(t.TempDir(), "part")
	c := &ContinueDownloader{roundTripper: rt, maxAttempts: 2}
	err := c.DownloadWithContinue("http://fake/file", target, 0, 5, 14, false)
	assert.NoError(t, err)
	assert.Equal(t, 2, attempts)

	data, readErr := os.ReadFile(target)
	assert.NoError(t, readErr)
	assert.Equal(t, "abcdef", string(data))
}

func TestDownloadWithContinueNoRetryOnClientError(t *testing.T) {
	attempts := 0
	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		return textResponse(req, http.StatusNotFound, "0", strings.NewReader("")), nil
	})

	target := filepath.Join(t.TempDir(), "out.bin")
	c := &ContinueDownloader{roundTripper: rt, maxAttempts: 3}
	err := c.DownloadWithContinue("http://fake/file", target, -1, -1, 0, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot download from http://fake/file")
	assert.Contains(t, err.Error(), "404")
	assert.Equal(t, 1, attempts) // a 404 is not worth a retry
}

func TestDownloadWithContinueRetryExhausted(t *testing.T) {
	attempts := 0
	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		status, length := http.StatusOK, "10"
		if attempts > 1 {
			// the resumed attempts keep breaking as well
			status, length = http.StatusPartialContent, "5"
		}
		return textResponse(req, status, length,
			&errorReader{data: "01234", err: io.ErrUnexpectedEOF}), nil
	})

	target := filepath.Join(t.TempDir(), "out.bin")
	c := &ContinueDownloader{roundTripper: rt, maxAttempts: 2}
	err := c.DownloadWithContinue("http://fake/file", target, -1, -1, 0, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected EOF")
	assert.Equal(t, 2, attempts)
}

func TestDownloadWithContinueRangeIgnored(t *testing.T) {
	// the resume attempt gets a full 200 response although a range was
	// requested: appending would corrupt the file, it must fail instead
	attempts := 0
	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return textResponse(req, http.StatusOK, "10",
				&errorReader{data: "01234", err: io.ErrUnexpectedEOF}), nil
		}
		return textResponse(req, http.StatusOK, "10",
			strings.NewReader("0123456789")), nil
	})

	target := filepath.Join(t.TempDir(), "out.bin")
	c := &ContinueDownloader{roundTripper: rt, maxAttempts: 3}
	err := c.DownloadWithContinue("http://fake/file", target, -1, -1, 0, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not support resuming")
	assert.Equal(t, 2, attempts) // the failure is not retryable

	data, readErr := os.ReadFile(target)
	assert.NoError(t, readErr)
	assert.Equal(t, "01234", string(data)) // not corrupted
}

func TestDownloadWithContinueCanceledWhileWaiting(t *testing.T) {
	attempts := 0
	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		return textResponse(req, http.StatusOK, "10",
			&errorReader{data: "01234", err: io.ErrUnexpectedEOF}), nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	c := &ContinueDownloader{roundTripper: rt, maxAttempts: 5, Context: ctx}
	go func() {
		for attempts == 0 {
			// wait until the first attempt started
		}
		cancel()
	}()

	target := filepath.Join(t.TempDir(), "out.bin")
	err := c.DownloadWithContinue("http://fake/file", target, -1, -1, 0, false)
	assert.Error(t, err)
	assert.Equal(t, 1, attempts) // no more attempts after the cancelation
}

func TestIsRetryable(t *testing.T) {
	assert.True(t, isRetryable(io.ErrUnexpectedEOF))
	assert.False(t, isRetryable(context.Canceled))
	assert.False(t, isRetryable(context.DeadlineExceeded))
	assert.False(t, isRetryable(&DownloadError{StatusCode: 404}))
	assert.False(t, isRetryable(&DownloadError{StatusCode: 403}))
	assert.False(t, isRetryable(&DownloadError{StatusCode: 200}))
	assert.True(t, isRetryable(&DownloadError{StatusCode: 500}))
	assert.True(t, isRetryable(&DownloadError{StatusCode: 503}))
	assert.True(t, isRetryable(&DownloadError{StatusCode: 429}))
	// local filesystem errors cannot be fixed by a retry
	assert.False(t, isRetryable(&fs.PathError{Op: "write", Path: "/dev/full", Err: errors.New("no space left on device")}))
	assert.False(t, isRetryable(&fs.PathError{Op: "open", Path: "no-access", Err: fs.ErrPermission}))
}

func TestDownloadWithContinueUninitializedProgressDisablesBar(t *testing.T) {
	// an injected indicator that was never initialized disables the
	// progress entirely, it must not fall back to standalone bars
	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return textResponse(req, http.StatusOK, "10", strings.NewReader("0123456789")), nil
	})

	target := filepath.Join(t.TempDir(), "out.bin")
	c := &ContinueDownloader{
		roundTripper: rt,
		progress:     &ProgressIndicator{Title: "not initialized"},
	}
	err := c.DownloadWithContinue("http://fake/file", target, -1, -1, 0, true)
	assert.NoError(t, err)
	assert.Nil(t, c.downloader.progressIndicator)
	assert.True(t, c.downloader.progressInjected)
}
