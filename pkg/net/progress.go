package net

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/k0kubun/go-ansi"
	"github.com/schollz/progressbar/v3"
)

// ProgressIndicator hold the progress of io operation
type ProgressIndicator struct {
	Writer io.Writer
	Reader io.Reader
	Title  string

	// bytes.Buffer
	Total float64
	line  int
	bar   *progressbar.ProgressBar
}

var line int = 0
var currentLine int = 0
var guard sync.Mutex = sync.Mutex{}

// GetCurrentLine returns the current line
func GetCurrentLine() int {
	return currentLine
}

// Init set the default value for progress indicator
func (i *ProgressIndicator) Init() {
	i.line = line
	line++
	i.bar = progressbar.NewOptions64(int64(i.Total),
		progressbar.OptionSetWriter(ansi.NewAnsiStdout()),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(10),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetDescription(fmt.Sprintf("[cyan][reset] %s", i.Title)),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}))
}

// Close shutdowns the ui process
func (i ProgressIndicator) Close() {
	_ = i.bar.Close()
}

// Write writes the progress
// See also https://en.wikipedia.org/wiki/ANSI_escape_code#Sequence_elements
func (i *ProgressIndicator) Write(p []byte) (n int, err error) {
	guard.Lock()
	defer guard.Unlock()
	bias := currentLine - i.line
	currentLine = i.line
	if bias > 0 {
		// move up
		fmt.Fprintf(os.Stdout, "\r\033[%dA", bias)
	} else if bias < 0 {
		// move down
		fmt.Fprintf(os.Stdout, "\r\033[%dB", -bias)
	}
	n, err = io.MultiWriter(i.Writer, i.bar).Write(p)
	return
}

// Read reads the progress
func (i *ProgressIndicator) Read(p []byte) (n int, err error) {
	n, err = io.MultiReader(i.Reader, i.bar).Read(p)
	return
}
