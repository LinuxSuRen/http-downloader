package compress

import (
	"archive/tar"
	"errors"
	"io"
	"log"
	"os"

	"github.com/xi2/xz"
)

// Xz implements a compress which based is based on xz
type Xz struct{}

// NewXz creates an instance of Xz
func NewXz() *Xz {
	return &Xz{}
}

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
		switch header.Typeflag {
		case tar.TypeReg:
			w, err := os.Create(header.Name)
			if err != nil {
				log.Fatal(err)
				break
			}
			_, err = io.Copy(w, tarReader)
			if err != nil {
				log.Fatal(err)
				break
			}
			w.Close()
		}
	}
	f.Close()
	return
}
