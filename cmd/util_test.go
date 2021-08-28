package cmd

import (
	"context"
	"net/http"
	"reflect"
	"testing"
)

func Test_getRoundTripper(t *testing.T) {
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name        string
		args        args
		wantTripper http.RoundTripper
	}{{
		name:        "context is nil",
		wantTripper: nil,
	}, {
		name: "invalid type of RounderTripper in the context",
		args: args{
			ctx: context.WithValue(context.TODO(), "roundTripper", "invalid"),
		},
		wantTripper: nil,
	}, {
		name: "valid type of RounderTripper in the context",
		args: args{
			ctx: context.WithValue(context.TODO(), "roundTripper", &http.Transport{}),
		},
		wantTripper: &http.Transport{},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotTripper := getRoundTripper(tt.args.ctx); !reflect.DeepEqual(gotTripper, tt.wantTripper) {
				t.Errorf("getRoundTripper() = %v, want %v", gotTripper, tt.wantTripper)
			}
		})
	}
}

func Test_getOrDefault(t *testing.T) {
	type args struct {
		key  string
		def  string
		data map[string]string
	}
	tests := []struct {
		name       string
		args       args
		wantResult string
	}{{
		name: "no key exist",
		args: args{
			key: "key",
			def: "def",
			data: map[string]string{},
		},
		wantResult: "def",
	}, {
		name: "key exist",
		args: args{
			key: "key",
			def: "def",
			data: map[string]string{
				"key": "key",
			},
		},
		wantResult: "key",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotResult := getOrDefault(tt.args.key, tt.args.def, tt.args.data); gotResult != tt.wantResult {
				t.Errorf("getOrDefault() = %v, want %v", gotResult, tt.wantResult)
			}
		})
	}
}
