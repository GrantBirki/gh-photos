package utils

import (
	"path/filepath"
)

// BackupPaths holds commonly used backup-related path construction utilities
type BackupPaths struct {
	backupPath string
}

// CreateBackupPaths creates a new BackupPaths instance
func CreateBackupPaths(backupPath string) *BackupPaths {
	return &BackupPaths{
		backupPath: backupPath,
	}
}

// ExtractionMetadata returns the path to the extraction metadata file
func (bp *BackupPaths) ExtractionMetadata() string {
	return filepath.Join(bp.backupPath, "extraction-metadata.json")
}

// ManifestPlist returns the path to the Manifest.plist file
func (bp *BackupPaths) ManifestPlist() string {
	return filepath.Join(bp.backupPath, "Manifest.plist")
}

// ManifestDB returns the path to the Manifest.db file
func (bp *BackupPaths) ManifestDB() string {
	return filepath.Join(bp.backupPath, "Manifest.db")
}

// MediaDCIM returns the path to the Media/DCIM directory for a given media domain
func (bp *BackupPaths) MediaDCIM(mediaDomain string) string {
	return filepath.Join(bp.backupPath, mediaDomain, "Media", "DCIM")
}

// DCIM returns the path to the DCIM directory for a given media domain
func (bp *BackupPaths) DCIM(mediaDomain string) string {
	return filepath.Join(bp.backupPath, mediaDomain, "DCIM")
}

// MediaPhotoData returns the path to the Media/PhotoData directory for a given media domain
func (bp *BackupPaths) MediaPhotoData(mediaDomain string) string {
	return filepath.Join(bp.backupPath, mediaDomain, "Media", "PhotoData")
}

// MediaDCIMFile returns the path to a specific file in the Media/DCIM directory
func (bp *BackupPaths) MediaDCIMFile(mediaDomain, filename string) string {
	return filepath.Join(bp.backupPath, mediaDomain, "Media", "DCIM", filename)
}

// MediaFile returns the path to a specific file in the Media directory
func (bp *BackupPaths) MediaFile(mediaDomain, filename string) string {
	return filepath.Join(bp.backupPath, mediaDomain, "Media", filename)
}

// DomainFile returns the path to a specific file in a media domain directory
func (bp *BackupPaths) DomainFile(mediaDomain, filename string) string {
	return filepath.Join(bp.backupPath, mediaDomain, filename)
}

// BackupSubdir returns the path to the Backup subdirectory
func (bp *BackupPaths) BackupSubdir() string {
	return filepath.Join(bp.backupPath, "Backup")
}

// RelativePath constructs a path relative to the backup path
func (bp *BackupPaths) RelativePath(relativePath string) string {
	return filepath.Join(bp.backupPath, relativePath)
}
