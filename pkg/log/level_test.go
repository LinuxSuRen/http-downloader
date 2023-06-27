package log_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/linuxsuren/http-downloader/pkg/log"
	"github.com/stretchr/testify/assert"
)

func TestLogger(t *testing.T) {
	t.Run("default level", func(t *testing.T) {
		logger := log.GetLoggerFromContextOrDefault(&fakeContextAwareObj{})
		assert.Equal(t, 3, logger.GetLevel())

		logger = log.GetLoggerFromContextOrDefault(&fakeContextAwareObj{ctx: context.Background()})
		assert.Equal(t, 3, logger.GetLevel())

		ctx := log.NewContextWithLogger(context.Background(), 5)
		logger = log.GetLoggerFromContextOrDefault(&fakeContextAwareObj{ctx: ctx})
		assert.Equal(t, 5, logger.GetLevel())
	})

	t.Run("print in different level", func(t *testing.T) {
		buf := new(bytes.Buffer)
		logger := log.GetLogger().SetOutput(buf)

		logger.Debug("debug")
		logger.Info("info")

		assert.Contains(t, buf.String(), "info")

		logger.SetLevel(7)
		logger.Debug("debug")
		assert.Contains(t, buf.String(), "debug")
	})
}

type fakeContextAwareObj struct {
	ctx context.Context
}

func (f *fakeContextAwareObj) Context() context.Context {
	return f.ctx
}
