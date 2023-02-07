package net

import (
	"fmt"
	"net/http"
	"os"
	"sync"
)

// MultiThreadDownloader is a download with multi-thread
type MultiThreadDownloader struct {
	noProxy                 bool
	keepParts, showProgress bool
	insecureSkipVerify      bool

	roundTripper      http.RoundTripper
	suggestedFilename string
}

// GetSuggestedFilename returns the suggested filename
func (d *MultiThreadDownloader) GetSuggestedFilename() string {
	return d.suggestedFilename
}

// WithInsecureSkipVerify set if skip the insecure verify
func (d *MultiThreadDownloader) WithInsecureSkipVerify(insecureSkipVerify bool) *MultiThreadDownloader {
	d.insecureSkipVerify = insecureSkipVerify
	return d
}

// WithoutProxy indicates not use HTTP proxy
func (d *MultiThreadDownloader) WithoutProxy(noProxy bool) *MultiThreadDownloader {
	d.noProxy = noProxy
	return d
}

// WithShowProgress indicate if show the download progress
func (d *MultiThreadDownloader) WithShowProgress(showProgress bool) *MultiThreadDownloader {
	d.showProgress = showProgress
	return d
}

// WithKeepParts indicates if keeping the part files
func (d *MultiThreadDownloader) WithKeepParts(keepParts bool) *MultiThreadDownloader {
	d.keepParts = keepParts
	return d
}

// WithRoundTripper sets RoundTripper
func (d *MultiThreadDownloader) WithRoundTripper(roundTripper http.RoundTripper) *MultiThreadDownloader {
	d.roundTripper = roundTripper
	return d
}

// Download starts to download the target URL
func (d *MultiThreadDownloader) Download(targetURL, targetFilePath string, thread int) (err error) {
	// get the total size of the target file
	var total int64
	var rangeSupport bool
	if total, rangeSupport, err = DetectSizeWithRoundTripper(targetURL, targetFilePath, d.showProgress,
		d.noProxy, d.insecureSkipVerify, d.roundTripper); rangeSupport && err != nil {
		return
	}

	if rangeSupport {
		unit := total / int64(thread)
		offset := total - unit*int64(thread)
		var wg sync.WaitGroup
		var partItems []string
		var m sync.Mutex

		defer func() {
			// remove all partial files
			for _, part := range partItems {
				_ = os.RemoveAll(part)
			}
		}()

		fmt.Printf("start to download with %d threads, size: %d, unit: %d\n", thread, total, unit)
		for i := 0; i < thread; i++ {
			wg.Add(1)
			go func(index int, wg *sync.WaitGroup) {
				defer wg.Done()
				output := fmt.Sprintf("%s-%d", targetFilePath, index)

				m.Lock()
				partItems = append(partItems, output)
				m.Unlock()

				end := unit*int64(index+1) - 1
				if index == thread-1 {
					// this is the last part
					end += offset
				}
				start := unit * int64(index)

				downloader := &ContinueDownloader{}
				downloader.WithoutProxy(d.noProxy).
					WithRoundTripper(d.roundTripper).
					WithInsecureSkipVerify(d.insecureSkipVerify)
				if downloadErr := downloader.DownloadWithContinue(targetURL, output,
					int64(index), start, end, d.showProgress); downloadErr != nil {
					fmt.Println(downloadErr)
				}
			}(i, &wg)
		}

		wg.Wait()
		ProgressIndicator{}.Close()

		// concat all these partial files
		var f *os.File
		if f, err = os.OpenFile(targetFilePath, os.O_CREATE|os.O_WRONLY, 0600); err == nil {
			defer func() {
				_ = f.Close()
			}()

			for i := 0; i < thread; i++ {
				partFile := fmt.Sprintf("%s-%d", targetFilePath, i)
				if data, ferr := os.ReadFile(partFile); ferr == nil {
					if _, err = f.Write(data); err != nil {
						err = fmt.Errorf("failed to write file: '%s'", partFile)
						break
					} else if !d.keepParts {
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
		downloader := &ContinueDownloader{}
		downloader.WithoutProxy(d.noProxy)
		downloader.WithRoundTripper(d.roundTripper)
		downloader.WithInsecureSkipVerify(d.insecureSkipVerify)
		err = downloader.DownloadWithContinue(targetURL, targetFilePath, -1, 0, 0, true)
		d.suggestedFilename = downloader.GetSuggestedFilename()
	}
	return
}
