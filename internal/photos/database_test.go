package photos

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/grantbirki/gh-photos/internal/types"
	"github.com/stretchr/testify/assert"
	_ "modernc.org/sqlite"
)

func TestCoreDataTimeToGoTime(t *testing.T) {
	tests := []struct {
		name     string
		seconds  float64
		expected time.Time
	}{
		{
			name:     "epoch time",
			seconds:  0,
			expected: time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "one day later",
			seconds:  86400, // 24 * 60 * 60
			expected: time.Date(2001, 1, 2, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "one hour later",
			seconds:  3600, // 60 * 60
			expected: time.Date(2001, 1, 1, 1, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := coreDataTimeToGoTime(tt.seconds)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestClassifyAsset(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		flags    types.AssetFlags
		expected types.AssetType
	}{
		{
			name:     "screenshot takes precedence",
			filename: "IMG_001.HEIC",
			flags:    types.AssetFlags{Screenshot: true},
			expected: types.AssetTypeScreenshot,
		},
		{
			name:     "live photo takes precedence over extension",
			filename: "IMG_002.HEIC",
			flags:    types.AssetFlags{LivePhoto: true},
			expected: types.AssetTypeLivePhoto,
		},
		{
			name:     "burst photo",
			filename: "IMG_003.HEIC",
			flags:    types.AssetFlags{Burst: true},
			expected: types.AssetTypeBurst,
		},
		{
			name:     "regular photo",
			filename: "IMG_004.HEIC",
			flags:    types.AssetFlags{},
			expected: types.AssetTypePhoto,
		},
		{
			name:     "regular video",
			filename: "VID_001.MOV",
			flags:    types.AssetFlags{},
			expected: types.AssetTypeVideo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyAsset(tt.filename, tt.flags)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetectSchema(t *testing.T) {
	tests := []struct {
		name                string
		setupTable          func(*sql.DB) error
		expectedCreation    string
		expectedMod         string
		expectedTrashed     string
		expectedBurst       string
		expectedScreenshot  string
		expectedAdjustments string
		expectedError       bool
	}{
		{
			name: "iOS 14+ schema with ZCREATIONDATE",
			setupTable: func(db *sql.DB) error {
				_, err := db.Exec(`CREATE TABLE ZASSET (
					Z_PK INTEGER PRIMARY KEY,
					ZFILENAME TEXT,
					ZDIRECTORY TEXT,
					ZCREATIONDATE REAL,
					ZMODIFICATIONDATE REAL,
					ZHIDDEN INTEGER,
					ZTRASHED INTEGER,
					ZKINDSUBTYPE INTEGER,
					ZBURSTIDENTIFIER TEXT,
					ZISSCREENSHOT INTEGER,
					ZHASADJUSTMENTS INTEGER
				)`)
				return err
			},
			expectedCreation:    "ZCREATIONDATE",
			expectedMod:         "ZMODIFICATIONDATE",
			expectedTrashed:     "ZTRASHED",
			expectedBurst:       "ZBURSTIDENTIFIER",
			expectedScreenshot:  "ZISSCREENSHOT",
			expectedAdjustments: "ZHASADJUSTMENTS",
			expectedError:       false,
		},
		{
			name: "iOS 12-13 schema with ZDATECREATED",
			setupTable: func(db *sql.DB) error {
				_, err := db.Exec(`CREATE TABLE ZASSET (
					Z_PK INTEGER PRIMARY KEY,
					ZFILENAME TEXT,
					ZDIRECTORY TEXT,
					ZDATECREATED REAL,
					ZDATEMODIFIED REAL,
					ZHIDDEN INTEGER,
					ZTRASHED INTEGER,
					ZKINDSUBTYPE INTEGER,
					ZBURSTIDENTIFIER TEXT,
					ZISSCREENSHOT INTEGER,
					ZHASADJUSTMENTS INTEGER
				)`)
				return err
			},
			expectedCreation: "ZDATECREATED",
			expectedMod:      "ZDATEMODIFIED",
			expectedTrashed:  "ZTRASHED",
			expectedError:    false,
		},
		{
			name: "fallback schema with ZADDEDDATE",
			setupTable: func(db *sql.DB) error {
				_, err := db.Exec(`CREATE TABLE ZASSET (
					Z_PK INTEGER PRIMARY KEY,
					ZFILENAME TEXT,
					ZDIRECTORY TEXT,
					ZADDEDDATE REAL,
					ZMODIFIEDDATE REAL,
					ZHIDDEN INTEGER,
					ZTRASHED INTEGER,
					ZKINDSUBTYPE INTEGER,
					ZBURSTIDENTIFIER TEXT,
					ZISSCREENSHOT INTEGER,
					ZHASADJUSTMENTS INTEGER
				)`)
				return err
			},
			expectedCreation: "ZADDEDDATE",
			expectedMod:      "ZMODIFIEDDATE",
			expectedTrashed:  "ZTRASHED",
			expectedError:    false,
		},
		{
			name: "mixed columns prefer ZCREATIONDATE",
			setupTable: func(db *sql.DB) error {
				_, err := db.Exec(`CREATE TABLE ZASSET (
					Z_PK INTEGER PRIMARY KEY,
					ZFILENAME TEXT,
					ZDIRECTORY TEXT,
					ZCREATIONDATE REAL,
					ZDATECREATED REAL,
					ZADDEDDATE REAL,
					ZMODIFICATIONDATE REAL,
					ZHIDDEN INTEGER,
					ZTRASHED INTEGER,
					ZKINDSUBTYPE INTEGER,
					ZBURSTIDENTIFIER TEXT,
					ZISSCREENSHOT INTEGER,
					ZHASADJUSTMENTS INTEGER
				)`)
				return err
			},
			expectedCreation: "ZCREATIONDATE",
			expectedMod:      "ZMODIFICATIONDATE",
			expectedTrashed:  "ZTRASHED",
			expectedError:    false,
		},
		{
			name: "newer iOS schema with ZTRASHEDSTATE",
			setupTable: func(db *sql.DB) error {
				_, err := db.Exec(`CREATE TABLE ZASSET (
					Z_PK INTEGER PRIMARY KEY,
					ZFILENAME TEXT,
					ZDIRECTORY TEXT,
					ZADDEDDATE REAL,
					ZMODIFICATIONDATE REAL,
					ZHIDDEN INTEGER,
					ZTRASHEDSTATE INTEGER,
					ZKINDSUBTYPE INTEGER,
					ZAVALANCHEUUID TEXT,
					ZISDETECTEDSCREENSHOT INTEGER,
					ZADJUSTMENTSSTATE INTEGER
				)`)
				return err
			},
			expectedCreation:    "ZADDEDDATE",
			expectedMod:         "ZMODIFICATIONDATE",
			expectedTrashed:     "ZTRASHEDSTATE",
			expectedBurst:       "ZAVALANCHEUUID",
			expectedScreenshot:  "ZISDETECTEDSCREENSHOT",
			expectedAdjustments: "CASE WHEN ZADJUSTMENTSSTATE > 0 THEN 1 ELSE 0 END",
			expectedError:       false,
		},
		{
			name: "no trashed column uses fallback",
			setupTable: func(db *sql.DB) error {
				_, err := db.Exec(`CREATE TABLE ZASSET (
					Z_PK INTEGER PRIMARY KEY,
					ZFILENAME TEXT,
					ZDIRECTORY TEXT,
					ZADDEDDATE REAL,
					ZMODIFICATIONDATE REAL,
					ZHIDDEN INTEGER,
					ZKINDSUBTYPE INTEGER,
					ZBURSTIDENTIFIER TEXT,
					ZISSCREENSHOT INTEGER,
					ZHASADJUSTMENTS INTEGER
				)`)
				return err
			},
			expectedCreation: "ZADDEDDATE",
			expectedMod:      "ZMODIFICATIONDATE",
			expectedTrashed:  "COALESCE(ZTRASHEDSTATE, 0)",
			expectedError:    false,
		},
		{
			name: "no date columns should error",
			setupTable: func(db *sql.DB) error {
				_, err := db.Exec(`CREATE TABLE ZASSET (
					Z_PK INTEGER PRIMARY KEY,
					ZFILENAME TEXT,
					ZDIRECTORY TEXT,
					ZHIDDEN INTEGER,
					ZTRASHED INTEGER
				)`)
				return err
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary database
			tmpDir := t.TempDir()
			dbPath := filepath.Join(tmpDir, "test.sqlite")

			db, err := sql.Open("sqlite", dbPath)
			if !assert.NoError(t, err) {
				return
			}
			defer db.Close()

			// Setup test table
			err = tt.setupTable(db)
			if !assert.NoError(t, err) {
				return
			}

			// Create Database instance
			photosDB := &Database{db: db}

			// Test schema detection
			schema, err := photosDB.detectSchema()

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, tt.expectedCreation, schema.CreationDateColumn)
			assert.Equal(t, tt.expectedMod, schema.ModDateColumn)
			assert.Equal(t, tt.expectedTrashed, schema.TrashedColumn)
			if tt.expectedBurst != "" {
				assert.Equal(t, tt.expectedBurst, schema.BurstColumn)
			}
			if tt.expectedScreenshot != "" {
				assert.Equal(t, tt.expectedScreenshot, schema.ScreenshotColumn)
			}
			if tt.expectedAdjustments != "" {
				assert.Equal(t, tt.expectedAdjustments, schema.AdjustmentsColumn)
			}
			assert.Equal(t, "ZASSET", schema.TableName)
		})
	}
}

func TestGetAssets_SchemaAdaptive(t *testing.T) {
	// This is an integration test that verifies GetAssets works with different schemas
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "Photos.sqlite")

	db, err := sql.Open("sqlite", dbPath)
	if !assert.NoError(t, err) {
		return
	}
	defer db.Close()

	// Create table with iOS 12-13 schema (ZDATECREATED instead of ZCREATIONDATE)
	_, err = db.Exec(`CREATE TABLE ZASSET (
		Z_PK INTEGER PRIMARY KEY,
		ZFILENAME TEXT,
		ZDIRECTORY TEXT,
		ZDATECREATED REAL,
		ZDATEMODIFIED REAL,
		ZHIDDEN INTEGER,
		ZTRASHED INTEGER,
		ZKINDSUBTYPE INTEGER,
		ZBURSTIDENTIFIER TEXT,
		ZISSCREENSHOT INTEGER,
		ZHASADJUSTMENTS INTEGER
	)`)
	if !assert.NoError(t, err) {
		return
	}

	// Insert test data using a known Core Data timestamp
	// Let's use a simple timestamp: 86400 seconds = 1 day after 2001-01-01 = 2001-01-02 00:00:00 UTC
	coreDataTimestamp := 86400.0
	_, err = db.Exec(`INSERT INTO ZASSET (
		Z_PK, ZFILENAME, ZDIRECTORY, ZDATECREATED, ZDATEMODIFIED,
		ZHIDDEN, ZTRASHED, ZKINDSUBTYPE, ZBURSTIDENTIFIER, ZISSCREENSHOT, ZHASADJUSTMENTS
	) VALUES (1, 'IMG_001.HEIC', '100APPLE', ?, ?, 0, 0, 0, NULL, 0, 0)`, coreDataTimestamp, coreDataTimestamp)
	if !assert.NoError(t, err) {
		return
	}

	// Create Database instance and test GetAssets
	photosDB := &Database{db: db}

	// This should work without the ZCREATIONDATE error
	assets, err := photosDB.GetAssets("/fake/dcim/path")
	if !assert.NoError(t, err) {
		return
	}
	if !assert.Len(t, assets, 1) {
		return
	}

	assert.Equal(t, "IMG_001.HEIC", assets[0].Filename)
	assert.Equal(t, "1", assets[0].ID)
	assert.False(t, assets[0].CreationDate.IsZero())

	// Verify the date conversion worked correctly
	expectedTime := time.Date(2001, 1, 2, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, expectedTime, assets[0].CreationDate)
}
