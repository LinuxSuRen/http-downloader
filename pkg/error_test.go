package pkg

import "testing"

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
