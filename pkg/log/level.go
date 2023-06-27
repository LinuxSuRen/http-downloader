package log

import (
	"context"
	"io"
	syslog "log"
)

// LevelLog is the wrapper of built-in log.Logger
type LevelLog struct {
	*syslog.Logger
	level int
}

// Info prints the info level message.
// Info level means the level >= 3
func (l *LevelLog) Info(v ...any) {
	if l.level >= 3 {
		l.Println(v...)
	}
}

// Debug prints the debug level message.
// Debug level means the level >= 7
func (l *LevelLog) Debug(v ...any) {
	if l.level >= 7 {
		l.Println(v...)
	}
}

// SetLevel sets the level of logger
func (l *LevelLog) SetLevel(level int) *LevelLog {
	l.level = level
	return l
}

// GetLevel returns the level of logger
func (l *LevelLog) GetLevel() int {
	return l.level
}

// SetOutput sets the output destination for the logger.
func (l *LevelLog) SetOutput(writer io.Writer) *LevelLog {
	l.Logger.SetOutput(writer)
	return l
}

// LoggerContext used to get and set context value
type LoggerContext string

// LoggerContextKey is the key of context for get/set Logger
const LoggerContextKey = LoggerContext("LoggerContext")

// ContextAware is the interface for getting context.Context
type ContextAware interface {
	// Context returns the instance of context.Context
	Context() context.Context
}

// GetLoggerFromContextOrDefault returns a Logger instance from context,
// or a default instance if no Logger in the context
func GetLoggerFromContextOrDefault(aware ContextAware) (logger *LevelLog) {
	var ok bool
	if aware.Context() != nil {
		val := aware.Context().Value(LoggerContextKey)
		logger, ok = val.(*LevelLog)
	}

	if !ok {
		logger = GetLogger()
	}
	return
}

// GetLogger returns an instance of Logger
func GetLogger() *LevelLog {
	return &LevelLog{
		Logger: syslog.Default(),
		level:  3,
	}
}

// NewContextWithLogger returns a new context with given logger level
func NewContextWithLogger(ctx context.Context, level int) context.Context {
	logger := GetLogger().SetLevel(level)
	return context.WithValue(ctx, LoggerContextKey, logger)
}
