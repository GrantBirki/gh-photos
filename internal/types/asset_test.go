package types

import "testing"

func TestAssetShouldExclude(t *testing.T) {
	tests := []struct {
		name                   string
		asset                  Asset
		includeHidden          bool
		includeRecentlyDeleted bool
		expected               bool
	}{
		{
			name:                   "exclude hidden by default",
			asset:                  Asset{Flags: AssetFlags{Hidden: true}},
			includeHidden:          false,
			includeRecentlyDeleted: false,
			expected:               true,
		},
		{
			name:                   "include hidden when flag set",
			asset:                  Asset{Flags: AssetFlags{Hidden: true}},
			includeHidden:          true,
			includeRecentlyDeleted: false,
			expected:               false,
		},
		{
			name:                   "exclude recently deleted by default",
			asset:                  Asset{Flags: AssetFlags{RecentlyDeleted: true}},
			includeHidden:          false,
			includeRecentlyDeleted: false,
			expected:               true,
		},
		{
			name:                   "include recently deleted when flag set",
			asset:                  Asset{Flags: AssetFlags{RecentlyDeleted: true}},
			includeHidden:          false,
			includeRecentlyDeleted: true,
			expected:               false,
		},
		{
			name:                   "include normal asset",
			asset:                  Asset{Flags: AssetFlags{}},
			includeHidden:          false,
			includeRecentlyDeleted: false,
			expected:               false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.asset.ShouldExclude(tt.includeHidden, tt.includeRecentlyDeleted)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestClassifyByExtension(t *testing.T) {
	tests := []struct {
		filename string
		expected AssetType
	}{
		{"IMG_001.HEIC", AssetTypePhoto},
		{"IMG_002.jpg", AssetTypePhoto},
		{"VIDEO_001.MOV", AssetTypeVideo},
		{"VIDEO_002.mp4", AssetTypeVideo},
		{"unknown.xyz", AssetTypePhoto}, // defaults to photo
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := ClassifyByExtension(tt.filename)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
