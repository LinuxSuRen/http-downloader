package compress

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Zip implements a compress which is base on zip file
type Zip struct {
	additionBinaries []string
}

// NewZip creates an instance of zip
func NewZip(additionBinaries []string) *Zip {
	return &Zip{additionBinaries: additionBinaries}
}

// make sure Zip implements the interface Compress
var _ Compress = &Zip{}

// ExtractFiles extracts files from a target compress file
func (z *Zip) ExtractFiles(sourceFile, targetName string) (err error) {
	var archive *zip.ReadCloser
	archive, err = zip.OpenReader(sourceFile)
	defer func() {
		_ = archive.Close()
	}()

	for _, f := range archive.File {
		if f.FileInfo().IsDir() {
			continue
		}

		if strings.HasPrefix(f.Name, "/"+targetName) {
			var targetFile *os.File
			if targetFile, err = os.OpenFile(fmt.Sprintf("%s/%s", filepath.Dir(sourceFile), targetName),
				os.O_CREATE|os.O_RDWR, f.Mode()); err != nil {
				return
			}

			var fileInArchive io.ReadCloser
			fileInArchive, err = f.Open()
			if err != nil {
				return
			}
			if _, err = io.Copy(targetFile, fileInArchive); err != nil {
				return
			}
			_ = targetFile.Close()
			return
		}
	}
	return
}
