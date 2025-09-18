package logger

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoggerOutput(t *testing.T) {
	tests := []struct {
		name        string
		level       LogLevel
		logFunc     func(*Logger, string, ...any)
		message     string
		expectLevel string
	}{
		{
			name:        "info message",
			level:       LevelInfo,
			logFunc:     (*Logger).Info,
			message:     "Starting iPhone photo backup process...",
			expectLevel: "INFO",
		},
		{
			name:        "debug message",
			level:       LevelDebug,
			logFunc:     (*Logger).Debug,
			message:     "Generated Photos.sqlite query: SELECT * FROM ZASSET",
			expectLevel: "DEBUG",
		},
		{
			name:        "error message",
			level:       LevelError,
			logFunc:     (*Logger).Error,
			message:     "Failed to parse assets",
			expectLevel: "ERROR",
		},
		{
			name:        "warn message",
			level:       LevelWarn,
			logFunc:     (*Logger).Warn,
			message:     "No trashed column found, using fallback",
			expectLevel: "WARN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			config := Config{
				Level:  tt.level,
				Output: &buf,
			}

			logger := New(config)
			tt.logFunc(logger, tt.message)

			output := buf.String()

			// Check that output matches expected format: "YYYY/MM/DD HH:MM:SS [LEVEL] message\n"
			assert.Contains(t, output, tt.expectLevel)
			assert.Contains(t, output, tt.message)

			// Check format structure
			parts := strings.Split(strings.TrimSpace(output), " ")
			assert.True(t, len(parts) >= 3)

			// Should start with date (YYYY/MM/DD)
			assert.Contains(t, parts[0], "/")

			// Should have time (HH:MM:SS)
			assert.Contains(t, parts[1], ":")

			// Should have level in brackets [LEVEL]
			assert.Equal(t, "["+tt.expectLevel+"]", parts[2])
		})
	}
}

func TestLogLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	config := Config{
		Level:  LevelInfo, // Set to INFO level
		Output: &buf,
	}

	logger := New(config)

	// These should be logged
	logger.Info("This should appear")
	logger.Warn("This should also appear")
	logger.Error("This should definitely appear")

	// This should NOT be logged (DEBUG < INFO)
	logger.Debug("This should NOT appear")

	output := buf.String()

	assert.Contains(t, output, "This should appear")
	assert.Contains(t, output, "This should also appear")
	assert.Contains(t, output, "This should definitely appear")
	assert.NotContains(t, output, "This should NOT appear")
}

func TestLoggerPrintfStyle(t *testing.T) {
	var buf bytes.Buffer
	config := Config{
		Level:  LevelInfo,
		Output: &buf,
	}

	logger := New(config)
	logger.Infof("Backup path: %s", "C:\\Users\\Birki\\Apple\\MobileSync\\Backup\\00008110-000939D83484801E")

	output := buf.String()
	assert.Contains(t, output, "[INFO]")
	assert.Contains(t, output, "Backup path: C:\\Users\\Birki\\Apple\\MobileSync\\Backup\\00008110-000939D83484801E")
}
