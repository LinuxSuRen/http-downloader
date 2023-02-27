package log

import (
	"context"
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
// Debug level means the level >= 3
func (l *LevelLog) Debug(v ...any) {
	if l.level >= 7 {
		l.Println(v...)
	}
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
	logger.level = 3
	return
}

// GetLogger returns an instance of Logger
func GetLogger() *LevelLog {
	return &LevelLog{
		Logger: syslog.Default(),
	}
}
