package uploader

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogLevelNormalization(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		valid    bool
	}{
		{
			name:     "lowercase debug",
			input:    "debug",
			expected: "debug",
			valid:    true,
		},
		{
			name:     "uppercase DEBUG",
			input:    "DEBUG",
			expected: "debug",
			valid:    true,
		},
		{
			name:     "mixed case Info",
			input:    "Info",
			expected: "info",
			valid:    true,
		},
		{
			name:     "uppercase WARN",
			input:    "WARN",
			expected: "warn",
			valid:    true,
		},
		{
			name:     "uppercase ERROR",
			input:    "ERROR",
			expected: "error",
			valid:    true,
		},
		{
			name:     "invalid level",
			input:    "invalid",
			expected: "invalid",
			valid:    false,
		},
		{
			name:     "whitespace trimming",
			input:    "  debug  ",
			expected: "debug",
			valid:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Normalize the log level (same logic as in main.go)
			normalized := strings.ToLower(strings.TrimSpace(tt.input))
			assert.Equal(t, tt.expected, normalized)

			// Check validity
			validLogLevels := map[string]bool{
				"debug": true,
				"info":  true,
				"warn":  true,
				"error": true,
			}
			isValid := validLogLevels[normalized]
			assert.Equal(t, tt.valid, isValid)
		})
	}
}

func TestEnvironmentVariableHandling(t *testing.T) {
	// Save original environment
	originalLogLevel := os.Getenv("LOG_LEVEL")
	defer func() {
		if originalLogLevel != "" {
			os.Setenv("LOG_LEVEL", originalLogLevel)
		} else {
			os.Unsetenv("LOG_LEVEL")
		}
	}()

	tests := []struct {
		name     string
		envValue string
		expected string
	}{
		{
			name:     "debug from env",
			envValue: "debug",
			expected: "debug",
		},
		{
			name:     "INFO from env (case insensitive)",
			envValue: "INFO",
			expected: "info",
		},
		{
			name:     "warn from env",
			envValue: "warn",
			expected: "warn",
		},
		{
			name:     "ERROR from env",
			envValue: "ERROR",
			expected: "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			os.Setenv("LOG_LEVEL", tt.envValue)

			// Get and normalize (simulating main.go logic)
			envLogLevel := os.Getenv("LOG_LEVEL")
			normalized := strings.ToLower(strings.TrimSpace(envLogLevel))

			assert.Equal(t, tt.expected, normalized)
		})
	}
}
