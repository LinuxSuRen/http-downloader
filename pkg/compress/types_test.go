package compress

import (
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
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetCompressor(tt.args.extension, tt.args.additionBinaries); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetCompressor() = %v, want %v", got, tt.want)
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
