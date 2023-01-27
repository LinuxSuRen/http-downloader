package version_test

import (
	"testing"

	"github.com/linuxsuren/http-downloader/pkg/version"
	"github.com/stretchr/testify/assert"
)

func TestGetVersion(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		expectVer string
	}{{
		name:      "invalid",
		output:    "abc",
		expectVer: "0.0.0",
	}, {
		name: "normal",
		output: `minikube version: v1.28.0
commit: 986b1ebd987211ed16f8cc10aed7d2c42fc8392f`,
		expectVer: "1.28.0",
	}, {
		name: "complex",
		output: `
____  __.________
|    |/ _/   __   \______
|      < \____    /  ___/
|    |  \   /    /\___ \
|____|__ \ /____//____  >
		\/            \/

Version:    v0.26.3
Commit:     0893f13b3ca6b563dd0c38fdebaefdb8be594825
Date:       2022-08-04T05:18:24Z`,
		expectVer: "0.26.3",
	}, {
		name:      "without prefix v",
		output:    "git version 2.34.1",
		expectVer: "2.34.1",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := version.GetVersion(tt.output)
			assert.Equal(t, tt.expectVer, result)
		})
	}
}

func TestGreatThan(t *testing.T) {
	tests := []struct {
		name   string
		target string
		output string
		expect bool
	}{{
		name:   "normal",
		target: "v1.28.1",
		output: `minikube version: v1.28.0
commit: 986b1ebd987211ed16f8cc10aed7d2c42fc8392f`,
		expect: true,
	}, {
		name:   "normal",
		target: "v1.28.0",
		output: `minikube version: v1.28.1
commit: 986b1ebd987211ed16f8cc10aed7d2c42fc8392f`,
		expect: false,
	}, {
		name:   "version is equal",
		target: "v1.28.0",
		output: `minikube version: v1.28.0
commit: 986b1ebd987211ed16f8cc10aed7d2c42fc8392f`,
		expect: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := version.GreatThan(tt.target, tt.output)
			assert.Equal(t, tt.expect, result)
		})
	}
}
