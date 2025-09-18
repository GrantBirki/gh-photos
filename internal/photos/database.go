package photos

import (
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"strings"
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

// SchemaInfo holds information about the Photos.sqlite schema
type SchemaInfo struct {
	CreationDateColumn string
	ModDateColumn      string
	TrashedColumn      string
	BurstColumn        string
	ScreenshotColumn   string
	AdjustmentsColumn  string
	TableName          string
}

// detectSchema analyzes the Photos.sqlite schema to determine column names
func (d *Database) detectSchema() (*SchemaInfo, error) {
	info := &SchemaInfo{
		TableName: "ZASSET", // Default table name
	}

	// Check if ZASSET table exists and get its columns
	rows, err := d.db.Query("PRAGMA table_info(ZASSET)")
	if err != nil {
		return nil, fmt.Errorf("failed to get ZASSET table info: %w", err)
	}
	defer rows.Close()

	var columns []string
	var columnDetails []string
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull, pk int
		var defaultValue sql.NullString

		err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk)
		if err != nil {
			return nil, fmt.Errorf("failed to scan column info: %w", err)
		}
		columns = append(columns, name)

		// Create detailed column info for debug logging
		detail := fmt.Sprintf("%s (%s)", name, dataType)
		if notNull == 1 {
			detail += " NOT NULL"
		}
		if pk == 1 {
			detail += " PRIMARY KEY"
		}
		columnDetails = append(columnDetails, detail)
	}

	// Debug log the available columns
	log.Printf("[DEBUG] Photos.sqlite ZASSET table columns found: %s", strings.Join(columns, ", "))
	log.Printf("[DEBUG] Photos.sqlite ZASSET column details: %s", strings.Join(columnDetails, " | "))

	if len(columns) == 0 {
		return nil, fmt.Errorf("ZASSET table has no columns or doesn't exist")
	}

	// Determine creation date column based on what's available
	// Priority order: ZCREATIONDATE (iOS 14+) > ZDATECREATED (iOS 12-13) > ZADDEDDATE (fallback)
	for _, col := range columns {
		switch col {
		case "ZCREATIONDATE":
			info.CreationDateColumn = "ZCREATIONDATE"
		case "ZDATECREATED":
			if info.CreationDateColumn == "" {
				info.CreationDateColumn = "ZDATECREATED"
			}
		case "ZADDEDDATE":
			if info.CreationDateColumn == "" {
				info.CreationDateColumn = "ZADDEDDATE"
			}
		}
	}

	// Determine modification date column
	for _, col := range columns {
		switch col {
		case "ZMODIFICATIONDATE":
			info.ModDateColumn = "ZMODIFICATIONDATE"
		case "ZDATEMODIFIED":
			if info.ModDateColumn == "" {
				info.ModDateColumn = "ZDATEMODIFIED"
			}
		case "ZMODIFIEDDATE":
			if info.ModDateColumn == "" {
				info.ModDateColumn = "ZMODIFIEDDATE"
			}
		}
	}

	// Determine trashed column
	// Priority order: ZTRASHED (older iOS) > ZTRASHEDSTATE (newer iOS)
	for _, col := range columns {
		switch col {
		case "ZTRASHED":
			info.TrashedColumn = "ZTRASHED"
		case "ZTRASHEDSTATE":
			if info.TrashedColumn == "" {
				info.TrashedColumn = "ZTRASHEDSTATE"
			}
		}
	}

	// Fallback to creation date if no modification date column found
	if info.ModDateColumn == "" {
		info.ModDateColumn = info.CreationDateColumn
		log.Printf("[DEBUG] No modification date column found, using creation date column as fallback")
	}

	// Set default for trashed column if none found
	if info.TrashedColumn == "" {
		info.TrashedColumn = "COALESCE(ZTRASHEDSTATE, 0)" // Fallback with default value
		log.Printf("[DEBUG] No trashed column found, using fallback with default value")
	}

	// Determine burst identifier column
	for _, col := range columns {
		switch col {
		case "ZBURSTIDENTIFIER":
			info.BurstColumn = "ZBURSTIDENTIFIER"
		case "ZAVALANCHEUUID":
			if info.BurstColumn == "" {
				info.BurstColumn = "ZAVALANCHEUUID" // Alternative name in some iOS versions
			}
		}
	}
	if info.BurstColumn == "" {
		info.BurstColumn = "NULL" // Fallback to NULL if not found
		log.Printf("[DEBUG] No burst identifier column found, using NULL fallback")
	}

	// Determine screenshot column
	for _, col := range columns {
		switch col {
		case "ZISSCREENSHOT":
			info.ScreenshotColumn = "ZISSCREENSHOT"
		case "ZISDETECTEDSCREENSHOT":
			if info.ScreenshotColumn == "" {
				info.ScreenshotColumn = "ZISDETECTEDSCREENSHOT" // Newer iOS versions
			}
		}
	}
	if info.ScreenshotColumn == "" {
		info.ScreenshotColumn = "0" // Fallback to 0 (not a screenshot) if not found
		log.Printf("[DEBUG] No screenshot column found, using 0 fallback")
	}

	// Determine adjustments column
	for _, col := range columns {
		switch col {
		case "ZHASADJUSTMENTS":
			info.AdjustmentsColumn = "ZHASADJUSTMENTS"
		case "ZADJUSTMENTSSTATE":
			if info.AdjustmentsColumn == "" {
				info.AdjustmentsColumn = "CASE WHEN ZADJUSTMENTSSTATE > 0 THEN 1 ELSE 0 END" // Convert state to boolean
			}
		}
	}
	if info.AdjustmentsColumn == "" {
		info.AdjustmentsColumn = "0" // Fallback to 0 (no adjustments) if not found
		log.Printf("[DEBUG] No adjustments column found, using 0 fallback")
	}

	if info.CreationDateColumn == "" {
		return nil, fmt.Errorf("no suitable creation date column found in ZASSET table")
	}

	// Debug log the selected columns
	log.Printf("[DEBUG] Selected creation date column: %s", info.CreationDateColumn)
	log.Printf("[DEBUG] Selected modification date column: %s", info.ModDateColumn)
	log.Printf("[DEBUG] Selected trashed column: %s", info.TrashedColumn)
	log.Printf("[DEBUG] Selected burst column: %s", info.BurstColumn)
	log.Printf("[DEBUG] Selected screenshot column: %s", info.ScreenshotColumn)
	log.Printf("[DEBUG] Selected adjustments column: %s", info.AdjustmentsColumn)

	return info, nil
}

// GetAssets retrieves all assets from the Photos database
func (d *Database) GetAssets(dcimPath string) ([]*types.Asset, error) {
	// Detect the schema to use appropriate column names
	schema, err := d.detectSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to detect schema: %w", err)
	}

	// Build query with detected column names
	query := fmt.Sprintf(`
		SELECT 
			Z_PK,
			ZFILENAME,
			ZDIRECTORY,
			%s,
			%s,
			ZHIDDEN,
			%s,
			ZKINDSUBTYPE,
			%s,
			%s,
			%s
		FROM %s 
		WHERE ZFILENAME IS NOT NULL
		ORDER BY %s ASC
	`, schema.CreationDateColumn, schema.ModDateColumn, schema.TrashedColumn, schema.BurstColumn, schema.ScreenshotColumn, schema.AdjustmentsColumn, schema.TableName, schema.CreationDateColumn)

	// Debug log the generated query
	log.Printf("[DEBUG] Generated Photos.sqlite query: %s", strings.TrimSpace(query))

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
			return nil, fmt.Errorf("failed to scan row (using schema with creation date column %s): %w", schema.CreationDateColumn, err)
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
