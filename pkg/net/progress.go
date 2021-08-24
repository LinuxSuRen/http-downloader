package net

import (
	"fmt"
	"github.com/gosuri/uiprogress"
	"io"
)

// ProgressIndicator hold the progress of io operation
type ProgressIndicator struct {
	Writer io.Writer
	Reader io.Reader
	Title  string

	// bytes.Buffer
	Total float64
	count float64
	bar   *uiprogress.Bar
}

var process *uiprogress.Progress

// Init set the default value for progress indicator
func (i *ProgressIndicator) Init() {
	// start rendering
	if process == nil {
		process = uiprogress.New()
		process.Start()
	}
	i.bar = process.AddBar(100) // Add a new bar

	// optionally, append and prepend completion and elapsed time
	i.bar.AppendCompleted()
	//i.bar.PrependElapsed()

	if i.Title != "" {
		i.bar.PrependFunc(func(_ *uiprogress.Bar) string {
			return fmt.Sprintf("%s: ", i.Title)
		})
	}
}

// Close shutdowns the ui process
func (i ProgressIndicator) Close() {
	if process != nil {
		process.Stop()
		process = nil
	}
}

// Write writes the progress
func (i *ProgressIndicator) Write(p []byte) (n int, err error) {
	n, err = i.Writer.Write(p)
	i.setBar(n)
	return
}

// Read reads the progress
func (i *ProgressIndicator) Read(p []byte) (n int, err error) {
	n, err = i.Reader.Read(p)
	i.setBar(n)
	return
}

func (i *ProgressIndicator) setBar(n int) {
	i.count += float64(n)

	if i.bar != nil {
		_ = i.bar.Set((int)(i.count * 100 / i.Total))
	}
}
