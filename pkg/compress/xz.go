package compress

import (
	"archive/tar"
	"errors"
	"fmt"
	"github.com/xi2/xz"
	"io"
	"log"
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

// ExtractFiles extracts files from a target compress file
func (x *Xz) ExtractFiles(sourceFile, targetName string) (err error) {
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

	// Create a xz Reader
	r, err := xz.NewReader(f, 0)
	if err != nil {
		log.Fatal(err)
		return
	}

	var header *tar.Header
	var found bool
	// Create a tar Reader
	tarReader := tar.NewReader(r)
	for {
		if header, err = tarReader.Next(); err == io.EOF {
			err = nil
			break
		}
		if err != nil {
			log.Fatal(err)
			return
		}
		name := header.Name

		switch header.Typeflag {
		case tar.TypeReg:
			if err = extraFile(name, targetName, sourceFile, header, tarReader); err == nil {
				found = true
			} else {
				break
			}

			for i := range x.additionBinaries {
				addition := x.additionBinaries[i]
				if err = extraFile(addition, addition, sourceFile, header, tarReader); err != nil {
					return
				}
			}
		}
	}
	if err == nil && !found {
		err = fmt.Errorf("cannot find item '%s' from '%s'", targetName, sourceFile)
	}
	return
}
