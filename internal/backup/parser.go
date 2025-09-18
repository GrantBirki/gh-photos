package backup

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/grantbirki/gh-photos/internal/photos"
	"github.com/grantbirki/gh-photos/internal/types"
)

// BackupParser handles parsing of iPhone backup directories
type BackupParser struct {
	backupPath   string
	photosDB     *photos.Database
	dcimPath     string
	manifestPath string
}

// NewBackupParser creates a new backup parser for the given backup directory
func NewBackupParser(backupPath string) (*BackupParser, error) {
	if err := validateBackupDirectory(backupPath); err != nil {
		return nil, fmt.Errorf("invalid backup directory: %w", err)
	}

	// Find Photos.sqlite in the backup
	photosDBPath, err := findPhotosDatabase(backupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to find Photos database: %w", err)
	}

	// Open the Photos database
	photosDB, err := photos.NewDatabase(photosDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open Photos database: %w", err)
	}

	// Find DCIM directory
	dcimPath, err := findDCIMDirectory(backupPath)
	if err != nil {
		photosDB.Close()
		return nil, fmt.Errorf("failed to find DCIM directory: %w", err)
	}

	manifestPath := filepath.Join(backupPath, "Manifest.plist")

	return &BackupParser{
		backupPath:   backupPath,
		photosDB:     photosDB,
		dcimPath:     dcimPath,
		manifestPath: manifestPath,
	}, nil
}

// Close closes the backup parser and releases resources
func (bp *BackupParser) Close() error {
	if bp.photosDB != nil {
		return bp.photosDB.Close()
	}
	return nil
}

// ParseAssets extracts all assets from the backup
func (bp *BackupParser) ParseAssets() ([]*types.Asset, error) {
	assets, err := bp.photosDB.GetAssets(bp.dcimPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get assets from database: %w", err)
	}

	// Validate and enrich assets with file information
	var validAssets []*types.Asset
	for _, asset := range assets {
		if err := bp.enrichAsset(asset); err != nil {
			// Log warning but continue processing
			fmt.Fprintf(os.Stderr, "Warning: failed to enrich asset %s: %v\n", asset.Filename, err)
			continue
		}

		if asset.IsValid() {
			validAssets = append(validAssets, asset)
		}
	}

	return validAssets, nil
}

// enrichAsset adds file system information to the asset
func (bp *BackupParser) enrichAsset(asset *types.Asset) error {
	// Check if the source file exists
	info, err := os.Stat(asset.SourcePath)
	if err != nil {
		return fmt.Errorf("source file not found: %w", err)
	}

	asset.FileSize = info.Size()

	// Infer MIME type from extension
	asset.MimeType = inferMimeType(asset.Filename)

	return nil
}

// validateBackupDirectory checks if the directory appears to be a valid iPhone backup
func validateBackupDirectory(backupPath string) error {
	info, err := os.Stat(backupPath)
	if err != nil {
		return fmt.Errorf("backup path does not exist: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("backup path is not a directory")
	}

	// Check for key backup files
	manifestPath := filepath.Join(backupPath, "Manifest.plist")
	if _, err := os.Stat(manifestPath); err != nil {
		return fmt.Errorf("Manifest.plist not found - this doesn't appear to be an iPhone backup")
	}

	return nil
}

// findPhotosDatabase locates the Photos.sqlite file in the backup directory
func findPhotosDatabase(backupPath string) (string, error) {
	// Common locations for Photos.sqlite in iPhone backups
	possiblePaths := []string{
		"Library/Photos/Photos.sqlite",
		"Library/Photos/Photos.sqlite-wal",
		"PhotoData/Photos.sqlite",
	}

	for _, relativePath := range possiblePaths {
		fullPath := filepath.Join(backupPath, relativePath)
		if _, err := os.Stat(fullPath); err == nil {
			// Validate it's actually a Photos database
			if err := photos.ValidateDatabase(fullPath); err == nil {
				return fullPath, nil
			}
		}
	}

	// Fallback: search for any .sqlite file that might be the Photos database
	var foundPath string
	err := filepath.Walk(backupPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue walking despite errors
		}

		if filepath.Ext(path) == ".sqlite" && filepath.Base(path) == "Photos.sqlite" {
			if err := photos.ValidateDatabase(path); err == nil {
				foundPath = path
				return filepath.SkipDir // Found it, stop searching
			}
		}
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("error searching for Photos database: %w", err)
	}

	if foundPath == "" {
		return "", fmt.Errorf("Photos.sqlite not found in backup directory")
	}

	return foundPath, nil
}

// findDCIMDirectory locates the DCIM directory containing the actual media files
func findDCIMDirectory(backupPath string) (string, error) {
	// Common locations for DCIM in iPhone backups
	possiblePaths := []string{
		"DCIM",
		"Media/DCIM",
		"PhotoData/DCIM",
		"Library/Photos/DCIM",
	}

	for _, relativePath := range possiblePaths {
		fullPath := filepath.Join(backupPath, relativePath)
		if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
			return fullPath, nil
		}
	}

	// Fallback: search for DCIM directory
	var foundPath string
	err := filepath.Walk(backupPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() && filepath.Base(path) == "DCIM" {
			foundPath = path
			return filepath.SkipDir
		}
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("error searching for DCIM directory: %w", err)
	}

	if foundPath == "" {
		return "", fmt.Errorf("DCIM directory not found in backup")
	}

	return foundPath, nil
}

// inferMimeType returns the MIME type based on file extension
func inferMimeType(filename string) string {
	ext := filepath.Ext(filename)
	switch ext {
	case ".heic", ".heif":
		return "image/heif"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".bmp":
		return "image/bmp"
	case ".tiff", ".tif":
		return "image/tiff"
	case ".webp":
		return "image/webp"
	case ".mov":
		return "video/quicktime"
	case ".mp4":
		return "video/mp4"
	case ".m4v":
		return "video/mp4"
	case ".avi":
		return "video/avi"
	case ".mkv":
		return "video/x-matroska"
	case ".webm":
		return "video/webm"
	default:
		return "application/octet-stream"
	}
}
