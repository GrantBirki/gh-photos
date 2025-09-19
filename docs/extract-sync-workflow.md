# Extract-Sync Workflow: Metadata Hydration Process

This document explains how the `gh-photos` CLI extracts iPhone backup metadata and makes it available for sync operations without requiring access to the original Photos.sqlite database.

## Overview

The extract-sync workflow is designed to solve a key architectural challenge: how to sync photos from an extracted backup directory while preserving all the filtering capabilities (hidden photos, recently deleted, screenshots, etc.) that depend on metadata from the Photos.sqlite database.

**Solution**: During extraction, we "dehydrate" all Photos.sqlite metadata into a JSON file. During sync, we "rehydrate" this metadata to apply filtering rules without needing the original database.

## Workflow Phases

### Phase 1: Extraction (`gh photos extract`)

#### 1.1 Photos.sqlite Database Query

The extraction process queries the Photos.sqlite database to collect comprehensive metadata:

```sql
SELECT 
    Z_PK,                    -- Asset ID
    ZFILENAME,              -- Filename (IMG_1099.PNG)
    ZDIRECTORY,             -- Directory path (100APPLE)
    ZCREATIONDATE,          -- Creation timestamp
    ZMODIFICATIONDATE,      -- Modification timestamp  
    ZHIDDEN,                -- Hidden flag (0/1)
    ZTRASHEDSTATE,          -- Recently deleted flag (0/1)
    ZKINDSUBTYPE,           -- Asset type (Live Photo = 2)
    ZBURSTIDENTIFIER,       -- Burst photo ID
    ZOVERALLADJUSTMENTTS,   -- Screenshot detection
    ZADJUSTMENTS            -- Has adjustments
FROM ZASSET 
WHERE ZFILENAME IS NOT NULL
ORDER BY ZCREATIONDATE ASC
```

#### 1.2 Metadata Conversion

Each database row is converted to a structured `Asset` object:

```go
type Asset struct {
    ID           string     `json:"id"`
    SourcePath   string     `json:"source_path"`
    Filename     string     `json:"filename"`
    Type         AssetType  `json:"type"`
    CreationDate time.Time  `json:"creation_date"`
    ModifiedDate time.Time  `json:"modified_date"`
    Flags        AssetFlags `json:"flags"`
    FileSize     int64      `json:"file_size"`
    MimeType     string     `json:"mime_type"`
}

type AssetFlags struct {
    Hidden           bool     // From ZHIDDEN column
    RecentlyDeleted  bool     // From ZTRASHEDSTATE column
    Screenshot       bool     // Computed from adjustments data
    Burst            bool     // From ZBURSTIDENTIFIER presence
    LivePhoto        bool     // From ZKINDSUBTYPE == 2
    BurstID          *string  // Burst group identifier
}
```

#### 1.3 Metadata Hydration

All metadata is saved to `extraction-metadata.json`:

```json
{
  "command_metadata": {
    "completed_at": "2025-09-19T00:03:53Z",
    "cli_version": "v0.0.10 (...)",
    "system": { "os": "windows", "arch": "amd64" },
    "ios_backup": {
      "device_name": "Grant's iPhone",
      "device_model": "iPhone14,5",
      "ios_version": "26.0"
    },
    "asset_counts": {
      "photos": 1247,
      "videos": 89,
      "screenshots": 156,
      "total": 1492
    }
  },
  "assets": [
    {
      "id": "12345",
      "filename": "IMG_1099.PNG",
      "source_path": "DCIM/111APPLE/IMG_1099.PNG",
      "type": "screenshots",
      "creation_date": "2023-06-15T14:30:00Z",
      "modified_date": "2023-06-15T14:30:05Z",
      "flags": {
        "Hidden": false,
        "RecentlyDeleted": true,
        "Screenshot": true,
        "Burst": false,
        "LivePhoto": false
      },
      "file_size": 0,
      "mime_type": ""
    }
  ]
}
```

#### 1.4 File Extraction

Simultaneously, the physical files are extracted from the hashed backup structure to readable directories:

```text
Original (hashed):     Extracted (readable):
├── ab/                ├── CameraRollDomain/
│   └── ab1234ef...    │   └── Media/
├── cd/                │       └── DCIM/
│   └── cd5678gh...    │           ├── 100APPLE/
                       │           │   ├── IMG_1001.HEIC
                       │           │   └── IMG_1002.PNG
                       │           └── 111APPLE/  
                       │               ├── IMG_1099.PNG
                       │               └── IMG_1100.PNG
```

### Phase 2: Sync Preparation (`gh photos sync` on extracted directory)

#### 2.1 Extracted Directory Detection

When sync is run on an extracted directory, the system detects this by checking for `extraction-metadata.json`:

```go
// From internal/backup/parser.go
func NewExtractedBackupParser(backupPath string) (*BackupParser, error) {
    metadataPath := filepath.Join(backupPath, "extraction-metadata.json")
    if _, err := os.Stat(metadataPath); err != nil {
        return nil, fmt.Errorf("extraction metadata not found: %w", err)
    }
    // Load and parse metadata...
}
```

#### 2.2 Metadata Rehydration

The JSON metadata is loaded back into memory as `Asset` structures with full `AssetFlags`:

```go
type ExtractionMetadata struct {
    CommandMetadata *CommandMetadata `json:"command_metadata"`
    Assets          []*types.Asset   `json:"assets"`
}
```

#### 2.3 Path Reconstruction  

Asset paths are updated to point to extracted files:

```go
// Original metadata path: "DCIM/111APPLE/IMG_1099.PNG"
// Reconstructed path: "/path/to/extracted/CameraRollDomain/Media/DCIM/111APPLE/IMG_1099.PNG"
for _, asset := range extractionMetadata.Assets {
    cleanPath := filepath.ToSlash(asset.SourcePath)
    if strings.Contains(cleanPath, "DCIM") {
        parts := strings.Split(cleanPath, "/")
        dcimIndex := findDCIMIndex(parts)
        if dcimIndex >= 0 {
            relativePath := filepath.Join(parts[dcimIndex:]...)
            asset.SourcePath = filepath.Join(backupPath, mediaDomain, "Media", relativePath)
        }
    }
}
```

### Phase 3: Sync Execution (`gh photos sync`)

#### 3.1 Asset Filtering

The sync command applies filtering rules using the rehydrated metadata:

```go
// From internal/types/asset.go
func (a *Asset) ShouldExclude(includeHidden, includeRecentlyDeleted bool) bool {
    if a.Flags.Hidden && !includeHidden {
        return true  // Uses saved metadata from JSON
    }
    if a.Flags.RecentlyDeleted && !includeRecentlyDeleted {
        return true  // Uses saved metadata from JSON
    }
    return false
}
```

#### 3.2 Date Range Filtering

Creation and modification dates from the metadata enable date-based filtering:

```go
// Filter by date range using hydrated timestamps
if asset.CreationDate.Before(startDate) || asset.CreationDate.After(endDate) {
    return true // Exclude asset
}
```

#### 3.3 Asset Type Organization

Assets are organized using their pre-classified types:

```go
// Target path generation using saved metadata
func (a *Asset) GenerateTargetPath(rootPrefix string) string {
    year := a.CreationDate.Format("2006")
    month := a.CreationDate.Format("01") 
    day := a.CreationDate.Format("02")
    return filepath.Join(rootPrefix, year, month, day, string(a.Type), a.Filename)
}
```

## Key Benefits

### 🎯 **Complete Metadata Preservation**

- All Photos.sqlite intelligence is captured in JSON
- No data loss between extraction and sync
- Supports all filtering capabilities

### 🚀 **Portability**

- Extracted directories work on any machine
- No need to transfer original Photos.sqlite
- Self-contained metadata approach

### 🔧 **Performance**

- No database queries during sync
- Fast JSON parsing vs SQL queries
- Efficient path reconstruction

### 🛡️ **Reliability**

- Metadata can't become "stale" or inconsistent
- Single source of truth in JSON file
- Extraction and sync are decoupled

## Implementation Details

### Schema Adaptability

The system detects different iOS versions and adapts the Photos.sqlite schema:

```go
// Supports multiple iOS schema versions
type SchemaInfo struct {
    TableName            string  // ZASSET vs Z_28ASSETS
    CreationDateColumn   string  // ZCREATIONDATE vs ZDATECREATED 
    TrashedColumn        string  // ZTRASHEDSTATE vs ZTRASHED
    // ... other version-specific columns
}
```

### Error Handling

- Graceful fallback if metadata is corrupted
- Warning messages for missing files
- Continues processing on individual asset failures

### Cross-Platform Support

- Handles Windows/Unix path separators
- Unicode filename support
- Timezone-aware timestamps

## Usage Examples

### Basic Extract-Sync Workflow

```bash
# Step 1: Extract backup with metadata hydration
gh photos extract /path/to/backup /path/to/extracted

# Step 2: Sync using extracted directory (metadata is embedded)
gh photos sync /path/to/extracted gdrive:Photos
```

### Advanced Filtering

```bash
# Sync with metadata-based filtering
gh photos sync /path/to/extracted s3:bucket/photos \
  --include-hidden \
  --exclude-screenshots \
  --start-date 2023-01-01 \
  --end-date 2023-12-31
```

All filtering operations use the hydrated metadata from `extraction-metadata.json`, not the original Photos.sqlite database.
