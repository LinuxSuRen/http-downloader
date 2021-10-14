package compress

import (
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
		want: NewGZip(nil),
	}, {
		name: ".zip",
		args: args{extension: ".zip"},
		want: NewZip(nil),
	}, {
		name: ".xz",
		args: args{extension: ".xz"},
		want: NewXz(nil),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetCompressor(tt.args.extension, tt.args.additionBinaries); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetCompressor() = %v, want %v", got, tt.want)
			}
		})
	}
}
