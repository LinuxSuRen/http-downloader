package common

import (
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

func TestExist(t *testing.T) {
	tests := []struct {
		name           string
		createFilepath func(*testing.T) string
		wantResult     bool
	}{{
		name: "a existing regular file",
		createFilepath: func(t *testing.T) string {
			var f *os.File
			var err error
			f, err = os.CreateTemp(os.TempDir(), "fake")
			assert.Nil(t, err)
			assert.NotNil(t, f)
			return f.Name()
		},
		wantResult: true,
	}, {
		name: "a existing directory",
		createFilepath: func(t *testing.T) string {
			dir, err := os.MkdirTemp(os.TempDir(), "fake")
			assert.Nil(t, err)
			assert.NotEmpty(t, dir)
			return dir
		},
		wantResult: true,
	}, {
		name: "non-exsit regular file",
		createFilepath: func(t *testing.T) string {
			return path.Join(os.TempDir(), fmt.Sprintf("%d", time.Now().Nanosecond()))
		},
		wantResult: false,
	}}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filepath := tt.createFilepath(t)
			if filepath != "" {
				defer func() {
					_ = os.RemoveAll(filepath)
				}()
			}

			ok := Exist(filepath)
			if tt.wantResult {
				assert.True(t, ok, "should exist in case [%d]-[%s]", i, tt.name)
			} else {
				assert.False(t, ok, "should not exist in case [%d]-[%s]", i, tt.name)
			}
		})
	}
}

func TestParseVersionNum(t *testing.T) {
	tests := []struct {
		name    string
		version string
		expect  string
	}{{
		name:    "version start with v",
		version: "v1.2.3",
		expect:  "1.2.3",
	}, {
		name:    "version has not prefix v",
		version: "1.2.3",
		expect:  "1.2.3",
	}, {
		name:    "have more prefix",
		version: "alpha-v1.2.3",
		expect:  "1.2.3",
	}}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version := ParseVersionNum(tt.version)
			assert.Equal(t, tt.expect, version, "expect [%s], got [%s] in case [%d]", tt.expect, version, i)
		})
	}
}

func TestIsDirWriteable(t *testing.T) {
	tests := []struct {
		name    string
		dir     string
		wantErr bool
	}{{
		name:    "should writeable",
		dir:     os.TempDir(),
		wantErr: false,
	}, {
		name:    "should not writable",
		dir:     path.Join(os.TempDir(), "fake", "dir"),
		wantErr: true,
	}}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := IsDirWriteable(tt.dir)
			if tt.wantErr {
				assert.NotNil(t, err, "expect error, but not in case [%d]", i)
			} else {
				assert.Nil(t, err, "expect not error, but have in case [%d]", i)
			}
		})
	}
}

func TestCheckDirPermission(t *testing.T) {
	tests := []struct {
		name    string
		dir     string
		perm    os.FileMode
		wantErr bool
	}{{
		name:    "dir is empty",
		dir:     "",
		wantErr: true,
	}, {
		name:    "non-exsiting dir",
		dir:     path.Join(os.TempDir(), "fake"),
		wantErr: true,
	}}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckDirPermission(tt.dir, tt.perm)
			if tt.wantErr {
				assert.NotNil(t, err, "expect error, but not in case [%d]", i)
			} else {
				assert.Nil(t, err, "expect not error, but have in case [%d]", i)
			}
		})
	}
}
