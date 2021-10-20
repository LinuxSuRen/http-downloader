package common

import (
	"fmt"
	"os"
	"testing"

	"github.com/magiconair/properties/assert"
)

func TestGetOrDefault(t *testing.T) {
	table := []testCase{
		{key: "fake", def: "def", data: map[string]string{}, expected: "def"},
		{key: "fake", def: "def", data: nil, expected: "def"},
		{key: "fake", def: "def", data: map[string]string{
			"fake": "good",
		}, expected: "good"},
	}

	for index, item := range table {
		result := GetOrDefault(item.key, item.def, item.data)
		assert.Equal(t, result, item.expected, fmt.Sprintf("test failed with '%d'", index))
	}
}

func TestGetReplacement(t *testing.T) {
	table := []testCase{
		{key: "fake", data: map[string]string{}, expected: "fake"},
		{key: "fake", data: nil, expected: "fake"},
		{key: "fake", data: map[string]string{
			"fake": "good",
		}, expected: "good"},
	}

	for index, item := range table {
		result := GetReplacement(item.key, item.data)
		assert.Equal(t, result, item.expected, fmt.Sprintf("test failed with '%d'", index))
	}
}

type testCase struct {
	key      string
	def      string
	data     map[string]string
	expected string
}

func TestGetEnvironment(t *testing.T) {
	type args struct {
		key             string
		alternativeKeys []string
	}
	tests := []struct {
		name    string
		initEnv func()
		args    args
		want    string
	}{{
		name: "Single key exists",
		initEnv: func() {
			os.Setenv("FAKE_KEY", "fake value")
		},
		args: args{
			key: "FAKE_KEY",
		},
		want: "fake value",
	}, {
		name: "Single key non exist",
		initEnv: func() {
			os.Unsetenv("FAKE_KEY")
		},
		args: args{
			key: "FAKE_ENV",
		},
		want: "",
	}, {
		name: "With one alternative key",
		initEnv: func() {
			os.Unsetenv("FAKE_KEY")
			os.Setenv("alt_key", "alt_value")
		},
		args: args{
			key:             "FAKE_KEY",
			alternativeKeys: []string{"alt_key"},
		},
		want: "alt_value",
	}, {
		name: "With one alternative key but key exists",
		initEnv: func() {
			os.Setenv("FAKE_KEY", "fake_value")
		},
		args: args{
			key:             "FAKE_KEY",
			alternativeKeys: []string{"alt_key"},
		},
		want: "fake_value",
	}, {
		name: "With one alternative key but both of them exist",
		initEnv: func() {
			os.Setenv("FAKE_KEY", "fake_value")
			os.Setenv("alt_key", "alt_value")
		},
		args: args{
			key:             "FAKE_KEY",
			alternativeKeys: []string{"alt_key"},
		},
		want: "fake_value",
	}, {
		name: "With two alternative keys",
		initEnv: func() {
			os.Unsetenv("FAKE_KEY")
			os.Unsetenv("alt_key_1")
			os.Setenv("alt_key_2", "alt_value_2")
		},
		args: args{
			key:             "FAKE_KEY",
			alternativeKeys: []string{"alt_key_1", "alt_key_2"},
		},
		want: "alt_value_2",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.initEnv != nil {
				tt.initEnv()
			}
			if got := GetEnvironment(tt.args.key, tt.args.alternativeKeys...); got != tt.want {
				t.Errorf("GetEnvironment() = %v, want %v", got, tt.want)
			}
		})
	}
}
