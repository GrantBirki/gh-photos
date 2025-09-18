package photos

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/grantbirki/gh-photos/internal/types"
	_ "modernc.org/sqlite"
)

// Database represents a connection to the Photos.sqlite database
type Database struct {
	db   *sql.DB
	path string
}

// NewDatabase creates a new Photos database connection
func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database %s: %w", dbPath, err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database %s: %w", dbPath, err)
	}

	return &Database{
		db:   db,
		path: dbPath,
	}, nil
}

// Close closes the database connection
func (d *Database) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// GetAssets retrieves all assets from the Photos database
func (d *Database) GetAssets(dcimPath string) ([]*types.Asset, error) {
	// Query to get asset information from Photos database
	// This query is based on common iPhone Photos.sqlite schema
	query := `
		SELECT 
			Z_PK,
			ZFILENAME,
			ZDIRECTORY,
			ZCREATIONDATE,
			ZMODIFICATIONDATE,
			ZHIDDEN,
			ZTRASHED,
			ZKINDSUBTYPE,
			ZBURSTIDENTIFIER,
			ZISSCREENSHOT,
			ZHASADJUSTMENTS
		FROM ZASSET 
		WHERE ZFILENAME IS NOT NULL
		ORDER BY ZCREATIONDATE ASC
	`

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query assets: %w", err)
	}
	defer rows.Close()

	var assets []*types.Asset

	for rows.Next() {
		var (
			id               int64
			filename         sql.NullString
			directory        sql.NullString
			creationDate     sql.NullFloat64
			modificationDate sql.NullFloat64
			hidden           sql.NullInt64
			trashed          sql.NullInt64
			kindSubtype      sql.NullInt64
			burstID          sql.NullString
			isScreenshot     sql.NullInt64
			hasAdjustments   sql.NullInt64
		)

		err := rows.Scan(
			&id,
			&filename,
			&directory,
			&creationDate,
			&modificationDate,
			&hidden,
			&trashed,
			&kindSubtype,
			&burstID,
			&isScreenshot,
			&hasAdjustments,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if !filename.Valid || filename.String == "" {
			continue
		}

		// Convert Core Data timestamps (seconds since 2001-01-01) to Go time
		var createdAt, modifiedAt time.Time
		if creationDate.Valid {
			createdAt = coreDataTimeToGoTime(creationDate.Float64)
		}
		if modificationDate.Valid {
			modifiedAt = coreDataTimeToGoTime(modificationDate.Float64)
		}

		// Build the source path
		sourcePath := filepath.Join(dcimPath, directory.String, filename.String)

		// Create asset flags
		flags := types.AssetFlags{
			Hidden:          hidden.Valid && hidden.Int64 == 1,
			RecentlyDeleted: trashed.Valid && trashed.Int64 == 1,
			Screenshot:      isScreenshot.Valid && isScreenshot.Int64 == 1,
			Burst:           burstID.Valid && burstID.String != "",
			LivePhoto:       kindSubtype.Valid && kindSubtype.Int64 == 2, // Live Photo subtype
		}

		if flags.Burst && burstID.Valid {
			flags.BurstID = &burstID.String
		}

		// Classify asset type
		assetType := classifyAsset(filename.String, flags)

		asset := &types.Asset{
			ID:           strconv.FormatInt(id, 10),
			SourcePath:   sourcePath,
			Filename:     filename.String,
			Type:         assetType,
			CreationDate: createdAt,
			ModifiedDate: modifiedAt,
			Flags:        flags,
		}

		// Get file size if file exists
		if fileInfo, err := filepath.Glob(sourcePath); err == nil && len(fileInfo) > 0 {
			if stat, err := filepath.Abs(fileInfo[0]); err == nil {
				if info, err := filepath.Abs(stat); err == nil {
					asset.SourcePath = info
				}
			}
		}

		assets = append(assets, asset)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return assets, nil
}

// coreDataTimeToGoTime converts Core Data timestamp to Go time
// Core Data stores time as seconds since 2001-01-01 00:00:00 UTC
func coreDataTimeToGoTime(seconds float64) time.Time {
	coreDataEpoch := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	return coreDataEpoch.Add(time.Duration(seconds) * time.Second)
}

// classifyAsset determines the asset type based on filename and flags
func classifyAsset(filename string, flags types.AssetFlags) types.AssetType {
	if flags.Screenshot {
		return types.AssetTypeScreenshot
	}
	if flags.LivePhoto {
		return types.AssetTypeLivePhoto
	}
	if flags.Burst {
		return types.AssetTypeBurst
	}

	// Use extension-based classification for regular photos/videos
	return types.ClassifyByExtension(filename)
}

// ValidateDatabase checks if the given path contains a valid Photos.sqlite database
func ValidateDatabase(dbPath string) error {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Check if required tables exist
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='ZASSET'").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check for ZASSET table: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("ZASSET table not found - this doesn't appear to be a valid Photos.sqlite database")
	}

	return nil
}
