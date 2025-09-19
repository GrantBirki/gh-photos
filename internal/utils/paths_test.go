package utils

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBackupPaths(t *testing.T) {
	tests := []struct {
		name     string
		method   func(*BackupPaths) string
		expected string
	}{
		{
			name:     "ExtractionMetadata",
			method:   (*BackupPaths).ExtractionMetadata,
			expected: filepath.Join("/test/backup", "extraction-metadata.json"),
		},
		{
			name:     "ManifestPlist",
			method:   (*BackupPaths).ManifestPlist,
			expected: filepath.Join("/test/backup", "Manifest.plist"),
		},
		{
			name:     "ManifestDB",
			method:   (*BackupPaths).ManifestDB,
			expected: filepath.Join("/test/backup", "Manifest.db"),
		},
		{
			name:     "BackupSubdir",
			method:   (*BackupPaths).BackupSubdir,
			expected: filepath.Join("/test/backup", "Backup"),
		},
	}

	bp := CreateBackupPaths("/test/backup")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.method(bp)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBackupPathsWithDomain(t *testing.T) {
	bp := CreateBackupPaths("/test/backup")
	mediaDomain := "CameraRollDomain"

	tests := []struct {
		name     string
		method   func(*BackupPaths, string) string
		expected string
	}{
		{
			name:     "MediaDCIM",
			method:   (*BackupPaths).MediaDCIM,
			expected: filepath.Join("/test/backup", "CameraRollDomain", "Media", "DCIM"),
		},
		{
			name:     "DCIM",
			method:   (*BackupPaths).DCIM,
			expected: filepath.Join("/test/backup", "CameraRollDomain", "DCIM"),
		},
		{
			name:     "MediaPhotoData",
			method:   (*BackupPaths).MediaPhotoData,
			expected: filepath.Join("/test/backup", "CameraRollDomain", "Media", "PhotoData"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.method(bp, mediaDomain)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBackupPathsWithDomainAndFile(t *testing.T) {
	bp := CreateBackupPaths("/test/backup")
	mediaDomain := "CameraRollDomain"
	filename := "IMG_001.HEIC"

	tests := []struct {
		name     string
		method   func(*BackupPaths, string, string) string
		expected string
	}{
		{
			name:     "MediaDCIMFile",
			method:   (*BackupPaths).MediaDCIMFile,
			expected: filepath.Join("/test/backup", "CameraRollDomain", "Media", "DCIM", "IMG_001.HEIC"),
		},
		{
			name:     "MediaFile",
			method:   (*BackupPaths).MediaFile,
			expected: filepath.Join("/test/backup", "CameraRollDomain", "Media", "IMG_001.HEIC"),
		},
		{
			name:     "DomainFile",
			method:   (*BackupPaths).DomainFile,
			expected: filepath.Join("/test/backup", "CameraRollDomain", "IMG_001.HEIC"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.method(bp, mediaDomain, filename)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRelativePath(t *testing.T) {
	bp := CreateBackupPaths("/test/backup")

	result := bp.RelativePath("some/relative/path")
	expected := filepath.Join("/test/backup", "some/relative/path")

	assert.Equal(t, expected, result)
}

func TestNewBackupPaths(t *testing.T) {
	backupPath := "/test/backup"
	bp := CreateBackupPaths(backupPath)

	assert.NotNil(t, bp)
	assert.Equal(t, backupPath, bp.backupPath)
}
