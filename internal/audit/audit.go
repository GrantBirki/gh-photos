package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/grantbirki/gh-photos/internal/types"
)

// Trail represents the complete audit trail manifest according to the specification
type Trail struct {
	Metadata Metadata     `json:"metadata"`
	Assets   []AssetEntry `json:"assets"`
}

// Metadata contains run-level context and summary information
type Metadata struct {
	RunID      string     `json:"run_id"`
	CLIVersion string     `json:"cli_version"`
	Device     DeviceInfo `json:"device"`
	Invocation Invocation `json:"invocation"`
	Summary    Summary    `json:"summary"`
	System     SystemInfo `json:"system"`
}

// DeviceInfo contains information about the source iOS device
type DeviceInfo struct {
	BackupPath string  `json:"backup_path"`
	DeviceName *string `json:"device_name,omitempty"`
	DeviceUUID *string `json:"device_uuid,omitempty"`
	IOSVersion *string `json:"ios_version,omitempty"`
}

// Invocation captures the exact command and flags used
type Invocation struct {
	Remote string          `json:"remote"`
	Flags  InvocationFlags `json:"flags"`
}

// InvocationFlags represents all the CLI flags used in the invocation
type InvocationFlags struct {
	IncludeHidden          bool       `json:"include_hidden"`
	IncludeRecentlyDeleted bool       `json:"include_recently_deleted"`
	Parallel               int        `json:"parallel"`
	SkipExisting           bool       `json:"skip_existing"`
	DryRun                 bool       `json:"dry_run"`
	LogLevel               string     `json:"log_level"`
	Types                  []string   `json:"types"`
	StartDate              *time.Time `json:"start_date"`
	EndDate                *time.Time `json:"end_date"`
	Root                   string     `json:"root,omitempty"`
	Verify                 bool       `json:"verify,omitempty"`
	Checksum               bool       `json:"checksum,omitempty"`
}

// Summary provides aggregate statistics about the operation
type Summary struct {
	AssetsTotal      int     `json:"assets_total"`
	AssetsUploaded   int     `json:"assets_uploaded"`
	AssetsSkipped    int     `json:"assets_skipped"`
	AssetsFailed     int     `json:"assets_failed"`
	BytesTransferred int64   `json:"bytes_transferred"`
	DurationSeconds  float64 `json:"duration_seconds"`
}

// SystemInfo captures system context information
type SystemInfo struct {
	OS       string `json:"os"`
	Hostname string `json:"hostname"`
	Arch     string `json:"arch"`
}

// AssetEntry represents a single asset record in the audit trail
type AssetEntry struct {
	UUID       string    `json:"uuid"`
	LocalPath  string    `json:"local_path"`
	RemotePath string    `json:"remote_path"`
	SizeBytes  int64     `json:"size_bytes"`
	SHA256     string    `json:"sha256,omitempty"`
	Type       string    `json:"type"`
	Hidden     bool      `json:"hidden"`
	Deleted    bool      `json:"deleted"`
	CreatedAt  time.Time `json:"created_at"`
	Status     string    `json:"status"` // uploaded, skipped, failed
}

// TrailManager manages audit trail creation and persistence
type TrailManager struct {
	trailDir   string
	cliVersion string
	startTime  time.Time
	trail      *Trail
}

// NewTrailManager creates a new audit trail manager
func NewTrailManager(cliVersion string) (*TrailManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	trailDir := filepath.Join(homeDir, "gh-photos")
	if err := os.MkdirAll(trailDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create audit trail directory: %w", err)
	}

	hostname, _ := os.Hostname()

	return &TrailManager{
		trailDir:   trailDir,
		cliVersion: cliVersion,
		startTime:  time.Now(),
		trail: &Trail{
			Metadata: Metadata{
				RunID:      time.Now().UTC().Format(time.RFC3339),
				CLIVersion: cliVersion,
				System: SystemInfo{
					OS:       runtime.GOOS,
					Hostname: hostname,
					Arch:     runtime.GOARCH,
				},
			},
			Assets: make([]AssetEntry, 0),
		},
	}, nil
}

// SetDeviceInfo sets the device information in the audit trail
func (tm *TrailManager) SetDeviceInfo(backupPath string, deviceName, deviceUUID, iosVersion *string) {
	tm.trail.Metadata.Device = DeviceInfo{
		BackupPath: backupPath,
		DeviceName: deviceName,
		DeviceUUID: deviceUUID,
		IOSVersion: iosVersion,
	}
}

// SetInvocation sets the command invocation details
func (tm *TrailManager) SetInvocation(remote string, flags InvocationFlags) {
	tm.trail.Metadata.Invocation = Invocation{
		Remote: remote,
		Flags:  flags,
	}
}

// AddAsset adds an asset entry to the audit trail
func (tm *TrailManager) AddAsset(asset *types.Asset, remotePath, status string) {
	entry := AssetEntry{
		UUID:       asset.ID,
		LocalPath:  asset.SourcePath,
		RemotePath: remotePath,
		SizeBytes:  asset.FileSize,
		SHA256:     asset.Checksum,
		Type:       tm.convertAssetTypeToAuditFormat(asset.Type),
		Hidden:     asset.Flags.Hidden,
		Deleted:    asset.Flags.RecentlyDeleted,
		CreatedAt:  asset.CreationDate,
		Status:     status,
	}
	tm.trail.Assets = append(tm.trail.Assets, entry)
}

// convertAssetTypeToAuditFormat converts AssetType to audit trail format (singular)
func (tm *TrailManager) convertAssetTypeToAuditFormat(assetType types.AssetType) string {
	switch assetType {
	case types.AssetTypePhoto:
		return "photo"
	case types.AssetTypeVideo:
		return "video"
	case types.AssetTypeScreenshot:
		return "screenshot"
	case types.AssetTypeBurst:
		return "burst"
	case types.AssetTypeLivePhoto:
		return "live_photo"
	default:
		return strings.ToLower(string(assetType))
	}
}

// Finalize completes the audit trail and writes it to disk
func (tm *TrailManager) Finalize() error {
	// Calculate summary statistics
	tm.calculateSummary()

	// Generate timestamped filename
	timestamp := tm.startTime.UTC().Format("2006-01-02T15-04-05Z")
	timestampedPath := filepath.Join(tm.trailDir, fmt.Sprintf("manifest_%s.json", timestamp))
	latestPath := filepath.Join(tm.trailDir, "manifest.json")

	// Write timestamped manifest
	if err := tm.writeManifest(timestampedPath); err != nil {
		return fmt.Errorf("failed to write timestamped manifest: %w", err)
	}

	// Write/overwrite latest manifest
	if err := tm.writeManifest(latestPath); err != nil {
		return fmt.Errorf("failed to write latest manifest: %w", err)
	}

	return nil
}

// SaveAdditionalCopy saves an additional copy of the manifest to the specified path
func (tm *TrailManager) SaveAdditionalCopy(path string) error {
	return tm.writeManifest(path)
}

// calculateSummary computes the summary statistics from the asset entries
func (tm *TrailManager) calculateSummary() {
	summary := Summary{
		DurationSeconds: time.Since(tm.startTime).Seconds(),
	}

	for _, asset := range tm.trail.Assets {
		summary.AssetsTotal++

		switch asset.Status {
		case "uploaded":
			summary.AssetsUploaded++
			summary.BytesTransferred += asset.SizeBytes
		case "skipped":
			summary.AssetsSkipped++
		case "failed":
			summary.AssetsFailed++
		}
	}

	tm.trail.Metadata.Summary = summary
}

// writeManifest writes the audit trail to the specified file path
func (tm *TrailManager) writeManifest(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create manifest file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(tm.trail); err != nil {
		return fmt.Errorf("failed to encode manifest: %w", err)
	}

	return nil
}

// LoadLatestManifest loads the latest manifest from ~/gh-photos/manifest.json
func LoadLatestManifest() (*Trail, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	manifestPath := filepath.Join(homeDir, "gh-photos", "manifest.json")
	return LoadManifest(manifestPath)
}

// LoadManifest loads a manifest from the specified file path
func LoadManifest(path string) (*Trail, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open manifest file: %w", err)
	}
	defer file.Close()

	var trail Trail
	decoder := json.NewDecoder(file)

	if err := decoder.Decode(&trail); err != nil {
		return nil, fmt.Errorf("failed to decode manifest: %w", err)
	}

	return &trail, nil
}
