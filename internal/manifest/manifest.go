package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/grantbirki/gh-photos/internal/types"
)

// Entry represents a single entry in the manifest
type Entry struct {
	SourcePath   string           `json:"source_path"`
	TargetPath   string           `json:"target_path"`
	Filename     string           `json:"filename"`
	AssetType    types.AssetType  `json:"asset_type"`
	CreationDate time.Time        `json:"creation_date"`
	FileSize     int64            `json:"file_size"`
	Checksum     string           `json:"checksum,omitempty"`
	MimeType     string           `json:"mime_type"`
	Status       OperationStatus  `json:"status"`
	Flags        types.AssetFlags `json:"flags"`
	Error        string           `json:"error,omitempty"`
}

// OperationStatus represents the status of an operation on an asset
type OperationStatus string

const (
	StatusPending  OperationStatus = "pending"
	StatusSkipped  OperationStatus = "skipped"
	StatusUploaded OperationStatus = "uploaded"
	StatusFailed   OperationStatus = "failed"
	StatusMissing  OperationStatus = "missing"
	StatusVerified OperationStatus = "verified"
)

// Manifest represents a collection of operations and their results
type Manifest struct {
	GeneratedAt  time.Time `json:"generated_at"`
	BackupPath   string    `json:"backup_path"`
	RemoteTarget string    `json:"remote_target"`
	Config       Config    `json:"config"`
	Summary      Summary   `json:"summary"`
	Entries      []Entry   `json:"entries"`
}

// Config captures the configuration used to generate the manifest
type Config struct {
	IncludeHidden          bool       `json:"include_hidden"`
	IncludeRecentlyDeleted bool       `json:"include_recently_deleted"`
	DryRun                 bool       `json:"dry_run"`
	SkipExisting           bool       `json:"skip_existing"`
	Verify                 bool       `json:"verify"`
	Parallel               int        `json:"parallel"`
	StartDate              *time.Time `json:"start_date,omitempty"`
	EndDate                *time.Time `json:"end_date,omitempty"`
	AssetTypes             []string   `json:"asset_types,omitempty"`
	PathGranularity        string     `json:"path_granularity,omitempty"`
}

// Summary provides aggregate statistics about the operation
type Summary struct {
	TotalAssets     int   `json:"total_assets"`
	ProcessedAssets int   `json:"processed_assets"`
	SkippedAssets   int   `json:"skipped_assets"`
	UploadedAssets  int   `json:"uploaded_assets"`
	FailedAssets    int   `json:"failed_assets"`
	MissingAssets   int   `json:"missing_assets"`
	VerifiedAssets  int   `json:"verified_assets"`
	TotalSize       int64 `json:"total_size"`
	UploadedSize    int64 `json:"uploaded_size"`
	DurationSeconds int64 `json:"duration_seconds"`
}

// Generator creates and manages manifests
type Generator struct {
	backupPath   string
	remoteTarget string
	config       Config
}

// CreateGenerator creates a new manifest generator
func CreateGenerator(backupPath, remoteTarget string, config Config) *Generator {
	return &Generator{
		backupPath:   backupPath,
		remoteTarget: remoteTarget,
		config:       config,
	}
}

// CreateManifest creates a new manifest from a list of assets
func (g *Generator) CreateManifest(assets []*types.Asset) *Manifest {
	manifest := &Manifest{
		GeneratedAt:  time.Now(),
		BackupPath:   g.backupPath,
		RemoteTarget: g.remoteTarget,
		Config:       g.config,
		Summary:      Summary{TotalAssets: len(assets)},
		Entries:      make([]Entry, 0, len(assets)),
	}

	// Determine granularity
	granularity := types.PathGranularity(g.config.PathGranularity)
	if granularity == "" {
		granularity = types.GranularityDay
	}

	for _, asset := range assets {
		// Generate target path (root prefix removed)
		targetPath := asset.GenerateTargetPath(granularity)

		entry := Entry{
			SourcePath:   asset.SourcePath,
			TargetPath:   targetPath,
			Filename:     asset.Filename,
			AssetType:    asset.Type,
			CreationDate: asset.CreationDate,
			FileSize:     asset.FileSize,
			Checksum:     asset.Checksum,
			MimeType:     asset.MimeType,
			Status:       StatusPending,
			Flags:        asset.Flags,
		}

		manifest.Entries = append(manifest.Entries, entry)
		manifest.Summary.TotalSize += asset.FileSize
	}

	manifest.Summary.ProcessedAssets = len(manifest.Entries)
	return manifest
}

// SaveToFile writes the manifest to a JSON file
func (m *Manifest) SaveToFile(filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create manifest file %s: %w", filePath, err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(m); err != nil {
		return fmt.Errorf("failed to encode manifest: %w", err)
	}

	return nil
}

// LoadFromFile reads a manifest from a JSON file
func LoadFromFile(filePath string) (*Manifest, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open manifest file %s: %w", filePath, err)
	}
	defer file.Close()

	var manifest Manifest
	decoder := json.NewDecoder(file)

	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("failed to decode manifest: %w", err)
	}

	return &manifest, nil
}

// UpdateEntry updates the status and details of a manifest entry
func (m *Manifest) UpdateEntry(index int, status OperationStatus, errorMsg string) {
	if index >= 0 && index < len(m.Entries) {
		m.Entries[index].Status = status
		if errorMsg != "" {
			m.Entries[index].Error = errorMsg
		}
		m.updateSummary()
	}
}

// updateSummary recalculates the summary statistics
func (m *Manifest) updateSummary() {
	summary := Summary{
		TotalAssets: len(m.Entries),
	}

	for _, entry := range m.Entries {
		summary.TotalSize += entry.FileSize

		switch entry.Status {
		case StatusUploaded:
			summary.UploadedAssets++
			summary.UploadedSize += entry.FileSize
		case StatusSkipped:
			summary.SkippedAssets++
		case StatusFailed:
			summary.FailedAssets++
		case StatusMissing:
			summary.MissingAssets++
		case StatusVerified:
			summary.VerifiedAssets++
		}

		if entry.Status != StatusPending {
			summary.ProcessedAssets++
		}
	}

	m.Summary = summary
}

// GetFilteredEntries returns entries matching the specified status
func (m *Manifest) GetFilteredEntries(status OperationStatus) []Entry {
	var filtered []Entry
	for _, entry := range m.Entries {
		if entry.Status == status {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// PrintSummary prints a human-readable summary of the manifest
func (m *Manifest) PrintSummary() {
	fmt.Printf("\nManifest Summary:\n")
	fmt.Printf("=================\n")
	fmt.Printf("Generated: %s\n", m.GeneratedAt.Format(time.RFC3339))
	fmt.Printf("Backup Path: %s\n", m.BackupPath)
	fmt.Printf("Remote Target: %s\n", m.RemoteTarget)
	fmt.Printf("\nAssets:\n")
	fmt.Printf("  Total: %d\n", m.Summary.TotalAssets)
	fmt.Printf("  Processed: %d\n", m.Summary.ProcessedAssets)
	fmt.Printf("  Uploaded: %d\n", m.Summary.UploadedAssets)
	fmt.Printf("  Skipped: %d\n", m.Summary.SkippedAssets)
	fmt.Printf("  Failed: %d\n", m.Summary.FailedAssets)
	fmt.Printf("  Missing: %d\n", m.Summary.MissingAssets)
	fmt.Printf("  Verified: %d\n", m.Summary.VerifiedAssets)
	fmt.Printf("\nSize:\n")
	fmt.Printf("  Total: %s\n", humanizeBytes(m.Summary.TotalSize))
	fmt.Printf("  Uploaded: %s\n", humanizeBytes(m.Summary.UploadedSize))
	if m.Summary.DurationSeconds > 0 {
		fmt.Printf("\nDuration: %s\n", time.Duration(m.Summary.DurationSeconds)*time.Second)
	}
}

// humanizeBytes converts bytes to human readable format
func humanizeBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
