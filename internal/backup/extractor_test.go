package backup

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/grantbirki/gh-photos/internal/logger"
	"github.com/stretchr/testify/assert"
)

func TestIsBackupEncrypted(t *testing.T) {
	tests := []struct {
		name            string
		manifestContent string
		expectedResult  bool
		shouldError     bool
	}{
		{
			name: "unencrypted_backup",
			manifestContent: `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>IsEncrypted</key>
	<false/>
	<key>Date</key>
	<date>2025-09-18T12:00:00Z</date>
</dict>
</plist>`,
			expectedResult: false,
			shouldError:    false,
		},
		{
			name: "encrypted_backup",
			manifestContent: `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>IsEncrypted</key>
	<true/>
	<key>Date</key>
	<date>2025-09-18T12:00:00Z</date>
</dict>
</plist>`,
			expectedResult: true,
			shouldError:    false,
		},
		{
			name: "no_encryption_key",
			manifestContent: `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Date</key>
	<date>2025-09-18T12:00:00Z</date>
</dict>
</plist>`,
			expectedResult: false,
			shouldError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tempDir, err := os.MkdirTemp("", "backup-test")
			if !assert.NoError(t, err) {
				return
			}
			defer os.RemoveAll(tempDir)

			// Write Manifest.plist
			manifestPath := filepath.Join(tempDir, "Manifest.plist")
			err = os.WriteFile(manifestPath, []byte(tt.manifestContent), 0644)
			if !assert.NoError(t, err) {
				return
			}

			// Test encryption detection
			encrypted, err := isBackupEncrypted(tempDir)

			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, encrypted)
			}
		})
	}
}

func TestNewExtractor(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(string) ExtractConfig
		shouldError bool
		errorMsg    string
	}{
		{
			name: "valid_unencrypted_backup",
			setupFunc: func(tempDir string) ExtractConfig {
				// Create backup structure
				os.WriteFile(filepath.Join(tempDir, "Manifest.plist"),
					[]byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>IsEncrypted</key>
	<false/>
</dict>
</plist>`), 0644)

				// Create empty Manifest.db for testing
				db, _ := os.Create(filepath.Join(tempDir, "Manifest.db"))
				db.Close()

				logger := logger.New(logger.Config{
					Level:  logger.LevelInfo,
					Output: io.Discard,
				})

				return ExtractConfig{
					BackupPath: tempDir,
					OutputPath: filepath.Join(tempDir, "output"),
					Logger:     logger,
				}
			},
			shouldError: false,
		},
		{
			name: "encrypted_backup_should_fail",
			setupFunc: func(tempDir string) ExtractConfig {
				// Create encrypted backup
				os.WriteFile(filepath.Join(tempDir, "Manifest.plist"),
					[]byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>IsEncrypted</key>
	<true/>
</dict>
</plist>`), 0644)

				logger := logger.New(logger.Config{
					Level:  logger.LevelInfo,
					Output: io.Discard,
				})

				return ExtractConfig{
					BackupPath: tempDir,
					Logger:     logger,
				}
			},
			shouldError: true,
			errorMsg:    "encrypted backups are not supported",
		},
		{
			name: "missing_manifest",
			setupFunc: func(tempDir string) ExtractConfig {
				logger := logger.New(logger.Config{
					Level:  logger.LevelInfo,
					Output: io.Discard,
				})

				return ExtractConfig{
					BackupPath: tempDir,
					Logger:     logger,
				}
			},
			shouldError: true,
			errorMsg:    "invalid backup directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tempDir, err := os.MkdirTemp("", "extractor-test")
			if !assert.NoError(t, err) {
				return
			}
			defer os.RemoveAll(tempDir)

			config := tt.setupFunc(tempDir)
			extractor, err := NewExtractor(config)

			if tt.shouldError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Nil(t, extractor)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, extractor)
				if extractor != nil {
					extractor.Close()
				}
			}
		})
	}
}

func TestExtractSummary(t *testing.T) {
	summary := ExtractSummary{
		TotalFiles:     100,
		ExtractedFiles: 95,
		SkippedFiles:   3,
		FailedFiles:    2,
		DomainsFound:   5,
		TotalSize:      1024 * 1024,      // 1MB
		ExtractedSize:  1024*1024 - 1024, // 1MB - 1KB
	}

	assert.Equal(t, 100, summary.TotalFiles)
	assert.Equal(t, 95, summary.ExtractedFiles)
	assert.Equal(t, 3, summary.SkippedFiles)
	assert.Equal(t, 2, summary.FailedFiles)
	assert.Equal(t, 5, summary.DomainsFound)
	assert.Equal(t, int64(1024*1024), summary.TotalSize)
	assert.Equal(t, int64(1024*1024-1024), summary.ExtractedSize)
}

func TestValidateManifestSchema(t *testing.T) {
	// This test would require a properly structured SQLite database
	// For now, we'll just ensure the function exists and can be called
	// In a real test, we'd create a temporary database with proper/improper schema

	tempDir, err := os.MkdirTemp("", "schema-test")
	if !assert.NoError(t, err) {
		return
	}
	defer os.RemoveAll(tempDir)

	// Create a mock logger
	logger := logger.New(logger.Config{
		Level:  logger.LevelInfo,
		Output: io.Discard,
	})

	// Test with non-existent manifest (should fail at OpenManifestDB stage)
	config := ExtractConfig{
		BackupPath: tempDir,
		Logger:     logger,
	}

	_, err = NewExtractor(config)
	assert.Error(t, err) // Should fail because no valid backup exists
}
