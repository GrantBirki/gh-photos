package rclone

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/grantbirki/gh-photos/internal/logger"
)

func TestBuildRemotePathConsistency(t *testing.T) {
	tests := []struct {
		name       string
		remote     string
		targetPath string
		expected   string
	}{
		{
			name:       "standard remote with colon suffix",
			remote:     "GoogleDrive:",
			targetPath: "Backups/iPhone/2021/09/file.jpg",
			expected:   "GoogleDrive:Backups/iPhone/2021/09/file.jpg",
		},
		{
			name:       "remote with embedded path",
			remote:     "GoogleDrive:Backups",
			targetPath: "iPhone/2021/09/file.jpg",
			expected:   "GoogleDrive:Backups/iPhone/2021/09/file.jpg",
		},
		{
			name:       "windows path separators normalized",
			remote:     "GoogleDrive:",
			targetPath: "Backups\\iPhone\\2021\\09\\file.jpg",
			expected:   "GoogleDrive:Backups/iPhone/2021/09/file.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := logger.Config{
				Output: os.Stdout,
				Level:  logger.LevelInfo,
			}
			l := logger.New(config)
			client := NewClient(tt.remote, 1, false, false, false, l, "info")
			result := client.buildRemotePath(tt.targetPath)
			if result != tt.expected {
				t.Errorf("buildRemotePath() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestVerifySingleFileVisibilityDryRun(t *testing.T) {
	// Test dry run behavior
	var logBuffer bytes.Buffer
	config := logger.Config{
		Output: &logBuffer,
		Level:  logger.LevelDebug,
	}
	l := logger.New(config)

	// Create client in dry run mode
	client := NewClient("test-remote", 1, false, true, false, l, "debug")

	// Test dry run - should skip verification
	err := client.verifySingleFileVisibility("remote:path/to/file.jpg")
	if err != nil {
		t.Errorf("Expected no error in dry run mode, got: %v", err)
	}

	// Check logs contain expected dry run message
	logOutput := logBuffer.String()
	if !containsString(logOutput, "dry-run verification skipped") {
		t.Error("Expected dry-run verification skip message in logs")
	}
}

func TestFindExtractionMetadataFile(t *testing.T) {
	// Create temporary directory and metadata file
	tempDir := t.TempDir()
	metadataFile := filepath.Join(tempDir, "extraction-metadata.json")

	// Create a sample metadata file
	err := os.WriteFile(metadataFile, []byte(`{"test": "data"}`), 0644)
	if err != nil {
		t.Fatalf("Failed to create test metadata file: %v", err)
	}

	config := logger.Config{
		Output: os.Stdout,
		Level:  logger.LevelInfo,
	}
	l := logger.New(config)
	client := NewClient("test-remote", 1, false, false, false, l, "info")

	// Test with no backup path set
	result := client.findExtractionMetadataFile()
	if result != "" {
		t.Errorf("Expected empty result with no backup path, got: %s", result)
	}

	// Test with backup path set
	client.SetBackupPath(tempDir)
	result = client.findExtractionMetadataFile()
	if result != metadataFile {
		t.Errorf("Expected %s, got: %s", metadataFile, result)
	}

	// Test with non-existent metadata file
	client.SetBackupPath("/non/existent/path")
	result = client.findExtractionMetadataFile()
	if result != "" {
		t.Errorf("Expected empty result with non-existent path, got: %s", result)
	}
}

func TestMetadataUploadDryRun(t *testing.T) {
	// Test dry run behavior for metadata upload
	var logBuffer bytes.Buffer
	config := logger.Config{
		Output: &logBuffer,
		Level:  logger.LevelDebug,
	}
	l := logger.New(config)

	// Create client in dry run mode
	client := NewClient("test-remote:", 1, false, true, false, l, "debug")

	// Test dry run - should skip upload
	err := client.testRemoteWriteCapabilityStartup()
	if err != nil {
		t.Errorf("Expected no error in dry run mode, got: %v", err)
	}

	// Check logs contain expected dry run message
	logOutput := logBuffer.String()
	if !containsString(logOutput, "dry-run write capability test skipped during startup") {
		t.Error("Expected dry-run write capability skip message in logs")
	}
}

// containsString checks if a string contains a substring (simple helper)
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || containsStringHelper(s, substr))
}

func containsStringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
