package audit

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/grantbirki/gh-photos/internal/types"
)

func TestNewTrailManager(t *testing.T) {
	tm, err := NewTrailManager("test-version")
	if err != nil {
		t.Fatalf("Failed to create trail manager: %v", err)
	}

	if tm.cliVersion != "test-version" {
		t.Errorf("Expected CLI version 'test-version', got '%s'", tm.cliVersion)
	}

	if tm.trail.Metadata.CLIVersion != "test-version" {
		t.Errorf("Expected trail CLI version 'test-version', got '%s'", tm.trail.Metadata.CLIVersion)
	}

	// Verify trail directory was created
	if _, err := os.Stat(tm.trailDir); os.IsNotExist(err) {
		t.Errorf("Trail directory was not created: %s", tm.trailDir)
	}
}

func TestSetDeviceInfo(t *testing.T) {
	tm, err := NewTrailManager("test-version")
	if err != nil {
		t.Fatalf("Failed to create trail manager: %v", err)
	}

	backupPath := "/test/backup/path"
	deviceName := "Test iPhone"
	deviceUUID := "test-uuid-1234"
	iosVersion := "17.6"

	tm.SetDeviceInfo(backupPath, &deviceName, &deviceUUID, &iosVersion)

	device := tm.trail.Metadata.Device
	if device.BackupPath != backupPath {
		t.Errorf("Expected backup path '%s', got '%s'", backupPath, device.BackupPath)
	}

	if device.DeviceName == nil || *device.DeviceName != deviceName {
		t.Errorf("Expected device name '%s', got %v", deviceName, device.DeviceName)
	}

	if device.DeviceUUID == nil || *device.DeviceUUID != deviceUUID {
		t.Errorf("Expected device UUID '%s', got %v", deviceUUID, device.DeviceUUID)
	}

	if device.IOSVersion == nil || *device.IOSVersion != iosVersion {
		t.Errorf("Expected iOS version '%s', got %v", iosVersion, device.IOSVersion)
	}
}

func TestSetInvocation(t *testing.T) {
	tm, err := NewTrailManager("test-version")
	if err != nil {
		t.Fatalf("Failed to create trail manager: %v", err)
	}

	remote := "gdrive:photos"
	flags := InvocationFlags{
		IncludeHidden:          false,
		IncludeRecentlyDeleted: false,
		Parallel:               4,
		SkipExisting:           true,
		DryRun:                 false,
		LogLevel:               "info",
		Types:                  []string{"photos", "videos"},
	}

	tm.SetInvocation(remote, flags)

	invocation := tm.trail.Metadata.Invocation
	if invocation.Remote != remote {
		t.Errorf("Expected remote '%s', got '%s'", remote, invocation.Remote)
	}

	if invocation.Flags.Parallel != 4 {
		t.Errorf("Expected parallel 4, got %d", invocation.Flags.Parallel)
	}

	if invocation.Flags.IncludeHidden != false {
		t.Errorf("Expected include hidden false, got %v", invocation.Flags.IncludeHidden)
	}
}

func TestAddAsset(t *testing.T) {
	tm, err := NewTrailManager("test-version")
	if err != nil {
		t.Fatalf("Failed to create trail manager: %v", err)
	}

	asset := &types.Asset{
		ID:           "test-asset-123",
		SourcePath:   "/test/source/IMG_001.HEIC",
		Filename:     "IMG_001.HEIC",
		Type:         types.AssetTypePhoto,
		CreationDate: time.Now(),
		FileSize:     1024000,
		Checksum:     "abc123hash",
		Flags: types.AssetFlags{
			Hidden:          false,
			RecentlyDeleted: false,
		},
	}

	remotePath := "photos/2025/09/18/photos/IMG_001.HEIC"
	status := "uploaded"

	tm.AddAsset(asset, remotePath, status)

	if len(tm.trail.Assets) != 1 {
		t.Fatalf("Expected 1 asset, got %d", len(tm.trail.Assets))
	}

	entry := tm.trail.Assets[0]
	if entry.UUID != asset.ID {
		t.Errorf("Expected UUID '%s', got '%s'", asset.ID, entry.UUID)
	}

	if entry.LocalPath != asset.SourcePath {
		t.Errorf("Expected local path '%s', got '%s'", asset.SourcePath, entry.LocalPath)
	}

	if entry.RemotePath != remotePath {
		t.Errorf("Expected remote path '%s', got '%s'", remotePath, entry.RemotePath)
	}

	if entry.Status != status {
		t.Errorf("Expected status '%s', got '%s'", status, entry.Status)
	}

	if entry.Type != "photo" {
		t.Errorf("Expected type 'photo', got '%s'", entry.Type)
	}
}

func TestFinalize(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "audit-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Override the trail directory for testing
	tm, err := NewTrailManager("test-version")
	if err != nil {
		t.Fatalf("Failed to create trail manager: %v", err)
	}
	tm.trailDir = tempDir

	// Add some test assets
	asset1 := &types.Asset{
		ID:           "asset-1",
		SourcePath:   "/test/IMG_001.HEIC",
		FileSize:     1024000,
		Type:         types.AssetTypePhoto,
		CreationDate: time.Now(),
	}

	asset2 := &types.Asset{
		ID:           "asset-2",
		SourcePath:   "/test/IMG_002.MOV",
		FileSize:     5120000,
		Type:         types.AssetTypeVideo,
		CreationDate: time.Now(),
	}

	tm.AddAsset(asset1, "photos/2025/09/18/photos/IMG_001.HEIC", "uploaded")
	tm.AddAsset(asset2, "photos/2025/09/18/videos/IMG_002.MOV", "skipped")

	// Finalize the trail
	if err := tm.Finalize(); err != nil {
		t.Fatalf("Failed to finalize trail: %v", err)
	}

	// Check that files were created
	manifestPath := filepath.Join(tempDir, "manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Errorf("Latest manifest file was not created: %s", manifestPath)
	}

	// Check that timestamped file exists (we can't predict exact filename)
	files, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read temp dir: %v", err)
	}

	timestampedFound := false
	for _, file := range files {
		if file.Name() != "manifest.json" && filepath.Ext(file.Name()) == ".json" {
			timestampedFound = true
			break
		}
	}

	if !timestampedFound {
		t.Error("Timestamped manifest file was not created")
	}

	// Verify summary statistics
	summary := tm.trail.Metadata.Summary
	if summary.AssetsTotal != 2 {
		t.Errorf("Expected 2 total assets, got %d", summary.AssetsTotal)
	}

	if summary.AssetsUploaded != 1 {
		t.Errorf("Expected 1 uploaded asset, got %d", summary.AssetsUploaded)
	}

	if summary.AssetsSkipped != 1 {
		t.Errorf("Expected 1 skipped asset, got %d", summary.AssetsSkipped)
	}

	if summary.BytesTransferred != 1024000 {
		t.Errorf("Expected 1024000 bytes transferred, got %d", summary.BytesTransferred)
	}
}

func TestLoadManifest(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "audit-load-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create and finalize a trail
	tm, err := NewTrailManager("test-version")
	if err != nil {
		t.Fatalf("Failed to create trail manager: %v", err)
	}
	tm.trailDir = tempDir

	tm.SetDeviceInfo("/test/backup", stringPtr("Test Device"), stringPtr("test-uuid"), stringPtr("17.6"))

	asset := &types.Asset{
		ID:           "test-asset",
		SourcePath:   "/test/IMG_001.HEIC",
		FileSize:     1024000,
		Type:         types.AssetTypePhoto,
		CreationDate: time.Now(),
	}

	tm.AddAsset(asset, "photos/2025/09/18/photos/IMG_001.HEIC", "uploaded")

	if err := tm.Finalize(); err != nil {
		t.Fatalf("Failed to finalize trail: %v", err)
	}

	// Load the manifest back
	manifestPath := filepath.Join(tempDir, "manifest.json")
	loadedTrail, err := LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("Failed to load manifest: %v", err)
	}

	// Verify loaded data
	if loadedTrail.Metadata.CLIVersion != "test-version" {
		t.Errorf("Expected CLI version 'test-version', got '%s'", loadedTrail.Metadata.CLIVersion)
	}

	if len(loadedTrail.Assets) != 1 {
		t.Fatalf("Expected 1 asset, got %d", len(loadedTrail.Assets))
	}

	loadedAsset := loadedTrail.Assets[0]
	if loadedAsset.UUID != "test-asset" {
		t.Errorf("Expected UUID 'test-asset', got '%s'", loadedAsset.UUID)
	}

	if loadedAsset.Status != "uploaded" {
		t.Errorf("Expected status 'uploaded', got '%s'", loadedAsset.Status)
	}
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
