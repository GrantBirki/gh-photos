package backup

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateBackupDirectory(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		expectError bool
	}{
		{
			name:        "non-existent directory",
			path:        "/non/existent/path",
			expectError: true,
		},
		{
			name:        "current directory should fail without Manifest.plist",
			path:        ".",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBackupDirectory(tt.path)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestInferMimeType(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"photo.HEIC", "image/heif"},
		{"photo.jpg", "image/jpeg"},
		{"photo.jpeg", "image/jpeg"},
		{"photo.png", "image/png"},
		{"video.mov", "video/quicktime"},
		{"video.mp4", "video/mp4"},
		{"unknown.xyz", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := inferMimeType(tt.filename)
			assert.Equal(t, tt.expected, result)
		})
	}
}
