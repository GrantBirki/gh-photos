package backup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/grantbirki/gh-photos/internal/logger"
	"github.com/grantbirki/gh-photos/internal/photos"
	"github.com/grantbirki/gh-photos/internal/types"
)

// BackupParser handles parsing of iPhone backup directories
type BackupParser struct {
	backupPath      string
	photosDB        *photos.Database
	dcimPath        string
	manifestPath    string
	isExtracted     bool
	extractedAssets []*types.Asset
}

// NewBackupParser creates a new backup parser for the given backup directory
func NewBackupParser(backupPath string, logger *logger.Logger) (*BackupParser, error) {
	// Resolve the backup path using smart directory walking
	resolvedPath, err := resolveBackupPath(backupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve backup path: %w", err)
	}
	backupPath = resolvedPath

	if err := validateBackupDirectory(backupPath); err != nil {
		return nil, fmt.Errorf("invalid backup directory: %w", err)
	}

	// Check if this is an extracted directory
	extractionMetadataPath := filepath.Join(backupPath, "extraction-metadata.json")
	if _, err := os.Stat(extractionMetadataPath); err == nil {
		// This is an extracted directory - create a parser that works with metadata
		return NewExtractedBackupParser(backupPath, extractionMetadataPath)
	}

	// This is an original backup directory - use traditional parsing
	// Find Photos.sqlite in the backup
	photosDBPath, err := findPhotosDatabase(backupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to find Photos database: %w", err)
	}

	// Open the Photos database
	photosDB, err := photos.NewDatabase(photosDBPath, logger)
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
		isExtracted:  false,
	}, nil
}

// NewExtractedBackupParser creates a backup parser for extracted directories
func NewExtractedBackupParser(backupPath, metadataPath string) (*BackupParser, error) {
	// Read extraction metadata
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read extraction metadata: %w", err)
	}

	var extractionMetadata struct {
		Assets []*types.Asset `json:"assets"`
	}
	if err := json.Unmarshal(data, &extractionMetadata); err != nil {
		return nil, fmt.Errorf("failed to parse extraction metadata: %w", err)
	}

	// Update asset source paths to point to extracted files
	for _, asset := range extractionMetadata.Assets {
		// The extracted structure organizes files by domain, e.g.:
		// MediaDomain/DCIM/100APPLE/IMG_001.HEIC
		// We need to reconstruct the full path from original SourcePath

		// Clean the path and handle both forward and backward slashes
		cleanPath := filepath.ToSlash(asset.SourcePath)

		if strings.Contains(cleanPath, "DCIM") {
			// Extract the DCIM path portion
			parts := strings.Split(cleanPath, "/")
			var dcimIndex int = -1
			for i, part := range parts {
				if part == "DCIM" {
					dcimIndex = i
					break
				}
			}
			if dcimIndex >= 0 {
				// Reconstruct path: backupPath/MediaDomain/DCIM/folder/filename
				relativePath := filepath.Join(parts[dcimIndex:]...)
				asset.SourcePath = filepath.Join(backupPath, "MediaDomain", relativePath)
			} else {
				// Fallback: assume it's in MediaDomain/DCIM with filename
				asset.SourcePath = filepath.Join(backupPath, "MediaDomain", "DCIM", asset.Filename)
			}
		} else {
			// For non-DCIM files, use the original relative path structure
			// Remove any leading domain parts and place under MediaDomain
			cleanedPath := strings.TrimPrefix(cleanPath, "MediaDomain/")
			asset.SourcePath = filepath.Join(backupPath, "MediaDomain", cleanedPath)
		}
	}

	return &BackupParser{
		backupPath:      backupPath,
		dcimPath:        backupPath,
		isExtracted:     true,
		extractedAssets: extractionMetadata.Assets,
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
	if bp.isExtracted {
		// For extracted directories, return pre-loaded assets from metadata
		var validAssets []*types.Asset
		for _, asset := range bp.extractedAssets {
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

	// For original backup directories, use Photos database
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

// ParseAssetsForExtraction extracts assets without enrichment (for use during extraction process)
func (bp *BackupParser) ParseAssetsForExtraction() ([]*types.Asset, error) {
	if bp.isExtracted {
		// For extracted directories, return pre-loaded assets from metadata
		return bp.extractedAssets, nil
	}

	// For original backup directories, use Photos database but skip enrichment
	assets, err := bp.photosDB.GetAssets(bp.dcimPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get assets from database: %w", err)
	}

	// Return assets without enrichment since files are being extracted
	// The enrichment will happen later when sync reads from extracted directory
	return assets, nil
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

// resolveBackupPath automatically walks directory structure to find the actual backup directory
// Supports common iPhone backup locations by looking for Manifest.db or Manifest.plist files
func resolveBackupPath(inputPath string) (string, error) {
	// Check if the current path is already a valid backup directory
	if isValidBackupDir(inputPath) {
		return inputPath, nil
	}

	// If this might be a current working directory check, provide clear error message
	if absPath, err := filepath.Abs(inputPath); err == nil {
		if cwd, err := os.Getwd(); err == nil && absPath == cwd {
			return "", fmt.Errorf("current directory does not contain iPhone backup files (Manifest.db or Manifest.plist not found) - please specify a valid backup directory path")
		}
	}

	// Try looking for a "Backup" subdirectory
	backupDir := filepath.Join(inputPath, "Backup")
	if info, err := os.Stat(backupDir); err == nil && info.IsDir() {
		// Check if this Backup directory contains the backup files directly
		if isValidBackupDir(backupDir) {
			return backupDir, nil
		}

		// If not, look for a single subdirectory in the Backup folder
		entries, err := os.ReadDir(backupDir)
		if err != nil {
			return inputPath, nil // Fall back to original path
		}

		// Find directories (excluding files like .DS_Store, etc.)
		var subDirs []string
		for _, entry := range entries {
			if entry.IsDir() {
				subDirs = append(subDirs, entry.Name())
			}
		}

		// Safety check: if multiple backup directories exist, require explicit selection
		if len(subDirs) > 1 {
			return "", fmt.Errorf("multiple backup directories found in %s - please specify the exact backup directory path for safety", backupDir)
		}

		// If there's exactly one subdirectory, check if it's a backup directory
		if len(subDirs) == 1 {
			potentialBackupDir := filepath.Join(backupDir, subDirs[0])
			if isValidBackupDir(potentialBackupDir) {
				return potentialBackupDir, nil
			}
		}
	}

	// Return the original path if no automatic resolution worked
	return inputPath, nil
}

// isValidBackupDir checks if a directory contains iPhone backup files
func isValidBackupDir(path string) bool {
	// Check for Manifest.plist (required for all iPhone backups)
	manifestPlist := filepath.Join(path, "Manifest.plist")
	if _, err := os.Stat(manifestPlist); err != nil {
		return false
	}

	// Additional check for Manifest.db (present in newer backups)
	manifestDB := filepath.Join(path, "Manifest.db")
	if _, err := os.Stat(manifestDB); err == nil {
		return true
	}

	// For older backups that might not have Manifest.db, check for other indicators
	// Look for the typical hex-named directories (00, 01, 02, etc.) or common backup files
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}

	hexDirCount := 0
	for _, entry := range entries {
		if entry.IsDir() && len(entry.Name()) == 2 {
			// Check if it's a hex directory (00-ff)
			if _, err := fmt.Sscanf(entry.Name(), "%02x", new(int)); err == nil {
				hexDirCount++
			}
		}
		// Also look for Status.plist which is common in iPhone backups
		if entry.Name() == "Status.plist" {
			return true
		}
	}

	// If we found several hex directories, it's likely a backup
	return hexDirCount > 10
}

// validateBackupDirectory checks if the directory appears to be a valid iPhone backup or extracted directory
func validateBackupDirectory(backupPath string) error {
	info, err := os.Stat(backupPath)
	if err != nil {
		return fmt.Errorf("backup path does not exist: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("backup path is not a directory")
	}

	// Check if this is an extracted directory - this is now valid for sync operations
	extractionMetadataPath := filepath.Join(backupPath, "extraction-metadata.json")
	if _, err := os.Stat(extractionMetadataPath); err == nil {
		return nil // Extracted directories are valid
	}

	// Check for key backup files for original backup directories
	manifestPath := filepath.Join(backupPath, "Manifest.plist")
	if _, err := os.Stat(manifestPath); err != nil {
		return fmt.Errorf("Manifest.plist not found - this doesn't appear to be an iPhone backup or extracted directory")
	}

	return nil
}

// findPhotosDatabase locates the Photos.sqlite file in the backup directory
func findPhotosDatabase(backupPath string) (string, error) {
	// First try to use Manifest.db for hashed iPhone backup structure
	manifestDB, err := OpenManifestDB(backupPath)
	if err == nil {
		defer manifestDB.Close()

		// Try to find Photos.sqlite through Manifest.db
		if path, err := manifestDB.FindPhotosDatabase(backupPath); err == nil {
			// Validate it's actually a Photos database
			if err := photos.ValidateDatabase(path); err == nil {
				return path, nil
			}
		}
	}

	// Fallback to traditional directory structure search
	// Common locations for Photos.sqlite in iPhone backups
	possiblePaths := []string{
		"Library/Photos/Photos.sqlite",
		"Library/Photos/Photos.sqlite-wal",
		"PhotoData/Photos.sqlite",
		"Media/PhotoData/Photos.sqlite",
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

	// Final fallback: search for any .sqlite file that might be the Photos database
	var foundPath string
	err = filepath.Walk(backupPath, func(path string, info os.FileInfo, err error) error {
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

// findDCIMDirectory locates the DCIM directory or verifies media files exist via Manifest.db
func findDCIMDirectory(backupPath string) (string, error) {
	// First try Manifest.db approach (for hashed iPhone backups)
	manifestDB, err := OpenManifestDB(backupPath)
	if err == nil {
		defer manifestDB.Close()

		// Check if there are any media files in the backup via Manifest.db
		if hasMediaFiles, err := manifestDB.HasMediaFiles(); err == nil && hasMediaFiles {
			// For hashed backups, return the backup path as the "DCIM" root
			// since files will be resolved through Manifest.db
			return backupPath, nil
		}
	}

	// Fallback to traditional directory structure search
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

	// Final fallback: search for DCIM directory
	var foundPath string
	err = filepath.Walk(backupPath, func(path string, info os.FileInfo, err error) error {
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
	ext := strings.ToLower(filepath.Ext(filename))
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
