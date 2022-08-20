package compress

import (
	"archive/tar"
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"reflect"
	"testing"
)

func TestGetCompressor(t *testing.T) {
	type args struct {
		extension        string
		additionBinaries []string
	}
	tests := []struct {
		name string
		args args
		want Compress
	}{{
		name: "unknown type",
		args: args{extension: ".xdf"},
		want: nil,
	}, {
		name: ".zip",
		args: args{extension: ".zip"},
		want: NewZip(nil),
	}, {
		name: ".xz",
		args: args{extension: ".xz"},
		want: NewXz(nil),
	}, {
		name: ".tar.gz",
		args: args{extension: ".tar.gz"},
		want: NewGZip(nil),
	}, {
		name: ".bz2",
		args: args{extension: ".bz2"},
		want: NewBzip2(nil),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetCompressor(tt.args.extension, tt.args.additionBinaries); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetCompressor() = %v, want %v", got, tt.want)
			} else if got != nil {
				err := got.ExtractFiles("", "")
				assert.NotNil(t, err)

				// test with a regular file
				var f *os.File
				if f, err = os.CreateTemp(os.TempDir(), "fake"); err != nil {
					return
				}
				assert.Nil(t, err)
				assert.NotNil(t, f)
				defer func() {
					_ = os.RemoveAll(f.Name())
				}()

				err = got.ExtractFiles(f.Name(), "fake")
				assert.NotNil(t, err)

				// try to read a non-exist file
				err = got.ExtractFiles(path.Join(os.TempDir(), "fake"), "fake")
				assert.NotNil(t, err)
			}
		})
	}
}

func TestIsSupport(t *testing.T) {
	type args struct {
		extension string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		name: "supported extension: .tar.gz",
		args: args{
			extension: path.Ext("test.tar.gz"),
		},
		want: true,
	}, {
		name: "not supported extension: .ab",
		args: args{
			extension: path.Ext("test.ab"),
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSupport(tt.args.extension); got != tt.want {
				t.Errorf("IsSupport() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_extraFile(t *testing.T) {
	type args struct {
		name       string
		targetName string
		tarFile    string
		header     *tar.Header
		tarReader  *tar.Reader
	}
	tests := []struct {
		name    string
		args    args
		wantErr assert.ErrorAssertionFunc
	}{{
		name: "invalid name",
		args: args{
			name:       "fake",
			targetName: "fake.go",
			tarFile:    "",
			header:     nil,
			tarReader:  nil,
		},
		wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
			assert.Nil(t, err)
			return true
		},
	}, {
		name: "normal",
		args: args{
			name:       "fake",
			targetName: "fake",
			tarFile:    "fake",
			header:     &tar.Header{Mode: 400},
			tarReader:  tar.NewReader(bytes.NewBufferString("fake")),
		},
		wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
			assert.Nil(t, err)
			return true
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var file *os.File
			var err error
			if tt.args.tarFile != "" {
				file, err = os.CreateTemp(os.TempDir(), tt.args.tarFile)
				assert.Nil(t, err)
				assert.NotNil(t, file)
				tt.args.tarFile = file.Name()
				tt.args.targetName = path.Base(file.Name())
				tt.args.name = tt.args.targetName
			}

			if err != nil || file == nil {
				return
			}

			defer func() {
				_ = os.RemoveAll(tt.args.tarFile)
			}()
			tt.wantErr(t, extraFile(tt.args.name, tt.args.targetName, tt.args.tarFile, tt.args.header, tt.args.tarReader), fmt.Sprintf("extraFile(%v, %v, %v, %v, %v)", tt.args.name, tt.args.targetName, tt.args.tarFile, tt.args.header, tt.args.tarReader))
		})
	}
}