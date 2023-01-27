package pkg

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDownloadError_Error(t *testing.T) {
	tests := []struct {
		name string
		e    DownloadError
		want string
	}{{
		name: "normal",
		e:    DownloadError{Message: "message", StatusCode: 1},
		want: "message: status code: 1",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.e.Error(); got != tt.want {
				t.Errorf("Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorWrap(t *testing.T) {
	sampleErr := errors.New("sample")

	type arg struct {
		err    error
		format string
		args   []string
	}
	tests := []struct {
		name   string
		arg    arg
		hasErr bool
	}{{
		name: "err is not nil",
		arg: arg{
			err:    sampleErr,
			format: "",
		},
		hasErr: true,
	}, {
		name:   "error is nil",
		arg:    arg{},
		hasErr: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ErrorWrap(tt.arg.err, tt.arg.format, tt.arg.args)
			assert.Equal(t, tt.hasErr, err != nil)
		})
	}
}
