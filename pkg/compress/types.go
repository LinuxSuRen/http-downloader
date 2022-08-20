package compress

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Compress is a common compress interface
type Compress interface {
	ExtractFiles(sourceFile, targetName string) error
}

func extraFile(name, targetName, tarFile string, header *tar.Header, tarReader *tar.Reader) (err error) {
	if name != targetName && !strings.HasSuffix(name, "/"+targetName) {
		return
	}
	var targetFile *os.File
	if targetFile, err = os.OpenFile(fmt.Sprintf("%s/%s", filepath.Dir(tarFile), targetName),
		os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode)); err != nil {
		return
	}
	defer func() {
		_ = targetFile.Close()
	}()
	_, err = io.Copy(targetFile, tarReader)
	return
}

// GetCompressor gets the compressor base on file extension
func GetCompressor(extension string, additionBinaries []string) Compress {
	// Select the right decompressor based on file type
	switch extension {
	case ".xz":
		return NewXz(additionBinaries)
	case ".zip":
		return NewZip(additionBinaries)
	case ".gz", ".tar.gz", ".tgz":
		return NewGZip(additionBinaries)
	case ".bz2":
		return NewBzip2(additionBinaries)
	}
	return nil
}

// IsSupport checks if the desired file extension
func IsSupport(extension string) bool {
	return GetCompressor(extension, nil) != nil
}
