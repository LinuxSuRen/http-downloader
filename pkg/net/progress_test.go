package net

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// silence the progress bar output during the tests
func init() {
	barOutput = io.Discard
}

func TestProgressIndicatorWithKnownSize(t *testing.T) {
	indicator := &ProgressIndicator{
		Title:  "Downloading",
		Total:  float64(10),
		Writer: &bytes.Buffer{},
	}
	indicator.Init()
	assert.NotNil(t, indicator.shared)
	assert.True(t, indicator.owner)

	_, err := indicator.Write([]byte("12345"))
	assert.NoError(t, err)
	assert.Equal(t, float64(5), indicator.shared.bar.State().CurrentBytes)

	indicator.Close()
	indicator.Close() // closing twice should not panic
}

func TestProgressIndicatorWithUnknownSize(t *testing.T) {
	indicator := &ProgressIndicator{
		Title:  "Downloading",
		Writer: &bytes.Buffer{},
	}
	indicator.Init()
	assert.NotNil(t, indicator.shared)

	// an unknown size should not block the progress reporting
	_, err := indicator.Write([]byte("12345"))
	assert.NoError(t, err)
	assert.Equal(t, float64(5), indicator.shared.bar.State().CurrentBytes)
}

func TestProgressIndicatorShare(t *testing.T) {
	root := &ProgressIndicator{
		Title:  "Downloading",
		Total:  float64(20),
		Writer: &bytes.Buffer{},
	}
	root.Init()

	part1 := root.Share()
	assert.NotNil(t, part1)
	assert.False(t, part1.owner)
	part1.Writer = &bytes.Buffer{}
	part2 := root.Share()
	part2.Writer = &bytes.Buffer{}

	_, err := part1.Write([]byte("12345"))
	assert.NoError(t, err)
	_, err = part2.Write([]byte("67890"))
	assert.NoError(t, err)

	// all the parts feed the same bar
	assert.Equal(t, float64(10), root.shared.bar.State().CurrentBytes)

	// closing a part should not finish the shared bar
	part1.Close()
	assert.Equal(t, float64(10), root.shared.bar.State().CurrentBytes)

	root.Close()
}

func TestProgressIndicatorWithoutBar(t *testing.T) {
	// no Init called, the indicator just passes the data through
	indicator := &ProgressIndicator{Writer: &bytes.Buffer{}}
	n, err := indicator.Write([]byte("12345"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "12345", indicator.Writer.(*bytes.Buffer).String())

	// Share of a non-initialized indicator disables the progress
	assert.Nil(t, indicator.Share())

	// should not panic
	indicator.Close()
}

func TestProgressIndicatorRead(t *testing.T) {
	indicator := &ProgressIndicator{
		Reader: strings.NewReader("12345"),
	}
	indicator.Init()

	buf := make([]byte, 3)
	n, err := indicator.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, 3, n)
	assert.Equal(t, float64(3), indicator.shared.bar.State().CurrentBytes)
}

func TestProgressIndicatorShareConcurrent(t *testing.T) {
	const partCount, chunkSize = 8, 12500

	root := &ProgressIndicator{
		Title:  "Downloading",
		Total:  float64(partCount * chunkSize),
		Writer: &bytes.Buffer{},
	}
	root.Init()

	// all the parts download in parallel, they share one bar
	var wg sync.WaitGroup
	for i := 0; i < partCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			part := root.Share()
			sink := &bytes.Buffer{}
			part.Writer = sink

			data := make([]byte, 500)
			for off := 0; off < chunkSize; off += 500 {
				n, err := part.Write(data)
				assert.NoError(t, err)
				assert.Equal(t, 500, n)
			}
			part.Close() // a part cannot finish the shared bar
		}()
	}
	wg.Wait()

	assert.Equal(t, float64(partCount*chunkSize), root.shared.bar.State().CurrentBytes)
	root.Close()
}

func TestHumanBytes(t *testing.T) {
	assert.Equal(t, "0 B", humanBytes(0))
	assert.Equal(t, "512 B", humanBytes(512))
	assert.Equal(t, "1.00 KB", humanBytes(1024))
	assert.Equal(t, "1.50 KB", humanBytes(1024+512))
	assert.Equal(t, "1.00 MB", humanBytes(1024*1024))
	assert.Equal(t, "2.00 GB", humanBytes(2*1024*1024*1024))
}
