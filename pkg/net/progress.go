package net

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/k0kubun/go-ansi"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/term"
)

// ProgressIndicator hold the progress of io operation.
//
// Multiple indicators are allowed to share one underlying progress bar, for
// example the parts of a multi-thread download can feed a single aggregated
// bar (see Share) when the output is not an interactive terminal. On an
// interactive terminal use MultiProgressIndicator instead, it renders one
// bar per part on dedicated terminal lines.
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

// isInteractiveTerminal tells if the standard output is attached to a
// terminal. Only on an interactive terminal the multi-line progress bars
// are rendered with relative cursor movements, a piped or redirected output
// falls back to a single aggregated bar which does not need them.
var isInteractiveTerminal = func() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// barThrottle is the minimal interval between two renders of a bar
var barThrottle = 65 * time.Millisecond

// Init set the default value for progress indicator
func (i *ProgressIndicator) Init() {
	if i.shared != nil {
		// already initialized
		return
	}
	i.initBar(barOutput, true)
}

// initBar creates the underlying progress bar rendering to the given writer.
// newlineOnCompletion controls if a newline is printed once the bar reached
// its end: standalone bars want it to leave the prompt on the next line,
// while the bars of a multi-bar group share the line management with their
// MultiProgressIndicator.
func (i *ProgressIndicator) initBar(output io.Writer, newlineOnCompletion bool) {
	max := int64(i.Total)
	if max <= 0 {
		// the total size is unknown, -1 puts the bar into the spinner mode
		// which still counts the transferred bytes
		max = -1
	}

	options := []progressbar.Option{
		progressbar.OptionSetWriter(output),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionShowElapsedTimeOnFinish(),
		progressbar.OptionFullWidth(),
		progressbar.OptionThrottle(barThrottle),
		progressbar.OptionSetDescription(fmt.Sprintf("[cyan]%s[reset]", i.Title)),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	}
	if newlineOnCompletion {
		options = append(options, progressbar.OptionOnCompletion(func() {
			// leave the terminal prompt on the next line once completed
			_, _ = fmt.Fprintln(output)
		}))
	}

	i.shared = &progressBarShared{bar: progressbar.NewOptions64(max, options...)}
	i.owner = true
}

// MultiProgressIndicator renders one progress bar per download part, every
// bar stays on its own terminal line. All the bars and the relative cursor
// movements go through a single locked renderer, so the lines never garble
// each other - unlike bars rendered independently.
// It is only suitable for an interactive terminal, use a single shared
// ProgressIndicator instead when the output is piped or redirected.
type MultiProgressIndicator struct {
	mu     sync.Mutex
	bars   []*ProgressIndicator
	cursor int // the terminal line the cursor currently stays on
}

// multiLineWriter renders everything of one bar on the line of that bar
type multiLineWriter struct {
	manager *MultiProgressIndicator
	line    int
}

// NewMultiProgressIndicator creates an empty multi-bar group. The cursor is
// expected to stay at the beginning of a fresh line.
func NewMultiProgressIndicator() *MultiProgressIndicator {
	return &MultiProgressIndicator{}
}

// Register allocates the next terminal line for a new bar and returns the
// indicator of it.
func (m *MultiProgressIndicator) Register(title string, total float64) *ProgressIndicator {
	m.mu.Lock()
	line := len(m.bars)
	indicator := &ProgressIndicator{Title: title, Total: total}
	indicator.initBar(&multiLineWriter{manager: m, line: line}, false)

	if line > 0 {
		// open a new terminal line below the current bars
		m.moveCursor(line - 1)
		_, _ = fmt.Fprintln(barOutput)
		m.cursor = line
	}
	m.bars = append(m.bars, indicator)
	m.mu.Unlock()

	// render the blank frame outside the lock: it goes back through the
	// line writer which acquires the lock on its own
	_ = indicator.shared.bar.RenderBlank()
	return indicator
}

// Close finishes all the bars and leaves the cursor below them.
func (m *MultiProgressIndicator) Close() {
	m.mu.Lock()
	bars := make([]*ProgressIndicator, len(m.bars))
	copy(bars, m.bars)
	m.mu.Unlock()

	// the bars render through the line writer, do not hold the lock here.
	// Only the unfinished bars get finished: closing an already finished
	// bar would erase its final frame (the renderer clears the line first
	// and then skips the re-render of a finished bar).
	for _, bar := range bars {
		if bar.shared.bar.State().CurrentPercent < 0.9999 {
			_ = bar.shared.bar.Close()
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.bars) > 0 {
		m.moveCursor(len(m.bars) - 1)
		_, _ = fmt.Fprintln(barOutput)
		m.cursor++
	}
}

// Write implements io.Writer, it moves the cursor to the line of the
// belonging bar before passing the data through
func (w *multiLineWriter) Write(p []byte) (n int, err error) {
	m := w.manager
	m.mu.Lock()
	defer m.mu.Unlock()

	m.moveCursor(w.line)
	return barOutput.Write(p)
}

// moveCursor moves the terminal cursor to the given line with the relative
// movement sequences, it must be called with the lock held
func (m *MultiProgressIndicator) moveCursor(line int) {
	if m.cursor == line {
		return
	}
	if line < m.cursor {
		_, _ = fmt.Fprintf(barOutput, "\r\x1b[%dA", m.cursor-line) // move up
	} else {
		_, _ = fmt.Fprintf(barOutput, "\r\x1b[%dB", line-m.cursor) // move down
	}
	m.cursor = line
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
