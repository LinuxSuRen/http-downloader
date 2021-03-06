package cmd

import (
	"fmt"
	"github.com/magiconair/properties/assert"
	"runtime"
	"testing"
)

func TestIsSupport(t *testing.T) {
	table := []struct {
		cfg     hdConfig
		expect  bool
		message string
	}{{
		cfg:     hdConfig{},
		expect:  true,
		message: "support all os and arch",
	}, {
		cfg: hdConfig{
			SupportOS:   []string{runtime.GOOS},
			SupportArch: []string{runtime.GOARCH},
		},
		expect:  true,
		message: "",
	}, {
		cfg: hdConfig{
			SupportOS:   []string{"fake"},
			SupportArch: []string{runtime.GOARCH},
		},
		expect:  false,
		message: "not support os",
	}, {
		cfg: hdConfig{
			SupportOS:   []string{runtime.GOOS},
			SupportArch: []string{"fake"},
		},
		expect:  false,
		message: "not support arch",
	}}

	for i, item := range table {
		opt := downloadOption{}

		result := opt.isSupport(item.cfg)
		assert.Equal(t, result, item.expect, fmt.Sprintf("index: %d, %s", i, item.message))
	}
}
