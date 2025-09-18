package manifest

import (
	"testing"
	"time"

	"github.com/grantbirki/gh-photos/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestGenerator_CreateManifest(t *testing.T) {
	config := Config{
		IncludeHidden:          false,
		IncludeRecentlyDeleted: false,
		DryRun:                 true,
		Parallel:               4,
	}

	generator := NewGenerator("/test/backup", "gdrive:Photos", config)

	// Create test assets
	now := time.Now()
	assets := []*types.Asset{
		{
			ID:           "1",
			SourcePath:   "/test/backup/IMG_001.HEIC",
			Filename:     "IMG_001.HEIC",
			Type:         types.AssetTypePhoto,
			CreationDate: now,
			FileSize:     1024,
			Checksum:     "abc123",
			MimeType:     "image/heif",
		},
		{
			ID:           "2",
			SourcePath:   "/test/backup/VID_001.MOV",
			Filename:     "VID_001.MOV",
			Type:         types.AssetTypeVideo,
			CreationDate: now,
			FileSize:     2048,
			Checksum:     "def456",
			MimeType:     "video/quicktime",
		},
	}

	manifest := generator.CreateManifest(assets, "photos")

	assert.Equal(t, "/test/backup", manifest.BackupPath)
	assert.Equal(t, "gdrive:Photos", manifest.RemoteTarget)
	assert.Equal(t, config, manifest.Config)
	assert.Equal(t, 2, manifest.Summary.TotalAssets)
	assert.Equal(t, int64(1024+2048), manifest.Summary.TotalSize)
	assert.Len(t, manifest.Entries, 2)

	// Check that target paths are generated correctly
	for _, entry := range manifest.Entries {
		assert.Contains(t, entry.TargetPath, "photos/")
		assert.Contains(t, entry.TargetPath, now.Format("2006"))
		assert.Contains(t, entry.TargetPath, now.Format("01"))
		assert.Contains(t, entry.TargetPath, now.Format("02"))
		assert.Equal(t, StatusPending, entry.Status)
	}
}

func TestManifest_UpdateEntry(t *testing.T) {
	manifest := &Manifest{
		Entries: []Entry{
			{SourcePath: "/test/file1.jpg", Status: StatusPending, FileSize: 1000},
			{SourcePath: "/test/file2.jpg", Status: StatusPending, FileSize: 2000},
		},
	}

	// Update first entry to uploaded
	manifest.UpdateEntry(0, StatusUploaded, "")

	assert.Equal(t, StatusUploaded, manifest.Entries[0].Status)
	assert.Equal(t, "", manifest.Entries[0].Error)
	assert.Equal(t, 1, manifest.Summary.UploadedAssets)
	assert.Equal(t, int64(1000), manifest.Summary.UploadedSize)

	// Update second entry to failed
	manifest.UpdateEntry(1, StatusFailed, "network error")

	assert.Equal(t, StatusFailed, manifest.Entries[1].Status)
	assert.Equal(t, "network error", manifest.Entries[1].Error)
	assert.Equal(t, 1, manifest.Summary.FailedAssets)
}

func TestManifest_GetFilteredEntries(t *testing.T) {
	manifest := &Manifest{
		Entries: []Entry{
			{SourcePath: "/test/file1.jpg", Status: StatusUploaded},
			{SourcePath: "/test/file2.jpg", Status: StatusFailed},
			{SourcePath: "/test/file3.jpg", Status: StatusUploaded},
			{SourcePath: "/test/file4.jpg", Status: StatusSkipped},
		},
	}

	uploaded := manifest.GetFilteredEntries(StatusUploaded)
	assert.Len(t, uploaded, 2)

	failed := manifest.GetFilteredEntries(StatusFailed)
	assert.Len(t, failed, 1)
	assert.Equal(t, "/test/file2.jpg", failed[0].SourcePath)

	skipped := manifest.GetFilteredEntries(StatusSkipped)
	assert.Len(t, skipped, 1)
}

func TestHumanizeBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := humanizeBytes(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}
