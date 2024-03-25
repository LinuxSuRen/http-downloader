package compress

import (
	"archive/tar"
	"errors"
	"github.com/xi2/xz"
	"os"
)

// Xz implements a compress which based is based on xz
type Xz struct {
	additionBinaries []string
}

// NewXz creates an instance of Xz
func NewXz(additionBinaries []string) *Xz {
	return &Xz{additionBinaries: additionBinaries}
}

// make sure Xz implements the interface Compress
var _ Compress = &Xz{}

// ExtractFiles extracts files from a target compress file
func (x *Xz) ExtractFiles(sourceFile, targetName string) (err error) {
	if sourceFile == "" || targetName == "" {
		err = errors.New("source or target filename is empty")
		return
	}
	var f *os.File
	if f, err = os.Open(sourceFile); err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	// Create a xz Reader
	r, err := xz.NewReader(f, 0)
	if err != nil {
		return
	}

	err = zipProcess(tar.NewReader(r), targetName, sourceFile, x.additionBinaries)
	return
}
