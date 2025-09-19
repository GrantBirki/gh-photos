package types

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AssetType represents the classification of an asset
type AssetType string

const (
	AssetTypePhoto      AssetType = "photos"
	AssetTypeVideo      AssetType = "videos"
	AssetTypeScreenshot AssetType = "screenshots"
	AssetTypeBurst      AssetType = "burst"
	AssetTypeLivePhoto  AssetType = "live_photos"
)

// AssetFlags represents various flags from the Photos database
type AssetFlags struct {
	Hidden           bool
	RecentlyDeleted  bool
	Screenshot       bool
	Burst            bool
	LivePhoto        bool
	BurstID          *string
	LivePhotoVideoID *string
}

// Asset represents a photo/video asset from an iPhone backup
type Asset struct {
	ID           string     `json:"id"`
	SourcePath   string     `json:"source_path"`
	Filename     string     `json:"filename"`
	Type         AssetType  `json:"type"`
	CreationDate time.Time  `json:"creation_date"`
	ModifiedDate time.Time  `json:"modified_date"`
	Flags        AssetFlags `json:"flags"`
	FileSize     int64      `json:"file_size"`
	Checksum     string     `json:"checksum,omitempty"`
	MimeType     string     `json:"mime_type"`
	TargetPath   string     `json:"target_path,omitempty"`
}

// ShouldExclude determines if an asset should be excluded based on default rules
func (a *Asset) ShouldExclude(includeHidden, includeRecentlyDeleted bool) bool {
	if a.Flags.Hidden && !includeHidden {
		return true
	}
	if a.Flags.RecentlyDeleted && !includeRecentlyDeleted {
		return true
	}
	return false
}

// GenerateTargetPath creates the target path following the YYYY/MM/DD/category structure
func (a *Asset) GenerateTargetPath() string {
	year := a.CreationDate.Format("2006")
	month := a.CreationDate.Format("01")
	day := a.CreationDate.Format("02")

	path := filepath.Join(year, month, day, string(a.Type), a.Filename)
	return filepath.Clean(path)
}

// ComputeChecksum calculates SHA256 checksum of the asset file
func (a *Asset) ComputeChecksum() error {
	if a.SourcePath == "" {
		return fmt.Errorf("source path is empty")
	}

	file, err := os.Open(a.SourcePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", a.SourcePath, err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("failed to compute checksum for %s: %w", a.SourcePath, err)
	}

	a.Checksum = fmt.Sprintf("%x", hasher.Sum(nil))
	return nil
}

// IsValid checks if the asset has valid required fields
func (a *Asset) IsValid() bool {
	return a.ID != "" && a.SourcePath != "" && a.Filename != "" && !a.CreationDate.IsZero()
}

// ClassifyByExtension determines asset type based on file extension
func ClassifyByExtension(filename string) AssetType {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".heic", ".heif", ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".webp":
		return AssetTypePhoto
	case ".mov", ".mp4", ".m4v", ".avi", ".mkv", ".webm":
		return AssetTypeVideo
	default:
		return AssetTypePhoto // Default to photo for unknown extensions
	}
}
