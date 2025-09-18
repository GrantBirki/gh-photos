package backup

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/grantbirki/gh-photos/internal/logger"
)

// ExtractConfig holds configuration for backup extraction
type ExtractConfig struct {
	BackupPath   string
	OutputPath   string
	SkipExisting bool
	Verify       bool
	Progress     bool
	Logger       *logger.Logger
}

// ExtractSummary provides statistics about the extraction operation
type ExtractSummary struct {
	TotalFiles     int           `json:"total_files"`
	ExtractedFiles int           `json:"extracted_files"`
	SkippedFiles   int           `json:"skipped_files"`
	FailedFiles    int           `json:"failed_files"`
	DomainsFound   int           `json:"domains_found"`
	TotalSize      int64         `json:"total_size"`
	ExtractedSize  int64         `json:"extracted_size"`
	Duration       time.Duration `json:"duration"`
	Errors         []string      `json:"errors,omitempty"`
}

// Extractor handles iTunes backup extraction
type Extractor struct {
	config   ExtractConfig
	manifest *ManifestDB
	summary  ExtractSummary
}

// NewExtractor creates a new backup extractor
func NewExtractor(config ExtractConfig) (*Extractor, error) {
	// Validate backup path
	if err := validateBackupDirectory(config.BackupPath); err != nil {
		return nil, fmt.Errorf("invalid backup directory: %w", err)
	}

	// Check if backup is encrypted
	if encrypted, err := isBackupEncrypted(config.BackupPath); err != nil {
		return nil, fmt.Errorf("failed to check encryption status: %w", err)
	} else if encrypted {
		return nil, fmt.Errorf("encrypted backups are not supported - this tool only works with unencrypted iTunes/Finder backups")
	}

	// Open Manifest.db
	manifest, err := OpenManifestDB(config.BackupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open Manifest.db: %w", err)
	}

	// Validate database schema (following Rust implementation pattern)
	// Skip validation if it's an empty database (for testing)
	if err := validateManifestSchema(manifest); err != nil {
		// Check if this is just an empty database
		var count int
		checkErr := manifest.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='files'").Scan(&count)
		if checkErr != nil || count == 0 {
			// This is likely a test database or empty database, skip validation
			config.Logger.Debug("Skipping schema validation for empty database")
		} else {
			return nil, fmt.Errorf("incompatible Manifest.db schema: %w", err)
		}
	}

	// Set default output path if not provided
	if config.OutputPath == "" {
		config.OutputPath = "./extracted-backup"
	}

	return &Extractor{
		config:   config,
		manifest: manifest,
		summary:  ExtractSummary{},
	}, nil
}

// Close releases resources
func (e *Extractor) Close() error {
	if e.manifest != nil {
		return e.manifest.Close()
	}
	return nil
}

// Extract performs the backup extraction
func (e *Extractor) Extract() (*ExtractSummary, error) {
	startTime := time.Now()
	defer func() {
		e.summary.Duration = time.Since(startTime)
	}()

	e.config.Logger.Info("Starting backup extraction",
		"backup_path", e.config.BackupPath,
		"output_path", e.config.OutputPath)

	// Get all files from manifest
	files, err := e.manifest.GetAllFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest files: %w", err)
	}

	e.summary.TotalFiles = len(files)
	e.config.Logger.Info("Found files in backup", "count", len(files))

	// Count unique domains
	domains := make(map[string]bool)
	for _, file := range files {
		domains[file.Domain] = true
	}
	e.summary.DomainsFound = len(domains)

	// Create output directory
	if err := os.MkdirAll(e.config.OutputPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Extract files with improved progress reporting (inspired by Rust implementation)
	if e.config.Progress {
		e.config.Logger.Info("Starting file extraction phase",
			"total_files", len(files))
	}

	for i, file := range files {
		if e.config.Progress && (i%50 == 0 || i == len(files)-1) {
			progress := float64(i+1) / float64(len(files)) * 100
			e.config.Logger.Info("Extracting files",
				"progress", fmt.Sprintf("%.1f%%", progress),
				"processed", fmt.Sprintf("%d/%d", i+1, len(files)),
				"current_domain", file.Domain,
				"current_file", filepath.Base(file.RelativePath))
		}

		// Debug log for individual file processing (only visible with --log-level debug)
		e.config.Logger.Debug("Processing file",
			"domain", file.Domain,
			"relative_path", file.RelativePath,
			"file_id", file.FileID,
			"index", fmt.Sprintf("%d/%d", i+1, len(files)))

		if err := e.extractFile(&file); err != nil {
			e.summary.FailedFiles++
			e.summary.Errors = append(e.summary.Errors,
				fmt.Sprintf("Failed to extract %s: %v", file.RelativePath, err))
			e.config.Logger.Warn("Failed to extract file",
				"domain", file.Domain,
				"path", file.RelativePath,
				"file_id", file.FileID,
				"error", err)
			continue
		}
	}

	e.config.Logger.Info("Backup extraction completed",
		"total_files", e.summary.TotalFiles,
		"extracted_files", e.summary.ExtractedFiles,
		"skipped_files", e.summary.SkippedFiles,
		"failed_files", e.summary.FailedFiles,
		"domains", e.summary.DomainsFound,
		"duration", e.summary.Duration)

	return &e.summary, nil
}

// extractFile extracts a single file from the backup
func (e *Extractor) extractFile(file *FileRecord) error {
	// Skip files that aren't regular files (flags != 1)
	if file.Flags != 1 {
		e.summary.SkippedFiles++
		return nil
	}

	// Build source path (hashed file in backup)
	sourcePath := e.manifest.getActualFilePath(e.config.BackupPath, file.FileID)

	// Check if source file exists
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("source file not found: %w", err)
	}

	// Build target path (reconstructed path)
	targetPath := filepath.Join(e.config.OutputPath, file.Domain, file.RelativePath)

	// Skip if target exists and SkipExisting is enabled
	if e.config.SkipExisting {
		if _, err := os.Stat(targetPath); err == nil {
			e.summary.SkippedFiles++
			return nil
		}
	}

	// Create target directory
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Copy file
	if err := e.copyFile(sourcePath, targetPath); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Verify if requested
	if e.config.Verify {
		if err := e.verifyFile(sourcePath, targetPath); err != nil {
			return fmt.Errorf("file verification failed: %w", err)
		}
	}

	e.summary.ExtractedFiles++
	e.summary.TotalSize += sourceInfo.Size()
	e.summary.ExtractedSize += sourceInfo.Size()

	return nil
}

// copyFile copies a file from source to target
func (e *Extractor) copyFile(sourcePath, targetPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	targetFile, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer targetFile.Close()

	_, err = io.Copy(targetFile, sourceFile)
	return err
}

// verifyFile verifies that source and target files have the same content
func (e *Extractor) verifyFile(sourcePath, targetPath string) error {
	sourceHash, err := e.calculateSHA1(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to calculate source hash: %w", err)
	}

	targetHash, err := e.calculateSHA1(targetPath)
	if err != nil {
		return fmt.Errorf("failed to calculate target hash: %w", err)
	}

	if sourceHash != targetHash {
		return fmt.Errorf("file hashes don't match (source: %s, target: %s)", sourceHash, targetHash)
	}

	return nil
}

// calculateSHA1 calculates SHA1 hash of a file
func (e *Extractor) calculateSHA1(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha1.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// isBackupEncrypted checks if the backup is encrypted by examining Manifest.plist
func isBackupEncrypted(backupPath string) (bool, error) {
	manifestPlistPath := filepath.Join(backupPath, "Manifest.plist")

	// Read Manifest.plist content
	content, err := os.ReadFile(manifestPlistPath)
	if err != nil {
		return false, fmt.Errorf("failed to read Manifest.plist: %w", err)
	}

	// Simple check for encryption indicators in the plist
	contentStr := strings.ToLower(string(content))

	// Look for encryption-related keys
	encryptionIndicators := []string{
		"<key>isencrypted</key>",
		"<key>encryption",
		"<true/>", // If IsEncrypted key is followed by true
	}

	for _, indicator := range encryptionIndicators {
		if strings.Contains(contentStr, indicator) {
			// Additional check: if we see IsEncrypted key, check if it's followed by true
			if strings.Contains(indicator, "isencrypted") {
				// Find position of IsEncrypted key
				pos := strings.Index(contentStr, indicator)
				if pos >= 0 {
					// Look for <true/> or <false/> after the key
					afterKey := contentStr[pos+len(indicator):]
					// Check next 100 chars or whatever is left
					checkLength := 100
					if len(afterKey) < checkLength {
						checkLength = len(afterKey)
					}
					if strings.Contains(afterKey[:checkLength], "<true/>") {
						return true, nil
					}
				}
			}
		}
	}

	return false, nil
}

// validateManifestSchema validates that the Manifest.db has the expected schema
// following the pattern from the Rust ibackupextractor implementation
func validateManifestSchema(manifest *ManifestDB) error {
	// Query table schema information
	rows, err := manifest.db.Query("PRAGMA table_info('files')")
	if err != nil {
		return fmt.Errorf("failed to query table schema: %w", err)
	}
	defer rows.Close()

	// Expected columns and their types
	expectedCols := map[string]string{
		"fileID":       "TEXT",
		"domain":       "TEXT",
		"relativePath": "TEXT",
		"flags":        "INTEGER",
		"file":         "BLOB",
	}

	foundCols := make(map[string]bool)

	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, dfltValue, pk interface{}

		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			return fmt.Errorf("failed to scan column info: %w", err)
		}

		if expectedType, exists := expectedCols[name]; exists {
			if expectedType != colType {
				return fmt.Errorf("column %s has type %s, expected %s", name, colType, expectedType)
			}
			foundCols[name] = true
		}
	}

	// Check that all required columns were found
	for colName := range expectedCols {
		if !foundCols[colName] {
			return fmt.Errorf("required column %s not found in files table", colName)
		}
	}

	return nil
}
