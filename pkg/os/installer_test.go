package os

import (
	"runtime"
	"testing"
)

func TestHasPackage(t *testing.T) {
	// currently, this function only support Linux
	if runtime.GOOS != "linux" {
		return
	}

	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		name: "fake",
		args: args{
			name: "fake",
		},
		want: false,
	}, {
		name: "docker",
		args: args{
			name: "docker",
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasPackage(tt.args.name); got != tt.want {
				t.Errorf("HasPackage() = %v, want %v", got, tt.want)
			}
		})
	}
}
