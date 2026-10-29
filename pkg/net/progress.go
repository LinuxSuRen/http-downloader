package net

import (
	"fmt"
	"io"
	"time"

	"github.com/k0kubun/go-ansi"
	"github.com/schollz/progressbar/v3"
)

// ProgressIndicator hold the progress of io operation.
//
// Multiple indicators are allowed to share one underlying progress bar, for
// example all the parts of a multi-thread download feed a single aggregated
// bar (see Share). Keeping the progress on a single terminal line avoids the
// relative cursor movement sequences which are not supported by all the
// terminals (e.g. the classic Windows console), so the output stays stable
// on both Windows and Linux.
type ProgressIndicator struct {
	Writer io.Writer
	Reader io.Reader
	Title  string

	// Total is the expected total size in bytes. Zero or a negative value
	// means the size is unknown, in that case the bar works in the spinner
	// mode which still reports the transferred bytes.
	Total float64

	// shared holds the underlying progress bar which might be shared with
	// other indicators. A nil value disables the progress rendering.
	shared *progressBarShared
	// owner indicates if this indicator is responsible for finishing the bar
	owner bool
}

// progressBarShared is the progress bar shared by the indicators
type progressBarShared struct {
	bar *progressbar.ProgressBar
}

// barOutput is the terminal output used by all the progress bars. It goes
// through github.com/k0kubun/go-ansi which enables the virtual terminal
// processing on Windows, so the ANSI sequences render properly.
var barOutput io.Writer = ansi.NewAnsiStdout()

// Init set the default value for progress indicator
func (i *ProgressIndicator) Init() {
	if i.shared != nil {
		// already initialized
		return
	}

	max := int64(i.Total)
	if max <= 0 {
		// the total size is unknown, -1 puts the bar into the spinner mode
		// which still counts the transferred bytes
		max = -1
	}

	i.shared = &progressBarShared{
		bar: progressbar.NewOptions64(max,
			progressbar.OptionSetWriter(barOutput),
			progressbar.OptionEnableColorCodes(true),
			progressbar.OptionShowBytes(true),
			progressbar.OptionShowCount(),
			progressbar.OptionShowElapsedTimeOnFinish(),
			progressbar.OptionFullWidth(),
			progressbar.OptionThrottle(65*time.Millisecond),
			progressbar.OptionOnCompletion(func() {
				// leave the terminal prompt on the next line once completed
				_, _ = fmt.Fprintln(barOutput)
			}),
			progressbar.OptionSetDescription(fmt.Sprintf("[cyan]%s[reset]", i.Title)),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        "[green]=[reset]",
				SaucerHead:    "[green]>[reset]",
				SaucerPadding: " ",
				BarStart:      "[",
				BarEnd:        "]",
			})),
	}
	i.owner = true
}

// Share returns a new indicator which feeds the same progress bar with this
// indicator. The returned indicator does not own the bar, which means closing
// it does not finish the bar. Typically used by the multi-thread downloads:
// all the parts write into one aggregated bar.
// It returns nil if the current indicator was not initialized (Init), so that
// the progress can be disabled globally.
func (i *ProgressIndicator) Share() *ProgressIndicator {
	if i == nil || i.shared == nil {
		return nil
	}
	return &ProgressIndicator{shared: i.shared}
}

// Close shutdowns the progress rendering. Only the owner of the underlying
// bar finishes it, shared indicators are not affected.
func (i *ProgressIndicator) Close() {
	if i == nil || i.shared == nil || !i.owner {
		return
	}
	_ = i.shared.bar.Close()
}

// Write writes the progress
func (i *ProgressIndicator) Write(p []byte) (n int, err error) {
	if i.Writer != nil {
		n, err = i.Writer.Write(p)
	} else {
		n = len(p)
	}
	i.report(n)
	return
}

// Read reads the progress
func (i *ProgressIndicator) Read(p []byte) (n int, err error) {
	if i.Reader != nil {
		n, err = i.Reader.Read(p)
		if err != nil {
			return
		}
	} else {
		err = io.EOF
	}
	i.report(n)
	return
}

// report adds the transferred bytes into the shared progress bar.
// The rendering issue of the bar never fails the data transfer itself.
func (i *ProgressIndicator) report(n int) {
	if i.shared != nil && n > 0 {
		_ = i.shared.bar.Add(n)
	}
}
