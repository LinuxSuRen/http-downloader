package compress

import (
	"compress/bzip2"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Bzip2 implements a compress which based is based on bzip2
type Bzip2 struct {
	additionBinaries []string
}

// NewBzip2 creates an instance of Bzip2
func NewBzip2(additionBinaries []string) *Bzip2 {
	return &Bzip2{additionBinaries: additionBinaries}
}

// make sure Bzip2 implements the interface Compress
var _ Compress = &Bzip2{}

// ExtractFiles extracts files from a target compress file
func (x *Bzip2) ExtractFiles(sourceFile, targetName string) (err error) {
	if targetName == "" {
		err = errors.New("target filename is empty")
		return
	}
	var f *os.File
	if f, err = os.Open(sourceFile); err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	// Create a Bzip2 Reader
	r := bzip2.NewReader(f)

	var targetFile *os.File
	if targetFile, err = os.OpenFile(fmt.Sprintf("%s/%s", filepath.Dir(sourceFile), targetName),
		os.O_CREATE|os.O_RDWR, 0744); err != nil {
		return
	}
	if _, err = io.Copy(targetFile, r); err != nil {
		return
	}
	return
}
