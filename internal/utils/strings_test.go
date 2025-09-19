package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "lowercase input",
			input:    "debug",
			expected: "debug",
		},
		{
			name:     "uppercase input",
			input:    "DEBUG",
			expected: "debug",
		},
		{
			name:     "mixed case input",
			input:    "Info",
			expected: "info",
		},
		{
			name:     "input with whitespace",
			input:    "  warn  ",
			expected: "warn",
		},
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
		{
			name:     "whitespace only",
			input:    "   ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateStringInSet(t *testing.T) {
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}

	tests := []struct {
		name        string
		input       string
		expectedStr string
		expectedOk  bool
	}{
		{
			name:        "valid lowercase",
			input:       "debug",
			expectedStr: "debug",
			expectedOk:  true,
		},
		{
			name:        "valid uppercase",
			input:       "DEBUG",
			expectedStr: "debug",
			expectedOk:  true,
		},
		{
			name:        "valid with whitespace",
			input:       "  info  ",
			expectedStr: "info",
			expectedOk:  true,
		},
		{
			name:        "invalid value",
			input:       "invalid",
			expectedStr: "invalid",
			expectedOk:  false,
		},
		{
			name:        "empty input",
			input:       "",
			expectedStr: "",
			expectedOk:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			str, ok := ValidateStringInSet(tt.input, validLogLevels)
			assert.Equal(t, tt.expectedStr, str)
			assert.Equal(t, tt.expectedOk, ok)
		})
	}
}
