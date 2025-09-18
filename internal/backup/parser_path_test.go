package backup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveBackupPath(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "backup-path-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name           string
		setupFunc      func(string) string // Returns the input path to test
		expectedResult string              // Expected resolved path (relative to tempDir)
		shouldError    bool
		description    string
	}{
		{
			name: "direct_backup_directory",
			setupFunc: func(baseDir string) string {
				backupDir := filepath.Join(baseDir, "direct-backup")
				os.Mkdir(backupDir, 0755)
				// Create Manifest.plist to make it a valid backup
				manifestPath := filepath.Join(backupDir, "Manifest.plist")
				statusPath := filepath.Join(backupDir, "Status.plist")
				os.WriteFile(manifestPath, []byte("test"), 0644)
				os.WriteFile(statusPath, []byte("test"), 0644)
				return backupDir
			},
			expectedResult: "direct-backup",
			description:    "Should return the same path if it's already a valid backup directory",
		},
		{
			name: "mobilesync_with_backup_subdir",
			setupFunc: func(baseDir string) string {
				mobileSyncDir := filepath.Join(baseDir, "MobileSync")
				backupDir := filepath.Join(mobileSyncDir, "Backup")
				actualBackupDir := filepath.Join(backupDir, "00008110-000939D83484801E")

				os.MkdirAll(actualBackupDir, 0755)

				// Create Manifest.plist in the actual backup directory
				manifestPath := filepath.Join(actualBackupDir, "Manifest.plist")
				statusPath := filepath.Join(actualBackupDir, "Status.plist")
				os.WriteFile(manifestPath, []byte("test"), 0644)
				os.WriteFile(statusPath, []byte("test"), 0644)

				return mobileSyncDir
			},
			expectedResult: "MobileSync/Backup/00008110-000939D83484801E",
			description:    "Should navigate from MobileSync -> Backup -> single backup folder",
		},
		{
			name: "backup_directory_with_manifest",
			setupFunc: func(baseDir string) string {
				backupParentDir := filepath.Join(baseDir, "BackupParent")
				backupDir := filepath.Join(backupParentDir, "Backup")

				os.MkdirAll(backupDir, 0755)

				// Create Manifest.plist directly in Backup directory
				manifestPath := filepath.Join(backupDir, "Manifest.plist")
				statusPath := filepath.Join(backupDir, "Status.plist")
				os.WriteFile(manifestPath, []byte("test"), 0644)
				os.WriteFile(statusPath, []byte("test"), 0644)

				return backupParentDir
			},
			expectedResult: "BackupParent/Backup",
			description:    "Should navigate to Backup subdirectory if it contains Manifest.plist",
		},
		{
			name: "multiple_backup_folders",
			setupFunc: func(baseDir string) string {
				parentDir := filepath.Join(baseDir, "MultipleBackups")
				backupDir := filepath.Join(parentDir, "Backup")
				backup1 := filepath.Join(backupDir, "backup1")
				backup2 := filepath.Join(backupDir, "backup2")

				os.MkdirAll(backup1, 0755)
				os.MkdirAll(backup2, 0755)

				// Create manifests in both (should error for safety)
				os.WriteFile(filepath.Join(backup1, "Manifest.plist"), []byte("test"), 0644)
				os.WriteFile(filepath.Join(backup1, "Status.plist"), []byte("test"), 0644)
				os.WriteFile(filepath.Join(backup2, "Manifest.plist"), []byte("test"), 0644)
				os.WriteFile(filepath.Join(backup2, "Status.plist"), []byte("test"), 0644)

				return parentDir
			},
			shouldError: true,
			description: "Should error when multiple backup directories exist for safety",
		},
		{
			name: "no_backup_directory",
			setupFunc: func(baseDir string) string {
				noBackupDir := filepath.Join(baseDir, "NoBackup")
				os.Mkdir(noBackupDir, 0755)
				return noBackupDir
			},
			expectedResult: "NoBackup",
			description:    "Should return original path if no Backup subdirectory exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputPath := tt.setupFunc(tempDir)

			result, err := resolveBackupPath(inputPath)

			if tt.shouldError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			// Convert expected result to absolute path
			expectedPath := filepath.Join(tempDir, tt.expectedResult)
			assert.Equal(t, expectedPath, result, tt.description)
		})
	}
}

func TestIsValidBackupDir(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "is-valid-backup-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name        string
		setupFunc   func(string) string
		expected    bool
		description string
	}{
		{
			name: "valid_backup_with_manifest_plist_and_status",
			setupFunc: func(baseDir string) string {
				backupDir := filepath.Join(baseDir, "valid-basic")
				os.Mkdir(backupDir, 0755)
				manifestPath := filepath.Join(backupDir, "Manifest.plist")
				statusPath := filepath.Join(backupDir, "Status.plist")
				os.WriteFile(manifestPath, []byte("test"), 0644)
				os.WriteFile(statusPath, []byte("test"), 0644)
				return backupDir
			},
			expected:    true,
			description: "Should be valid with Manifest.plist and Status.plist",
		},
		{
			name: "valid_backup_with_both_manifests",
			setupFunc: func(baseDir string) string {
				backupDir := filepath.Join(baseDir, "valid-both")
				os.Mkdir(backupDir, 0755)
				manifestPlistPath := filepath.Join(backupDir, "Manifest.plist")
				manifestDBPath := filepath.Join(backupDir, "Manifest.db")
				statusPath := filepath.Join(backupDir, "Status.plist")
				os.WriteFile(manifestPlistPath, []byte("test"), 0644)
				os.WriteFile(manifestDBPath, []byte("test"), 0644)
				os.WriteFile(statusPath, []byte("test"), 0644)
				return backupDir
			},
			expected:    true,
			description: "Should be valid with both manifest files",
		},
		{
			name: "valid_backup_with_hex_dirs",
			setupFunc: func(baseDir string) string {
				backupDir := filepath.Join(baseDir, "valid-hex")
				os.Mkdir(backupDir, 0755)
				manifestPath := filepath.Join(backupDir, "Manifest.plist")
				statusPath := filepath.Join(backupDir, "Status.plist")
				os.WriteFile(manifestPath, []byte("test"), 0644)
				os.WriteFile(statusPath, []byte("test"), 0644)

				// Create some hex directories
				for _, hexDir := range []string{"00", "01", "02", "ff", "ab", "cd", "ef", "12", "34", "56", "78", "9a"} {
					os.Mkdir(filepath.Join(backupDir, hexDir), 0755)
				}
				return backupDir
			},
			expected:    true,
			description: "Should be valid with manifest and hex directories",
		},
		{
			name: "invalid_no_manifest_plist",
			setupFunc: func(baseDir string) string {
				backupDir := filepath.Join(baseDir, "invalid-no-manifest")
				os.Mkdir(backupDir, 0755)
				return backupDir
			},
			expected:    false,
			description: "Should be invalid without Manifest.plist",
		},
		{
			name: "nonexistent_directory",
			setupFunc: func(baseDir string) string {
				return filepath.Join(baseDir, "nonexistent")
			},
			expected:    false,
			description: "Should be invalid for nonexistent directory",
		},
		{
			name: "file_instead_of_directory",
			setupFunc: func(baseDir string) string {
				filePath := filepath.Join(baseDir, "not-a-dir")
				os.WriteFile(filePath, []byte("test"), 0644)
				return filePath
			},
			expected:    false,
			description: "Should be invalid for file instead of directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testPath := tt.setupFunc(tempDir)
			result := isValidBackupDir(testPath)
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

func TestValidateBackupDirectoryExtracted(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func() string
		expectError bool
		errorMsg    string
	}{
		{
			name: "non-existent_directory",
			setupFunc: func() string {
				return "/path/that/does/not/exist"
			},
			expectError: true,
			errorMsg:    "backup path does not exist",
		},
		{
			name: "extracted_directory_should_succeed",
			setupFunc: func() string {
				tempDir, _ := os.MkdirTemp("", "extracted-backup-test")

				// Create extraction-metadata.json to simulate extracted directory
				metadataPath := filepath.Join(tempDir, "extraction-metadata.json")
				os.WriteFile(metadataPath, []byte(`{"command_metadata": {"completed_at": "2024-01-01T00:00:00Z"}}`), 0644)

				return tempDir
			},
			expectError: false,
			errorMsg:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testPath := tt.setupFunc()
			if strings.HasPrefix(testPath, "/tmp") || strings.HasPrefix(testPath, os.TempDir()) {
				defer os.RemoveAll(testPath)
			}

			err := validateBackupDirectory(testPath)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
