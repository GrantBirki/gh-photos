package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Logger wraps slog.Logger with application-specific methods
type Logger struct {
	*slog.Logger
}

// LogLevel represents the available log levels
type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
)

// Config holds logger configuration
type Config struct {
	Level   LogLevel
	Output  io.Writer
	Verbose bool
}

// New creates a new logger with the specified configuration
func New(config Config) *Logger {
	if config.Output == nil {
		config.Output = os.Stdout
	}

	// Parse log level
	var level slog.Level
	switch strings.ToLower(string(config.Level)) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Create custom handler that matches your existing format
	// This maintains the clean "[INFO] message" format you like
	handler := &CustomHandler{
		output: config.Output,
		level:  level,
	}

	return &Logger{
		Logger: slog.New(handler),
	}
}

// Info logs an info message (equivalent to your [INFO] format)
func (l *Logger) Info(msg string, args ...any) {
	l.Logger.Info(msg, args...)
}

// Debug logs a debug message (equivalent to your [DEBUG] format)
func (l *Logger) Debug(msg string, args ...any) {
	l.Logger.Debug(msg, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, args ...any) {
	l.Logger.Warn(msg, args...)
}

// Error logs an error message
func (l *Logger) Error(msg string, args ...any) {
	l.Logger.Error(msg, args...)
}

// Infof provides printf-style logging for info level
func (l *Logger) Infof(format string, args ...any) {
	l.Logger.Info(fmt.Sprintf(format, args...))
}

// Debugf provides printf-style logging for debug level
func (l *Logger) Debugf(format string, args ...any) {
	l.Logger.Debug(fmt.Sprintf(format, args...))
}

// Warnf provides printf-style logging for warn level
func (l *Logger) Warnf(format string, args ...any) {
	l.Logger.Warn(fmt.Sprintf(format, args...))
}

// Errorf provides printf-style logging for error level
func (l *Logger) Errorf(format string, args ...any) {
	l.Logger.Error(fmt.Sprintf(format, args...))
}
