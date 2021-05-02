package common

import (
	"fmt"
	"github.com/magiconair/properties/assert"
	"testing"
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
