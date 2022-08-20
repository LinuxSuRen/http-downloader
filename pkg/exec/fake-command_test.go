package exec

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLookPath(t *testing.T) {
	fake := FakeExecer{
		ExpectError: errors.New("fake"),
	}
	_, err := fake.LookPath("")
	assert.NotNil(t, err)

	fake.ExpectError = nil
	_, err = fake.LookPath("")
	assert.Nil(t, err)
}
