package compress

import (
	"archive/zip"
	"errors"
	"io"
	"os"
	"path/filepath"
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
	if sourceFile == "" || targetName == "" {
		err = errors.New("source or target filename is empty")
		return
	}

	var archive *zip.ReadCloser
	if archive, err = zip.OpenReader(sourceFile); err != nil {
		return
	}
	defer func() {
		_ = archive.Close()
	}()

	z.additionBinaries = append(z.additionBinaries, targetName)
	for _, f := range archive.File {
		if f.FileInfo().IsDir() {
			continue
		}

		for _, ff := range z.additionBinaries {
			if filepath.Base(f.Name) != ff {
				continue
			}

			var targetFile *os.File
			if targetFile, err = os.OpenFile(filepath.Join(filepath.Dir(sourceFile), ff),
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
			break
		}
	}
	return
}
