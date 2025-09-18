package backup

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// ManifestDB represents the iPhone backup's Manifest.db
type ManifestDB struct {
	db   *sql.DB
	path string
}

// FileRecord represents a file entry from the Manifest.db
type FileRecord struct {
	FileID       string
	Domain       string
	RelativePath string
	Flags        int64
	File         []byte
}

// OpenManifestDB opens the Manifest.db file from an iPhone backup
func OpenManifestDB(backupPath string) (*ManifestDB, error) {
	manifestPath := filepath.Join(backupPath, "Manifest.db")

	// Check if Manifest.db exists
	if _, err := os.Stat(manifestPath); err != nil {
		return nil, fmt.Errorf("Manifest.db not found at %s: %w", manifestPath, err)
	}

	db, err := sql.Open("sqlite", manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open Manifest.db: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping Manifest.db: %w", err)
	}

	return &ManifestDB{
		db:   db,
		path: manifestPath,
	}, nil
}

// Close closes the Manifest.db connection
func (m *ManifestDB) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

// FindPhotosDatabase searches for Photos.sqlite in the manifest
func (m *ManifestDB) FindPhotosDatabase(backupPath string) (string, error) {
	// Query patterns to look for Photos.sqlite
	patterns := []string{
		"%Photos.sqlite",
		"%PhotoData/Photos.sqlite",
		"%Photos/Photos.sqlite",
		"%Media/PhotoData/Photos.sqlite",
	}

	// Also try domain-based searches
	domains := []string{
		"MediaDomain",
		"CameraRollDomain",
		"AppDomain-com.apple.mobileslideshow",
	}

	var fileRecord *FileRecord
	var err error

	// First try pattern-based search
	for _, pattern := range patterns {
		fileRecord, err = m.findFileByPath(pattern)
		if err == nil && fileRecord != nil {
			break
		}
	}

	// If pattern search fails, try domain-based search
	if fileRecord == nil {
		for _, domain := range domains {
			fileRecord, err = m.findFileByDomainAndName(domain, "Photos.sqlite")
			if err == nil && fileRecord != nil {
				break
			}
		}
	}

	if fileRecord == nil {
		return "", fmt.Errorf("Photos.sqlite not found in Manifest.db")
	}

	// Convert fileID to actual file path
	actualPath := m.getActualFilePath(backupPath, fileRecord.FileID)

	// Verify the file exists
	if _, err := os.Stat(actualPath); err != nil {
		return "", fmt.Errorf("Photos.sqlite file not found at computed path %s: %w", actualPath, err)
	}

	return actualPath, nil
}

// findFileByPath searches for a file by relative path pattern
func (m *ManifestDB) findFileByPath(pathPattern string) (*FileRecord, error) {
	query := `
		SELECT fileID, domain, relativePath, flags, file 
		FROM Files 
		WHERE relativePath LIKE ? 
		AND relativePath NOT LIKE '%-wal' 
		AND relativePath NOT LIKE '%-shm'
		ORDER BY LENGTH(relativePath) ASC
		LIMIT 1
	`

	var record FileRecord
	err := m.db.QueryRow(query, pathPattern).Scan(
		&record.FileID,
		&record.Domain,
		&record.RelativePath,
		&record.Flags,
		&record.File,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query Files table: %w", err)
	}

	return &record, nil
}

// findFileByDomainAndName searches for a file by domain and filename
func (m *ManifestDB) findFileByDomainAndName(domain, filename string) (*FileRecord, error) {
	query := `
		SELECT fileID, domain, relativePath, flags, file 
		FROM Files 
		WHERE domain = ? 
		AND relativePath LIKE ?
		AND relativePath NOT LIKE '%-wal' 
		AND relativePath NOT LIKE '%-shm'
		ORDER BY LENGTH(relativePath) ASC
		LIMIT 1
	`

	var record FileRecord
	err := m.db.QueryRow(query, domain, "%"+filename).Scan(
		&record.FileID,
		&record.Domain,
		&record.RelativePath,
		&record.Flags,
		&record.File,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query Files table by domain: %w", err)
	}

	return &record, nil
}

// getActualFilePath converts a fileID to the actual hashed file path
func (m *ManifestDB) getActualFilePath(backupPath, fileID string) string {
	// iPhone backup files are stored in subdirectories based on first 2 chars of fileID
	if len(fileID) < 2 {
		return filepath.Join(backupPath, fileID)
	}

	subdir := fileID[:2]
	return filepath.Join(backupPath, subdir, fileID)
}

// ListPhotosRelatedFiles returns all Photos-related files for debugging
func (m *ManifestDB) ListPhotosRelatedFiles() ([]FileRecord, error) {
	query := `
		SELECT fileID, domain, relativePath, flags, file 
		FROM Files 
		WHERE (
			relativePath LIKE '%Photos%' 
			OR relativePath LIKE '%PhotoData%'
			OR relativePath LIKE '%DCIM%'
			OR domain LIKE '%Media%'
			OR domain LIKE '%Photo%'
			OR domain LIKE '%Camera%'
		)
		AND relativePath NOT LIKE '%-wal' 
		AND relativePath NOT LIKE '%-shm'
		ORDER BY domain, relativePath
	`

	rows, err := m.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query photos-related files: %w", err)
	}
	defer rows.Close()

	var records []FileRecord
	for rows.Next() {
		var record FileRecord
		err := rows.Scan(
			&record.FileID,
			&record.Domain,
			&record.RelativePath,
			&record.Flags,
			&record.File,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		records = append(records, record)
	}

	return records, nil
}

// GetDomains returns all unique domains in the backup for debugging
func (m *ManifestDB) GetDomains() ([]string, error) {
	query := `SELECT DISTINCT domain FROM Files ORDER BY domain`

	rows, err := m.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query domains: %w", err)
	}
	defer rows.Close()

	var domains []string
	for rows.Next() {
		var domain string
		if err := rows.Scan(&domain); err != nil {
			return nil, fmt.Errorf("failed to scan domain: %w", err)
		}
		domains = append(domains, domain)
	}

	return domains, nil
}

// HasMediaFiles checks if the backup contains media files (photos/videos)
func (m *ManifestDB) HasMediaFiles() (bool, error) {
	// Look for files in common media paths or with media extensions
	query := `
		SELECT COUNT(*) 
		FROM Files 
		WHERE (
			relativePath LIKE '%/DCIM/%' OR
			relativePath LIKE '%/Media/%' OR
			relativePath LIKE '%/Photos/%' OR
			relativePath LIKE '%/PhotoData/%' OR
			relativePath LIKE '%.jpg' OR
			relativePath LIKE '%.jpeg' OR
			relativePath LIKE '%.HEIC' OR
			relativePath LIKE '%.png' OR
			relativePath LIKE '%.mov' OR
			relativePath LIKE '%.mp4' OR
			relativePath LIKE '%.m4v'
		) AND
		flags = 1
		LIMIT 1
	`

	var count int
	err := m.db.QueryRow(query).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check for media files: %w", err)
	}

	return count > 0, nil
}
