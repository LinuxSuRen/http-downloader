package core

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRegistry(t *testing.T) {
	reg := &FakeRegistry{}
	reg.Registry("id", nil)

	reg.Walk(func(s string, installer Installer) {
		assert.Equal(t, "id", s)
		assert.Nil(t, installer)
	})
}
