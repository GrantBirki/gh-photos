package uploader

import (
	"testing"
	"time"

	"github.com/grantbirki/gh-photos/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestFilterAssetsWithIgnorePatterns(t *testing.T) {
	// Create test uploader with ignore patterns
	config := Config{
		IgnorePatterns: []string{"PhotoData", "*.tmp", "Thumbnails/*"},
	}
	uploader := &Uploader{config: config}

	// Create test assets with various paths
	assets := []*types.Asset{
		{
			ID:           "1",
			SourcePath:   "/extracted/CameraRollDomain/Media/DCIM/100APPLE/IMG_0001.HEIC",
			Filename:     "IMG_0001.HEIC",
			Type:         types.AssetTypePhoto,
			CreationDate: time.Now(),
			Flags:        types.AssetFlags{},
		},
		{
			ID:           "2",
			SourcePath:   "/extracted/CameraRollDomain/PhotoData/UBF/test.dat",
			Filename:     "test.dat",
			Type:         types.AssetTypePhoto,
			CreationDate: time.Now(),
			Flags:        types.AssetFlags{},
		},
		{
			ID:           "3",
			SourcePath:   "/extracted/CameraRollDomain/temp.tmp",
			Filename:     "temp.tmp",
			Type:         types.AssetTypePhoto,
			CreationDate: time.Now(),
			Flags:        types.AssetFlags{},
		},
		{
			ID:           "4",
			SourcePath:   "/extracted/CameraRollDomain/Thumbnails/thumb.jpg",
			Filename:     "thumb.jpg",
			Type:         types.AssetTypePhoto,
			CreationDate: time.Now(),
			Flags:        types.AssetFlags{},
		},
		{
			ID:           "5",
			SourcePath:   "/extracted/CameraRollDomain/Media/DCIM/100APPLE/IMG_0002.JPG",
			Filename:     "IMG_0002.JPG",
			Type:         types.AssetTypePhoto,
			CreationDate: time.Now(),
			Flags:        types.AssetFlags{},
		},
	}

	// Filter assets
	filtered := uploader.filterAssets(assets)

	// Should only have assets 1 and 5 (the regular photos in DCIM)
	assert.Len(t, filtered, 2)
	assert.Equal(t, "1", filtered[0].ID)
	assert.Equal(t, "5", filtered[1].ID)
}

func TestFilterAssetsWithoutIgnorePatterns(t *testing.T) {
	// Create test uploader without ignore patterns
	config := Config{
		IgnorePatterns: []string{},
	}
	uploader := &Uploader{config: config}

	// Create test assets
	assets := []*types.Asset{
		{
			ID:           "1",
			SourcePath:   "/extracted/CameraRollDomain/Media/DCIM/100APPLE/IMG_0001.HEIC",
			Filename:     "IMG_0001.HEIC",
			Type:         types.AssetTypePhoto,
			CreationDate: time.Now(),
			Flags:        types.AssetFlags{},
		},
		{
			ID:           "2",
			SourcePath:   "/extracted/CameraRollDomain/PhotoData/UBF/test.dat",
			Filename:     "test.dat",
			Type:         types.AssetTypePhoto,
			CreationDate: time.Now(),
			Flags:        types.AssetFlags{},
		},
	}

	// Filter assets
	filtered := uploader.filterAssets(assets)

	// Should have all assets since no ignore patterns
	assert.Len(t, filtered, 2)
}

func TestFilterAssetsIgnorePatternsWithWildcards(t *testing.T) {
	// Create test uploader with wildcard patterns
	config := Config{
		IgnorePatterns: []string{"*.metadata", "backup_*"},
	}
	uploader := &Uploader{config: config}

	// Create test assets
	assets := []*types.Asset{
		{
			ID:           "1",
			SourcePath:   "/extracted/CameraRollDomain/IMG_0001.HEIC",
			Filename:     "IMG_0001.HEIC",
			Type:         types.AssetTypePhoto,
			CreationDate: time.Now(),
			Flags:        types.AssetFlags{},
		},
		{
			ID:           "2",
			SourcePath:   "/extracted/CameraRollDomain/photo.metadata",
			Filename:     "photo.metadata",
			Type:         types.AssetTypePhoto,
			CreationDate: time.Now(),
			Flags:        types.AssetFlags{},
		},
		{
			ID:           "3",
			SourcePath:   "/extracted/CameraRollDomain/backup_file.txt",
			Filename:     "backup_file.txt",
			Type:         types.AssetTypePhoto,
			CreationDate: time.Now(),
			Flags:        types.AssetFlags{},
		},
	}

	// Filter assets
	filtered := uploader.filterAssets(assets)

	// Should only have asset 1
	assert.Len(t, filtered, 1)
	assert.Equal(t, "1", filtered[0].ID)
}
