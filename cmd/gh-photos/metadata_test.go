package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/grantbirki/gh-photos/internal/types"
)

func TestNewCommandMetadata(t *testing.T) {
	metadata := newCommandMetadata()

	// Check basic fields are set
	if metadata.CLIVersion == "" {
		t.Error("Expected CLI version to be set")
	}

	if metadata.System.OS != runtime.GOOS {
		t.Errorf("Expected OS to be %s, got %s", runtime.GOOS, metadata.System.OS)
	}

	if metadata.System.Arch != runtime.GOARCH {
		t.Errorf("Expected Arch to be %s, got %s", runtime.GOARCH, metadata.System.Arch)
	}

	// Check timestamp is recent (within last minute)
	if time.Since(metadata.CompletedAt) > time.Minute {
		t.Error("Expected CompletedAt to be recent")
	}
}

func TestSetAssetCounts(t *testing.T) {
	metadata := newCommandMetadata()

	// Create test assets
	assets := []*types.Asset{
		{Type: types.AssetTypePhoto},
		{Type: types.AssetTypePhoto},
		{Type: types.AssetTypeVideo},
		{Type: types.AssetTypeScreenshot},
		{Type: types.AssetTypeLivePhoto},
		{Type: types.AssetTypeBurst},
	}

	metadata.setAssetCounts(assets)

	// Check counts
	if metadata.AssetCounts.Total != 6 {
		t.Errorf("Expected total count 6, got %d", metadata.AssetCounts.Total)
	}
	if metadata.AssetCounts.Photos != 2 {
		t.Errorf("Expected photos count 2, got %d", metadata.AssetCounts.Photos)
	}
	if metadata.AssetCounts.Videos != 1 {
		t.Errorf("Expected videos count 1, got %d", metadata.AssetCounts.Videos)
	}
	if metadata.AssetCounts.Screenshots != 1 {
		t.Errorf("Expected screenshots count 1, got %d", metadata.AssetCounts.Screenshots)
	}
	if metadata.AssetCounts.LivePhotos != 1 {
		t.Errorf("Expected live photos count 1, got %d", metadata.AssetCounts.LivePhotos)
	}
	if metadata.AssetCounts.Burst != 1 {
		t.Errorf("Expected burst count 1, got %d", metadata.AssetCounts.Burst)
	}
}

func TestSetIOSBackupInfo(t *testing.T) {
	// Create temporary directory with test backup files
	tempDir, err := os.MkdirTemp("", "backup-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a simple Info.plist file
	infoPlistContent := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Device Name</key>
	<string>Test iPhone</string>
	<key>Product Type</key>
	<string>iPhone14,2</string>
	<key>Product Version</key>
	<string>17.6</string>
	<key>Date</key>
	<string>2024-01-15T10:30:00Z</string>
	<key>Unique Identifier</key>
	<string>test-uuid-12345</string>
</dict>
</plist>`

	infoPlistPath := filepath.Join(tempDir, "Info.plist")
	err = os.WriteFile(infoPlistPath, []byte(infoPlistContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write Info.plist: %v", err)
	}

	// Create Manifest.db to indicate hashed backup
	manifestDBPath := filepath.Join(tempDir, "Manifest.db")
	err = os.WriteFile(manifestDBPath, []byte("dummy"), 0644)
	if err != nil {
		t.Fatalf("Failed to write Manifest.db: %v", err)
	}

	metadata := newCommandMetadata()
	err = metadata.setIOSBackupInfo(tempDir)
	if err != nil {
		t.Fatalf("Failed to set iOS backup info: %v", err)
	}

	// Check backup path
	if metadata.IOSBackup.BackupPath != tempDir {
		t.Errorf("Expected backup path %s, got %s", tempDir, metadata.IOSBackup.BackupPath)
	}

	// Check backup type
	if metadata.IOSBackup.BackupType != "hashed" {
		t.Errorf("Expected backup type 'hashed', got %s", metadata.IOSBackup.BackupType)
	}

	// Check device info extracted from Info.plist
	if metadata.IOSBackup.DeviceName == nil || *metadata.IOSBackup.DeviceName != "Test iPhone" {
		t.Errorf("Expected device name 'Test iPhone', got %v", metadata.IOSBackup.DeviceName)
	}

	if metadata.IOSBackup.DeviceModel == nil || *metadata.IOSBackup.DeviceModel != "iPhone14,2" {
		t.Errorf("Expected device model 'iPhone14,2', got %v", metadata.IOSBackup.DeviceModel)
	}

	if metadata.IOSBackup.IOSVersion == nil || *metadata.IOSBackup.IOSVersion != "17.6" {
		t.Errorf("Expected iOS version '17.6', got %v", metadata.IOSBackup.IOSVersion)
	}
}

func TestSaveToManifest(t *testing.T) {
	// Create temporary file
	tempFile, err := os.CreateTemp("", "manifest-test-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	metadata := newCommandMetadata()
	metadata.AssetCounts.Total = 10
	metadata.AssetCounts.Photos = 7
	metadata.AssetCounts.Videos = 3

	err = metadata.saveToManifest(tempFile.Name())
	if err != nil {
		t.Fatalf("Failed to save manifest: %v", err)
	}

	// Read and verify the saved file
	data, err := os.ReadFile(tempFile.Name())
	if err != nil {
		t.Fatalf("Failed to read manifest file: %v", err)
	}

	var manifestData map[string]interface{}
	err = json.Unmarshal(data, &manifestData)
	if err != nil {
		t.Fatalf("Failed to parse manifest JSON: %v", err)
	}

	// Check that command_metadata is present
	cmdMetadata, exists := manifestData["command_metadata"]
	if !exists {
		t.Error("Expected command_metadata to be present in manifest")
	}

	// Verify the structure contains expected data
	metadataStr, _ := json.Marshal(cmdMetadata)
	if !strings.Contains(string(metadataStr), "asset_counts") {
		t.Error("Expected asset_counts to be present in metadata")
	}
}

func TestExtractPlistValue(t *testing.T) {
	content := `<?xml version="1.0" encoding="UTF-8"?>
<dict>
	<key>Device Name</key>
	<string>Test iPhone</string>
	<key>Product Version</key>
	<string>17.6</string>
</dict>`

	deviceName := extractPlistValue(content, "Device Name")
	if deviceName != "Test iPhone" {
		t.Errorf("Expected 'Test iPhone', got '%s'", deviceName)
	}

	version := extractPlistValue(content, "Product Version")
	if version != "17.6" {
		t.Errorf("Expected '17.6', got '%s'", version)
	}

	// Test non-existent key
	missing := extractPlistValue(content, "Missing Key")
	if missing != "" {
		t.Errorf("Expected empty string for missing key, got '%s'", missing)
	}
}

func TestGetOSVersion(t *testing.T) {
	version := getOSVersion()

	// Should return non-empty string
	if version == "" {
		t.Error("Expected non-empty OS version")
	}

	// Should contain expected OS name
	switch runtime.GOOS {
	case "darwin":
		if !strings.Contains(version, "macOS") {
			t.Errorf("Expected macOS version, got %s", version)
		}
	case "windows":
		if !strings.Contains(version, "Windows") {
			t.Errorf("Expected Windows version, got %s", version)
		}
	case "linux":
		// Linux version varies, just check it's not empty
		if version == "Linux" {
			t.Log("Got generic Linux version (expected on some systems)")
		}
	}
}

func TestPrintSummaryForExtract(t *testing.T) {
	metadata := newCommandMetadata()
	deviceName := "Test iPhone"
	metadata.IOSBackup.DeviceName = &deviceName
	metadata.IOSBackup.BackupType = "hashed"

	// Asset counts should not be displayed even if set
	metadata.AssetCounts = AssetCounts{
		Photos: 100,
		Videos: 50,
		Total:  150,
	}

	// This should not panic (we can't easily test the output without capturing stdout)
	// but we can ensure it doesn't crash and that the method exists
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printSummaryForExtract() panicked: %v", r)
		}
	}()

	metadata.printSummaryForExtract()
}
